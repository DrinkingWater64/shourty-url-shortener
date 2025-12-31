package storage

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	DB *sql.DB
}

func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresStore{DB: db}, nil
}

func (s *PostgresStore) GetOrCreateShortUrl(longUrl string, encodeFunc func(uint64) string) (string, error) {
	var shortCode string

	err := s.DB.QueryRow("SELECT short_url FROM urls WHERE long_url = $1", longUrl).Scan(&shortCode)
	if err == nil {
		return shortCode, nil
	}

	var id uint64
	err = s.DB.QueryRow("INSERT INTO urls (long_url) VALUES ($1) RETURNING id", longUrl).Scan(&id)
	if err != nil {
		return "", err
	}

	shortCode = encodeFunc(id)

	_, err = s.DB.Exec("UPDATE urls SET short_url = $1 WHERE id = $2", shortCode, id)
	return shortCode, err
}

func (s *PostgresStore) GetLongUrl(shortCode string) (string, error) {
	log.Printf("DB HIT: looking up %s", shortCode)
	var longUrl string
	err := s.DB.QueryRow("SELECT long_url FROM urls WHERE short_url = $1", shortCode).Scan(&longUrl)
	if err != nil {
		return "", err
	}
	return longUrl, nil
}
