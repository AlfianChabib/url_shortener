# Load Testing & Performance Benchmarking (Phase 3)

This directory contains `k6` load testing scripts designed to validate the Non-Functional Requirements (SLIs/SLOs) outlined in the **High-Performance URL Shortener PRD Section 2.1 & 8**.

## Performance Targets

| Metric | Target | Endpoint | Scenario |
| :--- | :--- | :--- | :--- |
| **Read Throughput** | $\ge 10.000\text{ RPS}$ | `GET /{short_code}` | `redirect_10k_rps.js` |
| **Read Latency (P50)** | $< 3\text{ ms}$ | `GET /{short_code}` | `redirect_10k_rps.js` |
| **Read Latency (P99)** | $< 15\text{ ms}$ | `GET /{short_code}` | `redirect_10k_rps.js` |
| **Write Throughput** | $\ge 1.000\text{ RPS}$ | `POST /api/v1/links` | `create_link_1k_rps.js` |
| **Write Latency (P99)**| $< 80\text{ ms}$ | `POST /api/v1/links` | `create_link_1k_rps.js` |
| **Error Rate** | $< 0.1\%$ | All Endpoints | All Scenarios |

---

## Prerequisites

1. Install `k6`:
   ```bash
   # Windows (via Chocolatey or Winget)
   choco install k6
   # or
   winget install k6

   # macOS (Homebrew)
   brew install k6

   # Linux
   sudo gpg -k
   sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
   echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
   sudo apt-get update && sudo apt-get install k6
   ```

2. Start the URL Shortener application:
   ```bash
   # Build & run
   go run ./cmd/api
   ```

---

## Running Load Tests

### 1. 10,000 RPS Redirect Benchmark
Tests the high-throughput read path and verifies sub-3ms P50 latency and HTTP 307 responses:
```bash
k6 run scripts/k6/redirect_10k_rps.js
```

### 2. 1,000 RPS Creation Benchmark
Tests the asynchronous write path and Base62 / Snowflake ID generation:
```bash
k6 run scripts/k6/create_link_1k_rps.js
```

### 3. Full System Enterprise Stress Test
Runs both read and write traffic simultaneously under sustained high load:
```bash
k6 run scripts/k6/enterprise_stress_test.js
```

---

## Native Go High-Concurrency Benchmarks

You can also run internal Go benchmarks directly without external dependencies:
```bash
go test -bench=. -benchmem ./cmd/api/...
```
