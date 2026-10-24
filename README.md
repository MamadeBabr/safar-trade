# Production-Style Go REST API & Financial Data Ingestion

A clean, production-grade REST API backend and financial event ingestion system built in Go for developers coming from **Node.js (Express / Hono / Fastify)**. This project follows idiomatic Go patterns, clean architecture, and the Go standard library, avoiding heavy web frameworks while maintaining enterprise-grade structure, observability, and testing.

---

## 🛠 Tech Stack & Ingestion Pipeline

- **Language**: Go 1.26
- **Routing**: Standard Library `net/http` with Go 1.22+ `http.ServeMux` (method + `{param}` pattern matching)
- **Database**: PostgreSQL with `pgx/v5` connection pool
- **Type-safe SQL**: `sqlc` (generates type-safe Go code directly from raw SQL)
- **Migrations**: `goose` (raw SQL migrations in `/migrations`)
- **TLS Fingerprint Impersonation**: `bogdanfinn/tls-client` (pure Go curl-impersonate alternative matching browser TLS ClientHello, ciphers, curves, and HTTP/2 settings to bypass bot protection / Cloudflare)
- **Validation**: `go-playground/validator/v10`
- **Logging**: Standard Library `log/slog` (structured JSON / text logging)
- **Zero Frameworks**: No Gin, Echo, Fiber, or Chi — pure standard library HTTP

---

## 📁 Project Structure

```
.
├── cmd/
│   └── api/
│       └── main.go              # Pure wiring: config, DB pool, router, middleware, background workers, shutdown
├── internal/                    # Private application code (Go prevents external modules from importing this)
│   ├── config/
│   │   └── config.go            # Strongly-typed environment configuration with defaults & stdlib .env parser
│   ├── ingestion/               # Financial data ingestion layer
│   │   └── forexfactory/
│   │       ├── client.go        # TLS-fingerprint HTTP client (Firefox profile, retries, headers)
│   │       ├── ingester.go      # Orchestrator: fetches, converts timezone to Tehran, batch upserts to DB & JSON
│   │       ├── ingester_test.go # Unit tests with mock client & repo
│   │       └── time_helper.go   # Tehran timezone formatter (+03:30)
│   ├── scheduler/               # Background task scheduling
│   │   └── scheduler.go         # Periodic ingestion worker running in a managed goroutine
│   ├── handler/                 # HTTP transport layer
│   │   ├── ingestion_handler.go # Endpoints to trigger ingestion and query economic events
│   │   ├── ingestion_handler_test.go # Tests for ingestion endpoints
│   │   ├── note_handler.go      # CRUD route handlers using Go 1.22+ ServeMux
│   │   ├── note_handler_test.go # HTTP unit tests using net/http/httptest
│   │   └── response.go          # Consistent JSON response & error envelope helpers
│   ├── service/                 # Business logic layer
│   │   ├── note_service.go      # NoteService interface & domain business rules
│   │   └── note_service_test.go # Table-driven unit tests with repository mock
│   ├── repository/              # Data access layer
│   │   ├── economic_event_repository.go # Batch upsert, list, and get operations for economic events
│   │   ├── note_repository.go   # NoteRepository interface & pgx/sqlc implementation
│   │   └── repository.go        # Shared sentinel errors (ErrNotFound, ErrConflict)
│   ├── model/
│   │   ├── economic_event.go    # ForexFactory API models, EconomicEvent entity, and DTOs
│   │   └── note.go              # Note domain entities, request/response DTOs, and validation tags
│   └── middleware/              # HTTP middleware chain
│       ├── middleware.go        # Middleware type and Chain helper
│       ├── request_id.go        # Generates & propagates X-Request-ID (crypto/rand)
│       ├── logger.go            # Structured HTTP request logging with duration and status
│       ├── recoverer.go         # Recovers from panics and returns 500 JSON without crashing
│       └── cors.go              # Cross-Origin Resource Sharing & preflight handler
├── db/
│   ├── queries/                 # Raw SQL queries annotated for sqlc
│   │   ├── economic_events.sql
│   │   ├── notes.sql
│   │   └── users.sql
│   ├── sqlc.yaml                # sqlc codegen configuration
│   ├── db.go                    # sqlc DBTX interface & Queries struct for pgx/v5
│   ├── models.go                # sqlc generated models
│   ├── economic_events.sql.go   # sqlc generated economic event queries
│   ├── notes.sql.go             # sqlc generated note queries
│   └── users.sql.go             # sqlc generated user queries
├── migrations/                  # Goose migration files
│   ├── 00001_create_users_table.sql
│   ├── 00002_create_notes_table.sql
│   └── 00003_create_economic_events_table.sql
├── Makefile                     # Developer tasks: run, build, test, migrations, sqlc
├── docker-compose.yml           # PostgreSQL 16 container for local development
├── Dockerfile                   # Multi-stage production container build (< 20MB final image)
├── .env.example                 # Sample environment variables
├── .gitignore                   # Git ignore file
└── README.md                    # Documentation & Node-to-Go guide
```

