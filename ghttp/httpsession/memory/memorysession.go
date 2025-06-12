package memory

import (
	"encoding/json"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"strconv"
	"time"

	buf "github.com/yetiz-org/goth-bytebuf"
)

type MemorySession struct {
	httpsession.DefaultSession
	updated *time.Time
	expired *time.Time
	timer   *time.Timer
	data    map[string]string
}

func _NewMemorySession(expire *time.Time) *MemorySession {
	now := time.Now()
	session := MemorySession{
		DefaultSession: *httpsession.NewAbstractSession(),
		updated:        &now,
		expired:        expire,
		data:           map[string]string{},
	}

	return &session
}

func (s *MemorySession) GetStruct(key string, obj any) {
	data, right := s.data[key]
	if !right {
		return
	}

	_ = json.Unmarshal(buf.NewByteBufString(data).Bytes(), obj)
}

func (s *MemorySession) GetString(key string) string {
	data, right := s.data[key]
	if !right {
		return ""
	}

	return data
}

func (s *MemorySession) PutString(key string, value string) httpsession.Session {
	s.data[key] = value
	return s
}

func (s *MemorySession) GetInt64(key string) int64 {
	data, right := s.data[key]
	if !right {
		return 0
	}

	if val, e := strconv.ParseInt(data, 10, 64); e == nil {
		return val
	}

	return 0
}

func (s *MemorySession) PutInt64(key string, value int64) httpsession.Session {
	s.data[key] = strconv.FormatInt(value, 10)
	return s
}

func (s *MemorySession) PutStruct(key string, value any) httpsession.Session {
	if marshal, e := json.Marshal(value); e == nil {
		s.data[key] = string(marshal)
	}

	return s
}

func (s *MemorySession) Clear() httpsession.Session {
	s.data = map[string]string{}
	return s
}

func (s *MemorySession) Delete(key string) {
	delete(s.data, key)
}

func (s *MemorySession) Expire() *time.Time {
	return s.expired
}

func (s *MemorySession) SetExpire(expire *time.Time) httpsession.Session {
	s.expired = expire
	return s
}

func (s *MemorySession) IsExpire() bool {
	return s.expired.Before(time.Now())
}

func (s *MemorySession) Save() httpsession.Session {
	return s
}

func (s *MemorySession) Reload() httpsession.Session {
	return s
}

func (s *MemorySession) Data() map[string]string {
	return s.data
}
