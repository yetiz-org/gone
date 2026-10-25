package ghttp

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"mime/multipart"
	"net"
	ghttp "net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/ghttp/httpstatus"

	"github.com/google/uuid"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/goth-base62"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-kklogger"
	"golang.org/x/text/language"
)

type Request struct {
	request           *ghttp.Request
	channel           channel.Channel
	trackID           string
	createdAt         time.Time
	remoteAddrs       []string
	socketAddr        string
	body              buf.ByteBuf
	rawBody           io.ReadCloser // the body as received, read once by bufferBody or by a streaming task
	bodyReadErr       error
	bodyBudget        serverBudgetBody
	bodyOnce          sync.Once
	forwarded         bool // the direct peer passed TrustForwardedFor
	session           httpsession.Session
	op                sync.Mutex
	dispatchOperation DispatchOperation
}

// TrustForwardedFor identifies trusted proxy addresses. The default trusts none. Configure an exact
// proxy IP/CIDR policy at startup; it is read per request unsynchronized. WrapRequest walks the
// X-Forwarded-For chain from the socket peer toward the client and stops at the first untrusted hop.
var TrustForwardedFor = func(peer net.IP) (trusted bool) {
	return false
}

// WrapRequest wraps req without reading its body. Body, BodyReadError and the first read of req.Body buffer
// the body, so every reader sees all of it.
func WrapRequest(ch channel.Channel, req *ghttp.Request) (request *Request) {
	u := uuid.New()
	request = &Request{
		request:    req,
		channel:    ch,
		trackID:    base62.FlipEncoding.EncodeToString(u[:]),
		createdAt:  time.Now(),
		socketAddr: req.RemoteAddr,
		rawBody:    req.Body,
	}

	if req.Body != nil {
		req.Body = lazyBody{request: request}
	}

	request.remoteAddrs = []string{req.RemoteAddr}
	request.forwarded = TrustForwardedFor != nil && TrustForwardedFor(request.RemoteIP())
	xForwardFor := request.Header().Get(httpheadername.XForwardedFor)
	if xForwardFor == "" || !request.forwarded {
		return request
	}

	xffu := strings.Split(xForwardFor, ",")
	for i := range xffu {
		xffu[i] = strings.TrimSpace(xffu[i])
		if net.ParseIP(xffu[i]) == nil {
			return request
		}
	}

	rAddr := req.RemoteAddr
	for i := len(xffu) - 1; i >= 0; i-- {
		ip := net.ParseIP(xffu[i])
		req.RemoteAddr = ip.String()
		if !TrustForwardedFor(ip) {
			break
		}
	}

	request.remoteAddrs = append(xffu, rAddr)
	return request
}

func (r *Request) Request() *ghttp.Request {
	return r.request
}

func (r *Request) Channel() channel.Channel {
	return r.channel
}

func (r *Request) CreatedAt() *time.Time {
	return &r.createdAt
}

func (r *Request) TrackID() string {
	return r.trackID
}

func (r *Request) Host() string {
	return r.request.Host
}

func (r *Request) Method() string {
	return r.request.Method
}

func (r *Request) Url() *url.URL {
	return r.request.URL
}

func (r *Request) ContentLength() int64 {
	return r.request.ContentLength
}

func (r *Request) Proto() string {
	return r.request.Proto
}

func (r *Request) UserAgent() string {
	return r.request.UserAgent()
}

func (r *Request) RequestURI() string {
	return r.request.RequestURI
}

// LogURI returns the request URI gone writes in logs: the result of the channel's ParamLogURI func when it
// sets one, otherwise RequestURI.
func (r *Request) LogURI() (uri string) {
	if r.channel != nil {
		if logURI, ok := r.channel.Param(ParamLogURI).(func(req *Request) string); ok && logURI != nil {
			return logURI(r)
		}
	}

	return r.RequestURI()
}

