package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// UserSecStore is the SQLite-backed repo.UserSecRepo, replacing InMemoryUserSec.
type UserSecStore struct{ db *sql.DB }

// NewUserSecStore returns a UserSecStore over db.
func NewUserSecStore(db *sql.DB) *UserSecStore { return &UserSecStore{db: db} }

const userColumns = `user_id, first_name, last_name, pwd_hash, user_type`

func (s *UserSecStore) Get(ctx context.Context, userID string) (domain.UserSec, error) {
	id := domain.NormalizeUserID(userID)
	row := s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE user_id = ?`, id)
	return scanUserSec(row)
}

func (s *UserSecStore) Create(ctx context.Context, u domain.UserSec) error {
	u.UserID = domain.NormalizeUserID(u.UserID)
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (`+userColumns+`) VALUES (?, ?, ?, ?, ?)`,
		u.UserID, u.FirstName, u.LastName, u.PwdHash, string(u.Type))
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: user create: %w", err)
	}
	return nil
}

func (s *UserSecStore) Update(ctx context.Context, u domain.UserSec) error {
	u.UserID = domain.NormalizeUserID(u.UserID)
	res, err := s.db.ExecContext(ctx, `UPDATE users SET first_name = ?, last_name = ?, pwd_hash = ?, user_type = ? WHERE user_id = ?`,
		u.FirstName, u.LastName, u.PwdHash, string(u.Type), u.UserID)
	if err != nil {
		return fmt.Errorf("sqlite: user update: %w", err)
	}
	return requireAffected(res)
}

func (s *UserSecStore) Delete(ctx context.Context, userID string) error {
	id := domain.NormalizeUserID(userID)
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE user_id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: user delete: %w", err)
	}
	return requireAffected(res)
}

func (s *UserSecStore) List(ctx context.Context) ([]domain.UserSec, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users ORDER BY user_id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: user list: %w", err)
	}
	defer rows.Close()

	var out []domain.UserSec
	for rows.Next() {
		u, err := scanUserSec(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func scanUserSec(sc scanner) (domain.UserSec, error) {
	var (
		u   domain.UserSec
		typ string
	)
	err := sc.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.PwdHash, &typ)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UserSec{}, repo.ErrNotFound
	}
	if err != nil {
		return domain.UserSec{}, fmt.Errorf("sqlite: user scan: %w", err)
	}
	if typ != "" {
		u.Type = domain.UserType(typ[0])
	}
	return u, nil
}
