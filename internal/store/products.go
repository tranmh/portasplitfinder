package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// ProductRepo reads and writes tracked products.
type ProductRepo struct {
	pool *pgxpool.Pool
}

// Upsert inserts or updates a product by id.
func (r *ProductRepo) Upsert(ctx context.Context, p domain.Product) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO products (id, name, model, image_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name, model = EXCLUDED.model, image_url = EXCLUDED.image_url
	`, p.ID, p.Name, p.Model, p.ImageURL)
	if err != nil {
		return fmt.Errorf("products upsert: %w", err)
	}
	return nil
}

// List returns all products ordered by name.
func (r *ProductRepo) List(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, model, image_url FROM products ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("products list: %w", err)
	}
	defer rows.Close()

	var out []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Model, &p.ImageURL); err != nil {
			return nil, fmt.Errorf("products scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
