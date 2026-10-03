# Product Requirements Document (PRD)
## High-Performance URL Shortener & Analytics Engine

| Parameter | Detail |
| :--- | :--- |
| **Dokumen Versi** | 1.1.0 |
| **Status** | Approved for Development (Updated: User Mode & JWT Authentication) |
| **Core Technology** | Go (Golang) $\ge$ 1.22, Fiber v3, GORM, Redis, PostgreSQL (UUIDv7) / ClickHouse |
| **Target Throughput** | $\ge 10.000\text{ RPS}$ (Read/Redirect), $\ge 1.000\text{ RPS}$ (Write/Creation) |
| **Target Latency** | $\text{P99} < 15\text{ ms}$, $\text{P50} < 3\text{ ms}$ (Redirect endpoint) |

---

## 1. Executive Summary & Problem Statement

Layanan penyingkat tautan (URL Shortener) konvensional kerap menghadapi degradasi performa drastis ketika menangani lonjakan trafik secara mendadak (*traffic spikes*). Masalah umum mencakup:
1. **Database Lock Contention**: Operasi pencatatan metrik analitik (*click count*, geolokasi) yang dijalankan secara sinkron bersamaan dengan proses *redirect* memperlambat respon ke pengguna akhir.
2. **Eksploitasi & Penyalahgunaan**: Menjadi vektor serangan *phishing*, penyebaran malware, SSRF (*Server-Side Request Forgery*), dan serangan DoS tanpa identitas.
3. **ID Collision**: Tabrakan ID pada sistem terdistribusi saat menghasilkan alias pendek dalam konkurensi tinggi.
4. **Ketiadaan Manajemen Kepemilikan (Multi-Tenancy)**: Tanpa sistem identitas, pengguna tidak dapat melacak, mengelola, atau mengamankan tautan yang telah dibuat.

Sistem ini dirancang sebagai mesin *shortener* berkinerja tinggi berbasis Go dengan arsitektur **asynchronous decoupled write-path**, pemanfaatan *multi-tiered caching*, sistem analitik terisolasi, serta **Dual Mode Operations (Anonymous vs User Mode)** yang diperkuat autentikasi **JWT (JSON Web Token)** menggunakan email, username, dan password.

---

## 2. System Architecture & Objectives

### 2.1 Non-Functional Requirements (SLIs/SLOs)
* **High Availability**: SLO $99.95\%$ *uptime* tahunan.
* **Latency Profile**:
  * Read / Redirection (`GET /{short_code}`): $\text{P50} \le 3\text{ ms}$, $\text{P99} \le 15\text{ ms}$.
  * Write / Creation (`POST /api/v1/links`): $\text{P99} \le 80\text{ ms}$.
  * Auth Endpoints (`POST /api/v1/auth/*`): $\text{P99} \le 150\text{ ms}$ (memperhitungkan komputasi hashing bcrypt/Argon2id).
* **Data Integrity**: Bebas dari *race condition* dan *duplicate short code* ($0\%$ collision rate). Primary Key seragam berbasis **UUIDv7** (time-ordered).
* **Graceful Degradation**: Jika broker analitik atau basis data sekunder padam, jalur pengalihan utama dan autentikasi token stateless **tetap wajib berjalan**.

### 2.2 Arsitektur Aliran Data (Data Flow)

```
[ Client Request ]
       │
       ▼
[ Reverse Proxy / NGINX ]
       │
       ▼
[ Go HTTP Engine (Fiber v3) ] ── (Hit / Miss) ──► [ Local Memory / Redis Cache ]
       │                                                      │
       ├─► (Cache Hit) ──► HTTP 307 Temporary Redirect         │
       │                                                      ▼ (Miss)
       │                                            [ Primary Database (PostgreSQL - GORM) ]
       ▼
[ Asynchronous Event Pipeline ]
  (Worker Pool / In-Memory Channel / Redis Stream)
       │
       ├─► Deduplikasi & Anonymisasi IP
       ├─► MaxMind GeoIP Lookup (City, Country)
       ├─► User-Agent Parsing (OS, Device, Browser)
       │
       ▼
[ ClickHouse / TimescaleDB (Time-Series Analytics) ]
```

---

## 3. Core Functional Requirements

