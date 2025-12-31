package storage

type URLStore interface {
	GetOrCreateShortUrl(longUrl string, encodeFunc func(uint64) string) (string, error)
	GetLongUrl(shortCode string) (string, error)
}