---

## 🚀 Quick Start (Zero to Running)

### 1. Prerequisites
- **Go**: 1.24+ (tested on Go 1.26+)
- **Docker & Docker Compose** (for local PostgreSQL)
- *(Optional CLI tools)*: `goose` and `sqlc`

Install `goose` and `sqlc` CLI tools via Go:
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

### 2. Start PostgreSQL
```bash
docker compose up -d
```

### 3. Run Migrations
Using `make`:
```bash
make migrate-up
```
Or directly via `goose`:
```bash
goose -dir migrations postgres "postgres://postgres:postgres@localhost:5432/safar_trade?sslmode=disable" up
```

### 4. Start the Application
Using `make`:
```bash
make run
```
Or directly via `go`:
```bash
go run ./cmd/api
```

The server starts on `http://localhost:8080`. On boot, the background scheduler runs an initial Forex Factory ingestion cycle (if `ENABLE_INGESTION_SCHEDULER=true`) and repeats every `INGESTION_INTERVAL`.

---

## 📡 Forex Factory Ingestion & Economic Events API

### 1. Trigger On-Demand Ingestion
```bash
curl -X POST http://localhost:8080/api/v1/ingestion/forex-factory
```
**Response (200 OK):**
```json
{
  "status": "success",
  "total_days": 7,
  "total_events": 48,
  "upserted": 48,
  "duration_ms": 1120
}
```
*Note: Also saves a local JSON dump to `forexfactory.json`.*

### 2. List Economic Events
```bash
curl -X GET "http://localhost:8080/api/v1/economic-events?limit=10&offset=0"
```
**Response (200 OK):**
```json
{
  "count": 10,
  "limit": 10,
  "offset": 0,
  "data": [
    {
      "id": 141940,
      "name": "CPI m/m",
      "country": "USD",
      "currency": "USD",
      "impact_name": "High",
      "actual": "0.3%",
      "previous": "0.2%",
      "forecast": "0.3%",
      "revision": "",
      "date": "2026-10-27",
      "url": "/calendar/141940-cpi-m-m",
      "time_label": "16:00",
      "dateline": 1729864800000,
      "created_at": "2026-10-25T00:25:00Z",
      "updated_at": "2026-10-25T00:25:00Z"
    }
  ]
}
```

### 3. Get Economic Event by ID
```bash
curl -X GET http://localhost:8080/api/v1/economic-events/141940
```

---

## 📡 Notes CRUD Endpoints

### 1. Health Check
```bash
curl -X GET http://localhost:8080/healthz
```

### 2. Create Note
```bash
curl -X POST http://localhost:8080/api/v1/notes \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Forex Ingestion Implemented",
    "content": "Uses tls-client with Firefox fingerprint.",
    "category": "trading"
  }'
```

### 3. List Notes
```bash
curl -X GET "http://localhost:8080/api/v1/notes?limit=10&offset=0"
```

---

