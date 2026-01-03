# System Design Notes

## 1. Why Redis?
In our current architecture, Redis isn't just a "nice to have"—it is the primary defense layer for our database shards.

*   **Read-Heavy Workload**: URL shorteners typically see a 100:1 read-to-write ratio. A redirect requires looking up the long URL. Doing this on Postgres (disk I/O) for every request is wasteful. Redis (memory) offers sub-millisecond retrieval.
*   **Viral Problem**: If a shortened link goes viral, we could see 50k+ requests/second for *one specific key*. Postgres cannot handle this concurrency on a single row/page due to lock contention and connection limits. Redis handles this effortlessly.
*   **Connection Economy**: Our implementation uses `db.SetMaxOpenConns(100)`. If valid traffic spikes, we might exhaust DB connections just for simple lookups. Redis offloads the vast majority of these "dumb reads," preserving precious DB connections for Writes (creating new links).
*   **Cost Efficiency**: Scaling RAM (Redis) for the active links is cheaper and more performance-linear than vertically scaling a relational database to keep the entire dataset in the buffer cache.

## 2. Why Database Sharding?
We chose a sharded architecture (splitting data across 3 Postgres instances) rather than a single large database.

*   **Write Throughput**: A single Postgres primary creates a hard ceiling on Write Operations Per Second (WOPS). Sharding limits the write load on any single node to `1/N` of total traffic.
*   **Reduced Blast Radius**: If `Shard-0` fails, only 33% of links are affected, not 100%. `Shard-1` and `Shard-2` remain operational.
*   **Horizontal Scalability**: We can add more commodity storage nodes rather than buying exponentially more expensive "super servers" (Vertical Scaling).

## 3. Other Design Decisions
*   **Nginx as Load Balancer**: We use Nginx as a reverse proxy/LB to terminate SSL and distribute traffic. This allows zero-downtime deployments (rolling updates) and protects our Go backends from direct internet traffic.
*   **Snowflake IDs**: We use Twitter's Snowflake algorithm for ID generation instead of Postgres `SERIAL`.
    *   **Why?**: In a sharded system, you can't rely on a central database for auto-incrementing IDs without creating a massive bottleneck. Snowflake allows each application node to generate unique, k-sortable IDs independently without coordination.

## 4. What breaks first at scale?
Even with Sharding (3 nodes) and Nginx, the current system has specific breaking points:

*   **Static Sharding Strategy (`hash % N`)**: 
    *   **The Problem**: Our code uses `fnv_hash % s.NumShards`. If we need to scale from 3 to 4 shards, **nearly 100% of the data mapping changes**. This requires a complete re-balancing of the entire dataset (downtime or complex double-write migrations).
    *   **Result**: We are effectively "locked in" to our initial shard count unless we invest heavily in migration tooling.
*   **Hot Shard Imbalance**:
    *   **The Problem**: A simple hash doesn't account for traffic load. If `google.com` and `amazon.com` both hash to `Shard-0` and are the most popular links, `Shard-0` will be at 100% CPU while `Shard-1` and `Shard-2` sit idle.
    *   **Result**: One shard failure cascades into an outage for 1/3rd of our users.
*   **Connection Exhaustion (The N x M Multiplier)**:
    *   **The Problem**: Every API instance connects to *every* shard.
    *   `Connections = (Num API Instances) * (Num Shards)`
    *   If we autoscale our API tier to 50 containers to handle traffic: `50 APIs * 3 Shards * 10 max_idle = 1500 connections`. This can easily exceed Postgres' `max_connections` limit, causing the DB to reject new API pods.

## 5. What I’d improve with more time

### Architecture & Reliability
*   **Consistent Hashing (Ring Topology)**: Replace `hash % N` with Consistent Hashing. This ensures that adding a new shard only requires moving ~1/N of the keys, making horizontal scaling operational.
*   **Circuit Breakers**: Implement patterns (like Hystrix) in the `GetLongUrl` path. If Shard-1 is down, the API should "fail fast" for those keys rather than hanging and consuming resources until timeouts occur.

### Data & Performance
*   **Eventual Consistency for Analytics**: Real-world shorteners need to track clicks. Writing a `click_count + 1` update to Postgres synchronously for every visit will tank performance. I would introduce a Message Queue (e.g., Kafka or Redis Streams) to buffer clicks and flush them to a specific analytics table (e.g., ClickHouse or a heavy-write Postgres table) asynchronously.
*   **Bloom Filters**: Currently we are allowing duplicate links. Before hitting the database for a `GetOrCreate` call (if we were checking for duplicates), check a Bloom Filter. If it says "No", we know for a fact the URL doesn't exist, saving a wasted DB query.
*   **Tiered Caching**: Implement a local in-memory cache (LRU) *inside* the Go API binary for the absolute hottest keys. This eliminates even the network round-trip to Redis.

### Observability
*   **Distributed Tracing (OpenTelemetry)**: Currently, if a request is slow, we don't know if it's Nginx, the Go Service, Redis, or the specific Postgres Shard. Tracing would visualize the entire request lifecycle.