func (r *Request) RemoteAddr() (ip net.IP, port string) {
	addr := r.request.RemoteAddr
	if strings.Count(addr, ":") > 1 {
		if strings.LastIndex(addr, "]") == -1 {
			return net.ParseIP(addr), ""
		}

		ip := net.ParseIP(addr[1 : strings.LastIndex(addr, ":")-1])
		port := addr[strings.LastIndex(addr, ":")+1:]
		return ip, port
	} else {
		if strings.LastIndex(addr, ":") == -1 {
			return net.ParseIP(addr), ""
		}

		ip := net.ParseIP(addr[:strings.LastIndex(addr, ":")])
		port := addr[strings.LastIndex(addr, ":")+1:]
		return ip, port
	}
}

func (r *Request) RemoteAddrs() []string {
	return r.remoteAddrs
}

func (r *Request) Cookies() []*ghttp.Cookie {
	return r.request.Cookies()
}

func (r *Request) Cookie(name string) (*ghttp.Cookie, error) {
	return r.request.Cookie(name)
}

func (r *Request) AddCookie(c *ghttp.Cookie) {
	r.request.AddCookie(c)
}

func (r *Request) Header() ghttp.Header {
	return r.request.Header
}

// Body returns the request body, reading it into memory on first use. Request().Body then replays
// the same bytes. A failed read leaves an empty body, its error in BodyReadError, and a Request().Body
// that returns that error.
func (r *Request) Body() (body buf.ByteBuf) {
	r.bodyOnce.Do(r.bufferBody)
	return r.body
}

// BodyReadError returns the error encountered while buffering the request body, buffering it first if needed.
// A body over the server limit reports an *http.MaxBytesError.
func (r *Request) BodyReadError() (rtErr error) {
	r.bodyOnce.Do(r.bufferBody)
	return r.bodyReadErr
}

func (r *Request) bufferBody() {
	if r.rawBody == nil {
		r.body = buf.EmptyByteBuf()
		return
	}

	var reader io.Reader = r.rawBody
	if r.bodyBudget.budget != nil {
		r.bodyBudget.Reader = r.rawBody
		reader = &r.bodyBudget
	}

	bs, err := io.ReadAll(reader)
	if err != nil {
		r.bodyBudget.release()

		// Leave the failed body open: net/http closes it after writing the response, while closing it here
		// would first wait for whatever the client is still sending.
		r.bodyReadErr = err
		r.body = buf.EmptyByteBuf()
		r.request.Body = failedBody{ReadCloser: ghttp.NoBody, err: err}
		return
	}

	r.rawBody.Close()
	r.body = buf.NewSharedByteBuf(bs)
	r.request.Body = io.NopCloser(bytes.NewReader(bs))
}

// streamBody hands a streaming task the body as received, unless something already buffered it.
func (r *Request) streamBody() {
	if _, lazy := r.request.Body.(lazyBody); lazy {
		r.request.Body = r.rawBody
	}
}

func (r *Request) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return r.request.FormFile(key)
}

func (r *Request) FormValue(key string) string {
	return r.request.FormValue(key)
}

func (r *Request) RemoteIP() (ip net.IP) {
	rip, _ := r.RemoteAddr()
	return rip
}

func (r *Request) Session() httpsession.Session {
	if r.session == nil {
		r.op.Lock()
		defer r.op.Unlock()
		if r.session == nil {
			r.session = GetSession(r)
		}
	}

	return r.session
}

// RenewSession replaces the request session with a new one and deletes the previous one from the store,
// so the previous session cookie no longer resolves to it.
func (r *Request) RenewSession() {
	previous := r.session
	if previous == nil {
		previous = _StoredSession(r)
	}

	if previous != nil {
		// Mark the held session too: an eviction may already have removed it from the store.
		if session, ok := previous.(interface{ MarkDeleted() }); ok {
			session.MarkDeleted()
		}

		SessionProvider().Delete(previous.Id())
	}

	r.session = _NewSession(r)
}

func (r *Request) AcceptLanguage() []LanguageValue {
	var languageValues []LanguageValue
	if tags, q, err := language.ParseAcceptLanguage(r.Header().Get(httpheadername.AcceptLanguage)); err == nil {
		for i, tag := range tags {
			languageValues = append(languageValues, LanguageValue{
				Value:  tag,
				Factor: q[i],
			})
		}

		return languageValues
	}

	return languageValues
}

