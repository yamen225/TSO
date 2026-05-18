# TSO Activation Service

A Go microservice for Transmission System Operator (TSO) electrical asset activation. Given a target power volume (in kW) and a date, the service selects which electrical assets to activate at minimum cost using four different optimization strategies.

Built with **Hexagonal Architecture** and strict **Test-Driven Development (TDD)**.

---

## Table of Contents

- [Problem Domain](#problem-domain)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Domain Models](#domain-models)
- [Activation Strategies](#activation-strategies)
- [API Reference](#api-reference)
- [Database Schema](#database-schema)
- [Getting Started](#getting-started)
- [Running Tests](#running-tests)
- [Swagger UI](#swagger-ui)
- [Design Decisions](#design-decisions)

---

## Problem Domain

A TSO manages a portfolio of electrical assets (generators, batteries, etc.), each with:
- A fixed activation cost (paid regardless of how much power is actually drawn)
- A capacity in kW
- A `price_per_kw` ratio (fixed_cost ÷ capacity_kw) — a measure of cost efficiency
- An availability date (the date on which the asset can be activated)

When the grid needs a certain volume of power on a given date, the TSO must select a subset of available assets whose **combined capacity meets or exceeds the target**, while **minimizing total cost**.

This service implements and compares four strategies for solving that selection problem.

---

## Architecture

The service follows **Hexagonal Architecture** (also known as Ports & Adapters), which isolates the core business logic from I/O concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP Client                          │
└─────────────────────────┬───────────────────────────────────┘
                          │ POST /api/v1/activation/*
┌─────────────────────────▼───────────────────────────────────┐
│              Application Layer  (internal/app)              │
│         Gin HTTP handlers — Inbound/Driving Adapter         │
│  Translates HTTP ↔ domain types; calls ActivationUseCase    │
└─────────────────────────┬───────────────────────────────────┘
                          │ request.ActivationUseCase interface
┌─────────────────────────▼───────────────────────────────────┐
│               Domain Layer  (internal/domain)               │
│  Models, Ports (interfaces), Services (business logic)      │
│            ZERO external dependencies                       │
└─────────────────────────┬───────────────────────────────────┘
                          │ obtain.AssetRepository interface
┌─────────────────────────▼───────────────────────────────────┐
│           Infrastructure Layer  (internal/infra)            │
│       PostgreSQL adapter — Outbound/Driven Adapter          │
│   Executes SQL, maps rows → domain models, hides pgx        │
└─────────────────────────┬───────────────────────────────────┘
                          │
                    ┌─────▼──────┐
                    │ PostgreSQL │
                    └────────────┘
```

### Layer Rules

| Layer | Package | Can import | Cannot import |
|---|---|---|---|
| Domain | `internal/domain` | stdlib only | Gin, pgx, anything external |
| Application | `internal/app` | domain, Gin | pgx, infra |
| Infrastructure | `internal/infra` | domain, GORM | Gin, app |
| Entry point | `cmd/server` | all layers | — |

---

## Project Structure

```
activation-service/
├── cmd/
│   └── server/
│       └── main.go                  # Entry point — wires all layers
├── internal/
│   ├── domain/
│   │   ├── model/
│   │   │   └── asset.go             # Core structs: Asset, ActivationRequest, AllocationResult
│   │   ├── ports/
│   │   │   ├── request/
│   │   │   │   └── activation.go    # Inbound port: ActivationUseCase interface
│   │   │   └── obtain/
│   │   │       └── asset_repo.go    # Outbound port: AssetRepository interface
│   │   └── service/
│   │       ├── helpers.go           # Shared: greedySelect, knapsackSelect algorithms
│   │       ├── greedy_baseline.go   # Strategy 1: in-memory filter + greedy pick
│   │       ├── greedy_db.go         # Strategy 2: DB-sorted + greedy pick
│   │       ├── knapsack_memory.go   # Strategy 3: in-memory filter + 0/1 DP
│   │       ├── knapsack_db.go       # Strategy 4: DB-pruned pool + 0/1 DP
│   │       └── strategy_test.go     # TDD tests for all 4 strategies
│   ├── infra/
│   │   └── repo/
│   │       ├── postgres_asset.go    # PostgreSQL outbound adapter
│   │       └── postgres_asset_test.go # SQL-level tests using go-sqlmock + GORM
│   └── app/
│       └── handler/
│           ├── activation.go        # Gin HTTP handlers with Swagger annotations
│           └── activation_test.go   # HTTP handler tests using httptest
├── docs/                            # Generated Swagger/OpenAPI artifacts (swag init)
├── Dockerfile                       # Multi-stage build (golang:1.26 → alpine:3.19)
├── docker-compose.yml               # db (postgres:15) + web services
├── Makefile                         # build, test, swagger, docker-up/down targets
├── init.sql                         # DB schema + seed data
├── go.mod
└── go.sum
```

---

## Domain Models

Defined in [internal/domain/model/asset.go](internal/domain/model/asset.go) with zero external imports.

### `Asset`
Represents one activatable electrical asset.

| Field | Type | Description |
|---|---|---|
| `ID` | `int` | Database primary key |
| `Name` | `string` | Human-readable name (e.g. "Alpha") |
| `CapacityKW` | `int` | Maximum power output in kW |
| `FixedCost` | `float64` | Cost to activate this asset (currency units) |
| `PricePerKW` | `float64` | Efficiency ratio: `FixedCost / CapacityKW` |
| `AvailDate` | `string` | Date available, format `YYYY-MM-DD` |

### `ActivationRequest`
The inbound JSON payload sent by API clients.

| Field | Type | Description |
|---|---|---|
| `date` | `string` | Target activation date (`YYYY-MM-DD`) |
| `target_volume_kw` | `int` | Minimum total capacity required |

### `AllocationResult`
The response returned by all four strategies.

| Field | Type | Description |
|---|---|---|
| `selected_assets` | `[]Asset` | The chosen subset of assets |
| `total_capacity_kw` | `int` | Sum of selected assets' capacity |
| `total_cost` | `float64` | Sum of selected assets' fixed cost |

---

## Activation Strategies

All four strategies implement the same `ActivationUseCase` interface:

```go
type ActivationUseCase interface {
    Execute(req model.ActivationRequest) (model.AllocationResult, error)
}
```

They differ in **where filtering/sorting happens** and **which selection algorithm** they use.

### Strategy 1 — Greedy Baseline (`greedy_baseline.go`)

**Where:** in-memory  
**Algorithm:** greedy sequential pick

1. Calls `repo.FetchAll()` — loads all assets from the database
2. Filters in memory: keeps only assets whose `AvailDate` matches the request date
3. Sorts the filtered list by `PricePerKW` ascending (cheapest per kW first)
4. Picks assets one by one until `TargetVolumeKW` is met

**Use case:** Simple baseline. Good when the asset pool is small or DB-side filtering is not available.

---

### Strategy 2 — Greedy DB (`greedy_db.go`)

**Where:** database  
**Algorithm:** greedy sequential pick

1. Calls `repo.FetchPruned(date, volume)` — the DB returns assets already filtered by date and sorted by `price_per_kw ASC`
2. Picks assets sequentially until `TargetVolumeKW` is met

**Use case:** Delegates the heavy lifting to PostgreSQL's indexed scan. More efficient than Strategy 1 when the asset pool is large.

---

### Strategy 3 — Knapsack Memory (`knapsack_memory.go`)

**Where:** in-memory  
**Algorithm:** 0/1 bottom-up dynamic programming

1. Calls `repo.FetchAll()` — loads all assets
2. Filters in memory by availability date
3. Runs the **0/1 knapsack DP** to find the subset with **minimum total FixedCost** that meets `TargetVolumeKW`

**Use case:** Optimal cost result. Greedy can miss cheaper combinations; DP finds the true minimum. Higher computational cost (O(n × maxCap)).

---

### Strategy 4 — Knapsack DB (`knapsack_db.go`)

**Where:** database (pre-pruned)  
**Algorithm:** 0/1 bottom-up dynamic programming

1. Calls `repo.FetchPruned(date, volume)` — DB returns a smaller candidate pool (assets with capacity ≤ `target × 1.5`)
2. Runs the same 0/1 DP on this reduced set

**Use case:** Best of both worlds — the DB reduces the search space, then DP finds the true cost minimum within that reduced pool.

---

### Algorithm Details: 0/1 Knapsack DP (`helpers.go`)

The problem: choose a subset of assets with total capacity ≥ `targetKW`, minimizing total `FixedCost`.

**DP state:** `dp[c]` = minimum fixed cost achievable with exactly `c` kW of capacity selected.

**Transitions:** For each asset `a` (iterating in reverse to enforce 0/1):
```
if dp[c - a.CapacityKW] + a.FixedCost < dp[c]:
    dp[c] = dp[c - a.CapacityKW] + a.FixedCost
    chosen[c] = index of a
```

**Answer:** The minimum over all `dp[c]` where `c ≥ targetKW`. Backtracks `chosen[]` to reconstruct which assets were selected.

Returns `ErrVolumeUnattainable` if total available capacity is less than `targetKW`.

---

## API Reference

Base URL: `http://localhost:8080`

All endpoints accept `POST` requests with a JSON body and return JSON.

### Request Body (all endpoints)

```json
{
  "date": "2025-06-01",
  "target_volume_kw": 250
}
```

### Response Body (success `200`)

```json
{
  "selected_assets": [
    {
      "ID": 4,
      "Name": "Delta",
      "CapacityKW": 300,
      "FixedCost": 900.00,
      "PricePerKW": 3.0,
      "AvailDate": "2025-06-01"
    }
  ],
  "total_capacity_kw": 300,
  "total_cost": 900.00
}
```

### Error Responses

| Status | When |
|---|---|
| `400 Bad Request` | Malformed JSON or missing required fields |
| `422 Unprocessable Entity` | Valid request but target volume cannot be met |

### Endpoints

| Method | Path | Strategy |
|---|---|---|
| `POST` | `/api/v1/activation/greedy-baseline` | Greedy, in-memory sort |
| `POST` | `/api/v1/activation/greedy-db` | Greedy, DB-sorted |
| `POST` | `/api/v1/activation/knapsack-memory` | 0/1 DP, in-memory filter |
| `POST` | `/api/v1/activation/knapsack-db` | 0/1 DP, DB-pruned pool |

---

## Database Schema

Defined in [init.sql](init.sql).

### `assets` table

| Column | Type | Notes |
|---|---|---|
| `id` | `SERIAL` | Primary key |
| `name` | `VARCHAR(255)` | Asset name |
| `capacity_kw` | `INT` | Power capacity |
| `fixed_cost` | `NUMERIC(12,2)` | Activation cost |
| `price_per_kw` | `NUMERIC(10,4)` | **Generated column**: `fixed_cost / capacity_kw` — stored and indexed |

### `asset_availabilities` table

| Column | Type | Notes |
|---|---|---|
| `id` | `SERIAL` | Primary key |
| `asset_id` | `INT` | FK → `assets.id` |
| `avail_date` | `DATE` | Date the asset is available |

`UNIQUE(asset_id, avail_date)` prevents duplicate availability entries.

### Indexes

```sql
-- Supports the capacity_kw filter + price_per_kw ORDER BY in FetchPruned.
CREATE INDEX idx_assets_capacity_cost
    ON assets (capacity_kw, price_per_kw ASC);

-- avail_date leads so the FetchPruned date-equality filter hits this index
-- rather than the UNIQUE(asset_id, avail_date) index whose column order is wrong.
CREATE INDEX idx_availability_date_asset
    ON asset_availabilities (avail_date, asset_id);
```

### `FetchPruned` capacity cap

The infrastructure adapter limits candidates to assets with `capacity_kw ≤ target × 1.5`. This prunes assets far too large to be cost-effective while leaving headroom for the DP algorithm to find valid combinations.

---

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- Go 1.26+ (for local builds/tests without Docker)
- [swag](https://github.com/swaggo/swag) CLI (for regenerating Swagger docs)

### Run with Docker

```bash
# Build images, start PostgreSQL + the Go service
make docker-up

# Verify both containers are healthy
docker compose ps

# Stop and remove containers + volumes
make docker-down
```

The service will be available at `http://localhost:8080`.

### Run Locally (without Docker)

```bash
# Start a PostgreSQL instance and export its connection string
export DATABASE_URL="postgres://activation:activation@localhost:5432/activation?sslmode=disable"

# Apply the schema
psql $DATABASE_URL < init.sql

# Build and run
make build
./activation-service
```

### Example Request

```bash
curl -s -X POST http://localhost:8080/api/v1/activation/knapsack-db \
  -H "Content-Type: application/json" \
  -d '{"date":"2025-06-01","target_volume_kw":250}' | jq
```

---

## Running Tests

The full test suite covers all three layers with **18 tests**:

```bash
make test
# or
go test -v ./...
```

| Package | Tests | What is tested |
|---|---|---|
| `internal/domain/service` | 10 | All 4 strategies: correct asset selection, cost minimization, edge cases (unattainable volume, repo errors) |
| `internal/infra/repo` | 2 | SQL query correctness using `go-sqlmock` + GORM — verifies `ORDER BY`, `WHERE` clauses |
| `internal/app/handler` | 6 | HTTP binding, 200/400/422 status codes, JSON response shape |

### Test Strategy (TDD)

Each layer was built Red → Green:

1. **Red:** Write the test file first, asserting against an interface or mock. Tests fail because no implementation exists.
2. **Green:** Implement the minimum code to make all tests pass.

Mocking strategy per layer:
- **Domain tests:** A plain `mockAssetRepo` struct implementing `obtain.AssetRepository` directly in the test file — no mock library needed
- **Infrastructure tests:** `go-sqlmock` injects a mock `*sql.DB` into GORM via `postgres.Config{Conn: sqlDB}`, allowing assertion of exact queries
- **Handler tests:** A `mockUseCase` struct implementing `request.ActivationUseCase` in the test file; uses `net/http/httptest` for HTTP simulation

---

## Swagger UI

API documentation is served at runtime via [gin-swagger](https://github.com/swaggo/gin-swagger).

```
http://localhost:8080/swagger/index.html
```

To regenerate the Swagger docs after changing handler annotations:

```bash
make swagger
# equivalent to: swag init -g ./cmd/server/main.go -o docs
```

This produces `docs/docs.go`, `docs/swagger.json`, and `docs/swagger.yaml`.

---

## Design Decisions

### Why four strategies?

The four strategies represent a 2×2 matrix of trade-offs:

|  | In-memory filter | DB-side filter |
|---|---|---|
| **Greedy** | `greedy-baseline` | `greedy-db` |
| **Optimal DP** | `knapsack-memory` | `knapsack-db` |

- **Greedy vs DP:** Greedy is O(n log n) but can return a suboptimal cost. DP guarantees minimum cost but is O(n × C) where C is total capacity.
- **In-memory vs DB:** Fetching all assets is simpler but loads the full table. DB pruning reduces data transfer and leverages the index, at the cost of a more complex query.

### Why a generated column for `price_per_kw`?

`price_per_kw` is stored as a PostgreSQL `GENERATED ALWAYS AS` column rather than computed in application code. This means:
- The value is always consistent with `fixed_cost` and `capacity_kw`
- It can be indexed for efficient `ORDER BY` without extra application logic
- The application never needs to update it manually

### Why interface injection everywhere?

`ActivationUseCase` and `AssetRepository` are interfaces, not concrete types. Every layer depends only on the interface:
- Domain services receive `obtain.AssetRepository` — they never see `pgxpool`
- HTTP handlers receive `request.ActivationUseCase` — they never see service internals
- This makes every layer independently testable with a plain mock struct

### Why GORM with the fluent builder instead of raw SQL?

The infrastructure adapter uses **GORM's fluent query builder** (`.Table().Select().Joins().Where().Order().Scan()`) rather than raw SQL strings:
- Parameterised placeholders (`?`) are handled automatically, eliminating manual SQL injection risk
- The join and column list are extracted into named constants (`selectColumns`, `joinClause`), making the two queries (`FetchAll`, `FetchPruned`) differ only by their `Where` clause
- `gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), ...)` lets `go-sqlmock` inject a mock `*sql.DB` directly, so tests assert against real GORM-generated SQL without needing a live database
