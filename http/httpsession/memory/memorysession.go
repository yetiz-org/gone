package memory

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"github.com/kklab-com/gone/http/httpsession"
)

type Session struct {
	httpsession.DefaultSession
	updated *time.Time
	expired *time.Time
	timer   *time.Timer
	data    map[string]string
}

func _NewMemorySession(expire *time.Time) *Session {
	now := time.Now()
	session := Session{
		DefaultSession: *httpsession.NewAbstractSession(),
		updated:        &now,
		expired:        expire,
		data:           map[string]string{},
	}

	return &session
}

func (s *Session) GetStruct(key string, obj interface{}) {
	data, right := s.data[key]
	if !right {
		return
	}

	_ = json.Unmarshal(bytes.NewBufferString(data).Bytes(), obj)
}

func (s *Session) GetString(key string) string {
	data, right := s.data[key]
	if !right {
		return ""
	}

	return data
}

func (s *Session) PutString(key string, value string) httpsession.Session {
	s.data[key] = value
	return s
}

func (s *Session) GetInt64(key string) int64 {
	data, right := s.data[key]
	if !right {
		return 0
	}

	if val, e := strconv.ParseInt(data, 10, 64); e == nil {
		return val
	}

	return 0
}

func (s *Session) PutInt64(key string, value int64) httpsession.Session {
	s.data[key] = strconv.FormatInt(value, 10)
	return s
}

func (s *Session) PutStruct(key string, value interface{}) httpsession.Session {
	if marshal, e := json.Marshal(value); e == nil {
		s.data[key] = string(marshal)
	}

	return s
}

func (s *Session) Clear() httpsession.Session {
	s.data = map[string]string{}
	return s
}

func (s *Session) Delete(key string) {
	delete(s.data, key)
}

func (s *Session) Expire() *time.Time {
	return s.expired
}

func (s *Session) SetExpire(expire *time.Time) httpsession.Session {
	s.expired = expire
	return s
}

func (s *Session) IsExpire() bool {
	return s.expired.Before(time.Now())
}

func (s *Session) Save() httpsession.Session {
	return s
}

func (s *Session) Reload() httpsession.Session {
	return s
}

func (s *Session) Data() map[string]string {
	return s.data
}
