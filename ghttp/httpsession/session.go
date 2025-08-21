package httpsession

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	buf "github.com/yetiz-org/goth-bytebuf"

	"github.com/google/uuid"
	"github.com/yetiz-org/goth-base62"
	"github.com/yetiz-org/goth-util"
)

type Session interface {
	Id() string
	GetString(key string) string
	PutString(key string, value string) Session
	GetInt64(key string) int64
	PutInt64(key string, value int64) Session
	GetStruct(key string, obj any)
	PutStruct(key string, value any) Session
	Clear() Session
	Delete(key string)
	Created() time.Time
	Updated() time.Time
	Expire() time.Time
	Save() error
	Reload() Session
	Data() map[string]string
	SetExpire(expire time.Time) Session
	IsExpire() bool
}

type DefaultSession struct {
	sessionProvider SessionProvider
	id              string
	created         int64
	updated         int64
	expired         int64
	data            map[string]string
}

func NewDefaultSession(sessionProvider SessionProvider) *DefaultSession {
	return &DefaultSession{
		sessionProvider: sessionProvider,
		id:              base62.ShiftEncoding.EncodeToString(kkutil.BytesFromUUID(uuid.New())),
		created:         time.Now().Unix(),
		updated:         time.Now().Unix(),
		expired:         math.MaxInt64,
		data:            map[string]string{},
	}
}

func (d *DefaultSession) Save() error {
	d.updated = time.Now().Unix()
	return d.sessionProvider.Save(d)
}

func (d *DefaultSession) Id() string {
	return d.id
}

func (d *DefaultSession) GetString(key string) string {
	if val, ok := d.data[key]; ok {
		return val
	}

	return ""
}

func (d *DefaultSession) PutString(key string, value string) Session {
	d.data[key] = value
	return d
}

func (d *DefaultSession) GetInt64(key string) int64 {
	if val, ok := d.data[key]; ok {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		} else {
			return 0
		}
	}

	return 0
}

func (d *DefaultSession) PutInt64(key string, value int64) Session {
	d.data[key] = strconv.FormatInt(value, 10)
	return d
}

func (d *DefaultSession) GetStruct(key string, obj any) {
	if data, f := d.data[key]; f {
		_ = json.Unmarshal(buf.NewByteBufString(data).Bytes(), obj)
	}
}

func (d *DefaultSession) PutStruct(key string, value any) Session {
	if marshal, e := json.Marshal(value); e == nil {
		d.data[key] = string(marshal)
	}

	return d
}

func (d *DefaultSession) Clear() Session {
	d.data = map[string]string{}
	return d
}

func (d *DefaultSession) Delete(key string) {
	delete(d.data, key)
}

func (d *DefaultSession) Created() time.Time {
	return time.Unix(d.created, 0)
}

func (d *DefaultSession) Updated() time.Time {
	return time.Unix(d.updated, 0)
}

func (d *DefaultSession) Expire() time.Time {
	return time.Unix(d.expired, 0)
}

func (d *DefaultSession) Reload() Session {
	session := d.sessionProvider.Session(d.Id())
	if session == nil {
		return d
	}

	d.id = session.Id()
	d.created = session.Created().Unix()
	d.updated = session.Updated().Unix()
	d.expired = session.Expire().Unix()
	d.data = session.Data()
	return d
}

func (d *DefaultSession) Data() map[string]string {
	return d.data
}

func (d *DefaultSession) SetExpire(expire time.Time) Session {
	d.expired = expire.Unix()
	return d
}

func (d *DefaultSession) IsExpire() bool {
	return d.expired < time.Now().Unix()
}

func (d *DefaultSession) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Id      string            `json:"id"`
		Created int64             `json:"created"`
		Updated int64             `json:"updated"`
		Expired int64             `json:"expired"`
		Data    map[string]string `json:"data"`
	}{
		Id:      d.id,
		Created: d.created,
		Updated: d.updated,
		Expired: d.expired,
		Data:    d.data,
	})
}

func (d *DefaultSession) UnmarshalJSON(data []byte) error {
	var v struct {
		Id      string            `json:"id"`
		Created int64             `json:"created"`
		Updated int64             `json:"updated"`
		Expired int64             `json:"expired"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	d.id = v.Id
	d.created = v.Created
	d.updated = v.Updated
	d.expired = v.Expired
	d.data = v.Data
	return nil
}
