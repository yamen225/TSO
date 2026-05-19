// Package integration_test contains end-to-end HTTP tests that exercise the
// full stack: Gin handlers → domain services → GORM repository → PostgreSQL.
//
// A real PostgreSQL container is started via testcontainers once for the
// entire test run (TestMain). Migrations are applied automatically, so no
// external database or DATABASE_URL is required. Run with:
//
//	go test -v -count=1 -timeout 120s ./test/integration/
package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"activation-service/internal/app/handler"
	"activation-service/internal/db"
	"activation-service/internal/domain/model"
	"activation-service/internal/domain/service"
	"activation-service/internal/infra/repo"
)

// ─── Seed data reference ────────────────────────────────────────────────────
//
// assets (from init.sql):
//   Alpha   100 kW  $500   price/kW = 5.0
//   Beta    200 kW  $800   price/kW = 4.0
//   Gamma   150 kW  $600   price/kW = 4.0
//   Delta   300 kW  $900   price/kW = 3.0
//   Epsilon  50 kW  $300   price/kW = 6.0
//
// asset_availabilities:
//   2025-06-01 → Alpha, Beta, Gamma, Delta, Epsilon   (total 800 kW)
//   2025-06-02 → Beta, Delta, Epsilon                 (total 550 kW, no price ties)
//   2025-06-03 → Alpha, Gamma                         (total 250 kW, no price ties)

// ─── Suite setup ─────────────────────────────────────────────────────────────

// sharedRouter is built once in TestMain and reused by every test function.
var sharedRouter *gin.Engine

// TestMain starts a PostgreSQL container, runs migrations, and wires up a
// single Gin router shared across all tests. One container per suite avoids
// the overhead of spinning up and tearing down a container per test.
func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:15-alpine",
		tcpostgres.WithDatabase("activation"),
		tcpostgres.WithUsername("activation"),
		tcpostgres.WithPassword("activation"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		log.Printf("could not start postgres container: %v", err)
		return 1
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("could not get connection string: %v", err)
		return 1
	}

	if err := db.RunMigrations(connStr); err != nil {
		log.Printf("migrations failed: %v", err)
		return 1
	}

	assetRepo, err := repo.NewPostgresAssetRepositoryFromURL(connStr)
	if err != nil {
		log.Printf("could not connect to database: %v", err)
		return 1
	}

	h := handler.NewActivationGinHandler(
		service.NewGreedyBaseline(assetRepo),
		service.NewGreedyDB(assetRepo),
		service.NewKnapsackMemory(assetRepo),
		service.NewKnapsackDB(assetRepo),
	)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1/activation")
	v1.POST("/greedy-baseline", h.HandleGreedyBaseline)
	v1.POST("/greedy-db", h.HandleGreedyDB)
	v1.POST("/knapsack-memory", h.HandleKnapsackMemory)
	v1.POST("/knapsack-db", h.HandleKnapsackDB)
	sharedRouter = r
	return m.Run()
}

// newRouter returns the shared Gin engine wired against the testcontainer DB.
func newRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return sharedRouter
}

func postJSON(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeResult(t *testing.T, w *httptest.ResponseRecorder) model.AllocationResult {
	t.Helper()
	var result model.AllocationResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	return result
}

// sortedNames returns asset names sorted alphabetically so assertions are
// independent of the order the database or service returns them.
func sortedNames(assets []model.Asset) []string {
	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = a.Name
	}
	sort.Strings(names)
	return names
}

// allEndpoints is every activation path in the service.
var allEndpoints = []string{
	"/api/v1/activation/greedy-baseline",
	"/api/v1/activation/greedy-db",
	"/api/v1/activation/knapsack-memory",
	"/api/v1/activation/knapsack-db",
}

// ─── Cross-endpoint tests (all four strategies behave identically) ────────────

