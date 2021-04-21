package http

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kklab-com/gone-httpheadername"
	"github.com/kklab-com/gone-httpstatus"
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http/httpsession"
	"github.com/kklab-com/goth-base62"
	"github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil"
	"github.com/kklab-com/goth-kkutil/validate"
	"golang.org/x/text/language"
)

const MaxMultiPartMemory = 1024 * 1024 * 256

type Request struct {
	http.Request
	trackID     string
	CreatedAt   time.Time
	RemoteAddrs []string
	Body        []byte
	session     httpsession.Session
	lock        sync.Mutex
	channel     channel.NetChannel
}

func NewRequest(ch channel.NetChannel, req http.Request) *Request {
	request := Request{
		Request:   req,
		CreatedAt: time.Now(),
		channel:   ch,
	}

	if bodyBytes, e := ioutil.ReadAll(request.Request.Body); e == nil {
		request.Request.Body.Close()
		request.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		request.Request.ParseMultipartForm(channel.GetParamInt64Default(ch, ParamMaxMultiPartMemory, MaxMultiPartMemory))
		request.Request.Body.Close()
		request.Body = bodyBytes
	}

	if xForwardFor := request.Header.Get(httpheadername.XForwardedFor); xForwardFor != "" {
		xffu := strings.Split(xForwardFor, ", ")
		rAddr := request.Request.RemoteAddr
		for i := len(xffu) - 1; i >= 0; i-- {
			current := xffu[i]
			if validate.IsPublicIP(net.ParseIP(current)) {
				request.Request.RemoteAddr = current
				break
			}
		}

		if xffu == nil {
			xffu = []string{}
		}

		request.RemoteAddrs = append(xffu, rAddr)
	} else {
		request.RemoteAddrs = []string{request.Request.RemoteAddr}
	}

	return &request
}

func (r *Request) Channel() (ch channel.NetChannel) {
	return r.channel
}

func (r *Request) TrackID() string {
	if r.trackID == "" {
		r.lock.Lock()
		defer r.lock.Unlock()
		if r.trackID == "" {
			r.trackID = base62.ShiftEncoding.EncodeToString(kkutil.BytesFromUUID(uuid.New()))
		}
	}

	return r.trackID
}

func (r *Request) RemoteIP() (ip net.IP) {
	rip, _ := r.RemoteAddr()
	return rip
}

func (r *Request) RemoteAddr() (ip net.IP, port string) {
	addr := r.Request.RemoteAddr
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

func (r *Request) Session() httpsession.Session {
	if r.session == nil {
		r.lock.Lock()
		defer r.lock.Unlock()
		if r.session == nil {
			r.session = GetSession(r)
		}
	}

	return r.session
}

func (r *Request) RenewSession() {
	r.session = _NewSession(r)
}

func (r *Request) AcceptLanguage() []LanguageValue {
	var languageValues []LanguageValue
	if tags, q, err := language.ParseAcceptLanguage(r.Header.Get(httpheadername.AcceptLanguage)); err == nil {
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
	return _DecodeQualityValueField(r.Header.Get(httpheadername.Accept))
}

func (r *Request) AcceptEncoding() []QualityValue {
	return _DecodeQualityValueField(r.Header.Get(httpheadername.AcceptEncoding))
}

func (r *Request) TE() []QualityValue {
	return _DecodeQualityValueField(r.Header.Get(httpheadername.TE))
}

func (r *Request) AcceptCharset() []QualityValue {
	return _DecodeQualityValueField(r.Header.Get(httpheadername.AcceptCharset))
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

	for _, entity := range strings.Split(strings.TrimSpace(text), ",") {
		qv := QualityValue{}
		split := strings.Split(entity, ";")
		if len(split) == 2 {
			factor := strings.Split(split[1], "=")
			if len(factor) == 2 {
				if cf, err := strconv.ParseFloat(factor[1], 32); err == nil {
					qv.Factor = float32(cf)
				}
			}
		}

		qv.Value = split[0]
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
	return r.Header.Get(httpheadername.Referer)
}

func (r *Request) Origin() string {
	return r.Header.Get(httpheadername.Origin)
}

type Response struct {
	request    *Request
	statusCode int
	header     http.Header
	cookies    map[string][]http.Cookie
	body       *bytes.Buffer
}

func WrapResponse(ch channel.NetChannel, response *http.Response) *Response {
	resp := &Response{
		request: &Request{
			Request:   *response.Request,
			CreatedAt: time.Now(),
			channel:   ch,
		},
		statusCode: response.StatusCode,
		header:     response.Header,
	}

	return resp
}

func (r *Response) Request() *Request {
	return r.request
}

func NewResponse(request *Request) *Response {
	response := &Response{}
	response.request = request
	response.statusCode = httpstatus.OK
	response.header = map[string][]string{}
	response.cookies = map[string][]http.Cookie{}
	response.body = bytes.NewBuffer([]byte{})
	return response
}

func (r *Response) ResponseError(er ErrorResponse) {
	er = er.Clone()
	er.ErrorData()["cid"] = r.request.Channel().ID()
	er.ErrorData()["tid"] = r.request.TrackID()
	r.SetStatusCode(er.ErrorStatusCode()).
		JsonResponse(bytes.NewBufferString(er.Error()))
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

func (r *Response) Header() http.Header {
	return r.header
}

func (r *Response) GetHeader(name string) string {
	return r.header.Get(name)
}

func (r *Response) Cookie(name string) *http.Cookie {
	if cookie, f := r.cookies[name]; f {
		return &cookie[0]
	}

	return nil
}

func (r *Response) SetCookie(cookie *http.Cookie) *Response {
	r.cookies[cookie.Name] = []http.Cookie{*cookie}
	return r
}

func (r *Response) Cookies() map[string][]http.Cookie {
	return r.cookies
}

func (r *Response) Body() []byte {
	return r.body.Bytes()
}

func (r *Response) SetBody(buffer *bytes.Buffer) {
	r.body = buffer
}

func (r *Response) TextResponse(buffer *bytes.Buffer) {
	r.SetHeader(httpheadername.ContentType, "text/plain")
	r.SetBody(buffer)
}

func (r *Response) JsonResponse(obj interface{}) {
	r.SetHeader(httpheadername.ContentType, "application/json")

	switch body := obj.(type) {
	case *bytes.Buffer:
		r.SetBody(body)
	case []byte:
		r.SetBody(bytes.NewBuffer(body))
	case string:
		obj = struct{ Data string }{Data: body}
	default:
		if body, e := json.Marshal(obj); e == nil {
			r.SetBody(bytes.NewBuffer(body))
			return
		} else {
			kklogger.ErrorJ("gone:Response.JsonResponse#JsonMarshal", e.Error())
		}
	}
}

type ResponseWriter interface {
	http.ResponseWriter
}

type Pack struct {
	Request   *Request               `json:"request"`
	Response  *Response              `json:"response"`
	RouteNode RouteNode              `json:"route_node"`
	Params    map[string]interface{} `json:"params"`
	Writer    ResponseWriter         `json:"writer"`
}

func _UnPack(obj interface{}) *Pack {
	if pkg, true := obj.(*Pack); true {
		return pkg
	}

	return nil
}
