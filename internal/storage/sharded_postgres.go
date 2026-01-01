package storage

import (
	"database/sql"
	"hash/fnv"
	"log"
	"time"

	"github.com/bwmarrin/snowflake"
	_ "github.com/lib/pq"
)

type ShardedPostgresStore struct {
	Shards    []*sql.DB
	NumShards int
	Node      *snowflake.Node
}

func NewShardedPostgresStore(connStrings []string) (*ShardedPostgresStore, error) {
	shards := make([]*sql.DB, len(connStrings))

	for i, connStr := range connStrings {
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return nil, err
		}

		if err := db.Ping(); err != nil {
			return nil, err
		}

		// Connection pooling settings
		db.SetMaxOpenConns(100)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		shards[i] = db
		log.Printf("Connected to shard %d", i)
	}

	// Create snowflake node for ID generation
	node, err := snowflake.NewNode(1)
	if err != nil {
		return nil, err
	}

	return &ShardedPostgresStore{
		Shards:    shards,
		NumShards: len(connStrings),
		Node:      node,
	}, nil
}

func (s *ShardedPostgresStore) getShard(shortCode string) *sql.DB {
	h := fnv.New32a()
	h.Write([]byte(shortCode))
	shardIndex := h.Sum32() % uint32(s.NumShards)

	log.Printf("Routing %s to shard %d", shortCode, shardIndex)
	return s.Shards[shardIndex]
}

// Note: We allow duplicates - same long_url may get different short codes
func (s *ShardedPostgresStore) GetOrCreateShortUrl(longUrl string, encodeFunc func(uint64) string) (string, error) {
	snowflakeID := s.Node.Generate()
	id := uint64(snowflakeID.Int64())

	shortCode := encodeFunc(id)

	shard := s.getShard(shortCode)

	// 4. Insert into the selected shard
	_, err := shard.Exec(
		"INSERT INTO urls (id, long_url, short_url) VALUES ($1, $2, $3)",
		id, longUrl, shortCode,
	)
	if err != nil {
		return "", err
	}

	return shortCode, nil
}

// Routes to the correct shard using the same hash function
func (s *ShardedPostgresStore) GetLongUrl(shortCode string) (string, error) {
	// Route to correct shard
	shard := s.getShard(shortCode)

	log.Printf("DB HIT: looking up %s", shortCode)

	var longUrl string
	err := shard.QueryRow(
		"SELECT long_url FROM urls WHERE short_url = $1",
		shortCode,
	).Scan(&longUrl)

	if err != nil {
		return "", err
	}

	return longUrl, nil
}

func (s *ShardedPostgresStore) Close() error {
	for i, shard := range s.Shards {
		if err := shard.Close(); err != nil {
			log.Printf("Error closing shard %d: %v", i, err)
		}
	}
	return nil
}
