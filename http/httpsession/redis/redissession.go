package redis

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/kklab-com/gone/http/httpsession"
	"github.com/kklab-com/goth-base62"
	"github.com/kklab-com/goth-kkutil"
)

type Session struct {
	entity SessionEntity
}

type SessionEntity struct {
	ID       string
	LifeTime int64
	Expired  int64
	Data     map[string]string
}

func _NewRedisSession(expire *time.Time) *Session {
	if expire.After(time.Now()) {
		session := Session{
			entity: SessionEntity{
				ID:       base62.ShiftEncoding.EncodeToString(kkutil.BytesFromUUID(uuid.New())),
				LifeTime: expire.Unix() - time.Now().Unix(),
				Expired:  expire.Unix(),
				Data:     map[string]string{},
			},
		}

		return &session
	}

	return nil
}

func (s *Session) ID() string {
	return s.entity.ID
}

func (s *Session) GetStruct(key string, obj interface{}) {
	data, right := s.entity.Data[key]
	if !right {
		return
	}

	_ = json.Unmarshal(bytes.NewBufferString(data).Bytes(), obj)
}

func (s *Session) GetString(key string) string {
	data, right := s.entity.Data[key]
	if !right {
		return ""
	}

	return data
}

func (s *Session) PutString(key string, value string) httpsession.Session {
	s.entity.Data[key] = value
	return s
}

func (s *Session) GetInt64(key string) int64 {
	data, right := s.entity.Data[key]
	if !right {
		return 0
	}

	if val, e := strconv.ParseInt(data, 10, 64); e == nil {
		return val
	}

	return 0
}

func (s *Session) PutInt64(key string, value int64) httpsession.Session {
	s.entity.Data[key] = strconv.FormatInt(value, 10)
	return s
}

func (s *Session) PutStruct(key string, value interface{}) httpsession.Session {
	if marshal, e := json.Marshal(value); e == nil {
		s.entity.Data[key] = string(marshal)
	}

	return s
}

func (s *Session) Clear() httpsession.Session {
	s.entity.Data = map[string]string{}
	return s
}

func (s *Session) Delete(key string) {
	delete(s.entity.Data, key)
}

func (s *Session) Remove() {
	_DeleteRedisSession(s)
}

func (s *Session) Expire() *time.Time {
	unix := time.Unix(s.entity.Expired, 0)
	return &unix
}

func (s *Session) SetExpire(expire *time.Time) httpsession.Session {
	if expire.After(time.Now()) {
		s.entity.LifeTime = expire.Unix() - time.Now().Unix()
		s.entity.Expired = expire.Unix()
	}

	return s
}

func (s *Session) IsExpire() bool {
	return time.Unix(s.entity.Expired, 0).Before(time.Now())
}

func (s *Session) Save() httpsession.Session {
	if _StoreRedisSession(s) == nil {
		return s
	} else {
		return nil
	}
}

func (s *Session) Reload() httpsession.Session {
	if entityBytes := _LoadRedisSessionEntity(s.ID()); entityBytes != nil {
		entity := SessionEntity{}
		if json.Unmarshal(entityBytes, &entity) == nil {
			s.entity.Data = entity.Data
			s.entity.Expired = entity.Expired
			s.entity.LifeTime = entity.LifeTime
		}
	}

	return s
}

func (s *Session) Data() map[string]string {
	return s.entity.Data
}
