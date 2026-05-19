-- Assets master table
CREATE TABLE IF NOT EXISTS assets (
    id           SERIAL PRIMARY KEY,
    name         VARCHAR(255) NOT NULL,
    capacity_kw  INT          NOT NULL,
    fixed_cost   NUMERIC(12,2) NOT NULL,
    price_per_kw NUMERIC(10,4) GENERATED ALWAYS AS (fixed_cost / NULLIF(capacity_kw, 0)) STORED
);

-- Asset availabilities (one row per asset per date)
CREATE TABLE IF NOT EXISTS asset_availabilities (
    id         SERIAL PRIMARY KEY,
    asset_id   INT  NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    avail_date DATE NOT NULL,
    UNIQUE (asset_id, avail_date)
);

-- Composite index on assets: supports the capacity_kw filter + price_per_kw sort in FetchPruned.
CREATE INDEX IF NOT EXISTS idx_assets_capacity_cost
    ON assets (capacity_kw, price_per_kw ASC);

-- Composite index on asset_availabilities: avail_date leads so the equality filter
-- in FetchPruned hits the index rather than the UNIQUE(asset_id, avail_date) index
-- which has asset_id as the leading column.
CREATE INDEX IF NOT EXISTS idx_availability_date_asset
    ON asset_availabilities (avail_date, asset_id);

-- Seed data
INSERT INTO assets (name, capacity_kw, fixed_cost) VALUES
    ('Alpha',   100, 500.00),
    ('Beta',    200, 800.00),
    ('Gamma',   150, 600.00),
    ('Delta',   300, 900.00),
    ('Epsilon',  50, 300.00)
ON CONFLICT DO NOTHING;

-- Seed availabilities
-- 2025-06-01: all five assets available
INSERT INTO asset_availabilities (asset_id, avail_date)
SELECT a.id, '2025-06-01'
FROM assets a
WHERE a.name IN ('Alpha', 'Beta', 'Gamma', 'Delta', 'Epsilon')
ON CONFLICT DO NOTHING;

-- 2025-06-02: only high-capacity assets (Beta, Delta) + Epsilon available
INSERT INTO asset_availabilities (asset_id, avail_date)
SELECT a.id, '2025-06-02'
FROM assets a
WHERE a.name IN ('Beta', 'Delta', 'Epsilon')
ON CONFLICT DO NOTHING;

-- 2025-06-03: mid-range assets (Alpha, Gamma) available
INSERT INTO asset_availabilities (asset_id, avail_date)
SELECT a.id, '2025-06-03'
FROM assets a
WHERE a.name IN ('Alpha', 'Gamma')
ON CONFLICT DO NOTHING;
