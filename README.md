# Distributed Rate Limiter with FIFO Eviction

A high-performance rate-limiting middleware built in **Go** and **Redis**. 

### The "Modified Leaky Bucket" Algorithm
Unlike standard Leaky Bucket implementations that drop overflowing traffic (HTTP 429), this system implements a **FIFO (First-In-First-Out) Eviction Policy**. 

**Why?** In high-frequency telemetry (GPS updates, stock tickers), the latest data point is the only one that matters. This system ensures the "Bucket" always contains the N-most recent requests.

### Technical Architecture
- **Atomicity:** Core logic is implemented via **Redis Lua Scripting** to prevent race conditions during concurrent bursts.
- **Data Structure:** Uses **Redis Sorted Sets (ZSET)**. Timestamps serve as both the score and the member, allowing $O(\log N)$ cleaning of stale data.
- **Observability:** Injects custom HTTP headers (`X-RateLimit-Remaining`, `X-RateLimit-Evicted`) for client-side throttling awareness.
- **Performance:** Benchmarked using **k6** with a p99 latency of <2ms.

### Use Cases
- Real-time IoT sensor data ingestion.
- Live financial data feeds.
- High-concurrency "Freshness-First" APIs.
