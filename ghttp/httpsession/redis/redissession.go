package redis

import (
	"encoding/json"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yetiz-org/goth-base62"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-kkutil"
)

type RedisSession struct {
	entity SessionEntity
}

type SessionEntity struct {
	ID       string
	LifeTime int64
	Expired  int64
	Data     map[string]string
}

func _NewRedisSession(expire *time.Time) *RedisSession {
	if expire.After(time.Now()) {
		session := RedisSession{
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

func (s *RedisSession) ID() string {
	return s.entity.ID
}

func (s *RedisSession) GetStruct(key string, obj any) {
	data, right := s.entity.Data[key]
	if !right {
		return
	}

	_ = json.Unmarshal(buf.NewByteBufString(data).Bytes(), obj)
}

func (s *RedisSession) GetString(key string) string {
	data, right := s.entity.Data[key]
	if !right {
		return ""
	}

	return data
}

func (s *RedisSession) PutString(key string, value string) httpsession.Session {
	s.entity.Data[key] = value
	return s
}

func (s *RedisSession) GetInt64(key string) int64 {
	data, right := s.entity.Data[key]
	if !right {
		return 0
	}

	if val, e := strconv.ParseInt(data, 10, 64); e == nil {
		return val
	}

	return 0
}

func (s *RedisSession) PutInt64(key string, value int64) httpsession.Session {
	s.entity.Data[key] = strconv.FormatInt(value, 10)
	return s
}

func (s *RedisSession) PutStruct(key string, value any) httpsession.Session {
	if marshal, e := json.Marshal(value); e == nil {
		s.entity.Data[key] = string(marshal)
	}

	return s
}

func (s *RedisSession) Clear() httpsession.Session {
	s.entity.Data = map[string]string{}
	return s
}

func (s *RedisSession) Delete(key string) {
	delete(s.entity.Data, key)
}

func (s *RedisSession) Remove() {
	_DeleteRedisSession(s)
}

func (s *RedisSession) Expire() *time.Time {
	unix := time.Unix(s.entity.Expired, 0)
	return &unix
}

func (s *RedisSession) SetExpire(expire *time.Time) httpsession.Session {
	if expire.After(time.Now()) {
		s.entity.LifeTime = expire.Unix() - time.Now().Unix()
		s.entity.Expired = expire.Unix()
	}

	return s
}

func (s *RedisSession) IsExpire() bool {
	return time.Unix(s.entity.Expired, 0).Before(time.Now())
}

func (s *RedisSession) Save() httpsession.Session {
	if _StoreRedisSession(s) == nil {
		return s
	} else {
		return nil
	}
}

func (s *RedisSession) Reload() httpsession.Session {
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

func (s *RedisSession) Data() map[string]string {
	return s.entity.Data
}