### 3.1 URL Shortening Engine
1. **Shortcode Generation**:
   * Menggunakan representasi **Base62** ($[a-zA-Z0-9]$).
   * Panjang default: 7 karakter, menyediakan total kombinasi ruang kunci:
     $$62^7 \approx 3.52 \times 10^{12} \text{ kombinasi unik}$$
   * Strategi ID: Distributed ID Generator (Snowflake 64-bit integer / UUIDv7 entropy) yang dikonversi ke Base62 untuk menjamin keterurutan waktu (*time-ordered*) dan menghindari *lock contention* basis data.
2. **Custom Alias**:
   * **Hanya tersedia bagi Pengguna Terotentikasi (User Mode)** guna mencegah *cybersquatting* dan eksploitasi oleh bot anonim.
   * Panjang alias: 4–32 karakter alfanumerik beserta simbol minus (`-`) dan garis bawah (`_`).
3. **Lifecycle & Expiration**:
   * Opsi *Time-to-Live* (TTL) kedaluwarsa URL (1 jam, 24 jam, 30 hari, atau permanen).
   * Otomatisasi pengembalian status `410 Gone` untuk URL yang telah habis masa berlakunya.

### 3.2 High-Throughput Redirection Engine
1. **HTTP Status Code**:
   * Menggunakan **HTTP 307 Temporary Redirect** (atau **302 Found**). 
   * *Catatan*: **Hindari HTTP 301 Permanent Redirect** pada browser client agar browser tidak melakukan *caching* permanen di sisi klien, sehingga setiap interaksi klik tetap dapat dicatat oleh server analitik.
2. **Multi-Tier Caching Architecture**:
   * **Tier 1 (In-Memory Hot Cache)**: Penyimpanan lokal di memori aplikasi Go (menggunakan `sync.Map` teroptimasi atau library seperti `Ristretto` / `BigCache`) untuk 10.000 tautan paling sering diakses.
   * **Tier 2 (Distributed Cache)**: Redis Cluster dengan strategi *Cache-Aside* + TTL jitter (mengurangi risiko *cache stampede*).
   * **Tier 3 (Persistent Storage)**: PostgreSQL dengan partisi berbasis tanggal atau *hash sharding*.

### 3.3 Asynchronous Analytics Pipeline
Jalur pengalihan klien tidak boleh menunggu penulisan log analitik ke database.
1. **Telemetry Collector**:
   * Mengambil metadata dari HTTP Request Context:
     * Header `X-Forwarded-For` / `RemoteAddr`
     * Header `User-Agent`
     * Header `Referer`
     * Header `Accept-Language`
2. **In-Memory Channel Buffer & Worker Pool**:
   * Request data dimasukkan ke dalam Go buffered channel (`chan AnalyticEvent`, kapasitas buffer e.g., $100.000$).
   * Kumpulan worker Go routines membaca buffer secara *batch* (contoh: *flush* setiap 500 event atau setiap interval 500 ms) ke sink data (Redis Streams / ClickHouse).
3. **Aggregations & Metrics**:
   * Total Clicks.
   * Unique Visitors (dihitung menggunakan pendekatan probabilistic *HyperLogLog*).
   * Rincian Geografis (Negara, Kota).
   * Rincian Sistem Operasi, Browser, dan Tipe Perangkat (Desktop, Mobile, Tablet, Bot).
   * Analisis Perujuk (*Referrer Source*).

### 3.4 User Mode & Identity Management (JWT Authentication)

Sistem menerapkan arsitektur identitas hibrida dengan membagi akses ke dalam 2 mode:

```
                  ┌─────────────────────────────────────────┐
                  │           Client Request                │
                  └───────────────────┬─────────────────────┘
                                      │
                         Header 'Authorization'?
                                     / \
                                    /   \
                             (Ya)  /     \ (Tidak / Kosong)
                                  ▼       ▼
                    ┌──────────────────┐ ┌───────────────────┐
                    │    User Mode     │ │  Anonymous Mode   │
                    │ (Authenticated)  │ │      (Guest)      │
                    └─────────┬────────┘ └─────────┬─────────┘
                              │                    │
          ┌───────────────────┴───┐      ┌─────────┴─────────┐
          │ • Custom Alias OK     │      │ • Auto Shortcode  │
          │ • Rate: 1.000 req/min │      │ • Rate: 10 req/min│
          │ • Link Ownership      │      │ • No Edit/Delete  │
          │ • Dashboard & History │      │ • user_id = NULL  │
          └───────────────────────┘      └───────────────────┘
```

