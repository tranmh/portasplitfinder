package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// SubscriptionRepo manages notification subscriptions and their token flows.
type SubscriptionRepo struct {
	pool *pgxpool.Pool
}

// Create inserts a new, unconfirmed subscription and returns its id.
func (r *SubscriptionRepo) Create(ctx context.Context, s domain.Subscription) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO subscriptions (email, plz, radius_km, product_id, confirm_token, unsub_token)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, $5, $6)
		RETURNING id
	`, s.Email, s.PLZ, s.RadiusKm, deref(s.ProductID), s.ConfirmToken, s.UnsubToken).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("subscriptions create: %w", err)
	}
	return id, nil
}

// ConfirmByToken confirms the subscription matching the confirm token and
// returns its email. ok=false when the token is unknown.
func (r *SubscriptionRepo) ConfirmByToken(ctx context.Context, token string) (email string, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `
		UPDATE subscriptions
		SET confirmed_at = COALESCE(confirmed_at, now())
		WHERE confirm_token = $1
		RETURNING email
	`, token).Scan(&email)
	if err != nil {
		if isNoRows(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("subscriptions confirm: %w", err)
	}
	return email, true, nil
}

// UnsubByToken marks the subscription matching the unsub token as unsubscribed.
// ok=false when the token is unknown.
func (r *SubscriptionRepo) UnsubByToken(ctx context.Context, token string) (ok bool, err error) {
	var id string
	err = r.pool.QueryRow(ctx, `
		UPDATE subscriptions
		SET unsubbed_at = COALESCE(unsubbed_at, now())
		WHERE unsub_token = $1
		RETURNING id
	`, token).Scan(&id)
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, fmt.Errorf("subscriptions unsub: %w", err)
	}
	return true, nil
}

// ListConfirmed returns all confirmed, not-unsubscribed subscriptions.
func (r *SubscriptionRepo) ListConfirmed(ctx context.Context) ([]domain.Subscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, plz, radius_km, product_id, confirm_token, unsub_token, created_at, confirmed_at, unsubbed_at
		FROM subscriptions
		WHERE confirmed_at IS NOT NULL AND unsubbed_at IS NULL
		ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("subscriptions list confirmed: %w", err)
	}
	defer rows.Close()

	var out []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		if err := rows.Scan(&s.ID, &s.Email, &s.PLZ, &s.RadiusKm, &s.ProductID, &s.ConfirmToken, &s.UnsubToken, &s.CreatedAt, &s.ConfirmedAt, &s.UnsubbedAt); err != nil {
			return nil, fmt.Errorf("subscriptions scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetByConfirmToken fetches a subscription by its confirm token (test helper /
// confirm flow).
func (r *SubscriptionRepo) GetByConfirmToken(ctx context.Context, token string) (domain.Subscription, bool, error) {
	return r.getByToken(ctx, "confirm_token", token)
}

// GetByUnsubToken fetches a subscription by its unsub token.
func (r *SubscriptionRepo) GetByUnsubToken(ctx context.Context, token string) (domain.Subscription, bool, error) {
	return r.getByToken(ctx, "unsub_token", token)
}

func (r *SubscriptionRepo) getByToken(ctx context.Context, col, token string) (domain.Subscription, bool, error) {
	var s domain.Subscription
	q := fmt.Sprintf(`
		SELECT id, email, plz, radius_km, product_id, confirm_token, unsub_token, created_at, confirmed_at, unsubbed_at
		FROM subscriptions WHERE %s = $1`, col)
	err := r.pool.QueryRow(ctx, q, token).Scan(
		&s.ID, &s.Email, &s.PLZ, &s.RadiusKm, &s.ProductID, &s.ConfirmToken, &s.UnsubToken, &s.CreatedAt, &s.ConfirmedAt, &s.UnsubbedAt)
	if err != nil {
		if isNoRows(err) {
			return domain.Subscription{}, false, nil
		}
		return domain.Subscription{}, false, fmt.Errorf("subscriptions get by token: %w", err)
	}
	return s, true, nil
}
