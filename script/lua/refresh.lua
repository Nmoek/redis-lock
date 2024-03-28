local key = KEYS[1];
local val = ARGV[1];
local ms = ARGV[2];


--1. 检查锁的归属权是否正确, 是否是当前实例的锁
if redis.call("get", key) == val  then
    -- 1.1. 如果正确则删除
    return redis.call("pexpire", key, ms)
else
    -- 1.2. 如果不正确则返回错误码
    -- 说明key不存在 或者 key存在但是val不相等
    return 0
end
