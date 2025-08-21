package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/joho/godotenv"
)

type RequestFormat struct {
	Wallets []string `json:"wallets"`
}

type ResponseFormat struct {
	Wallets map[string]float64 `json:"wallets"`
	Errors  map[string]string  `json:"errors"`
}

var _ = godotenv.Load()

var client = rpc.New(os.Getenv("RPC_CLIENT_URL"))

//var client = rpc.New(rpc.MainNetBeta_RPC)

func GetSolBalance(solAddress string) (float64, bool, error) {
	//Try cache first
	balance, err := GetBalanceFromCache(solAddress)
	if err == nil && balance != nil {
		return balance.Amount, true, nil // Return cached balance if available
	}

	//Cache miss - fetch from Solana RPC
	pubKey := solana.MustPublicKeyFromBase58(solAddress)

	out, err := client.GetBalance(
		context.TODO(),
		pubKey,
		rpc.CommitmentFinalized,
	)

	if err != nil {
		return 0.0, false, err
	}

	lamportsOnAccount := new(big.Float).SetUint64(out.Value)
	solBalance := new(big.Float).Quo(lamportsOnAccount, new(big.Float).SetUint64(solana.LAMPORTS_PER_SOL))

	solBalanceFormatted, _ := solBalance.Float64()

	go SetBalanceInCache(&Balance{
		Wallet:    solAddress,
		Amount:    solBalanceFormatted,
		FetchedAt: time.Now().Unix(),
	})

	return solBalanceFormatted, false, nil
}

func main() {
	SetupAuthInitialLoad()

	go StartPollingAPIKeys()

	app := fiber.New()

	app.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		},
	}))
	//Define global RPC client

	app.Post("/api/get-balance", AuthMiddleware, func(c fiber.Ctx) error {
		var reqBody RequestFormat
		if err := c.Bind().Body(&reqBody); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request format",
			})
		}
		//Reform the below to handle multiple wallets using goroutines and channels
		if len(reqBody.Wallets) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "No wallets provided",
			})
		}

		//Prep
		walletBalances := make(map[string]float64, len(reqBody.Wallets))
		walletErrors := make(map[string]string, len(reqBody.Wallets))
		var mu sync.Mutex
		var errMu sync.Mutex
		var wg sync.WaitGroup
		channel := make(chan struct{}, 50) // Limit concurrency to 50
		allFromCache := true
		var cacheMu sync.Mutex

		for _, wallet := range reqBody.Wallets {
			wg.Add(1)
			go func(address string) {
				defer wg.Done()

				channel <- struct{}{} // Block if channel is full

				//Get and lock mutex for the address so only one goroutine can get its balance at a time
				walletMutexLock := NewMutex(address)
				if err := walletMutexLock.Lock(); err != nil {
					fmt.Println("Failed to acquire lock for address:", address, "Error:", err)
				}

				balance, fromCache, err := GetSolBalance(address)
				<-channel // Release the slot in the channel

				if err != nil {
					balance = 0.0 //Set balance to 0 if there's an error
					errMu.Lock()
					walletErrors[address] = err.Error()
					errMu.Unlock()
				}

				if !fromCache {
					cacheMu.Lock()
					allFromCache = false
					cacheMu.Unlock()
				}

				if ok, err := walletMutexLock.Unlock(); !ok || err != nil {
					fmt.Println("Failed to release lock for address:", address, "Error:", err)
				}

				mu.Lock()
				walletBalances[address] = balance
				mu.Unlock()
			}(wallet)
		}

		wg.Wait()

		if allFromCache {
			c.Set("X-Cache", "HIT")
		} else {
			c.Set("X-Cache", "MISS")
		}

		return c.JSON(ResponseFormat{
			Wallets: walletBalances,
			Errors:  walletErrors,
		})
	})

	app.Listen(":3000")

}
