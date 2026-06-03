package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

func sampleCustomer(id int64) *domain.CustomerRecord {
	return &domain.CustomerRecord{
		CustID:               id,
		CustFirstName:        "JANE",
		CustMiddleName:       "Q",
		CustLastName:         "DOE",
		CustAddrLine1:        "1 MAIN ST",
		CustAddrLine2:        "APT 2",
		CustAddrLine3:        "",
		CustAddrStateCode:    "CA",
		CustAddrCountryCode:  "USA",
		CustAddrZip:          "90210",
		CustPhoneNum1:        "5550001111",
		CustPhoneNum2:        "5550002222",
		CustSSN:              123456789,
		CustGovtIssuedID:     "X1234567",
		CustDOB:              "1980-01-01",
		CustEFTAccountID:     "EFT0001",
		CustPriCardHolderInd: "Y",
		CustFICOCreditScore:  720,
	}
}

func TestCustomerStore(t *testing.T) {
	ctx := context.Background()
	s := NewCustomerStore(newTestDB(t))

	c1 := sampleCustomer(1)
	c2 := sampleCustomer(2)
	c3 := sampleCustomer(3)

	for _, c := range []*domain.CustomerRecord{c1, c2, c3} {
		if err := s.Create(ctx, c); err != nil {
			t.Fatalf("create %d: %v", c.CustID, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CustID != c1.CustID || got.CustFirstName != c1.CustFirstName ||
		got.CustLastName != c1.CustLastName || got.CustSSN != c1.CustSSN ||
		got.CustAddrZip != c1.CustAddrZip || got.CustFICOCreditScore != c1.CustFICOCreditScore ||
		got.CustDOB != c1.CustDOB || got.CustPriCardHolderInd != c1.CustPriCardHolderInd {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", c1, got)
	}

	// Update and verify.
	got.CustLastName = "SMITH"
	got.CustAddrZip = "10001"
	got.CustFICOCreditScore = 800
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, 1)
	if reread.CustLastName != "SMITH" || reread.CustAddrZip != "10001" || reread.CustFICOCreditScore != 800 {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, sampleCustomer(999)); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// Browse full range.
	all, err := s.Browse(ctx, 0, 0)
	if err != nil {
		t.Fatalf("browse all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("browse all: want 3, got %d", len(all))
	}

	// Browse cursor: start from ID 2.
	page, err := s.Browse(ctx, 2, 10)
	if err != nil {
		t.Fatalf("browse cursor: %v", err)
	}
	if len(page) != 2 || page[0].CustID != 2 || page[1].CustID != 3 {
		t.Fatalf("browse cursor: want [2,3], got %v", page)
	}

	// Delete.
	if err := s.Delete(ctx, 2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, 2); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, 2); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, sampleCustomer(1)); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}
