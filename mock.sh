#!/usr/bin/env sh

echo "start mock tool..."

mockgen -package=redismocks -destination=./mocks/redis_cmdable.mock.go github.com/redis/go-redis/v9 Cmdable

go mod tidy