func (r *Request) Accept() []QualityValue {
	return _DecodeQualityValueField(r.Header().Get(httpheadername.Accept))
}

func (r *Request) AcceptEncoding() []QualityValue {
	return _DecodeQualityValueField(r.Header().Get(httpheadername.AcceptEncoding))
}

func (r *Request) TE() []QualityValue {
	return _DecodeQualityValueField(r.Header().Get(httpheadername.TE))
}

func (r *Request) AcceptCharset() []QualityValue {
	return _DecodeQualityValueField(r.Header().Get(httpheadername.AcceptCharset))
}

// SocketRemoteAddr returns the original direct peer before trusted proxy processing.
func (r *Request) SocketRemoteAddr() (address string) {
	return r.socketAddr
}

type QualityValue struct {
	Value  string
	Factor float32
}

type LanguageValue struct {
	Value  language.Tag
	Factor float32
}

func _DecodeQualityValueField(text string) []QualityValue {
	var qualities []QualityValue
	if text == "" || text == "*" {
		return qualities
	}

	for entity := range strings.SplitSeq(strings.TrimSpace(text), ",") {
		qv := QualityValue{}
		value, params, hasParams := strings.Cut(entity, ";")
		if hasParams {
			if _, factorVal, found := strings.Cut(params, "="); found {
				if cf, err := strconv.ParseFloat(factorVal, 32); err == nil {
					qv.Factor = float32(cf)
				}
			}
		}

		qv.Value = value
		qualities = append(qualities, qv)
	}

	return qualities
}

func (r *Request) PreferLang() *language.Tag {
	lang := r.AcceptLanguage()
	if len(lang) > 0 {
		return &lang[0].Value
	} else {
		return nil
	}
}

func (r *Request) Referer() string {
	return r.Header().Get(httpheadername.Referer)
}

func (r *Request) Origin() string {
	return r.Header().Get(httpheadername.Origin)
}

// ErrResponseBodyTooLarge means bounded internal dispatch discarded an oversized response.
var ErrResponseBodyTooLarge = errors.New("ghttp: response body exceeds limit")

type Response struct {
	response          *ghttp.Response
	request           *Request
	statusCode        int
	header            ghttp.Header
	cookies           map[string][]ghttp.Cookie
	body              buf.ByteBuf
	done              channel.Future
	headerWritten     bool
	jsonEncoderConfig func(*json.Encoder)
	bodyLimit         int64
	bodyErr           error
}

func WrapResponse(ch channel.NetChannel, response *ghttp.Response) (resp *Response) {
	resp = &Response{
		request: &Request{
			request: response.Request,
			channel: ch,
		},
		statusCode: response.StatusCode,
		header:     response.Header,
		done:       channel.NewFuture(ch),
	}

	// The client request body was already sent, so Body keeps returning nil instead of reading it.
	resp.request.bodyOnce.Do(func() {})
	return resp
}

func (r *Response) Request() *Request {
	return r.request
}

func NewResponse(request *Request) *Response {
	response := &Response{}
	response.request = request
	response.statusCode = 0
	response.header = map[string][]string{}
	response.cookies = map[string][]ghttp.Cookie{}
	response.body = buf.EmptyByteBuf()
	response.done = channel.NewFuture(request.Channel())
	return response
}

func EmptyResponse() *Response {
	response := &Response{}
	response.header = map[string][]string{}
	response.cookies = map[string][]ghttp.Cookie{}
	response.body = buf.EmptyByteBuf()
	return response
}

func (r *Response) DoneFuture() channel.Future {
	return r.done
}

func (r *Response) ResponseError(er ErrorResponse) {
	er = er.Clone()
	er.ErrorData()["cid"] = r.request.Channel().ID()
	er.ErrorData()["tid"] = r.request.TrackID()
	r.SetStatusCode(er.ErrorStatusCode())
	text := er.Error()
	if r.bodyLimit > 0 && int64(len(text)) > r.bodyLimit {
		r.SetHeader(httpheadername.ContentType, "application/json")
		r.bodyErr = ErrResponseBodyTooLarge
		r.body = buf.EmptyByteBuf()
		return
	}

	r.JsonResponse(buf.NewByteBufString(text))
}

