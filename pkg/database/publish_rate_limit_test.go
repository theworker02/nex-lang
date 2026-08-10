package database_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"nex-lang/pkg/database"
)

func TestPublishRateLimitCooldown(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/nex_registry?sslmode=disable"
	}
	ctx := context.Background()
	db, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	defer db.Close()

	_, err = db.Pool().Exec(ctx, `
CREATE TABLE IF NOT EXISTS publish_rate_limits (
    user_id         BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_success_at TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	if err != nil {
		t.Fatalf("ensure table: %v", err)
	}

	stamp := time.Now().UnixNano()
	u, err := db.CreateUser(ctx, fmt.Sprintf("rlu%d", stamp), fmt.Sprintf("rlu%d@example.com", stamp), "hash", "", "", false)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	st, err := db.CheckPublishRateLimit(ctx, u.ID, 30*time.Minute)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !st.Allowed {
		t.Fatalf("expected allowed before any publish, got %+v", st)
	}

	if err := db.RecordPublishSuccess(ctx, u.ID); err != nil {
		t.Fatalf("record: %v", err)
	}

	st, err = db.CheckPublishRateLimit(ctx, u.ID, 30*time.Minute)
	if err != nil {
		t.Fatalf("check after: %v", err)
	}
	if st.Allowed {
		t.Fatalf("expected blocked after publish, got %+v", st)
	}
	if st.RetryAfterSeconds < 1 || st.RetryAfterSeconds > 30*60 {
		t.Fatalf("unexpected retry_after_seconds=%d", st.RetryAfterSeconds)
	}
	if st.CooldownMinutes != 30 {
		t.Fatalf("cooldown_minutes=%d", st.CooldownMinutes)
	}

	time.Sleep(1100 * time.Millisecond)
	st, err = db.CheckPublishRateLimit(ctx, u.ID, time.Second)
	if err != nil {
		t.Fatalf("check short: %v", err)
	}
	if !st.Allowed {
		t.Fatalf("expected allowed after short cooldown elapsed, got %+v", st)
	}
}
