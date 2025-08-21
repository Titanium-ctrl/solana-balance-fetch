package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type APIKey struct {
	Key         string    `bson:"key"`
	Active      bool      `bson:"active"`
	LastUpdated time.Time `bson:"last_updated"`
}

var (
	mongoClient  *mongo.Client
	apiKeys      = make(map[string]struct{}) //in-memory api key store
	lastSyncTime time.Time
	collection   *mongo.Collection
)

func LoadAllAPIKeys() error {
	// Load all API keys from the database into the in-memory store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{"active": true})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	temp := make(map[string]struct{})
	for cursor.Next(ctx) {
		var apiKey APIKey
		if err := cursor.Decode(&apiKey); err != nil {
			return err
		}
		temp[apiKey.Key] = struct{}{}
		if apiKey.LastUpdated.After(lastSyncTime) {
			lastSyncTime = apiKey.LastUpdated
		}
	}
	apiKeys = temp
	return nil
}

func FetchUpdatedAPIKeys() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{"last_updated": bson.M{"$gt": lastSyncTime}})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var apiKey APIKey
		if err := cursor.Decode(&apiKey); err != nil {
			return err
		}
		if apiKey.Active {
			apiKeys[apiKey.Key] = struct{}{}
		} else {
			delete(apiKeys, apiKey.Key)
		}
		if apiKey.LastUpdated.After(lastSyncTime) {
			lastSyncTime = apiKey.LastUpdated
		}
	}
	return nil
}

func StartPollingAPIKeys() {
	timeTicker := time.NewTicker(1 * time.Minute)
	for range timeTicker.C {
		if err := FetchUpdatedAPIKeys(); err != nil {
			fmt.Println("Error fetching updated API keys:", err)
		}
	}
}

// Fiber v3 auth middleware function
func AuthMiddleware(c fiber.Ctx) error {
	apiKey := c.Get("Authorization")
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "API key is required",
		})
	}
	if _, exists := apiKeys[apiKey]; !exists {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or inactive API key",
		})
	}
	return c.Next()
}

func SetupAuthInitialLoad() {
	mongoURL := os.Getenv("MONGODB_URL")
	// Connect to MongoDB
	client, err := mongo.Connect(options.Client().ApplyURI(mongoURL))
	if err != nil {
		panic(err)
	}
	mongoClient = client
	collection = mongoClient.Database("customers").Collection("api_keys")
	if err := LoadAllAPIKeys(); err != nil {
		panic(err)
	}
}