func (r *Response) Redirect(redirectUrl string) {
	r.SetStatusCode(httpstatus.Found).
		SetHeader(httpheadername.Location, redirectUrl)
}

func (r *Response) StatusCode() int {
	return r.statusCode
}

func (r *Response) SetStatusCode(statusCode int) *Response {
	r.statusCode = statusCode
	return r
}

func (r *Response) AddHeader(name string, value string) *Response {
	r.header.Add(name, value)
	return r
}

func (r *Response) SetHeader(name string, value string) *Response {
	r.header.Set(name, value)
	return r
}

func (r *Response) DelHeader(name string) *Response {
	r.header.Del(name)
	return r
}

func (r *Response) Header() ghttp.Header {
	return r.header
}

func (r *Response) GetHeader(name string) string {
	return r.header.Get(name)
}

func (r *Response) Cookie(name string) *ghttp.Cookie {
	if cookie, f := r.cookies[name]; f {
		return &cookie[0]
	}

	return nil
}

func (r *Response) SetCookie(cookie *ghttp.Cookie) *Response {
	r.cookies[cookie.Name] = []ghttp.Cookie{*cookie}
	return r
}

func (r *Response) Cookies() map[string][]ghttp.Cookie {
	return r.cookies
}

func (r *Response) Body() buf.ByteBuf {
	return r.body
}

func (r *Response) SetBody(body buf.ByteBuf) {
	if r.bodyLimit > 0 {
		if r.bodyErr != nil {
			return
		}

		if body != nil && int64(body.ReadableBytes()) > r.bodyLimit {
			r.bodyErr = ErrResponseBodyTooLarge
			r.body = buf.EmptyByteBuf()
			return
		}
	}

	r.body = body
}

func (r *Response) SetContentType(ct string) {
	r.SetHeader(httpheadername.ContentType, ct)
}

func (r *Response) TextResponse(buf buf.ByteBuf) {
	r.
		SetHeader(httpheadername.ContentType, "text/plain").
		SetBody(buf)
}

// DefaultJsonEncoderConfig configures the json.Encoder used by Response.JsonResponse.
// It disables HTML escaping by default so &, <, > stay literal (still valid JSON).
// Set it at startup (read per request, unsynchronized); override per response via
// Response.SetJsonEncoderConfig. nil falls back to the encoding/json default.
var DefaultJsonEncoderConfig = func(enc *json.Encoder) {
	enc.SetEscapeHTML(false)
}

var defaultJSONEncoderConfig = DefaultJsonEncoderConfig

// SetJsonEncoderConfig overrides DefaultJsonEncoderConfig for this response; nil clears it.
func (r *Response) SetJsonEncoderConfig(config func(*json.Encoder)) *Response {
	r.jsonEncoderConfig = config
	return r
}

func (r *Response) JsonResponse(obj any) {
	r.SetHeader(httpheadername.ContentType, "application/json")
	if r.bodyLimit > 0 && r.bodyErr != nil {
		return
	}

	switch body := obj.(type) {
	case *bytes.Buffer:
		if r.bodyLimit > 0 {
			r._SetBoundedBytes(body.Bytes())
			return
		}

		r.SetBody(buf.NewByteBuf(body.Bytes()))
		return
	case buf.ByteBuf:
		r.SetBody(body)
		return
	case []byte:
		if r.bodyLimit > 0 {
			r._SetBoundedBytes(body)
			return
		}

		r.SetBody(buf.NewByteBuf(body))
		return
	case string:
		obj = struct{ Data string }{Data: body}
	}

	if r.bodyLimit > 0 {
		if err := r._EncodeBoundedJSON(obj); err != nil {
			r.bodyErr = err
			r.body = buf.EmptyByteBuf()
		}

		return
	}

	config := r.jsonEncoderConfig
	if config == nil {
		config = DefaultJsonEncoderConfig
	}

	// Encoder.Encode appends a trailing newline; trim it to match json.Marshal output.
	jsonBuf := &bytes.Buffer{}
	encoder := json.NewEncoder(jsonBuf)
	if config != nil {
		config(encoder)
	}

	if e := encoder.Encode(obj); e == nil {
		r.SetBody(buf.NewByteBuf(bytes.TrimRight(jsonBuf.Bytes(), "\n")))
	} else {
		kklogger.ErrorJ("ghttp:Response.JsonResponse#json_marshal!marshal_error", e.Error())
	}
}

