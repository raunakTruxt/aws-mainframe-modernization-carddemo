package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

func TestUserSecStore(t *testing.T) {
	ctx := context.Background()
	s := NewUserSecStore(newTestDB(t))

	admin := domain.UserSec{UserID: "admin01", FirstName: "ADA", LastName: "ADMIN", PwdHash: "hash-a", Type: domain.UserTypeAdmin}
	user := domain.UserSec{UserID: "user01", FirstName: "URI", LastName: "USER", PwdHash: "hash-u", Type: domain.UserTypeUser}

	if err := s.Create(ctx, admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := s.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Lookup normalises the user id (upper-case), matching InMemoryUserSec.
	got, err := s.Get(ctx, "admin01")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UserID != "ADMIN01" || got.Type != domain.UserTypeAdmin || got.PwdHash != "hash-a" {
		t.Fatalf("get mismatch: %+v", got)
	}

	got.LastName = "BOSS"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, "ADMIN01")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.LastName != "BOSS" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("list: want 2, got %d", len(all))
	}

	if err := s.Create(ctx, admin); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}

	if err := s.Delete(ctx, "user01"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "user01"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}