1. **Dual Operating Mode**:
   * **Anonymous Mode (Guest)**:
     * Pembuatan link cepat tanpa kewajiban mendaftar/login.
     * Kode pendek selalu digenerate otomatis (*random Base62*). Permintaan dengan `custom_alias` wajib ditolak dengan status `401 Unauthorized` atau `403 Forbidden`.
     * Tautan disimpan dengan `user_id = NULL`.
     * Batasan ketat: *rate limit* 10 request/menit per IP publik. Tautan tidak dapat diedit atau dihapus oleh guest.
   * **User Mode (Authenticated)**:
     * Pengguna terdaftar dan terautentikasi melalui JSON Web Token (JWT).
     * Berhak menentukan `custom_alias` untuk *branding* tautan.
     * *Rate limit* tinggi: 1.000 request/menit.
     * Tautan tercatat dengan referensi relasional `user_id` (UUIDv7).
     * Memiliki hak akses terhadap manajemen tautan pribadi (melihat daftar riwayat tautan, metrik agregat per akun, serta menonaktifkan/menghapus tautan miliknya).

2. **Kredensial & Autentikasi**:
   * **Registrasi Akun**: Menggunakan tiga atribut wajib:
     * `email`: Alamat email valid, unik (case-insensitive), maksimal 255 karakter.
     * `username`: Nama pengguna alfanumerik unik (case-insensitive), panjang 3–50 karakter (`^[a-zA-Z0-9_.]+$`).
     * `password`: Kata sandi dengan panjang minimal 8 karakter (maksimal 72 karakter untuk kompatibilitas bcrypt), wajib memenuhi kebijakan keamanan karakter.
   * **Password Hashing**: Kata sandi wajib di-hash menggunakan algoritma **bcrypt** (faktor *cost* minimal 10) atau **Argon2id** sebelum disimpan ke database. Dilarang keras menyimpan password dalam format *plain-text* atau enkripsi dua arah.
   * **Fleksibilitas Login (Identifier)**: Pengguna dapat melakukan login menggunakan kombinasi `email` + `password` ATAU `username` + `password` melalui satu field input `identifier`.

3. **Spesifikasi JSON Web Token (JWT)**:
   * **Standar**: RFC 7519 dengan algoritma penandatanganan kriptografis **HMAC-SHA256 (`HS256`)**.
   * **Token Claims Structure**:
     * `sub`: User ID (`UUIDv7` string).
     * `email`: Alamat email pengguna.
     * `username`: Nama pengguna.
     * `exp`: Unix timestamp kedaluwarsa token (default: 24 jam / 86.400 detik).
     * `iat`: Unix timestamp penerbitan token (*Issued At*).
     * `nbf`: Unix timestamp token mulai berlaku (*Not Before*).
   * **Protokol Transmisi**: Klien mengirimkan token pada HTTP Request Header:
     ```http
     Authorization: Bearer <JWT_ACCESS_TOKEN>
     ```
   * **Stateless Verification & Revocation**:
     * Verifikasi token dilakukan secara lokal oleh middleware tanpa query database pada setiap request.
     * Mekanisme pencabutan instan (*revocation/logout*) didukung melalui *Redis Token Blacklist* dengan TTL sesuai sisa masa berlaku token.

---

## 4. API Interface Specification

### 4.1 Register User
* **Endpoint:** `POST /api/v1/auth/register`
* **Headers:** `Content-Type: application/json`
* **Request Payload:**
```json
{
  "email": "developer@example.com",
  "username": "superdev",
  "password": "SecurePassword123!"
}
```
* **Response (201 Created):**
```json
{
  "success": true,
  "message": "User registered successfully",
  "data": {
    "id": "018f3a2b-8a71-7000-8000-000000000001",
    "email": "developer@example.com",
    "username": "superdev",
    "created_at": "2026-10-03T14:30:00Z"
  }
}
```
* **Error Response (400 Bad Request / 409 Conflict):**
```json
{
  "success": false,
  "message": "Email or username already exists",
  "errors": {
    "email": "email already registered"
  }
}
```

### 4.2 Login User
* **Endpoint:** `POST /api/v1/auth/login`
* **Headers:** `Content-Type: application/json`
* **Request Payload:**
```json
{
  "identifier": "developer@example.com", 
  "password": "SecurePassword123!"
}
```
*(Catatan: `identifier` dapat diisi email maupun username)*
* **Response (200 OK):**
```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "018f3a2b-8a71-7000-8000-000000000001",
      "email": "developer@example.com",
      "username": "superdev"
    }
  }
}
```