// BodyError reports a sticky failure from bounded internal response construction.
func (r *Response) BodyError() (rtErr error) {
	return r.bodyErr
}

func (r *Response) _SetBoundedBytes(data []byte) {
	if int64(len(data)) > r.bodyLimit {
		r.bodyErr = ErrResponseBodyTooLarge
		r.body = buf.EmptyByteBuf()
		return
	}

	r.SetBody(buf.NewByteBuf(data))
}

func (r *Response) _EncodeBoundedJSON(obj any) (rtErr error) {
	capture := &_ResponseBodyBuffer{limit: r.bodyLimit}
	config := r.jsonEncoderConfig
	if config == nil {
		config = DefaultJsonEncoderConfig
	}

	// Go funcs are not comparable; only the unchanged default can use the equivalent streaming encoder.
	if r.jsonEncoderConfig == nil && config != nil && reflect.ValueOf(config).Pointer() == reflect.ValueOf(defaultJSONEncoderConfig).Pointer() {
		rtErr = jsonv2.MarshalWrite(capture, obj, json.DefaultOptionsV1(), jsontext.EscapeForHTML(false))
	} else {
		encoder := json.NewEncoder(capture)
		if config != nil {
			config(encoder)
		}

		capture.trimNewline = true
		rtErr = encoder.Encode(obj)
	}

	if rtErr != nil {
		return rtErr
	}

	r.SetBody(buf.NewSharedByteBuf(capture.data))
	return nil
}

type _ResponseBodyBuffer struct {
	data        []byte
	limit       int64
	trimNewline bool
}

func (b *_ResponseBodyBuffer) Write(data []byte) (written int, rtErr error) {
	written = len(data)
	if b.trimNewline {
		data = bytes.TrimRight(data, "\n")
	}

	if int64(len(data)) > b.limit-int64(len(b.data)) {
		b.data = nil
		return 0, ErrResponseBodyTooLarge
	}

	required := len(b.data) + len(data)
	if required > cap(b.data) {
		capacity := b.limit
		if int64(cap(b.data)) <= b.limit/2 {
			capacity = max(int64(required), int64(cap(b.data))*2)
		}

		grown := make([]byte, len(b.data), int(capacity))
		copy(grown, b.data)
		b.data = grown
	}

	b.data = append(b.data, data...)
	return written, nil
}

type ResponseWriter interface {
	ghttp.ResponseWriter
}

type Pack struct {
	Request           *Request       `json:"request"`
	Response          *Response      `json:"response"`
	RouteNode         RouteNode      `json:"route_node"`
	Params            map[string]any `json:"params"`
	Writer            ResponseWriter `json:"writer"`
	writeSeparateMode bool
	rawMode           bool
}

func _UnPack(obj any) *Pack {
	if pkg, ok := obj.(*Pack); ok {
		return pkg
	}

	return nil
}

// failedBody fails every read with err, so a later reader of Request().Body sees why the body is missing.
// lazyBody buffers the request body on its first read and then reads the buffered copy.
type lazyBody struct {
	request *Request
}

func (b lazyBody) Read(p []byte) (n int, rtErr error) {
	b.request.bodyOnce.Do(b.request.bufferBody)
	return b.request.request.Body.Read(p)
}

func (b lazyBody) Close() (rtErr error) {
	return nil
}

type failedBody struct {
	io.ReadCloser
	err error
}

func (b failedBody) Read(p []byte) (n int, rtErr error) {
	return 0, b.err
}
