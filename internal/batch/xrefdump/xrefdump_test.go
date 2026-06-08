package xrefdump

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

	xrefStore := sqlite.NewCardXrefStore(db)
	if err := xrefStore.Create(ctx, &domain.CardXrefRecord{
		XrefCardNum: "4111111111111111",
		XrefCustID:  1,
		XrefAcctID:  10000000001,
	}); err != nil {
		t.Fatalf("create xref: %v", err)
	}

	var out bytes.Buffer
	s, err := Run(ctx, Config{CardXrefs: xrefStore, Out: &out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Count != 1 {
		t.Errorf("count = %d, want 1", s.Count)
	}
	if !strings.Contains(out.String(), "4111111111111111") {
		t.Errorf("card number not in output: %s", out.String())
	}
}

func TestRun_empty(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	s, err := Run(context.Background(), Config{CardXrefs: sqlite.NewCardXrefStore(db)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Count != 0 {
		t.Errorf("count = %d, want 0", s.Count)
	}
}