### 4.3 Get Current User Profile
* **Endpoint:** `GET /api/v1/auth/me`
* **Headers:** `Authorization: Bearer <JWT_TOKEN>`
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "id": "018f3a2b-8a71-7000-8000-000000000001",
    "email": "developer@example.com",
    "username": "superdev",
    "created_at": "2026-10-03T14:30:00Z"
  }
}
```

### 4.4 Get User's Created Links (Protected)
* **Endpoint:** `GET /api/v1/user/links?page=1&limit=20`
* **Headers:** `Authorization: Bearer <JWT_TOKEN>`
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "total": 42,
    "page": 1,
    "limit": 20,
    "links": [
      {
        "id": "018f3a2b-8a71-7000-8000-000000000099",
        "short_code": "spring-sale",
        "short_url": "https://s.id/spring-sale",
        "original_url": "https://example.com/products/spring-sale",
        "is_active": true,
        "created_at": "2026-10-03T14:30:00Z",
        "expires_at": "2026-10-10T14:30:00Z"
      }
    ]
  }
}
```

### 4.5 Create Short URL (Anonymous & Authenticated)
* **Endpoint:** `POST /api/v1/links`
* **Headers:** `Content-Type: application/json`, `Authorization: Bearer <JWT_TOKEN>` *(Opsional bagi anonymous, Wajib jika menyertakan `custom_alias`)*
* **Request Payload (Dengan Custom Alias - Memerlukan Auth):**
```json
{
  "original_url": "https://example.com/products/electronics/item-990214?ref=campaign",
  "custom_alias": "spring-sale",
  "expires_in_hours": 168
}
```
* **Request Payload (Anonymous - Tanpa Custom Alias):**
```json
{
  "original_url": "https://example.com/products/electronics/item-990214"
}
```
* **Response (201 Created):**
```json
{
  "success": true,
  "data": {
    "short_code": "spring-sale",
    "short_url": "https://s.id/spring-sale",
    "original_url": "https://example.com/products/electronics/item-990214?ref=campaign",
    "created_at": "2026-10-03T14:30:00Z",
    "expires_at": "2026-10-10T14:30:00Z"
  }
}
```

### 4.6 Redirect URL
* **Endpoint:** `GET /{short_code}`
* **Headers:** Standard HTTP headers
* **Response (307 Temporary Redirect):**
  * Header: `Location: https://example.com/...`
  * Header: `Cache-Control: private, max-age=60`

### 4.7 Get Analytics Data
* **Endpoint:** `GET /api/v1/links/{short_code}/analytics?interval=7d`
* **Headers:** Standard HTTP headers / Optional `Authorization: Bearer <JWT_TOKEN>`
* **Response (200 OK):**
```json
{
  "short_code": "spring-sale",
  "total_clicks": 45892,
  "unique_clicks": 38102,
  "top_countries": [
    { "code": "ID", "clicks": 32000 },
    { "code": "SG", "clicks": 8000 }
  ],
  "devices": {
    "mobile": 72.4,
    "desktop": 24.1,
    "bot": 3.5
  },
  "timeseries": [
    { "timestamp": "2026-10-03T00:00:00Z", "count": 1250 }
  ]
}
```

---

## 5. Database Schema & Storage Design

### 5.1 Relational Schema (PostgreSQL)

Seluruh entitas menggunakan **UUIDv7** sebagai *Primary Key* guna menjamin keterurutan data berdasarkan waktu (*time-ordered indexing*), memangkas *index fragmentation* pada B-Tree PostgreSQL, dan memfasilitasi pembuatan ID di sisi aplikasi tanpa lock terpusat.

```sql
-- 1. Users Table
CREATE TABLE users (
    id UUID PRIMARY KEY, -- UUIDv7
    email VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_users_email_lower ON users (LOWER(email));
CREATE UNIQUE INDEX idx_users_username_lower ON users (LOWER(username));

-- 2. Links Table
CREATE TABLE links (
    id UUID PRIMARY KEY, -- UUIDv7
    short_code VARCHAR(32) NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_links_short_code ON links(short_code);
CREATE INDEX idx_links_user_id ON links(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_links_expires_at ON links(expires_at) WHERE expires_at IS NOT NULL;
```

