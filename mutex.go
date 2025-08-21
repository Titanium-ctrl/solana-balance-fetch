package main

import (
	"time"

	"github.com/go-redsync/redsync/v4"
	redigo "github.com/go-redsync/redsync/v4/redis/goredis/v9"
)

var pool = redigo.NewPool(redisClient)
var rs = redsync.New(pool)

func NewMutex(address string) *redsync.Mutex {
	return rs.NewMutex("lock:"+address,
		redsync.WithExpiry(3*time.Second),            // Set the expiry to 3s
		redsync.WithTries(3),                         // Set the number of tries to 3
		redsync.WithRetryDelay(200*time.Millisecond), // Set the retry delay to 200ms
	)
}
