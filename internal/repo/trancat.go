package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// TranCatRepository mirrors VSAM KSDS TRANCATG file.
// Primary key: (TranTypeCode, TranCatCode) composite.
type TranCatRepository interface {
	Get(ctx context.Context, typeCD string, catCD int64) (*domain.TranCatRecord, error)
	GetByType(ctx context.Context, typeCD string) ([]*domain.TranCatRecord, error)
	Create(ctx context.Context, r *domain.TranCatRecord) error
	Update(ctx context.Context, r *domain.TranCatRecord) error
	Delete(ctx context.Context, typeCD string, catCD int64) error
	List(ctx context.Context) ([]*domain.TranCatRecord, error)
}
