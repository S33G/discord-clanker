-- Rate limit check and increment
-- KEYS[1] = minute key
-- KEYS[2] = hour key
-- ARGV[1] = minute limit (0 = unlimited)
-- ARGV[2] = hour limit (0 = unlimited)
-- ARGV[3] = minute TTL (60)
-- ARGV[4] = hour TTL (3600)
-- Returns: {status, seconds_until_reset}
--   status: 1 = OK, -1 = minute limit exceeded, -2 = hour limit exceeded

local minute = tonumber(redis.call('GET', KEYS[1]) or "0")
local hour = tonumber(redis.call('GET', KEYS[2]) or "0")

local minute_limit = tonumber(ARGV[1])
local hour_limit = tonumber(ARGV[2])

-- Check minute limit
if minute_limit > 0 and minute >= minute_limit then
    local ttl = redis.call('TTL', KEYS[1])
    return {-1, ttl > 0 and ttl or 60}
end

-- Check hour limit  
if hour_limit > 0 and hour >= hour_limit then
    local ttl = redis.call('TTL', KEYS[2])
    return {-2, ttl > 0 and ttl or 3600}
end

-- Increment counters
if minute == 0 then
    redis.call('SET', KEYS[1], 1, 'EX', tonumber(ARGV[3]))
else
    redis.call('INCR', KEYS[1])
end

if hour == 0 then
    redis.call('SET', KEYS[2], 1, 'EX', tonumber(ARGV[4]))
else
    redis.call('INCR', KEYS[2])
end

return {1, 0}
