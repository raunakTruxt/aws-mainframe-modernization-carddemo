package repo

import (
	"context"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
)

// DiscGroupRepository mirrors VSAM KSDS DISCGRP file.
// Primary key: (DisAcctGroupID, DisTranTypeCD, DisTranCatCode) composite.
type DiscGroupRepository interface {
	Get(ctx context.Context, groupID, typeCD string, catCD int64) (*domain.DiscGroupRecord, error)
	GetByGroup(ctx context.Context, groupID string) ([]*domain.DiscGroupRecord, error)
	Create(ctx context.Context, r *domain.DiscGroupRecord) error
	Update(ctx context.Context, r *domain.DiscGroupRecord) error
	Delete(ctx context.Context, groupID, typeCD string, catCD int64) error
	List(ctx context.Context) ([]*domain.DiscGroupRecord, error)
}
