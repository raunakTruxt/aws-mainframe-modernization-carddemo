package wait

import (
	"context"
	"testing"
	"time"
)

func TestRun_zero(t *testing.T) {
	s, err := Run(context.Background(), Config{Centiseconds: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Waited != 0 {
		t.Errorf("waited %v, want 0", s.Waited)
	}
}

func TestRun_small(t *testing.T) {
	start := time.Now()
	s, err := Run(context.Background(), Config{Centiseconds: 2}) // 20ms
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 15*time.Millisecond {
		t.Errorf("waited too short: %v", elapsed)
	}
	if s.Waited != 20*time.Millisecond {
		t.Errorf("summary waited %v, want 20ms", s.Waited)
	}
}

func TestRun_cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, Config{Centiseconds: 100})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestRun_negative(t *testing.T) {
	_, err := Run(context.Background(), Config{Centiseconds: -1})
	if err == nil {
		t.Fatal("expected error for negative centiseconds")
	}
}
