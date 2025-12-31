package storage

import (
	"database/sql"
	"log"
	"time"

	"github.com/bwmarrin/snowflake"
	_ "github.com/lib/pq"
)

type PostgresStore struct {
	DB   *sql.DB
	Node *snowflake.Node
}

func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Connection Pooling Guidelines
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	node, err := snowflake.NewNode(1)
	if err != nil {
		return nil, err
	}

	return &PostgresStore{DB: db, Node: node}, nil
}

func (s *PostgresStore) GetOrCreateShortUrl(longUrl string, encodeFunc func(uint64) string) (string, error) {
	var shortCode string

	err := s.DB.QueryRow("SELECT short_url FROM urls WHERE long_url = $1", longUrl).Scan(&shortCode)
	if err == nil {
		return shortCode, nil
	}

	// Generate ID
	snowflakeID := s.Node.Generate()
	id := uint64(snowflakeID.Int64())

	// Generate short code
	shortCode = encodeFunc(id)

	// Insert into DB
	_, err = s.DB.Exec("INSERT INTO urls (id, long_url, short_url) VALUES ($1, $2, $3)", id, longUrl, shortCode)
	if err != nil {
		return "", err
	}

	return shortCode, nil
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
