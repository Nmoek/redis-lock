// Package redis_lock
// @Description: 集成测试包
package redis_lock

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type RedisLockE2ESuite struct {
	suite.Suite
	client redis.Cmdable
}

func (r *RedisLockE2ESuite) SetupSuite() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	r.client = client
}

func (r *RedisLockE2ESuite) TearDownTest() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := r.client.FlushAll(ctx).Err()
	assert.NoError(r.T(), err)
}

func (r *RedisLockE2ESuite) TestTryLock() {
	t := r.T()
	testCases := []struct {
		name string

		before func()
		after  func()

		key        string
		expiration time.Duration

		wantLock *Lock
		wantErr  error
	}{
		// 没有错误
		{
			name:       "no error",
			key:        "test1",
			expiration: time.Minute,
			before:     func() {},
			after: func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				// key 必须要在
				res, err := r.client.Exists(ctx, "test1").Result()
				assert.NoError(t, err)
				assert.Equal(t, res, int64(1))
			},
			wantLock: &Lock{
				key: "test1",
			},
		},
		// 未抢占到锁
		{
			name:       "lock not hold",
			key:        "test3",
			expiration: time.Minute,
			before: func() {
				err := r.client.Set(context.Background(), "test3", "123", time.Minute).Err()
				assert.NoError(t, err)
			},
			after: func() {
				val := r.client.Get(context.Background(), "test3").Val()
				assert.Equal(t, "123", val)
			},
			wantErr: ErrFailedToPreemptLock,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.before()
			defer tc.after()

			c := NewClient(r.client)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()

			l, err := c.TryLock(ctx, tc.key, tc.expiration)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assert.NotNil(t, l.client)
			assert.Equal(t, l.key, tc.key)
			assert.NotEmpty(t, l.val)
		})
	}
}

func (r *RedisLockE2ESuite) TestUnLock() {
	t := r.T()
	testCases := []struct {
		name string

		before func()
		after  func()

		key string
		val string

		wantErr error
	}{
		// 解锁成功, 没有错误
		{
			name: "no error",
			key:  "key1",
			val:  "val1",
			before: func() {
				err := r.client.Set(context.Background(), "key1", "val1", time.Minute).Err()
				assert.NoError(t, err)
			},
			after: func() {
				// key 必须没有
				res, err := r.client.Exists(context.Background(), "key1").Result()
				assert.NoError(t, err)
				assert.Equal(t, int64(0), res)
			},
		},
		// 不是自己的锁
		{
			name: "lock belong to others",
			key:  "key2",
			val:  "val2",
			before: func() {
				err := r.client.Set(context.Background(), "key2", "123", time.Minute).Err()
				assert.NoError(t, err)
			},
			after: func() {
				// key必须存在且不能被删除
				val := r.client.Get(context.Background(), "key2").Val()
				assert.Equal(t, "123", val)
			},
			wantErr: ErrLockNotHold,
		},
		// key不存在, 没有锁
		{
			name:   "lock dont exists",
			key:    "key3",
			val:    "val3",
			before: func() {},
			after: func() {
				// key不存在
				res, err := r.client.Exists(context.Background(), "key3").Result()
				assert.NoError(t, err)
				assert.Equal(t, int64(0), res)
			},
			wantErr: ErrLockNotHold,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.before()
			defer tc.after()

			l := newLock(r.client, tc.key, tc.val)
			err := l.Unlock(context.Background())
			assert.Equal(t, tc.wantErr, err)
			assert.NotNil(t, l.client)
			assert.Equal(t, l.key, tc.key)
			assert.Equal(t, l.val, tc.val)
		})
	}
}

func TestRedisLockE2e(t *testing.T) {
	suite.Run(t, &RedisLockE2ESuite{})
}
