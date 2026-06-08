package parity_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/tranpost"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

// TestParity_CBTRN02C_PostingMatchesGolden seeds a fresh DB from the canonical
// EBCDIC sample data, runs the tran-post step (CBTRN02C / POSTTRAN.jcl), then
// compares the resulting account balances against a checked-in golden file.
func TestParity_CBTRN02C_PostingMatchesGolden(t *testing.T) {
	db := seedTestDB(t)
	ctx := context.Background()

	dalytranPath := dataDir() + "/AWS.M2.CARDDEMO.DALYTRAN.PS"
	input, err := os.ReadFile(dalytranPath)
	if err != nil {
		t.Fatalf("read DALYTRAN: %v", err)
	}

	var rejects bytes.Buffer
	fixedNow := time.Date(2022, 7, 18, 12, 0, 0, 0, time.UTC)
	s, err := tranpost.Run(ctx, tranpost.Config{
		DB:      db,
		Input:   bytes.NewReader(input),
		Rejects: &rejects,
		Now:     fixedNow,
	})
	if err != nil {
		t.Fatalf("tranpost.Run: %v", err)
	}
	t.Logf("CBTRN02C: posted=%d rejected=%d", s.Posted, s.Rejected)

	got := snapshotAccountBalances(t, ctx, db)
	compareGolden(t, "post.txt", got)
}

// snapshotAccountBalances returns a deterministic text snapshot of every
// account's current balance, suitable for golden-file comparison.
func snapshotAccountBalances(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	acctStore := sqlite.NewAccountStore(db)
	var sb strings.Builder
	startID := int64(0)
	for {
		recs, err := acctStore.Browse(ctx, startID, 500)
		if err != nil {
			t.Fatalf("browse accounts: %v", err)
		}
		if len(recs) == 0 {
			break
		}
		for _, r := range recs {
			fmt.Fprintf(&sb, "ACCT-ID=%011d BAL=%s CREDIT-LIMIT=%s\n",
				r.AcctID,
				r.AcctCurrBal.StringFixed(2),
				r.AcctCreditLimit.StringFixed(2))
		}
		startID = recs[len(recs)-1].AcctID + 1
		if len(recs) < 500 {
			break
		}
	}
	return sb.String()
}
