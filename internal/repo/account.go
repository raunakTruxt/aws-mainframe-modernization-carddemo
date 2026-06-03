package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// AccountRepository mirrors VSAM KSDS ACCTDAT operations.
// Primary key: AcctID. No alternate indexes on this file.
type AccountRepository interface {
	Get(ctx context.Context, acctID int64) (*domain.AccountRecord, error)
	Create(ctx context.Context, r *domain.AccountRecord) error
	Update(ctx context.Context, r *domain.AccountRecord) error
	Delete(ctx context.Context, acctID int64) error
	// Browse returns up to limit records with acctID >= startID, ordered by acctID.
	// startID=0 returns from the beginning.
	Browse(ctx context.Context, startID int64, limit int) ([]*domain.AccountRecord, error)
}
