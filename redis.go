package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type Balance struct {
	Wallet    string  `json:"wallet"`
	Amount    float64 `json:"amount"`
	FetchedAt int64   `json:"fetched_at"`
}

var opt, _ = redis.ParseURL(os.Getenv("REDIS_URL"))

var redisClient = redis.NewClient(opt)

func GetBalanceFromCache(address string) (*Balance, error) {
	bal, err := redisClient.Get(context.Background(), "wallet:"+address).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	} else if err != nil {
		return nil, err // Error fetching from cache
	}

	var balance Balance
	if err := json.Unmarshal([]byte(bal), &balance); err != nil {
		return nil, err // Error unmarshalling cached data
	}
	return &balance, nil // Cache hit
}

func SetBalanceInCache(balance *Balance) error {
	bal, err := json.Marshal(balance)
	if err != nil {
		return err // Error marshalling balance data
	}
	return redisClient.Set(context.Background(), "wallet:"+balance.Wallet, bal, 10*time.Second).Err()
}
