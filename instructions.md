
**

# AI Agent Specification: Hexagonal TDD Implementation for TSO Activation

Objective: Implement a Go-based microservice for TSO electrical asset activation using strict Test-Driven Development (TDD) and Hexagonal Architecture.

Architectural Rules:

1. Domain Layer (internal/domain): Must contain Aggregate Roots, Models, Request Ports (Inbound), Obtain Ports (Outbound), and Domain Services (Calculation Logic). ZERO external dependencies (no Gin, no pgx, no HTTP).
2. Application Layer (internal/app): Contains the Gin HTTP handlers. Acts as the Inbound/Driving Adapter. Must interact with the Domain strictly via Request Ports.
3. Infrastructure Layer (internal/infra): Contains the PostgreSQL repository. Acts as the Outbound/Driven Adapter. Must implement Obtain Ports.
4. TDD Enforcement: For every logical layer, write the .go interface/struct definitions first, then the _test.go file with failing assertions (Red), then implement the logic to pass the tests (Green).

## Phase 0: Initialization & Dependency Management

1. Execute go mod init activation-service.
2. Fetch required dependencies:
   go get -u github.com/gin-gonic/gin
   go get -u github.com/jackc/pgx/v5/pgxpool
   go get -u github.com/stretchr/testify # For TDD assertions and mocks
   go get -u github.com/swaggo/swag/cmd/swag
3. Create the strict directory tree: cmd/server, internal/domain/model, internal/domain/ports/request, internal/domain/ports/obtain, internal/domain/service, internal/app/handler, internal/infra/repo, docs.

## Phase 1: Domain Layer (Core Business Rules)

### 1.1 Models & Ports (Scaffolding)

Create the pure Go definitions first so tests can compile.

* internal/domain/model/asset.go: Define Asset, ActivationRequest, and AllocationResult.
* internal/domain/ports/request/activation.go: Define ActivationUseCase interface with Execute(req model.ActivationRequest) (model.AllocationResult, error).
* internal/domain/ports/obtain/asset_repo.go: Define AssetRepository interface with FetchAll() and FetchPruned(date string, volume int).

### 1.2 Domain TDD (Red Phase)

Create internal/domain/service/strategy_test.go.

* Implement a mock struct satisfying obtain.AssetRepository directly in the test file returning a hardcoded slice of 5 varying assets.
* Write unit tests for the 4 expected services: TestGreedyBaseline, TestGreedyDB, TestKnapsackMemory, TestKnapsackDB.
* Assert against exact target volumes, expected cost minimizations, and edge cases (e.g., volume unattainable).
* Note: Tests will fail because the services do not exist yet.

### 1.3 Domain Implementation (Green Phase)

Implement the 4 calculation services in internal/domain/service/ to satisfy request.ActivationUseCase:

1. greedy_baseline.go: Filter by date, sort by price/kW in memory, pick sequentially.
2. greedy_db.go: Assume list is pre-filtered and pre-sorted by the repo. Pick sequentially.
3. knapsack_memory.go: Filter by date in memory, run 0/1 bottom-up Dynamic Programming to find absolute minimum fixed cost.
4. knapsack_db.go: Assume pool is pre-pruned by the repo. Run the exact same 0/1 DP matrix.

* Run go test ./internal/domain/.... Ensure all tests pass.

## Phase 2: Infrastructure Layer (PostgreSQL Adapter)

### 2.1 Database Setup Files

* Create init.sql at the project root containing assets and asset_availabilities tables. Include the generated column cost_per_kw and the composite index on (available_date, cost_per_kw).

### 2.2 Infrastructure TDD (Red Phase)

Create internal/infra/repo/postgres_asset_test.go.

* Mocking directive: Use a mock library (like pashagolub/pgxmock) or an in-memory SQLite wrapper to test the SQL queries.
* Write TestFetchAll to assert the query executes with ORDER BY a.cost_per_kw ASC.
* Write TestFetchPruned to assert the query applies the WHERE volume <= target * 1.5 constraints.

### 2.3 Infrastructure Implementation (Green Phase)

Implement internal/infra/repo/postgres_asset.go.

* Must instantiate a pgxpool.Pool.
* Implement FetchAll() and FetchPruned() executing standard SQL queries and mapping results to []model.Asset.
* Run tests and verify SQL scanning logic.

## Phase 3: Application Layer (Gin HTTP Adapter)

### 3.1 Application TDD (Red Phase)

Create internal/app/handler/activation_test.go.

* Create a mock struct implementing request.ActivationUseCase to bypass domain logic.
* Use net/http/httptest and gin.SetMode(gin.TestMode) to create response recorders.
* Write tests asserting that POST /api/v1/activation/greedy-baseline (and the other 3 endpoints) correctly bind the JSON payload, call the appropriate Domain UseCase, and return a 200 OK with the AllocationResult JSON.
* Test 400 Bad Request on invalid JSON.

### 3.2 Application Implementation (Green Phase)

Implement internal/app/handler/activation.go.

* Inject the 4 domain services (request.ActivationUseCase) via the constructor.
* Create the Gin handler methods (HandleGreedyBaseline, etc.).
* Add Swagger/OpenAPI annotations directly above the handler methods.
* Run go test ./internal/app/... to verify HTTP binding and routing.

## Phase 4: Bootstrapping & Container Orchestration

### 4.1 System Wiring

Create cmd/server/main.go.

1. Read DATABASE_URL from the environment.
2. Instantiate repo.NewPostgresAssetRepository.
3. Inject the repo into the 4 domain services (service.New...).
4. Inject the services into handler.NewActivationGinHandler.
5. Initialize gin.Default(), map the routes to the handler, and run on :8080.

### 4.2 Docker & Makefile

* Generate a multi-stage Dockerfile (golang:1.22 builder to alpine runtime).
* Generate docker-compose.yml declaring db (postgres:15-alpine) and web (the Go build). Wire init.sql as a volume mount for db. Implement a health check.
* Generate Makefile with targets: all, build, test (running go test -v ./...), swagger (running swag init), docker-up, and docker-down.

### 4.3 Final Execution Check

Run the final validation pipeline:

make test
make swagger
make docker-up

Ensure all tests pass and containers boot cleanly.

**
