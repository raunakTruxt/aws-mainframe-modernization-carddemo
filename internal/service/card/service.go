// Package card implements the CardDemo credit-card business logic: list,
// view, and update (ports of COCRDLIC, COCRDSLC, COCRDUPC).
package card

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// PageSize matches the BMS list screen (7 data rows per page, like COCRDLIC).
const PageSize = 7

// ErrValidation is returned by Update when the caller supplies invalid field values.
var ErrValidation = errors.New("validation error")

// Service orchestrates COCRDLIC / COCRDSLC / COCRDUPC business logic.
type Service struct {
	cards repo.CardRepository
	xrefs repo.CardXrefRepository
}

// NewService creates a Service backed by the given repositories.
func NewService(cards repo.CardRepository, xrefs repo.CardXrefRepository) *Service {
	return &Service{cards: cards, xrefs: xrefs}
}

// CardSummary is the masked view used on the list screen (COCRDLIC).
// The card number shows only the last 4 digits; full number is carried in
// CardNum for action hyperlinks.
type CardSummary struct {
	CardNum            string // full, unmasked — use only in URLs/actions
	CardNumMasked      string // **** **** **** NNNN — safe to display
	CardAcctID         int64
	CardEmbossedName   string
	CardExpirationDate string
	CardActiveStatus   string
}

// ListPage is the result of a List call: a slice of summaries plus a
// pagination cursor.
type ListPage struct {
	Cards   []*CardSummary
	NextKey string // empty means this is the last page
	HasNext bool
}

// List returns one page of cards.
//
// When accountID > 0 the result is scoped to cards belonging to that account
// (the non-admin user path from COCRDLIC). When accountID == 0 all cards are
// browsed (admin path). startKey is the card_num cursor for forward pagination;
// empty starts from the first record. pageSize <= 0 uses PageSize.
func (s *Service) List(ctx context.Context, accountID int64, startKey string, pageSize int) (*ListPage, error) {
	if pageSize <= 0 {
		pageSize = PageSize
	}

	var raw []*domain.CardRecord
	var err error

	if accountID > 0 {
		all, e := s.cards.GetByAccount(ctx, accountID)
		if e != nil {
			return nil, fmt.Errorf("card service list: %w", e)
		}
		raw = applyStartKey(all, startKey, pageSize+1)
	} else {
		raw, err = s.cards.Browse(ctx, startKey, pageSize+1)
		if err != nil {
			return nil, fmt.Errorf("card service list: %w", err)
		}
	}

	var nextKey string
	if len(raw) > pageSize {
		nextKey = raw[pageSize].CardNum
		raw = raw[:pageSize]
	}

	summaries := make([]*CardSummary, len(raw))
	for i, c := range raw {
		summaries[i] = &CardSummary{
			CardNum:            c.CardNum,
			CardNumMasked:      MaskCardNum(c.CardNum),
			CardAcctID:         c.CardAcctID,
			CardEmbossedName:   c.CardEmbossedName,
			CardExpirationDate: c.CardExpirationDate,
			CardActiveStatus:   c.CardActiveStatus,
		}
	}

	return &ListPage{
		Cards:   summaries,
		NextKey: nextKey,
		HasNext: nextKey != "",
	}, nil
}

// Get returns the full, unmasked card record for the detail / update screen.
func (s *Service) Get(ctx context.Context, cardNum string) (*domain.CardRecord, error) {
	return s.cards.Get(ctx, cardNum)
}

// Update validates and persists changes to a card record.
//
// Editable fields (matching COCRDUPC): CardActiveStatus, CardEmbossedName,
// CardExpirationDate. The caller must supply a complete CardRecord (typically
// obtained via Get and then mutated).
func (s *Service) Update(ctx context.Context, card *domain.CardRecord) error {
	if err := validateCard(card); err != nil {
		return err
	}
	if err := s.cards.Update(ctx, card); err != nil {
		return fmt.Errorf("card service update: %w", err)
	}
	return nil
}

// validateCard enforces the COCRDUPC edit rules.
func validateCard(card *domain.CardRecord) error {
	status := strings.TrimSpace(card.CardActiveStatus)
	if status != "Y" && status != "N" {
		return fmt.Errorf("%w: card active status must be Y or N", ErrValidation)
	}

	name := strings.TrimSpace(card.CardEmbossedName)
	if name == "" {
		return fmt.Errorf("%w: card name must not be blank", ErrValidation)
	}
	for _, ch := range name {
		if !unicode.IsLetter(ch) && ch != ' ' {
			return fmt.Errorf("%w: card name can only contain letters and spaces", ErrValidation)
		}
	}

	// CardExpirationDate is stored as YYYY-MM-DD (domain convention).
	parts := strings.SplitN(strings.TrimSpace(card.CardExpirationDate), "-", 3)
	if len(parts) != 3 {
		return fmt.Errorf("%w: invalid expiry date format (expected YYYY-MM-DD)", ErrValidation)
	}
	var year, month int
	if _, err := fmt.Sscanf(parts[0], "%d", &year); err != nil || year < 1950 || year > 2099 {
		return fmt.Errorf("%w: invalid card expiry year", ErrValidation)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &month); err != nil || month < 1 || month > 12 {
		return fmt.Errorf("%w: card expiry month must be between 1 and 12", ErrValidation)
	}
	// Expiry is valid if card's year-month >= current year-month (day is ignored).
	now := time.Now()
	if year < now.Year() || (year == now.Year() && month < int(now.Month())) {
		return fmt.Errorf("%w: card expiry date must not be in the past", ErrValidation)
	}

	return nil
}

// MaskCardNum returns a masked display string "**** **** **** NNNN".
// Exported so HTML templates can call it if needed.
func MaskCardNum(n string) string {
	n = strings.TrimSpace(n)
	if len(n) < 4 {
		return strings.Repeat("*", len(n))
	}
	return fmt.Sprintf("**** **** **** %s", n[len(n)-4:])
}

// applyStartKey filters all to records with CardNum >= startKey and caps at limit.
func applyStartKey(all []*domain.CardRecord, startKey string, limit int) []*domain.CardRecord {
	var out []*domain.CardRecord
	for _, c := range all {
		if startKey == "" || c.CardNum >= startKey {
			out = append(out, c)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}
