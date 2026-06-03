package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// CardXrefRepository mirrors VSAM KSDS CARDXREF plus the AIX keyed by account
// and by customer.
// Primary key: XrefCardNum. Alternate keys: XrefAcctID, XrefCustID.
type CardXrefRepository interface {
	Get(ctx context.Context, cardNum string) (*domain.CardXrefRecord, error)
	GetByAccount(ctx context.Context, acctID int64) ([]*domain.CardXrefRecord, error)
	GetByCustomer(ctx context.Context, custID int64) ([]*domain.CardXrefRecord, error)
	Create(ctx context.Context, r *domain.CardXrefRecord) error
	Update(ctx context.Context, r *domain.CardXrefRecord) error
	Delete(ctx context.Context, cardNum string) error
	Browse(ctx context.Context, startNum string, limit int) ([]*domain.CardXrefRecord, error)
}
