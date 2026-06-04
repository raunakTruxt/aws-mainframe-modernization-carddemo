package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// TranTypeRepository mirrors VSAM KSDS TRANTYPE file.
// Primary key: TranType code.
type TranTypeRepository interface {
	Get(ctx context.Context, tranType string) (*domain.TranTypeRecord, error)
	Create(ctx context.Context, r *domain.TranTypeRecord) error
	Update(ctx context.Context, r *domain.TranTypeRecord) error
	Delete(ctx context.Context, tranType string) error
	List(ctx context.Context) ([]*domain.TranTypeRecord, error)
}