## 🧪 Testing

Run all unit and handler tests across the whole project:
```bash
go test -v ./...
```

Run tests with code coverage:
```bash
go test -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

- **Forex Factory Ingestion Tests**: [`internal/ingestion/forexfactory/ingester_test.go`](file:///c:/Users/Hossen/Desktop/coding/safar-trade/internal/ingestion/forexfactory/ingester_test.go) tests client parsing, dateline conversion to epoch milliseconds, Tehran time mapping, and repository batch upserting.
- **Ingestion HTTP Handler Tests**: [`internal/handler/ingestion_handler_test.go`](file:///c:/Users/Hossen/Desktop/coding/safar-trade/internal/handler/ingestion_handler_test.go) verifies HTTP responses for triggering ingestion and querying economic events.
- **Service Layer Tests**: [`internal/service/note_service_test.go`](file:///c:/Users/Hossen/Desktop/coding/safar-trade/internal/service/note_service_test.go) uses table-driven testing with mock repository.
- **Handler Layer Tests**: [`internal/handler/note_handler_test.go`](file:///c:/Users/Hossen/Desktop/coding/safar-trade/internal/handler/note_handler_test.go) uses `net/http/httptest`.

---

## 💡 "Go for Node Developers" Cheat Sheet

Coming from **Node.js (Express, Hono, Fastify, TypeScript)**, here are the key conceptual shifts in Go:

### 1. TLS Fingerprint Impersonation: `impit` / `curl-impersonate` vs `tls-client`
| Node.js / Bun (`forex-factory.ts`) | Go (`internal/ingestion/forexfactory/client.go`) |
| :--- | :--- |
| `import { Impit } from 'impit';`<br>`const impit = new Impit({ browser: 'firefox', ignoreTlsErrors: true });` | `tls_client.NewHttpClient(..., tls_client.WithClientProfile(profiles.Firefox_120), tls_client.WithInsecureSkipVerify())` |
| Uses native C/Rust bindings or child processes under the hood. | `bogdanfinn/tls-client` uses pure Go uTLS & fhttp to mimic browser TLS ClientHello, ciphers, ALPN, and HTTP/2 pseudo-headers without CGO. |

### 2. Error Handling: Explicit vs Exceptions
| Node.js / TypeScript | Go |
| :--- | :--- |
| `try { const data = await fetch(); } catch (err) { ... }` | `data, err := fetch()`<br>`if err != nil { return nil, err }` |
| Sentinel errors checked with `instanceof`. | Sentinel errors: `var ErrNotFound = errors.New("...")`, checked via `errors.Is(err, repository.ErrNotFound)`. |

### 3. Interfaces: Implicit Duck Typing vs Explicit `implements`
| TypeScript | Go |
| :--- | :--- |
| `class PgRepo implements NoteRepository` | Structs satisfy interfaces **implicitly**. No `implements` keyword. Compile-time check: `var _ NoteRepository = (*pgNoteRepository)(nil)`. |

### 4. Concurrency: Goroutines & Channels vs Single-Threaded Event Loop
| Node.js / JavaScript | Go |
| :--- | :--- |
| Single-threaded event loop + `async/await`. | Goroutines (`go fn()`) are lightweight user-space green threads (~2KB). Channels and `sync.WaitGroup` coordinate concurrency. |
| `AbortController` (`signal: controller.signal`) | `ctx context.Context`: Passed as the 1st parameter to propagate cancellation, timeouts, and deadlines across goroutines. |

### 5. Resource Cleanup: `defer` vs `finally`
| Node.js / TypeScript | Go |
| :--- | :--- |
| `try { ... } finally { db.close(); }` | `defer dbPool.Close()` runs in LIFO order right before the enclosing function returns. |

---

## 🐳 Docker Deployment

Build and run the production image:
```bash
docker build -t safar-trade-api .
docker run -p 8080:8080 --env-file .env.example safar-trade-api
```
Produces a lightweight, static Alpine Linux container with non-root security (< 20 MB).
