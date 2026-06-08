package custdump

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

func TestRun_basic(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	custStore := sqlite.NewCustomerStore(db)
	if err := custStore.Create(ctx, &domain.CustomerRecord{
		CustID:        1,
		CustFirstName: "JOHN",
		CustLastName:  "DOE",
		CustAddrZip:   "12345",
	}); err != nil {
		t.Fatalf("create customer: %v", err)
	}

	var out bytes.Buffer
	s, err := Run(ctx, Config{Customers: custStore, Out: &out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Count != 1 {
		t.Errorf("count = %d, want 1", s.Count)
	}
	if !strings.Contains(out.String(), "JOHN") {
		t.Errorf("customer name not in output: %s", out.String())
	}
}

func TestRun_empty(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	s, err := Run(context.Background(), Config{Customers: sqlite.NewCustomerStore(db)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Count != 0 {
		t.Errorf("count = %d, want 0", s.Count)
	}
}
