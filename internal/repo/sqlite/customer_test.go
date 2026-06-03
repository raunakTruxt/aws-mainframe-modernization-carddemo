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

	if err := s.Create(ctx, sampleCustomer(1)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, sampleCustomer(2)); err != nil {
		t.Fatalf("create 2: %v", err)
	}

	got, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CustLastName != "DOE" || got.CustSSN != 123456789 {
		t.Fatalf("get mismatch: %+v", got)
	}

	got.CustLastName = "SMITH"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.CustLastName != "SMITH" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.Browse(ctx, 0, 10)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("browse: want 2, got %d", len(all))
	}

	if err := s.Delete(ctx, 2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, 2); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}