// Malformed JSON must be rejected by the Gin binding layer before it ever
// reaches the domain, so every endpoint returns 400.
func TestIntegration_AllEndpoints_BadJSON_Returns400(t *testing.T) {
	r := newRouter(t)
	for _, ep := range allEndpoints {
		req := httptest.NewRequest(http.MethodPost, ep, strings.NewReader(`{not valid json}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "endpoint: %s", ep)
	}
}

// A well-formed request for a date that has no availability rows must return
// 422 for every strategy: the domain cannot meet any positive target volume.
func TestIntegration_AllEndpoints_DateWithNoAssets_Returns422(t *testing.T) {
	r := newRouter(t)
	body := model.ActivationRequest{Date: model.MustParseDate("2099-01-01"), TargetVolumeKW: 100}
	for _, ep := range allEndpoints {
		w := postJSON(t, r, ep, body)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "endpoint: %s", ep)
	}
}

// 2025-06-03 has Alpha (100 kW) + Gamma (150 kW) = 250 kW total.
// A target of 300 kW cannot be met, so every strategy returns 422.
func TestIntegration_AllEndpoints_VolumeExceedsAvailableCapacity_Returns422(t *testing.T) {
	r := newRouter(t)
	body := model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 300}
	for _, ep := range allEndpoints {
		w := postJSON(t, r, ep, body)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "endpoint: %s", ep)
	}
}

// 2025-06-03 has exactly 250 kW total (Alpha + Gamma). There is only one valid
// subset that meets the target, so all four strategies must choose the same
// two assets regardless of their algorithm.
func TestIntegration_AllEndpoints_ExactCapacityMatch_Returns200(t *testing.T) {
	r := newRouter(t)
	body := model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 250}
	for _, ep := range allEndpoints {
		w := postJSON(t, r, ep, body)
		require.Equal(t, http.StatusOK, w.Code, "endpoint: %s", ep)
		result := decodeResult(t, w)
		assert.Equal(t, []string{"Alpha", "Gamma"}, sortedNames(result.SelectedAssets), "endpoint: %s", ep)
		assert.Equal(t, 250, result.TotalCapacityKW, "endpoint: %s", ep)
		assert.InDelta(t, 1100.0, result.TotalCost, 0.001, "endpoint: %s", ep)
	}
}

// ─── Zero-volume edge cases ───────────────────────────────────────────────────
//
// When target_volume_kw = 0 the greedy strategies immediately satisfy the
// target with zero assets selected and return 200.
//
// knapsack-db behaves differently: FetchPruned caps candidates at
// volume × 1.5 = 0 kW, so the pool is empty, and knapsackSelect([]) returns
// ErrVolumeUnattainable → 422. This is an intentional design consequence of
// the capacity-cap pruning strategy.

func TestIntegration_GreedyBaseline_ZeroVolume_Returns200Empty(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/greedy-baseline",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-01"), TargetVolumeKW: 0})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Empty(t, result.SelectedAssets)
	assert.Equal(t, 0, result.TotalCapacityKW)
	assert.InDelta(t, 0.0, result.TotalCost, 0.001)
}

func TestIntegration_GreedyDB_ZeroVolume_Returns200Empty(t *testing.T) {
	r := newRouter(t)
	// FetchPruned(date, 0) → capacity cap = 0 → empty pool.
	// greedySelect([], 0): totalCap(0) >= targetKW(0) → break → return empty 200.
	w := postJSON(t, r, "/api/v1/activation/greedy-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-01"), TargetVolumeKW: 0})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Empty(t, result.SelectedAssets)
	assert.Equal(t, 0, result.TotalCapacityKW)
}

func TestIntegration_KnapsackMemory_ZeroVolume_Returns200Empty(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/knapsack-memory",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-01"), TargetVolumeKW: 0})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Empty(t, result.SelectedAssets)
	assert.Equal(t, 0, result.TotalCapacityKW)
}

// knapsack-db's FetchPruned returns an empty pool when target=0 (cap=0),
// and knapsackSelect([]) immediately returns ErrVolumeUnattainable.
func TestIntegration_KnapsackDB_ZeroVolume_EmptyPool_Returns422(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/knapsack-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-01"), TargetVolumeKW: 0})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ─── greedy-baseline ─────────────────────────────────────────────────────────
//
// greedy-baseline calls FetchAll (all dates), filters in memory, sorts by
// price/kW, then picks sequentially.

// 2025-06-02: Delta (3.0 $/kW, 300 kW) → 300 < 350 → continue → Beta (4.0 $/kW)
// → 500 kW ≥ 350.  Cost = $1700.  DP can achieve $1200 (Delta + Epsilon).
func TestIntegration_GreedyBaseline_PicksSuboptimalCost(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/greedy-baseline",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-02"), TargetVolumeKW: 350})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Beta", "Delta"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 500, result.TotalCapacityKW)
	assert.InDelta(t, 1700.0, result.TotalCost, 0.001)
}

// 2025-06-03: sort order is Gamma (4.0 $/kW) then Alpha (5.0 $/kW).
// Greedy picks Gamma (150 kW) for a 100 kW target even though Alpha alone
// costs $100 less ($500 vs $600).
func TestIntegration_GreedyBaseline_PicksLowerRatioAsset_OverCheaperOption(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/greedy-baseline",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 100})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Gamma"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 150, result.TotalCapacityKW)
	assert.InDelta(t, 600.0, result.TotalCost, 0.001)
}

// ─── greedy-db ───────────────────────────────────────────────────────────────
//
// greedy-db delegates filtering and sorting to FetchPruned, which caps
// candidates at volume × 1.5 kW, then picks sequentially.

// Same suboptimal outcome as greedy-baseline for 2025-06-02 / target 350:
// the capacity cap (525 kW) is wide enough to include all three assets, and
// the sequential greedy pick still chooses Delta then Beta.
func TestIntegration_GreedyDB_PicksSuboptimalCost(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/greedy-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-02"), TargetVolumeKW: 350})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Beta", "Delta"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 500, result.TotalCapacityKW)
	assert.InDelta(t, 1700.0, result.TotalCost, 0.001)
}

// 2025-06-03 / target 100: FetchPruned cap = 150 kW, so both Alpha (100) and
// Gamma (150) qualify.  Greedy picks Gamma first (lower price/kW ratio) even
// though Alpha meets the target at $100 less.
func TestIntegration_GreedyDB_PicksLowerRatioAsset_OverCheaperOption(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/greedy-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 100})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Gamma"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 150, result.TotalCapacityKW)
	assert.InDelta(t, 600.0, result.TotalCost, 0.001)
}

// ─── knapsack-memory ─────────────────────────────────────────────────────────
//
// knapsack-memory filters in memory then runs 0/1 DP to minimise total cost.

// 2025-06-02 / target 350:
// Delta (300 kW) + Epsilon (50 kW) = 350 kW / $1200, cheaper than
// Delta (300 kW) + Beta (200 kW) = 500 kW / $1700.
func TestIntegration_KnapsackMemory_FindsOptimalCombination(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/knapsack-memory",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-02"), TargetVolumeKW: 350})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Delta", "Epsilon"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 350, result.TotalCapacityKW)
	assert.InDelta(t, 1200.0, result.TotalCost, 0.001)
}

// 2025-06-03 / target 100:
// DP picks Alpha (100 kW / $500) rather than Gamma (150 kW / $600).
func TestIntegration_KnapsackMemory_PicksCheaperAssetOverGreedyChoice(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/knapsack-memory",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 100})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Alpha"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 100, result.TotalCapacityKW)
	assert.InDelta(t, 500.0, result.TotalCost, 0.001)
}

// ─── knapsack-db ─────────────────────────────────────────────────────────────
//
// knapsack-db combines DB-side pruning with 0/1 DP.

// 2025-06-02 / target 350:
// FetchPruned cap = 525 kW — all three assets qualify.
// DP finds Delta + Epsilon = 350 kW / $1200.
func TestIntegration_KnapsackDB_FindsOptimalCombination(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/knapsack-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-02"), TargetVolumeKW: 350})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Delta", "Epsilon"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 350, result.TotalCapacityKW)
	assert.InDelta(t, 1200.0, result.TotalCost, 0.001)
}

// 2025-06-03 / target 100:
// FetchPruned cap = 150 kW → both Alpha (100) and Gamma (150) qualify.
// DP picks Alpha ($500) over Gamma ($600).
func TestIntegration_KnapsackDB_PicksCheaperAssetOverGreedyChoice(t *testing.T) {
	r := newRouter(t)
	w := postJSON(t, r, "/api/v1/activation/knapsack-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 100})
	require.Equal(t, http.StatusOK, w.Code)
	result := decodeResult(t, w)
	assert.Equal(t, []string{"Alpha"}, sortedNames(result.SelectedAssets))
	assert.Equal(t, 100, result.TotalCapacityKW)
	assert.InDelta(t, 500.0, result.TotalCost, 0.001)
}

// ─── Greedy vs DP divergence summary ─────────────────────────────────────────
//
// This test makes the cost difference between greedy and DP explicit in a
// single function by running both endpoints with the same input and asserting
// that DP finds a cheaper result.

func TestIntegration_GreedyVsDP_DivergentCosts(t *testing.T) {
	r := newRouter(t)

	// Scenario A: 2025-06-02, target 350 kW
	// Greedy (DB): Delta + Beta = $1700
	// DP (DB):     Delta + Epsilon = $1200
	wGreedy := postJSON(t, r, "/api/v1/activation/greedy-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-02"), TargetVolumeKW: 350})
	wDP := postJSON(t, r, "/api/v1/activation/knapsack-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-02"), TargetVolumeKW: 350})

	require.Equal(t, http.StatusOK, wGreedy.Code)
	require.Equal(t, http.StatusOK, wDP.Code)

	greedyResult := decodeResult(t, wGreedy)
	dpResult := decodeResult(t, wDP)

	assert.Greater(t, greedyResult.TotalCost, dpResult.TotalCost,
		"DP should find a cheaper selection than greedy")
	assert.InDelta(t, 1700.0, greedyResult.TotalCost, 0.001, "greedy cost")
	assert.InDelta(t, 1200.0, dpResult.TotalCost, 0.001, "DP cost")

	// Scenario B: 2025-06-03, target 100 kW
	// Greedy (DB): Gamma = $600  (lower price/kW ratio, but higher absolute cost)
	// DP (DB):     Alpha = $500
	wGreedy2 := postJSON(t, r, "/api/v1/activation/greedy-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 100})
	wDP2 := postJSON(t, r, "/api/v1/activation/knapsack-db",
		model.ActivationRequest{Date: model.MustParseDate("2025-06-03"), TargetVolumeKW: 100})

	require.Equal(t, http.StatusOK, wGreedy2.Code)
	require.Equal(t, http.StatusOK, wDP2.Code)

	greedyResult2 := decodeResult(t, wGreedy2)
	dpResult2 := decodeResult(t, wDP2)

	assert.Greater(t, greedyResult2.TotalCost, dpResult2.TotalCost,
		"DP should find a cheaper selection than greedy")
	assert.InDelta(t, 600.0, greedyResult2.TotalCost, 0.001, "greedy cost")
	assert.InDelta(t, 500.0, dpResult2.TotalCost, 0.001, "DP cost")
}
