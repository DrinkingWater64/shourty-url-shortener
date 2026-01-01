package storage

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	next   URLStore
	client *redis.Client
}

func NewRedisStore(next URLStore, client *redis.Client) *RedisStore {
	return &RedisStore{
		next:   next,
		client: client,
	}
}

func (r *RedisStore) GetLongUrl(shortCode string) (string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("short:%s", shortCode)

	val, err := r.client.Get(ctx, key).Result()
	if err == nil {
		log.Printf("CACHE HIT: %s -> %s", shortCode, val)
		return val, nil
	}
	log.Printf("CACHE MISS: %s", shortCode)

	longUrl, err := r.next.GetLongUrl(shortCode)
	if err != nil {
		return "", err
	}

	if err := r.client.Set(ctx, key, longUrl, 1*time.Hour).Err(); err != nil {
		fmt.Println("Failed to cache long URL:", err)
	}

	return longUrl, nil
}

func (r *RedisStore) GetOrCreateShortUrl(longUrl string, encodeFunc func(uint64) string) (string, error) {
	return r.next.GetOrCreateShortUrl(longUrl, encodeFunc)
}

type RedisBloom struct {
	client *redis.Client
	key    string
}

func NewRedisBloom(client *redis.Client, key string) *RedisBloom {
	err := client.Do(context.Background(), "BF.RESERVE", key, 0.01, 1000000).Err()
	if err != nil {
		if strings.Contains(err.Error(), "key exists") || strings.Contains(err.Error(), "item exists") {
			log.Println("Bloom filter already exists, skipping creation.")
		} else {
			log.Fatal(err)
		}
	}
	return &RedisBloom{client: client, key: key}
}

func (r *RedisBloom) Add(item string) error {
	return r.client.Do(context.Background(), "BF.ADD", r.key, item).Err()
}

func (r *RedisBloom) Exists(item string) (bool, error) {
	val, err := r.client.Do(context.Background(), "BF.EXISTS", r.key, item).Bool()
	return val, err
}
