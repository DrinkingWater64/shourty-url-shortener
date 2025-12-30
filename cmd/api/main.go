package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"shourty/internal/base62"
	"shourty/internal/storage"
	"strings"
)

type ShortenResponse struct {
	ShortUrl string `json:"short_url"`
}

type ShortenRequest struct {
	LongURL string `json:"long_url"`
}

type Server struct {
	store *storage.Storage
}

func main() {
	connStr := "postgres://user:password@localhost:5432/shortener?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("cannot ping to db %v", err)
	}

	s := &Server{
		store: &storage.Storage{DB: db},
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
		ShortUrl: fmt.Sprintf("http://localhost:8080/%s", shortCode),
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

	var longUrl string
	err := s.store.DB.QueryRow("SELECT long_url FROM urls WHERE short_code = $1", code).Scan(&longUrl)
	if err == sql.ErrNoRows {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Database Error", http.StatusInternalServerError)
	}

	w.Header().Set("Cache-Control", "no-cache") // Optional: prevents heavy browser caching during testing
	http.Redirect(w, r, longUrl, http.StatusMovedPermanently)
}
