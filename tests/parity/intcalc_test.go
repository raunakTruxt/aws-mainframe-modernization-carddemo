package parity_test

import (
	"context"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/intcalc"
)

// TestParity_CBACT04C_IntCalcMatchesGolden seeds a fresh DB from the EBCDIC
// fixtures, runs the interest-calculation step (CBACT04C / INTCALC.jcl), and
// compares the resulting account balances against a golden file.
func TestParity_CBACT04C_IntCalcMatchesGolden(t *testing.T) {
	db := seedTestDB(t)
	ctx := context.Background()

	s, err := intcalc.Run(ctx, intcalc.Config{
		DB:       db,
		ParmDate: "2022071800", // YYYYMMDDNN — fixed date for determinism
	})
	if err != nil {
		t.Fatalf("intcalc.Run: %v", err)
	}
	t.Logf("CBACT04C: accounts=%d transactions=%d", s.AccountsProcessed, s.TransactionsWritten)

	got := snapshotAccountBalances(t, ctx, db)
	compareGolden(t, "intcalc.txt", got)
}
