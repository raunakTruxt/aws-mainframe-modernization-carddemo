// Package acctdump replaces CBACT01C / READACCT.jcl (RAU-44).
// It browses the account table sequentially and writes a formatted report.
package acctdump

import (
	"context"
	"fmt"
	"io"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the dependencies for the acct-dump subcommand.
type Config struct {
	Accounts repo.AccountRepository
	Out      io.Writer
}

// Summary reports how many records were processed.
type Summary struct {
	Count int
}

// Run browses all accounts and writes a formatted report to cfg.Out.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	var s Summary
	const batchSize = 500
	var startID int64
	for {
		recs, err := cfg.Accounts.Browse(ctx, startID, batchSize)
		if err != nil {
			return s, fmt.Errorf("acctdump: browse: %w", err)
		}
		if len(recs) == 0 {
			break
		}
		for _, r := range recs {
			fmt.Fprintf(cfg.Out,
				"ACCT-ID=%-11d STATUS=%-1s BAL=%14s LIMIT=%14s CASH-LIMIT=%14s\n"+
					"  OPEN=%-10s EXP=%-10s REISSUE=%-10s ZIP=%-10s GROUP=%-10s\n"+
					"  CYC-CREDIT=%12s CYC-DEBIT=%12s\n",
				r.AcctID, r.AcctActiveStatus,
				r.AcctCurrBal.StringFixed(2), r.AcctCreditLimit.StringFixed(2), r.AcctCashCreditLimit.StringFixed(2),
				r.AcctOpenDate, r.AcctExpirationDate, r.AcctReissueDate,
				r.AcctAddrZip, r.AcctGroupID,
				r.AcctCurrCycCredit.StringFixed(2), r.AcctCurrCycDebit.StringFixed(2),
			)
			s.Count++
			startID = r.AcctID + 1
		}
		if len(recs) < batchSize {
			break
		}
	}
	fmt.Fprintf(cfg.Out, "\nTotal accounts: %d\n", s.Count)
	return s, nil
}
