package gmcp

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// _ResponseWriter sanitizes errors the SDK writes before its middleware runs. Each HTTP request owns one; the SDK
// streaming goroutine and _Finish share its mutex. JSON is written at _Finish, and SSE events as soon as they complete.
type _ResponseWriter struct {
	_Writer    http.ResponseWriter
	_Mutex     sync.Mutex
	_Status    int
	_Committed bool
	_Body      bytes.Buffer
}

// Header returns the underlying response header.
func (w *_ResponseWriter) Header() (header http.Header) {
	return w._Writer.Header()
}

// WriteHeader records the first status until the response is committed.
func (w *_ResponseWriter) WriteHeader(status int) {
	w._Mutex.Lock()
	defer w._Mutex.Unlock()
	if w._Status == 0 {
		w._Status = status
	}
}

// Write buffers JSON and SSE output and drops other media types; complete SSE events are sanitized and written.
func (w *_ResponseWriter) Write(data []byte) (n int, err error) {
	w._Mutex.Lock()
	defer w._Mutex.Unlock()
	if w._Status == 0 {
		w._Status = http.StatusOK
	}

	mediaType := w.Header().Get("Content-Type")
	if !strings.HasPrefix(mediaType, "application/json") && !strings.HasPrefix(mediaType, "text/event-stream") {
		// Non-JSON HTTP rejections never keep the SDK text.
		return len(data), nil
	}

	w._Body.Write(data)
	if strings.HasPrefix(mediaType, "text/event-stream") {
		for {
			event, _, found := bytes.Cut(w._Body.Bytes(), []byte("\n\n"))
			if !found {
				break
			}

			safe := w._SanitizeEvent(event)
			w._Body.Next(len(event) + 2)
			w._Commit()
			if _, err = w._Writer.Write(safe); err != nil {
				return 0, err
			}
		}
	}

	return len(data), nil
}

// FlushError flushes SSE output; an incomplete event stays buffered.
func (w *_ResponseWriter) FlushError() (err error) {
	w._Mutex.Lock()
	defer w._Mutex.Unlock()
	if strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		w._Commit()
		return http.NewResponseController(w._Writer).Flush()
	}

	return nil
}

// _Commit writes the status once; callers hold _Mutex. There is no Unwrap, so ResponseController cannot bypass the
// sanitizer.
func (w *_ResponseWriter) _Commit() {
	if w._Committed {
		return
	}

	if w._Status == 0 {
		w._Status = http.StatusOK
	}

	w.Header().Del("Content-Length")
	w._Writer.WriteHeader(w._Status)
	w._Committed = true
}

// _Finish writes the sanitized JSON body, or a public error for a bodiless error status, and commits the response.
func (w *_ResponseWriter) _Finish() {
	w._Mutex.Lock()
	defer w._Mutex.Unlock()
	defer w._Body.Reset()
	mediaType := w.Header().Get("Content-Type")
	if strings.HasPrefix(mediaType, "text/event-stream") {
		// Write already sent every complete event; an incomplete one is dropped.
		w._Commit()
		return
	}

	var data []byte
	if strings.HasPrefix(mediaType, "application/json") && w._Body.Len() > 0 {
		data = w._SanitizeJSON(w._Body.Bytes())
	} else if w._Status >= http.StatusBadRequest {
		w.Header().Set("Content-Type", "application/json")
		data = _StatusErrorCode(w._Status)._JSON()
	}

	w._Commit()
	if len(data) > 0 {
		_, _ = w._Writer.Write(data)
	}
}

// _SanitizeJSON keeps successful JSON-RPC responses and rewrites error responses, including batches, with public codes.
func (w *_ResponseWriter) _SanitizeJSON(data []byte) (safe []byte) {
	internal := _InternalError._Public()
	fallback := fmt.Appendf(nil, `{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":%q,"data":{"code":%q,"message":%q}}}`, internal.Message, internal.Code, internal.Message)
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '[' {
		var batch []jsontext.Value
		if err := json.Unmarshal(data, &batch); err != nil {
			return fallback
		}

		for i := range batch {
			batch[i] = w._SanitizeJSON(batch[i])
		}

		safe, err := json.Marshal(batch)
		if err != nil {
			return fallback
		}

		return safe
	}

	var envelope map[string]jsontext.Value
	if err := json.Unmarshal(data, &envelope); err != nil || envelope == nil {
		return fallback
	}

	rawError, exists := envelope["error"]
	if !exists || bytes.Equal(rawError, []byte("null")) {
		return data
	}

	type protocolError struct {
		Code    int          `json:"code"`
		Message string       `json:"message"`
		Data    _PublicError `json:"data"`
	}

	var original struct {
		Code int `json:"code"`
	}

	original.Code = -32603
	if err := json.Unmarshal(rawError, &original); err != nil {
		original.Code = -32603
	}

	code := _InternalError
	switch original.Code {
	case -32700, -32600, -32602:
		code = _InvalidArgument
	case -32601:
		code = _NotFound
	}

	public := code._Public()
	id := envelope["id"]
	if len(id) == 0 {
		id = jsontext.Value("null")
	}

	safe, err := json.Marshal(struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      jsontext.Value `json:"id"`
		Error   protocolError  `json:"error"`
	}{JSONRPC: "2.0", ID: id, Error: protocolError{Code: original.Code, Message: public.Message, Data: public}})
	if err != nil {
		return fallback
	}

	return safe
}

// _SanitizeEvent rewrites the data lines of one SSE event with _SanitizeJSON and keeps the other lines.
func (w *_ResponseWriter) _SanitizeEvent(event []byte) (safe []byte) {
	var metadata, data bytes.Buffer
	for line := range bytes.SplitSeq(event, []byte("\n")) {
		if value, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}

			data.Write(bytes.TrimPrefix(value, []byte(" ")))
			continue
		}

		metadata.Write(line)
		metadata.WriteByte('\n')
	}

	if data.Len() > 0 {
		metadata.WriteString("data: ")
		metadata.Write(w._SanitizeJSON(data.Bytes()))
		metadata.WriteByte('\n')
	}

	metadata.WriteByte('\n')
	return metadata.Bytes()
}
