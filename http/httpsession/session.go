package httpsession

import (
	"time"

	"github.com/google/uuid"
	"github.com/kklab-com/goth-base62"
	"github.com/kklab-com/goth-kkutil"
)

type Session interface {
	ID() string
	GetString(key string) string
	PutString(key string, value string) Session
	GetInt64(key string) int64
	PutInt64(key string, value int64) Session
	GetStruct(key string, obj any)
	PutStruct(key string, value any) Session
	Clear() Session
	Delete(key string)
	Expire() *time.Time
	Save() Session
	Reload() Session
	Data() map[string]string
	SetExpire(expire *time.Time) Session
	IsExpire() bool
}

type DefaultSession struct {
	id string
}

func (d *DefaultSession) Save() Session {
	panic("implement me")
}

func NewAbstractSession() *DefaultSession {
	return &DefaultSession{id: base62.ShiftEncoding.EncodeToString(kkutil.BytesFromUUID(uuid.New()))}
}

func (d *DefaultSession) ID() string {
	return d.id
}

func (d *DefaultSession) GetString(key string) string {
	panic("implement me")
}

func (d *DefaultSession) PutString(key string, value string) Session {
	panic("implement me")
}

func (d *DefaultSession) GetInt64(key string) int64 {
	panic("implement me")
}

func (d *DefaultSession) PutInt64(key string, value int64) Session {
	panic("implement me")
}

func (d *DefaultSession) GetStruct(key string, obj any) {
	panic("implement me")
}

func (d *DefaultSession) PutStruct(key string, value any) Session {
	panic("implement me")
}

func (d *DefaultSession) Clear() Session {
	panic("implement me")
}

func (d *DefaultSession) Delete(key string) {
	panic("implement me")
}

func (d *DefaultSession) Expire() *time.Time {
	panic("implement me")
}

func (d *DefaultSession) Reload() Session {
	panic("implement me")
}

func (d *DefaultSession) Data() map[string]string {
	panic("implement me")
}

func (d *DefaultSession) SetExpire(expire *time.Time) Session {
	panic("implement me")
}

func (d *DefaultSession) IsExpire() bool {
	panic("implement me")
}
