package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// CustomerRepository mirrors VSAM KSDS CUSTDAT.
// Primary key: CustID.
type CustomerRepository interface {
	Get(ctx context.Context, custID int64) (*domain.CustomerRecord, error)
	Create(ctx context.Context, r *domain.CustomerRecord) error
	Update(ctx context.Context, r *domain.CustomerRecord) error
	Delete(ctx context.Context, custID int64) error
	// Browse returns up to limit records with custID >= startID, ordered by custID.
	// startID=0 returns from the beginning. limit <= 0 returns all matching rows.
	Browse(ctx context.Context, startID int64, limit int) ([]*domain.CustomerRecord, error)
}
