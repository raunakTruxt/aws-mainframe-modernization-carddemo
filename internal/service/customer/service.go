// Package customer provides the business logic for customer look-ups used by
// the account view/update screens (COACTVWC / COACTUPC).
package customer

import (
	"context"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Service provides read-only customer queries used by the account screens.
type Service struct {
	customers repo.CustomerRepository
	xref      repo.CardXrefRepository
}

// New constructs a Service.
func New(custRepo repo.CustomerRepository, xrefRepo repo.CardXrefRepository) *Service {
	return &Service{customers: custRepo, xref: xrefRepo}
}

// GetByID returns the customer with the given ID.
// Returns repo.ErrNotFound (wrapped) if not found.
func (s *Service) GetByID(ctx context.Context, custID int64) (*domain.CustomerRecord, error) {
	c, err := s.customers.Get(ctx, custID)
	if err != nil {
		return nil, fmt.Errorf("customer: GetByID: %w", err)
	}
	return c, nil
}

// ListByAccount returns all customers linked to acctID via the card xref.
// Returns an empty slice (no error) when no xref entries exist.
func (s *Service) ListByAccount(ctx context.Context, acctID int64) ([]*domain.CustomerRecord, error) {
	xrefs, err := s.xref.GetByAccount(ctx, acctID)
	if err != nil {
		return nil, fmt.Errorf("customer: ListByAccount: xref: %w", err)
	}
	out := make([]*domain.CustomerRecord, 0, len(xrefs))
	for _, x := range xrefs {
		c, err := s.customers.Get(ctx, x.XrefCustID)
		if err != nil {
			return nil, fmt.Errorf("customer: ListByAccount: cust %d: %w", x.XrefCustID, err)
		}
		out = append(out, c)
	}
	return out, nil
}
