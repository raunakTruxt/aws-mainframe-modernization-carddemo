package customer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/customer"
)

func openSeeded(t *testing.T) (*svc.Service, int64, int64) {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	custStore := sqlite.NewCustomerStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	ctx := context.Background()
	cust := &domain.CustomerRecord{
		CustID:               1001,
		CustFirstName:        "John",
		CustLastName:         "Doe",
		CustAddrStateCode:    "CA",
		CustAddrZip:          "90210",
		CustAddrCountryCode:  "USA",
		CustEFTAccountID:     "1234567890",
		CustPriCardHolderInd: "Y",
		CustFICOCreditScore:  750,
	}
	if err := custStore.Create(ctx, cust); err != nil {
		t.Fatalf("create customer: %v", err)
	}

	xref := &domain.CardXrefRecord{
		XrefCardNum: "4111111111111111",
		XrefCustID:  1001,
		XrefAcctID:  70000001001,
	}
	if err := xrefStore.Create(ctx, xref); err != nil {
		t.Fatalf("create xref: %v", err)
	}

	s := svc.New(custStore, xrefStore)
	return s, 1001, 70000001001
}

func TestGetByID_HappyPath(t *testing.T) {
	s, custID, _ := openSeeded(t)
	c, err := s.GetByID(context.Background(), custID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if c.CustID != custID {
		t.Errorf("expected CustID=%d, got %d", custID, c.CustID)
	}
	if c.CustFirstName != "John" {
		t.Errorf("expected FirstName=John, got %q", c.CustFirstName)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	s, _, _ := openSeeded(t)
	_, err := s.GetByID(context.Background(), 99999)
	if !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListByAccount_HappyPath(t *testing.T) {
	s, custID, acctID := openSeeded(t)
	custs, err := s.ListByAccount(context.Background(), acctID)
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(custs) != 1 {
		t.Fatalf("expected 1 customer, got %d", len(custs))
	}
	if custs[0].CustID != custID {
		t.Errorf("expected CustID=%d, got %d", custID, custs[0].CustID)
	}
}

func TestListByAccount_NoXref(t *testing.T) {
	s, _, _ := openSeeded(t)
	custs, err := s.ListByAccount(context.Background(), 99999999)
	if err != nil {
		t.Fatalf("ListByAccount empty: %v", err)
	}
	if len(custs) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(custs))
	}
}
