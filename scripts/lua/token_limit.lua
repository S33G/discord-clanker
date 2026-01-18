-- Token limit check and increment
-- KEYS[1] = token usage key
-- ARGV[1] = token limit (0 = unlimited/bypass)
-- ARGV[2] = TTL in seconds
-- ARGV[3] = tokens to add
-- Returns: {status, tokens_used, tokens_remaining, seconds_until_reset}
--   status: 1 = OK, -1 = limit exceeded

local limit = tonumber(ARGV[1])

-- Bypass if limit is 0
if limit == 0 then
    return {1, 0, 0, 0}
end

local used = tonumber(redis.call('GET', KEYS[1]) or "0")
local to_add = tonumber(ARGV[3])

-- Check if adding tokens would exceed limit
if used + to_add > limit then
    local ttl = redis.call('TTL', KEYS[1])
    return {-1, used, limit - used, ttl > 0 and ttl or tonumber(ARGV[2])}
end

-- Increment token count
if used == 0 then
    redis.call('SET', KEYS[1], to_add, 'EX', tonumber(ARGV[2]))
else
    redis.call('INCRBY', KEYS[1], to_add)
end

used = used + to_add
local remaining = limit - used

return {1, used, remaining, 0}
