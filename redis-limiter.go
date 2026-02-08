package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

var rateLimiter = redis.NewScript(`
local bucket_key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

-- Step 1: Leak (Remove timestamps outside the time window)
local clean_before = now - window
redis.call('ZREMRANGEBYSCORE', bucket_key, 0, clean_before)

-- Step 2: Check Capacity
local current_count = redis.call('ZCARD', bucket_key)
local status = 0

if current_count >= limit then
	-- Step 3: THE MODIFICATION (Evict the oldest timestamp)
	-- We remove the element with the lowest score (oldest)
	redis.call('ZPOPMIN', bucket_key)
	status = 1
end

-- Step 4: Add the new request
redis.call('ZADD', bucket_key, now, now)

-- Step 5: Refresh TTL (Cleanup)
redis.call('EXPIRE', bucket_key, math.ceil(window / 1000))

local remaining = limit - current_count

return {status, remaining}
`)

func isAllowed(rdb *redis.Client, userID string, limit int, windowsMs int) (int, int, error) {
	now := time.Now().UnixMilli()

	res, err := rateLimiter.Run(ctx, rdb, []string{"limit:" + userID}, now, windowsMs, limit).Result()

	if err != nil {
		return 0, 0, err
	}

	vals := res.([]interface{})

	status := int(vals[0].(int64))
	remaining := int(vals[1].(int64))
	return status, remaining, nil
}

func RateLimiterMiddleware(rdb *redis.Client, limit int, windowsMs int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Header.Get("X-User-ID")
			if userID == "" {
				userID = "anonymous"
			}

			status, rem, err := isAllowed(rdb, userID, limit, windowsMs)

			if err != nil {
				fmt.Println("Redis Error: ", err)
				next.ServeHTTP(w, r)
				return
			}

			visualBucket := ""
			for i := 0; i < limit; i++ {
				if i < (limit - rem) {
					visualBucket += "🔴 " // Filled slot
				} else {
					visualBucket += "⚪ " // Empty slot
				}
			}

			statusMsg := "OK"
			if status == 1 {
				statusMsg = "EVICTED ♻️"
			}

			fmt.Printf("[%s] User: %-8s | %s | Status: %s\n",
				time.Now().Format("15:04:05"), userID, visualBucket, statusMsg)

			w.Header().Set("X-RateLimiter-Limit", fmt.Sprint(limit))
			w.Header().Set("X-RateLimiter-Remaining", fmt.Sprint(rem))

			if status == 1 {
				w.Header().Set("X-RateLimiter-Evicted", "True")
			} else {
				w.Header().Set("X-RateLimiter-Evicted", "False")
			}

			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	fmt.Println("Modified Leaky Bucket Alpha Test (With Capacity Tracking)")

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Successfully reached the API"))
	})

	protectedHandler := RateLimiterMiddleware(rdb, 5, 2000)(finalHandler)

	fmt.Println("Server starting on port 8081")
	http.ListenAndServe(":8081", protectedHandler)
}
