// DEPRECATED: This file is currently dead code and is NOT used in the running application.
// The active storage implementation is in sharded_postgres.go, which supports database sharding.
// This file is kept for historical reference only.

package storage

import (
	"database/sql"
	"log"
	"time"

	"github.com/bwmarrin/snowflake"
	_ "github.com/lib/pq"
)

type PostgresStore struct {
	DB     *sql.DB
	Node   *snowflake.Node
	Filter BloomFilter
}

func NewPostgresStore(connStr string, filter BloomFilter) (*PostgresStore, error) {
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

	rows, err := db.Query("SELECT long_url FROM urls")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var longUrl string
		if err := rows.Scan(&longUrl); err != nil {
			return nil, err
		}
		filter.Add(longUrl)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	store := &PostgresStore{DB: db, Node: node, Filter: filter}

	return store, nil
}

func (s *PostgresStore) GetOrCreateShortUrl(longUrl string, encodeFunc func(uint64) string) (string, error) {

	var shortCode string

	if exists, err := s.Filter.Exists(longUrl); exists && err == nil {
		log.Printf("Found in Bloom Filter: %s", longUrl)
		err := s.DB.QueryRow("SELECT short_url FROM urls WHERE long_url = $1", longUrl).Scan(&shortCode)
		if err == nil {
			return shortCode, nil
		}
	} else if err != nil {
		return "", err
	}

	log.Printf("Not Found in Bloom Filter: %s", longUrl)
	// Generate ID
	snowflakeID := s.Node.Generate()
	id := uint64(snowflakeID.Int64())

	// Generate short code
	shortCode = encodeFunc(id)

	// Insert into DB
	_, err := s.DB.Exec("INSERT INTO urls (id, long_url, short_url) VALUES ($1, $2, $3)", id, longUrl, shortCode)
	if err != nil {
		return "", err
	}

	s.Filter.Add(longUrl)

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
