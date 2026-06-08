package parity_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/tranreport"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

// TestParity_CBTRN03C_TranReportMatchesGolden seeds a fresh DB from the EBCDIC
// fixtures, runs the transaction-report step (CBTRN03C / TRANREPT.jcl), and
// compares the output against a golden file.
func TestParity_CBTRN03C_TranReportMatchesGolden(t *testing.T) {
	db := seedTestDB(t)
	ctx := context.Background()

	var out bytes.Buffer
	s, err := tranreport.Run(ctx, tranreport.Config{
		Transactions: sqlite.NewTransactionStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
		TranTypes:    sqlite.NewTranTypeStore(db),
		TranCats:     sqlite.NewTranCatStore(db),
		Out:          &out,
	})
	if err != nil {
		t.Fatalf("tranreport.Run: %v", err)
	}
	t.Logf("CBTRN03C: pages=%d transactions=%d", s.Pages, s.Transactions)

	compareGolden(t, "tran-report.txt", out.String())
}
