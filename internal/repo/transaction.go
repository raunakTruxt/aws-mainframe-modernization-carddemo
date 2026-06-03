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
	GetByCard(ctx context.Context, cardNum string, limit int) ([]*domain.TransactionRecord, error)
	Create(ctx context.Context, r *domain.TransactionRecord) error
	Update(ctx context.Context, r *domain.TransactionRecord) error
	Delete(ctx context.Context, tranID string) error
	Browse(ctx context.Context, startID string, limit int) ([]*domain.TransactionRecord, error)
}