### 5.2 Time-Series Analytics Schema (ClickHouse / TimescaleDB)

```sql
CREATE TABLE click_events (
    short_code LowCardinality(String),
    clicked_at DateTime64(3, 'UTC'),
    ip_hash FixedString(32),
    country_code LowCardinality(FixedString(2)),
    city String,
    device_type LowCardinality(String),
    browser LowCardinality(String),
    os LowCardinality(String),
    referer String
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(clicked_at)
ORDER BY (short_code, clicked_at);
```

---

## 6. Comprehensive Security Specifications

### 6.1 SSRF (Server-Side Request Forgery) & Safe Browsing
URL shortener kerap dijadikan tameng untuk memicu serangan internal ke infrastruktur backend (*metadata services, private subnets*).
1. **URL Scheme Whitelisting**:
   * Hanya terima skema `http://` dan `https://`.
   * Tolak secara mutlak skema: `ftp://`, `file://`, `data:`, `javascript:`, `gopher://`, `dict://`.
2. **Blacklist Private IP & Localhost Resolution**:
   * Sebelum menyimpan tautan, sistem wajib melakukan resolusi DNS (`net.LookupIP`) terhadap domain target.
   * Blokir tautan jika IP domain ter-resolve ke segmen privat/lokal:
     * `127.0.0.0/8` (Loopback)
     * `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` (Private RFC 1918)
     * `169.254.0.0/16` (Link-Local / AWS Metadata `169.254.169.254`)
     * `::1`, `fc00::/7` (IPv6 loopback & ULA)
3. **Loop Redirection Protection**:
   * Larang pendaftaran domain penyingkat itu sendiri sebagai target tautan guna menghindari *infinite loop*.
4. **Malicious Domain Check**:
   * Integrasi asynchronous dengan *Google Safe Browsing API* atau *Cloudflare Radar API*. Tautan ditandai `is_flagged` jika masuk database malware/phishing.

### 6.2 Rate Limiting & DoS Mitigation
* **Algoritma**: Token Bucket atau Sliding-Window Counter menggunakan Redis.
* **Batas Pemanggilan per Kategori**:
  * `POST /api/v1/links`:
    * Anonymous Mode (Guest): Maksimal 10 request/menit per IP.
    * User Mode (Authenticated): Maksimal 1000 request/menit per User ID.
  * `POST /api/v1/auth/login`: Maksimal 5 percobaan gagal per 5 menit per IP (mitigasi *brute-force* dan *credential stuffing*).
  * `POST /api/v1/auth/register`: Maksimal 3 registrasi per jam per IP (mitigasi *mass bot creation*).
  * `GET /{short_code}`: 100 request/detik per subnet `/24` (IPv4) untuk mencegah pemindaian brute-force kode tautan.
* **Shortcode Entropy Protection**:
  * Tautan anonim acak memiliki entropy minimal $62^7$.
  * Terapkan *exponential backoff* jika client menerima berturut-turut status `404 Not Found` (mendeteksi aktivitas scanning).

### 6.3 Input Validation & Sanitization
* **Max Length Restrictions**:
  * Batasi panjang URL asli maksimal 2048 karakter.
  * Batasi panjang custom alias maksimal 32 karakter.
* **Regex Sanitization**:
  * Custom alias divalidasi ketat dengan pola: `^[a-zA-Z0-9_-]{4,32}$`.
  * Username divalidasi dengan pola: `^[a-zA-Z0-9_.]{3,50}$`.
  * Format email wajib mematuhi standar RFC 5322.

### 6.4 Data Privacy & Compliance (GDPR / UU PDP)
* **IP Anonymization**:
  * Alamat IP asli pengguna yang melakukan klik dilarang disimpan secara mentah (*plain-text*) pada basis data analitik.
  * Hashing menggunakan SHA-256 dipadu dengan *rotating daily secret salt*:
    $$\text{AnonIP} = \text{HMAC-SHA256}(\text{IP}, \text{Salt}_{\text{day}})$$
  * Atau nol-kan blok terakhir (e.g. `192.168.1.120` $\rightarrow$ `192.168.1.0`).

### 6.5 Web App Hardening
* Menyematkan HTTP Security Headers pada seluruh endpoint:
  * `X-Content-Type-Options: nosniff`
  * `X-Frame-Options: DENY`
  * `Content-Security-Policy: default-src 'none'`
  * `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload`
  * `X-XSS-Protection: 1; mode=block`
  * `Referrer-Policy: strict-origin-when-cross-origin`

