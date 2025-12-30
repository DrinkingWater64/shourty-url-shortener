package storage

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type Storage struct {
	DB *sql.DB
}

func NewPostgresStorage(connStr string) (*Storage, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Storage{DB: db}, nil
}

func (s *Storage) GetOrCreateShortUrl(longUrl string, encodeFunc func(uint64) string) (string, error) {
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

func (s *Storage) GetLongUrl(shortCode string) (string, error) {
	var longUrl string
	err := s.DB.QueryRow("SELECT long_url FROM urls WHERE short_url = $1", shortCode).Scan(&longUrl)
	if err != nil {
		return "", err
	}
	return longUrl, nil
}
