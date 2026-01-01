package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"shourty/internal/base62"
	"shourty/internal/storage"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type ShortenResponse struct {
	ShortUrl string `json:"short_url"`
}

type ShortenRequest struct {
	LongURL string `json:"long_url"`
}

type Server struct {
	store   storage.URLStore
	baseURL string
}

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Get base URL from environment
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		log.Fatal("BASE_URL environment variable is required")
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("REDIS_ADDR environment variable is required")
	}

	// ============================================
	// Initialize Sharded PostgreSQL Store
	// ============================================
	numShardsStr := os.Getenv("NUM_SHARDS")
	if numShardsStr == "" {
		numShardsStr = "3"
	}
	numShards, err := strconv.Atoi(numShardsStr)
	if err != nil {
		log.Fatal("Invalid NUM_SHARDS value")
	}

	// Collect all shard connection strings
	connStrings := make([]string, numShards)
	for i := 0; i < numShards; i++ {
		envKey := fmt.Sprintf("DATABASE_SHARD_%d", i)
		connStr := os.Getenv(envKey)
		if connStr == "" {
			log.Fatalf("%s environment variable is required", envKey)
		}
		connStrings[i] = connStr
	}

	// Create sharded store
	shardedStore, err := storage.NewShardedPostgresStore(connStrings)
	if err != nil {
		log.Fatalf("Failed to create sharded store: %v", err)
	}
	defer shardedStore.Close()

	log.Printf("Connected to %d database shards", numShards)

	// ============================================
	// Initialize Redis (for caching)
	// ============================================
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})

	var ctx = context.Background()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("cannot ping to redis %v", err)
	}

	redisStore := storage.NewRedisStore(shardedStore, redisClient)

	s := &Server{
		store:   redisStore,
		baseURL: baseURL,
	}

	http.HandleFunc("/shorten", s.handleShorten)
	http.HandleFunc("/", s.handleRedirect)
	fmt.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *Server) handleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	shortCode, err := s.store.GetOrCreateShortUrl(req.LongURL, base62.Encode)
	if err != nil {
		log.Printf("Error shortening URL: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := ShortenResponse{
		ShortUrl: fmt.Sprintf("%s/%s", s.baseURL, shortCode),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/")
	if len(code) > 20 {
		http.Error(w, "Invalid short code format", http.StatusBadRequest)
		return
	}

	longUrl, err := s.store.GetLongUrl(code)
	if err == sql.ErrNoRows {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Database Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache") // Optional: prevents heavy browser caching during testing
	http.Redirect(w, r, longUrl, http.StatusMovedPermanently)
}