### 6.6 JWT & Credential Hardening
1. **Password Policy**:
   * Panjang minimal 8 karakter, maksimal 72 karakter.
   * Wajib memadukan minimal 1 huruf besar, 1 huruf kecil, dan 1 angka.
2. **JWT Secret Management**:
   * Secret key penandatanganan JWT wajib memiliki entropi minimal 256-bit (32 byte acak).
   * Disediakan melalui *environment variable* (`JWT_SECRET`) dan dilarang keras di-hardcode dalam kode sumber.
3. **Token Blacklisting**:
   * Endpoint logout mencatat token yang dicabut ke dalam Redis dengan format key `blacklist:jwt:<token_hash>` dengan TTL sama dengan sisa waktu expired token tersebut.

---

## 7. Performance Engineering in Go

### 7.1 Memory Allocations & Zero-Copy Operations
* Manfaatkan `sync.Pool` untuk alokasi objek JSON encoder/decoder dan buffer analitik guna mengurangi tekanan pada *Garbage Collector* (GC).
* Validasi token JWT secara stateless dalam middleware tanpa melakukan query database berulang pada setiap siklus request.

### 7.2 Database & Cache Connection Pooling
* Konfigurasi koneksi GORM & Database Pool:
  * `MaxConns`: Dikalibrasi berdasarkan rumus:
    $$\text{MaxConns} = (\text{CPU Cores} \times 2) + \text{Disk Spindle Count}$$
  * `MinConns`: Minimal 20% dari kapasitas maksimum untuk menghindari jeda saat *cold start*.
  * `MaxConnIdleTime`: 15 Menit.
* Konfigurasi Redis:
  * Pipa (*Pipelining*) untuk update metrik klik.
  * Redis `MGET` jika melakukan fetch batch.

### 7.3 Concurrency & Graceful Shutdown
* Penanganan sinyal OS (`SIGINT`, `SIGTERM`) dengan `context.WithTimeout`:
  1. Hentikan penerimaan request HTTP baru.
  2. Tunggu koneksi aktif selesai (*grace period* 10 detik).
  3. Flush semua buffered channel analitik yang tersisa ke database penyimpanan.
  4. Tutup pool koneksi database dan Redis.

---

## 8. Development Roadmap & Phasing

```
[ Phase 1: MVP Core & Auth ] ──► [ Phase 2: Analytics & Concurrency ] ──► [ Phase 3: Enterprise Hardening ]
  • UUIDv7 + Base62 Engine         • Buffered Channel Worker Pool         • SSRF & Safe Browsing Integration
  • User Auth (JWT, email, pwd)    • MaxMind GeoIP Engine                 • Redis Token Blacklist & Sliding Rate Limiter
  • Dual Mode (Guest vs User)      • ClickHouse Data Partitioning         • Load Testing via k6 (10k RPS target)
  • GORM + Redis Cache-Aside       • User Link Management API             • Automated Geo/Device Aggregation
```

1. **Phase 1: Foundation, User Mode & Core Link Engine (Minggu 1-2)**
   * Setup proyek Go Clean Architecture dengan Fiber v3 & GORM.
   * Model relasional: `users` dan `links` (UUIDv7 primary keys).
   * Modul autentikasi: Register, Login (email/username + password), dan JWT middleware.
   * Dual mode: Anonymous link creation vs Authenticated custom alias.
   * Persistence layer (PostgreSQL + Redis caching layer).
   * Unit tests & integrasi endpoint pengalihan (HTTP 307).
2. **Phase 2: Analytics Engine & Concurrency (Minggu 3-4)**
   * Implementasi asynchronous worker pool untuk event logging.
   * Integrasi geo-ip parser & user-agent parser.
   * Setup ClickHouse/TimescaleDB untuk query agregasi metrik.
   * Endpoint dashboard pengguna (`GET /api/v1/user/links`).
3. **Phase 3: Security & Benchmarking (Minggu 5)**
   * Implementasi SSRF DNS resolver validator.
   * Setup sliding-window rate limiting middleware (Anonymous vs User limits).
   * Token revocation dengan Redis blacklist.
   * Load testing komprehensif menggunakan `k6` atau `wrk` dengan target beban 10.000 RPS.