package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// CardRepository mirrors VSAM KSDS CARDDAT plus the AIX keyed by account.
// Primary key: CardNum. Alternate key: CardAcctID.
type CardRepository interface {
	Get(ctx context.Context, cardNum string) (*domain.CardRecord, error)
	GetByAccount(ctx context.Context, acctID int64) ([]*domain.CardRecord, error)
	Create(ctx context.Context, r *domain.CardRecord) error
	Update(ctx context.Context, r *domain.CardRecord) error
	Delete(ctx context.Context, cardNum string) error
	// Browse returns up to limit records with cardNum >= startNum, ordered by cardNum.
	// startNum="" returns from the beginning. limit <= 0 returns all matching rows.
	Browse(ctx context.Context, startNum string, limit int) ([]*domain.CardRecord, error)
}
