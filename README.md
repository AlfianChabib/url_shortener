# 🚀 High-Performance Enterprise URL Shortener

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![Framework](https://img.shields.io/badge/Framework-Fiber%20v3-00ACD7?style=flat)](https://gofiber.io)
[![Database](https://img.shields.io/badge/Database-PostgreSQL%2016-336791?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![Cache](https://img.shields.io/badge/Cache-Redis%207-DC382D?style=flat&logo=redis)](https://redis.io/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A production-grade, distributed, high-throughput URL Shortener designed to sustain **10,000+ RPS read redirections** (P99 < 15ms) and **1,000+ RPS link creations** (P99 < 80ms) with zero downtime and strict enterprise security protections.

---

## 🌟 Key Features

* **⚡ Ultra High-Throughput Redirection**:
  * Built on **Fiber v3** (fasthttp-based) with sub-3ms median latency.
  * Multi-tier caching architecture (Redis L1 cache with fallback).
  * Returns `HTTP 307 Temporary Redirect` to ensure analytics integrity across all clicks.

* **🆔 Distributed Unique ID & Custom Aliases**:
  * Uses **Twitter Snowflake (64-bit)** integer generation converted into compact **Base62** strings (e.g. `https://domain/7bXk9`).
  * Collision-free, k-sorted, and distributed across nodes without database sequence locks.
  * Supports Custom Aliases for authenticated users with regex validation (`^[a-zA-Z0-9_-]{4,32}$`).

* **🛡️ Enterprise SSRF & Security Hardening**:
  * **SSRF Validator**: Restricts target URLs from pointing to private RFC 1918, RFC 6598, carrier-grade NAT, or cloud metadata endpoints (`169.254.169.254`).
  * **Anti-Loopback & Self-Redirection**: Prohibits users from shortening internal domains.
  * **In-Memory DNS Cache**: Ultra-fast DNS caching (5-minute TTL) eliminates DNS resolution bottlenecks and prevents latency spikes during massive traffic.
  * **Security Headers & CORS**: Pre-configured security headers (XSS protection, Frame-Options, Content-Type Options).

* **📊 Non-Blocking Asynchronous Analytics**:
  * Dedicated buffered worker pool (`buffer: 10,000`, `batch: 100`, `flush: 200ms`) writes click logs asynchronously to PostgreSQL.
  * Click redirection execution path is 100% non-blocking.
  * Captures Referer, User-Agent (Browser, OS, Device Type), and Geolocation (Country, City, IP).

* **🔐 Authentication & Access Control**:
  * JWT-based authentication (HMAC-SHA256) with secure bcrypt password hashing.
  * Redis-backed Token Blacklisting for instant invalidation on `/api/v1/auth/logout`.
  * Public anonymous link creation + authenticated user links dashboard.

* **🚦 Sliding-Window Rate Limiting**:
  * Redis sliding-window counter rate limiters configured per IP for authentication, link creation, and redirection protection.

---

## 🏛️ System Architecture

```mermaid
flowchart TD
    Client([Client / Browser]) -->|HTTP Request| FiberApp[Fiber v3 HTTP Router]
    
    subgraph Middlewares
        FiberApp --> RecoverMW[Recover Middleware]
        RecoverMW --> SecurityMW[Security Headers & CORS]
        SecurityMW --> RateLimitMW[Rate Limiter (Redis)]
        RateLimitMW --> AuthMW[JWT Auth / Blacklist Check]
    end

    subgraph Controllers & Services
        AuthMW --> AuthCtrl[Auth Controller]
        AuthMW --> LinkCtrl[Link Controller]
        
        LinkCtrl --> LinkService[Link Service]
        LinkService --> SSRF[SSRF Validator + In-Memory DNS Cache]
        LinkService --> Snowflake[Snowflake Node / Base62]
    end

    subgraph Data Stores
        LinkService --> Repo[Link Repository]
        Repo -->|Cache Hit/Miss| Redis[(Redis 7)]
        Repo -->|Persistent Storage| Postgres[(PostgreSQL 16)]
    end

    subgraph Asynchronous Pipeline
        LinkCtrl -.->|Non-blocking Enqueue| WorkerPool[Analytics Worker Pool]
        WorkerPool -->|Batch Flush| Postgres
    end
```

---

## 🛠️ Tech Stack

| Layer | Technology |
| :--- | :--- |
| **Language** | Go 1.24+ |
| **Web Framework** | [Fiber v3](https://github.com/gofiber/fiber/v3) |
| **Database ORM** | [GORM](https://gorm.io/) with `pgx/v5` driver pool |
| **Relational Database** | [PostgreSQL 16](https://www.postgresql.org/) |
| **In-Memory Cache** | [Redis 7](https://redis.io/) via `go-redis/v9` |
| **Dependency Injection**| [Google Wire](https://github.com/google/wire) |
| **Configuration** | [Viper](https://github.com/spf13/viper) (`.env`) |
| **Validation** | `go-playground/validator/v10` |
| **Load Testing** | [k6](https://k6.io/) |

---

## 📂 Project Directory Structure

```text
url_shortener/
├── cmd/
│   └── api/                    # Application entrypoint & Wire injector
│       ├── main.go
│       ├── wire.go
│       └── wire_gen.go
├── internal/
│   ├── analytics/              # Async click event batching worker pool
│   ├── config/                 # Viper environment configuration
│   ├── controller/             # Fiber HTTP controllers (Auth, Link, Health)
│   ├── database/               # PostgreSQL & GORM connection pool setup
│   ├── middleware/             # Auth, RateLimiter, CORS, Security headers
│   ├── model/
│   │   ├── domain/             # Core database domain entities (User, Link, ClickEvent)
│   │   └── web/                # HTTP request/response DTOs
│   ├── repository/             # Database & Redis access implementations
│   ├── router/                 # Route setup & middleware binding
│   └── service/                # Business logic (Link & Auth services)
├── pkg/
│   ├── errs/                   # Standardized application error types
│   ├── geoip/                  # Geolocation resolver
│   ├── useragent/              # User-Agent device/OS parser
│   ├── utils/                  # Snowflake ID & Base62 codecs
│   └── validator/              # SSRF protection & in-memory DNS caching
├── db/
│   ├── migrations/             # SQL schema migrations (golang-migrate)
│   └── seed/                   # Development database seed script
├── scripts/
│   └── k6/                     # Performance benchmarks & stress test suites
├── docker-compose.yml          # Local container definitions (PostgreSQL & Redis)
├── justfile                    # Task runner command recipes
├── go.mod & go.sum
└── README.md
```

---

## 🚀 Quick Start Guide

### Prerequisites
* [Go 1.24+](https://golang.org/dl/)
* [Docker & Docker Compose](https://docs.docker.com/get-docker/)
* [Just CLI](https://github.com/casey/just) *(recommended task runner)*
* [golang-migrate](https://github.com/golang-migrate/migrate) *(optional, for manual migrations)*

### 1. Clone & Setup Environment

```bash
git clone https://github.com/AlfianChabib/url_shortener.git
cd url_shortener
```

Inspect and update `.env` if necessary:

```env
# Application
APP_NAME=url_shortener
APP_ENV=development
APP_PORT=3000
APP_BASE_URL=http://localhost:3000

# PostgreSQL Pool Settings
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=url_shortener_db
DB_SSL_MODE=disable
DB_MAX_CONNS=35       # Set 35-40 for 1GB RAM (Docker WSL), or 80+ for production
DB_MIN_CONNS=15

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Security & Snowflake
JWT_SECRET=super-secret-jwt-key-minimum-32-chars-long!
JWT_EXPIRES_IN_HOUR=24
NODE_ID=1
```

> **Note on Resource Limits (e.g., Docker WSL with 1 GB RAM):**
> PostgreSQL allocates ~5-10 MB per connection process. Keeping `DB_MAX_CONNS` at **35-40** ensures memory consumption stays well under 400 MB, preventing OOM Killer crashes while easily sustaining 10,000+ RPS.

### 2. Start Services & Run Migrations

Using `just` recipes:
```powershell
# 1. Start PostgreSQL & Redis
just docker-up

# 2. Run Database Migrations
just migrate-up

# 3. (Optional) Seed Database with Initial Developer Account & Links
just db-seed
```

*(Alternatively without `just`)*:
```powershell
docker compose up -d
migrate -path db/migrations -database "postgres://postgres:postgres@localhost:5432/url_shortener_db?sslmode=disable" up
```

### 3. Run the Application

```powershell
# Standard run
just run

# Or with Air (Hot reload in development)
just dev

# Or build binary
just build
just start
```

The server will start listening on `http://localhost:3000`.

---

## 📡 API Reference

### 1. Link Creation (`POST /api/v1/links`)
Create a shortened URL. Anonymous users can generate links; authenticated users can also provide a `custom_alias`.

**Request:**
```http
POST /api/v1/links HTTP/1.1
Host: localhost:3000
Content-Type: application/json
Authorization: Bearer <JWT_TOKEN> (Optional)

{
  "original_url": "https://github.com/AlfianChabib/url_shortener",
  "custom_alias": "my-shortener",
  "expires_in_hours": 168
}
```

**Response (`201 Created`):**
```json
{
  "code": 201,
  "status": "Created",
  "data": {
    "short_code": "my-shortener",
    "short_url": "http://localhost:3000/my-shortener",
    "original_url": "https://github.com/AlfianChabib/url_shortener",
    "created_at": "2026-10-04T12:00:00Z",
    "expires_at": "2026-10-11T12:00:00Z"
  }
}
```

---

### 2. High-Speed Redirection (`GET /:short_code`)
Redirects visitors to the original URL and enqueues an asynchronous click event.

**Request:**
```http
GET /my-shortener HTTP/1.1
Host: localhost:3000
```

**Response:**
```http
HTTP/1.1 307 Temporary Redirect
Location: https://github.com/AlfianChabib/url_shortener
```

---

### 3. Link Analytics (`GET /api/v1/links/:short_code/analytics`)
Retrieve aggregated click stats, device breakdowns, OS, browser, and geographic analytics.

**Response (`200 OK`):**
```json
{
  "code": 200,
  "status": "OK",
  "data": {
    "short_code": "my-shortener",
    "original_url": "https://github.com/AlfianChabib/url_shortener",
    "total_clicks": 14205,
    "top_countries": {
      "ID": 8200,
      "US": 4100,
      "SG": 1905
    },
    "browsers": {
      "Chrome": 9800,
      "Safari": 3200,
      "Firefox": 1205
    },
    "platforms": {
      "Windows": 7000,
      "Android": 4500,
      "iOS": 2705
    }
  }
}
```

---

### 4. Authentication Endpoints

| Method | Endpoint | Description | Protected |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/auth/register` | Register a new user account | No |
| `POST` | `/api/v1/auth/login` | Login and obtain JWT token | No |
| `POST` | `/api/v1/auth/logout` | Logout and blacklist JWT token | Yes |
| `GET` | `/api/v1/auth/me` | Fetch currently logged-in user profile | Yes |
| `GET` | `/api/v1/user/links` | List all links created by current user | Yes |

---

## ⚡ Performance Benchmarks & Stress Tests

This project includes automated [k6](https://k6.io/) load testing scenarios in `scripts/k6/` to validate PRD targets:

| Scenario | Target RPS | Target Latency | Command |
| :--- | :--- | :--- | :--- |
| **Link Redirection** | $\ge 10.000\text{ RPS}$ | P50 < 3ms, P99 < 15ms | `just loadtest-redirect` |
| **Link Creation** | $\ge 1.000\text{ RPS}$ | P99 < 80ms, Errors < 0.5% | `just loadtest-create` |
| **Full Stress Test** | Sustained Enterprise Mix | Read + Write traffic | `just loadtest-all` |

### Internal Go Benchmarks
Run Go microbenchmarks to evaluate memory allocations and Base62 / Snowflake throughput:
```powershell
just bench
```

---

## 📋 Justfile Commands Reference

```powershell
just dev                 # Run with Air live-reload
just run                 # Run application directly
just build               # Build binary to bin/app.exe
just test                # Run all Go tests
just bench               # Run Go benchmark tests
just loadtest-redirect   # Run k6 10k RPS redirect test
just loadtest-create     # Run k6 1k RPS creation test
just loadtest-all        # Run k6 enterprise stress test
just wire                # Regenerate Google Wire DI code
just docker-up           # Start PostgreSQL & Redis containers
just docker-down         # Stop docker containers
just migrate-up          # Apply database schema migrations
just db-seed             # Seed database with sample data
just clean               # Clean build artifacts
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
