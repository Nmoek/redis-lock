// Package redis_lock
// @Description: 单元测试包
package redis_lock

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	redismocks "redis-lock/mocks"
	"testing"
	"time"
)

type RedisLockSuite struct {
	suite.Suite
}

func (r *RedisLockSuite) TestTryLock() {
	t := r.T()
	testCases := []struct {
		name string

		mock func(ctrl *gomock.Controller) redis.Cmdable

		key        string
		expiration time.Duration

		wantLock *Lock
		wantErr  error
	}{
		// 没有错误
		{
			name:       "no error",
			key:        "test1",
			expiration: time.Second,
			mock: func(ctrl *gomock.Controller) redis.Cmdable {

				client := redismocks.NewMockCmdable(ctrl)
				res := redis.NewBoolResult(true, nil)
				client.EXPECT().SetNX(gomock.Any(), "test1", gomock.Any(), time.Second).
					Return(res)

				return client
			},
			wantLock: &Lock{
				key: "test1",
			},
		},
		// redis错误
		{
			name:       "redis error",
			key:        "test2",
			expiration: time.Second * 2,
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				client := redismocks.NewMockCmdable(ctrl)
				res := redis.NewBoolResult(false, errors.New("redis内部错误"))
				client.EXPECT().SetNX(gomock.Any(), "test2", gomock.Any(), time.Second*2).
					Return(res)

				return client
			},
			wantErr: errors.New("redis内部错误"),
		},
		// 未抢占到锁
		{
			name:       "lock not hold",
			key:        "test3",
			expiration: time.Second,
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				client := redismocks.NewMockCmdable(ctrl)
				res := redis.NewBoolResult(false, nil)
				client.EXPECT().SetNX(gomock.Any(), "test3", gomock.Any(), time.Second).
					Return(res)

				return client
			},
			wantErr: ErrFailedToPreemptLock,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := NewClient(tc.mock(ctrl))
			l, err := c.TryLock(context.Background(), tc.key, tc.expiration)
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

func (r *RedisLockSuite) TestUnLock() {
	t := r.T()
	testCases := []struct {
		name string

		mock func(ctrl *gomock.Controller) redis.Cmdable

		key string
		val string

		wantErr error
	}{
		// 没有错误
		{
			name: "no error",
			key:  "test1",
			val:  "test1",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				client := redismocks.NewMockCmdable(ctrl)

				client.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{"test1"}, gomock.Any()).
					Return(redis.NewCmdResult(int64(1), nil))

				return client
			},
		},
		// redis错误
		{
			name: "redis error",
			key:  "test2",
			val:  "test2",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				client := redismocks.NewMockCmdable(ctrl)

				client.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{"test2"}, gomock.Any()).
					Return(redis.NewCmdResult(int64(0), errors.New("redis内部错误")))

				return client
			},
			wantErr: errors.New("redis内部错误"),
		},
		// key不存在
		{
			name: "key dont exists",
			key:  "test3",
			val:  "test3",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				client := redismocks.NewMockCmdable(ctrl)

				client.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{"test3"}, gomock.Any()).
					Return(redis.NewCmdResult(int64(0), redis.Nil))

				return client
			},
			wantErr: ErrLockNotHold,
		},
		// 锁过期
		{
			name: "lock timeout",
			key:  "test4",
			val:  "test4",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				client := redismocks.NewMockCmdable(ctrl)

				client.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{"test4"}, gomock.Any()).
					Return(redis.NewCmdResult(int64(0), nil))

				return client
			},
			wantErr: ErrLockNotHold,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			l := newLock(tc.mock(ctrl), tc.key, tc.val)

			err := l.Unlock(context.Background())
			assert.NotNil(t, l.client)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, l.key, tc.key)
			assert.Equal(t, l.val, tc.val)
		})
	}
}

func TestRedisLock(t *testing.T) {
	suite.Run(t, &RedisLockSuite{})
}
