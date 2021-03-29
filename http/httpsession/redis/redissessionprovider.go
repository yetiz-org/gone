package redis

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kklab-com/gone/http/httpsession"
	"github.com/kklab-com/goth-kkdatastore"
	"github.com/kklab-com/goth-kklogger"
)

var (
	provider *RedisSessionProvider
	once     sync.Once

	RedisName          = "redis"
	RedisSessionPrefix = "httpsession"
)

type RedisSessionProvider struct {
	Master *datastore.KKRedisOp
	Slave  *datastore.KKRedisOp
}

func _Init() {
	once.Do(func() {
		provider = &RedisSessionProvider{}
		redis := datastore.KKREDIS(RedisName)
		provider.Master = redis.Master()
		provider.Slave = redis.Slave()
	})
}

func _LoadRedisSession(key string) interface{} {
	if entityBytes := _LoadRedisSessionEntity(key); entityBytes == nil {
		return nil
	} else {
		session := Session{entity: SessionEntity{}}
		if json.Unmarshal(entityBytes, &session.entity) == nil {
			return &session
		}
	}

	return nil
}

func _LoadRedisSessionEntity(key string) []byte {
	_Init()
	c := provider.Slave.Conn()
	defer c.Close()

	err := c.Err()
	if err != nil {
		return nil
	}

	redisKey := fmt.Sprintf("%s-%s", RedisSessionPrefix, key)
	if entityBytes, err := c.Do("GET", redisKey); err != nil {
		return nil
	} else if entityBytes == nil {
		return nil
	} else {
		return entityBytes.([]byte)
	}
}

func _StoreRedisSession(session *Session) error {
	_Init()
	c := provider.Master.Conn()
	if c == nil {
		return nil
	}
	defer c.Close()

	key := fmt.Sprintf("%s-%s", RedisSessionPrefix, session.ID())
	entityBytes, err := json.Marshal(&session.entity)
	if err != nil {
		return err
	}

	entityString := string(entityBytes)
	_, err = c.Do("SETEX", key, session.entity.LifeTime, entityString)
	if err != nil {
		return err
	}

	return nil
}

func _DeleteRedisSession(session *Session) {
	_Init()
	c := provider.Master.Conn()
	key := fmt.Sprintf("%s-%s", RedisSessionPrefix, session.ID())
	if _, err := c.Do("DEL", key); err != nil {
		kklogger.WarnJ("RedisDelete", fmt.Sprintf("key %s delete fail", key))
	}
}

func (r *RedisSessionProvider) NewSession(expire *time.Time) httpsession.Session {
	session := _NewRedisSession(expire)
	if err := _StoreRedisSession(session); err != nil {
		kklogger.ErrorJ("StoreSession", err.Error())
		return nil
	}

	return session
}

func (r *RedisSessionProvider) Sessions() map[string]httpsession.Session {
	// TODO
	return make(map[string]httpsession.Session, 0)
}

func (r *RedisSessionProvider) Session(key string) httpsession.Session {
	if session := _LoadRedisSession(key); session != nil {
		return session.(httpsession.Session)
	}

	return nil
}

func SessionProvider() httpsession.SessionProvider {
	_Init()
	return provider
}
