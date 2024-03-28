package redis_lock

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

var (
	ErrLockNotHold         = errors.New("未持有锁")
	ErrFailedToPreemptLock = errors.New("加锁失败")
)

var (
	//go:embed script/lua/unlock.lua
	luaUnlock string
	//go:embed script/lua/refresh.lua
	luaRefresh string
)

type Client struct {
	client redis.Cmdable
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{
		client: client,
	}
}

func (c *Client) TryLock(ctx context.Context, key string, expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	res, err := c.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !res {
		return nil, ErrFailedToPreemptLock
	}

	return newLock(c.client, key, val, expiration), nil
}

type Lock struct {
	client     redis.Cmdable
	key        string
	val        string
	expiration time.Duration
	unlock     chan struct{}
}

func newLock(client redis.Cmdable, key string, val string, expiration time.Duration) *Lock {
	return &Lock{
		client:     client,
		key:        key,
		val:        val,
		expiration: expiration,
		unlock:     make(chan struct{}, 1), // 有缓冲否则会阻塞
	}
}

func (l *Lock) AutoRefresh(interval time.Duration, timeout time.Duration) error {
	ch := make(chan struct{})
	defer close(ch)
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ch:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			if err == context.DeadlineExceeded {
				ch <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			if err == context.DeadlineExceeded {
				ch <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		case <-l.unlock:
			fmt.Printf("已经被解锁")
			return nil

		}
	}
}

func (l *Lock) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaRefresh, []string{l.key}, l.val, l.expiration.Milliseconds()).Result()
	// 锁根本不存在
	if err == redis.Nil {
		return ErrLockNotHold
	}
	// redis内部出错
	if err != nil {
		return err
	}
	// 锁存在但不属于当前实例
	if res == 0 {
		return ErrLockNotHold
	}

	return nil
}

func (l *Lock) Unlock(ctx context.Context) error {
	defer func() {
		l.unlock <- struct{}{}
		close(l.unlock)
	}()
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
