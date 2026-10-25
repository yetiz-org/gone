package httpsession

import (
	"encoding/json"
	"maps"
	"math"
	"strconv"
	"sync"
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

// DefaultSession is safe for concurrent use, since a provider may hand one session to concurrent requests.
type DefaultSession struct {
	sessionProvider SessionProvider
	id              string
	created         int64
	updated         int64
	expired         int64
	data            map[string]string
	deleted         bool
	mu              sync.RWMutex
	saveMu          sync.Mutex // orders Save against MarkDeleted
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

// Save stores the session through its provider; it does nothing once the provider has marked it deleted.
func (d *DefaultSession) Save() (rtErr error) {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	d.mu.Lock()
	deleted := d.deleted
	d.updated = time.Now().Unix()
	d.mu.Unlock()
	if deleted {
		return nil
	}

	return d.sessionProvider.Save(d)
}

// MarkDeleted makes later Save calls do nothing and waits for a Save in progress. A provider that hands
// one session to concurrent requests calls it before deleting the session, so no request still holding
// it can store it again.
func (d *DefaultSession) MarkDeleted() {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deleted = true
}

// StoreIfActive publishes through store while renewal and removal are excluded. The callback must
// only change provider state; it must not call session methods or acquire another session's lock.
func (d *DefaultSession) StoreIfActive(store func()) (stored bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.deleted || d.expired <= time.Now().Unix() {
		return false
	}

	store()
	return true
}

// RemoveIfCurrent revokes this object only when remove deletes its current provider entry. With
// expiredOnly, a renewal that wins the lock survives. The callback has StoreIfActive's restrictions.
func (d *DefaultSession) RemoveIfCurrent(expiredOnly bool, remove func() bool) (removed bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if expiredOnly && !d.deleted && d.expired > time.Now().Unix() {
		return false
	}

	if !remove() {
		return false
	}

	d.deleted = true
	return true
}

func (d *DefaultSession) Id() (id string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.id
}

func (d *DefaultSession) GetString(key string) (value string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.data[key]
}

func (d *DefaultSession) PutString(key string, value string) (session Session) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.data[key] = value
	return d
}

func (d *DefaultSession) GetInt64(key string) (value int64) {
	if i, err := strconv.ParseInt(d.GetString(key), 10, 64); err == nil {
		return i
	}

	return 0
}

func (d *DefaultSession) PutInt64(key string, value int64) (session Session) {
	return d.PutString(key, strconv.FormatInt(value, 10))
}

func (d *DefaultSession) GetStruct(key string, obj any) {
	d.mu.RLock()
	data, f := d.data[key]
	d.mu.RUnlock()
	if f {
		_ = json.Unmarshal(buf.NewByteBufString(data).Bytes(), obj)
	}
}

func (d *DefaultSession) PutStruct(key string, value any) (session Session) {
	if marshal, e := json.Marshal(value); e == nil {
		d.PutString(key, string(marshal))
	}

	return d
}

func (d *DefaultSession) Clear() (session Session) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.data = map[string]string{}
	return d
}

func (d *DefaultSession) Delete(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.data, key)
}

func (d *DefaultSession) Created() (created time.Time) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return time.Unix(d.created, 0)
}

func (d *DefaultSession) Updated() (updated time.Time) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return time.Unix(d.updated, 0)
}

func (d *DefaultSession) Expire() (expire time.Time) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return time.Unix(d.expired, 0)
}

func (d *DefaultSession) Reload() (session Session) {
	stored := d.sessionProvider.Session(d.Id())
	if stored == nil || stored == Session(d) {
		// A provider that hands out the stored object itself has nothing newer to copy.
		return d
	}

	var data map[string]string
	if snapshot, ok := stored.(interface{ Snapshot() map[string]string }); ok {
		data = snapshot.Snapshot()
	} else {
		data = maps.Clone(stored.Data())
	}

	if data == nil {
		data = map[string]string{}
	}

	id, created, updated, expired := stored.Id(), stored.Created().Unix(), stored.Updated().Unix(), stored.Expire().Unix()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.id, d.created, d.updated, d.expired, d.data = id, created, updated, expired, data
	return d
}

// Data returns the live map of session values. It is not synchronized: read or change it only while no
// other request uses the session, or read a Snapshot instead.
func (d *DefaultSession) Data() (data map[string]string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.data
}

// Snapshot returns a copy of the session values taken under the session lock, for readers such as a
// template render while other requests may write the same session.
func (d *DefaultSession) Snapshot() (data map[string]string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return maps.Clone(d.data)
}

func (d *DefaultSession) SetExpire(expire time.Time) (session Session) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.expired = expire.Unix()
	return d
}

func (d *DefaultSession) IsExpire() (expired bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.deleted || d.expired <= time.Now().Unix()
}

func (d *DefaultSession) MarshalJSON() (data []byte, rtErr error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
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

func (d *DefaultSession) UnmarshalJSON(data []byte) (rtErr error) {
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

	if v.Data == nil {
		v.Data = map[string]string{}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.id = v.Id
	d.created = v.Created
	d.updated = v.Updated
	d.expired = v.Expired
	d.data = v.Data
	return nil
}
