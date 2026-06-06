package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// TranCatBalRepository mirrors VSAM KSDS TCATBALF file.
// Primary key: (TrancatAcctID, TrancatTypeCD, TrancatCode) composite.
type TranCatBalRepository interface {
	Get(ctx context.Context, acctID int64, typeCD string, catCD int64) (*domain.TranCatBalRecord, error)
	GetByAccount(ctx context.Context, acctID int64) ([]*domain.TranCatBalRecord, error)
	Create(ctx context.Context, r *domain.TranCatBalRecord) error
	Update(ctx context.Context, r *domain.TranCatBalRecord) error
	Upsert(ctx context.Context, r *domain.TranCatBalRecord) error
	Delete(ctx context.Context, acctID int64, typeCD string, catCD int64) error
	List(ctx context.Context) ([]*domain.TranCatBalRecord, error)
}
