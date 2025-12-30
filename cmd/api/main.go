package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"shourty/internal/base62"
	"shourty/internal/storage"
	"strings"

	"github.com/joho/godotenv"
)

type ShortenResponse struct {
	ShortUrl string `json:"short_url"`
}

type ShortenRequest struct {
	LongURL string `json:"long_url"`
}

type Server struct {
	store   *storage.Storage
	baseURL string
}

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Get database connection string from environment
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Get base URL from environment
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		log.Fatal("BASE_URL environment variable is required")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("cannot ping to db %v", err)
	}

	s := &Server{
		store:   &storage.Storage{DB: db},
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
	if len(code) != 7 {
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
