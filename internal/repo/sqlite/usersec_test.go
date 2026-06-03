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

	// Full-field Get assertion; normalised user ID.
	got, err := s.Get(ctx, "admin01")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UserID != "ADMIN01" || got.FirstName != "ADA" || got.LastName != "ADMIN" ||
		got.PwdHash != "hash-a" || got.Type != domain.UserTypeAdmin {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", admin, got)
	}

	// Update and verify.
	got.LastName = "BOSS"
	got.PwdHash = "hash-b"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, "ADMIN01")
	if reread.LastName != "BOSS" || reread.PwdHash != "hash-b" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, domain.UserSec{UserID: "MISSING", Type: domain.UserTypeUser}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// List returns all users sorted by user_id.
	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("list: want 2, got %d", len(all))
	}
	// Verify order (ADMIN01 < USER01 lexicographically).
	if all[0].UserID != "ADMIN01" || all[1].UserID != "USER01" {
		t.Fatalf("list order: %v", all)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, admin); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}

	// Delete.
	if err := s.Delete(ctx, "user01"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "user01"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, "user01"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}
}
