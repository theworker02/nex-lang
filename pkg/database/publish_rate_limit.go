package database

import (
	"context"
	"fmt"
	"math"
	"time"
)

// PublishRateLimitStatus is the result of a per-user publish cooldown check.
type PublishRateLimitStatus struct {
	Allowed            bool      `json:"allowed"`
	RetryAfterSeconds  int64     `json:"retry_after_seconds"`
	CooldownMinutes    int64     `json:"cooldown_minutes"`
	LastSuccessAt      *time.Time `json:"last_success_at,omitempty"`
	SecondsSinceLast   int64     `json:"seconds_since_last,omitempty"`
}

// CheckPublishRateLimit returns whether userID may perform a successful publish.
// cooldown is clamped by the caller (typically 1–60 minutes). Persistence uses
// publish_rate_limits, with a fallback to the latest versions.published_by_user_id timestamp.
func (db *DB) CheckPublishRateLimit(ctx context.Context, userID int64, cooldown time.Duration) (*PublishRateLimitStatus, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Minute
	}

	status := &PublishRateLimitStatus{
		Allowed:         true,
		CooldownMinutes: int64(math.Round(cooldown.Minutes())),
	}

	var last *time.Time
	err := db.pool.QueryRow(ctx, `
SELECT GREATEST(
  (SELECT last_success_at FROM publish_rate_limits WHERE user_id = $1),
  (SELECT MAX(created_at) FROM versions WHERE published_by_user_id = $1)
)`, userID).Scan(&last)
	if err != nil {
		return nil, fmt.Errorf("check publish rate limit: %w", err)
	}
	if last == nil {
		return status, nil
	}

	status.LastSuccessAt = last
	elapsed := time.Since(last.UTC())
	if elapsed < 0 {
		elapsed = 0
	}
	status.SecondsSinceLast = int64(elapsed.Seconds())
	if elapsed >= cooldown {
		return status, nil
	}

	remain := cooldown - elapsed
	sec := int64(math.Ceil(remain.Seconds()))
	if sec < 1 {
		sec = 1
	}
	status.Allowed = false
	status.RetryAfterSeconds = sec
	return status, nil
}

// RecordPublishSuccess updates the durable per-user publish cooldown clock.
func (db *DB) RecordPublishSuccess(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user id")
	}
	_, err := db.pool.Exec(ctx, `
INSERT INTO publish_rate_limits (user_id, last_success_at, updated_at)
VALUES ($1, NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE
SET last_success_at = EXCLUDED.last_success_at,
    updated_at = NOW()`, userID)
	if err != nil {
		return fmt.Errorf("record publish success: %w", err)
	}
	return nil
}
