package repo_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"activation-service/internal/domain/model"
	"activation-service/internal/infra/repo"
)

func newTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	return db, mock
}

func TestFetchAllSortedByCost(t *testing.T) {
	db, mock := newTestDB(t)

	rows := sqlmock.NewRows([]string{"id", "name", "capacity_kw", "fixed_cost", "price_per_kw", "avail_date"}).
		AddRow(4, "Delta", 300, 900.0, 3.0, "2026-06-01").
		AddRow(2, "Beta", 200, 800.0, 4.0, "2026-06-01")

	mock.ExpectQuery(`ORDER BY assets\.price_per_kw ASC`).WillReturnRows(rows)

	r := repo.NewPostgresAssetRepository(db)
	assets, err := r.FetchAllSortedByCost()

	require.NoError(t, err)
	assert.Len(t, assets, 2)
	assert.Equal(t, "Delta", assets[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFetchPruned(t *testing.T) {
	db, mock := newTestDB(t)

	date := model.MustParseDate("2026-06-01")
	targetVolume := 300
	maxCap := float64(targetVolume) * 1.5

	rows := sqlmock.NewRows([]string{"id", "name", "capacity_kw", "fixed_cost", "price_per_kw", "avail_date"}).
		AddRow(4, "Delta", 300, 900.0, 3.0, date).
		AddRow(2, "Beta", 200, 800.0, 4.0, date)

	mock.ExpectQuery(`asset_availabilities\.avail_date`).
		WithArgs(date, maxCap).
		WillReturnRows(rows)

	r := repo.NewPostgresAssetRepository(db)
	assets, err := r.FetchPruned(date, targetVolume)

	require.NoError(t, err)
	assert.Len(t, assets, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

