package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// StoreRepo reads and writes physical stores.
type StoreRepo struct {
	pool *pgxpool.Pool
}

// isNoRows reports whether err is pgx's no-rows sentinel.
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// Upsert inserts or updates a store. Stores are identified by (source_key, name)
// for upsert purposes, but a stable id may be supplied (used by the mock so the
// id stays deterministic across runs).
func (r *StoreRepo) Upsert(ctx context.Context, s domain.Store) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO stores (id, source_key, name, street, plz, city, lat, lon)
		VALUES (COALESCE(NULLIF($1,'')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (source_key, name) DO UPDATE
		SET street = EXCLUDED.street, plz = EXCLUDED.plz, city = EXCLUDED.city,
		    lat = EXCLUDED.lat, lon = EXCLUDED.lon
		RETURNING id
	`, s.ID, s.SourceKey, s.Name, s.Street, s.PLZ, s.City, s.Lat, s.Lon).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("stores upsert: %w", err)
	}
	return id, nil
}

// List returns all stores ordered by city, name.
func (r *StoreRepo) List(ctx context.Context) ([]domain.Store, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, source_key, name, street, plz, city, lat, lon FROM stores ORDER BY city, name
	`)
	if err != nil {
		return nil, fmt.Errorf("stores list: %w", err)
	}
	defer rows.Close()
	return scanStores(rows)
}

// Get returns a single store by id.
func (r *StoreRepo) Get(ctx context.Context, id string) (domain.Store, bool, error) {
	var s domain.Store
	err := r.pool.QueryRow(ctx, `
		SELECT id, source_key, name, street, plz, city, lat, lon FROM stores WHERE id = $1
	`, id).Scan(&s.ID, &s.SourceKey, &s.Name, &s.Street, &s.PLZ, &s.City, &s.Lat, &s.Lon)
	if err != nil {
		if isNoRows(err) {
			return domain.Store{}, false, nil
		}
		return domain.Store{}, false, fmt.Errorf("stores get: %w", err)
	}
	return s, true, nil
}

func scanStores(rows pgx.Rows) ([]domain.Store, error) {
	var out []domain.Store
	for rows.Next() {
		var s domain.Store
		if err := rows.Scan(&s.ID, &s.SourceKey, &s.Name, &s.Street, &s.PLZ, &s.City, &s.Lat, &s.Lon); err != nil {
			return nil, fmt.Errorf("stores scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
