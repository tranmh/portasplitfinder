package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// SourceRepo reads and writes source catalogue + scrape status.
type SourceRepo struct {
	pool *pgxpool.Pool
}

// Upsert inserts or updates a source by key. It preserves existing scrape status
// when re-seeding (does not overwrite last_* on conflict).
func (r *SourceRepo) Upsert(ctx context.Context, s domain.Source) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sources (key, name, type, base_url, last_status)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5,''),'never'))
		ON CONFLICT (key) DO UPDATE
		SET name = EXCLUDED.name, type = EXCLUDED.type, base_url = EXCLUDED.base_url
	`, s.Key, s.Name, string(s.Type), s.BaseURL, string(s.LastStatus))
	if err != nil {
		return fmt.Errorf("sources upsert: %w", err)
	}
	return nil
}

// UpdateStatus records the outcome of a scrape for a source.
func (r *SourceRepo) UpdateStatus(ctx context.Context, key string, checkedAt time.Time, status domain.LastStatus, lastErr *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sources SET last_checked_at = $2, last_status = $3, last_error = $4 WHERE key = $1
	`, key, checkedAt, string(status), lastErr)
	if err != nil {
		return fmt.Errorf("sources update status: %w", err)
	}
	return nil
}

// List returns all sources ordered by key.
func (r *SourceRepo) List(ctx context.Context) ([]domain.Source, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT key, name, type, base_url, last_checked_at, last_status, last_error
		FROM sources ORDER BY key
	`)
	if err != nil {
		return nil, fmt.Errorf("sources list: %w", err)
	}
	defer rows.Close()

	var out []domain.Source
	for rows.Next() {
		var s domain.Source
		var typ, status string
		if err := rows.Scan(&s.Key, &s.Name, &typ, &s.BaseURL, &s.LastCheckedAt, &status, &s.LastError); err != nil {
			return nil, fmt.Errorf("sources scan: %w", err)
		}
		s.Type = domain.SourceType(typ)
		s.LastStatus = domain.LastStatus(status)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get returns a single source by key.
func (r *SourceRepo) Get(ctx context.Context, key string) (domain.Source, bool, error) {
	var s domain.Source
	var typ, status string
	err := r.pool.QueryRow(ctx, `
		SELECT key, name, type, base_url, last_checked_at, last_status, last_error
		FROM sources WHERE key = $1
	`, key).Scan(&s.Key, &s.Name, &typ, &s.BaseURL, &s.LastCheckedAt, &status, &s.LastError)
	if err != nil {
		if isNoRows(err) {
			return domain.Source{}, false, nil
		}
		return domain.Source{}, false, fmt.Errorf("sources get: %w", err)
	}
	s.Type = domain.SourceType(typ)
	s.LastStatus = domain.LastStatus(status)
	return s, true, nil
}
