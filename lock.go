package redis_lock

import (
	"context"
	_ "embed"
	"errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

var (
	ErrLockNotHold         = errors.New("未持有锁")
	ErrFailedToPreemptLock = errors.New("加锁失败")
)

var (
	//go:embed unlock.lua
	luaUnlock string
)

type Client struct {
	client redis.Cmdable
}

type Lock struct {
	client redis.Cmdable
	key    string
	val    string
}

func newLock(client redis.Cmdable, key string, val string) *Lock {
	return &Lock{
		client: client,
		key:    key,
		val:    val,
	}
}

func (l *Lock) TryLock(ctx context.Context, key string, expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	res, err := l.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !res {
		return nil, ErrFailedToPreemptLock
	}

	return newLock(l.client, key, val), nil
}

func (l *Lock) Unlock(ctx context.Context) error {
	count, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.val).Int64()
	if err == redis.Nil {
		return ErrLockNotHold
	}
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrLockNotHold
	}
	return nil
}