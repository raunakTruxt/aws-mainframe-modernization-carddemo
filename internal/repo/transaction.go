package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// TransactionRepository mirrors VSAM KSDS TRANSACT plus the AIX keyed by card
// number.
// Primary key: TranID. Alternate key: TranCardNum.
type TransactionRepository interface {
	Get(ctx context.Context, tranID string) (*domain.TransactionRecord, error)
	// GetByCard returns up to limit transactions for cardNum, ordered by tran_id.
	// limit <= 0 returns all matching rows.
	GetByCard(ctx context.Context, cardNum string, limit int) ([]*domain.TransactionRecord, error)
	Create(ctx context.Context, r *domain.TransactionRecord) error
	Update(ctx context.Context, r *domain.TransactionRecord) error
	Delete(ctx context.Context, tranID string) error
	// Browse returns up to limit records with tranID >= startID, ordered by tranID.
	// startID="" returns from the beginning. limit <= 0 returns all matching rows.
	Browse(ctx context.Context, startID string, limit int) ([]*domain.TransactionRecord, error)
	// NextID returns the next available TRAN-ID — the current max numeric tran_id
	// incremented by one, zero-padded to 16 digits (legacy PIC 9(16) format).
	// Mirrors the COBOL READPREV / max-id+1 pattern from COTRN02C.
	// Must be called inside a database transaction to prevent collisions.
	NextID(ctx context.Context) (string, error)
}
