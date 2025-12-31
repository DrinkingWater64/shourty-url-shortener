# Shourty - URL Shortener

This project is an educational experiment to demonstrate how to scale APIs horizontally and optimize database interactions. It starts as a simple Go application and evolves into a distributed system.

## Features

- Shorten long URLs to 7-character codes
- Automatic redirect from short URLs to original URLs
- PostgreSQL database for persistence
- Environment-based configuration
- Docker Compose for easy database setup

## Architecture & Optimization

This project demonstrates several key scalability patterns:

### 1. Load Balancing (Nginx)
We use Nginx as a reverse proxy to distribute incoming traffic across **3 concurrent API instances** (`api-1`, `api-2`, `api-3`) using a Round-Robin strategy. This ensures no single instance becomes a bottleneck and provides redundancy.

### 2. Caching (Redis)
To reduce load on the primary PostgreSQL database, we use **Redis** to cache URL mappings. Since URL redirects are 90% read-heavy, this significantly improves response times and throughput.

### 3. Database Connection Pooling
The application implements **Connection Pooling** to efficiently manage database resources. Instead of opening a new connection for every request, we maintain a pool of reusable connections to handle high concurrency without exhausting database limits.

## Prerequisites

- Go 1.25.5 or higher
- Docker and Docker Compose (for database)

## Quick Start

### 1. Start the Database

```bash
docker compose up -d
```

This will start a PostgreSQL database with the following configuration:
- **User**: `shourty_user`
- **Password**: `shourty_pass`
- **Database**: `shortener`
- **Port**: `5432`

### 2. Configure Environment Variables

The `.env` file is already configured to work with the Docker database. If you need to customize:

```bash
cp .env.example .env
# Edit .env with your preferred values
```

### 3. Run the Application

```bash
go run cmd/api/main.go
```

Or build and run:

```bash
go build -o bin/api ./cmd/api
./bin/api
```

The server will start on `http://localhost:8080`

## API Usage

### Shorten a URL

```bash
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"long_url": "https://www.example.com/very/long/url"}'
```

Response:
```json
{
  "short_url": "http://localhost:8080/abc1234"
}
```

### Access Short URL

Simply visit the short URL in your browser or use curl:

```bash
curl -L http://localhost:8080/abc1234
```

## Database Management

### View Database Logs

```bash
docker compose logs -f postgres
```

### Stop Database

```bash
docker compose down
```

### Stop and Remove Data

```bash
docker compose down -v
```

### Access Database CLI

```bash
docker compose exec postgres psql -U shourty_user -d shortener
```

## Environment Variables

- `DATABASE_URL`: PostgreSQL connection string
- `BASE_URL`: Base URL for generating short URLs (useful for production deployments)

## Project Structure

```
shourty/
├── cmd/
│   └── api/
│       └── main.go          # Main application entry point
├── internal/
│   ├── base62/              # Base62 encoding for short codes
│   └── storage/             # Database operations
│       └── schema.sql       # Database schema
├── docker-compose.yml       # Docker Compose configuration
├── init.sql                 # Database initialization script
├── .env                     # Environment variables (not in git)
├── .env.example             # Environment variables template
└── go.mod                   # Go module dependencies
```

## Development

### Database Schema

The database uses a simple schema with one table:

```sql
CREATE TABLE urls (
    id BIGSERIAL PRIMARY KEY,
    long_url TEXT NOT NULL UNIQUE,
    short_url VARCHAR(7) UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

## License

MIT
