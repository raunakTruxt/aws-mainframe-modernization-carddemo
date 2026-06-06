// Package intcalc replaces CBACT04C / INTCALC.jcl (RAU-44).
//
// It reads the transaction-category-balance table grouped by account,
// computes monthly interest using the discount-group rates, writes an interest
// transaction per category, and updates each account's current balance.
//
// COBOL formula: WS-MONTHLY-INT = (TRAN-CAT-BAL * DIS-INT-RATE) / 1200
// where DIS-INT-RATE is an annual percentage (e.g. 24.99 for 24.99% APR).
package intcalc

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

var twelve = decimal.NewFromInt(1200)

// discGroupDefault is the COBOL fallback group key (CVTRA02Y.cpy:6, X(10) space-padded).
const discGroupDefault = "DEFAULT   "

// Config holds the dependencies for the intcalc subcommand.
type Config struct {
	DB       *sql.DB
	ParmDate string    // YYYYMMDDNN (e.g. "2022071800"); defaults to today +"00"
	Now      time.Time // injected for tests; zero means time.Now()
	Out      io.Writer
}

// Summary reports counts of processed records.
type Summary struct {
	AccountsProcessed   int
	TransactionsWritten int
}

// Run computes monthly interest for every account that has category-balance
// rows, writes one interest transaction per category per account to the
// transactions table, and updates AcctCurrBal / zeroes cycle counters.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.DB == nil {
		return Summary{}, fmt.Errorf("intcalc: DB is required")
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	parmDate := cfg.ParmDate
	if parmDate == "" {
		parmDate = now.Format("20060102") + "00"
	}

	tcatbalStore := sqlite.NewTranCatBalStore(cfg.DB)
	discStore := sqlite.NewDiscGroupStore(cfg.DB)
	acctStore := sqlite.NewAccountStore(cfg.DB)
	xrefStore := sqlite.NewCardXrefStore(cfg.DB)
	tranStore := sqlite.NewTransactionStore(cfg.DB)

	// Load all tran-cat-bal rows ordered by account id.
	allRows, err := tcatbalStore.List(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("intcalc: list tcatbal: %w", err)
	}

	// Group by account ID.
	type group struct {
		acctID int64
		rows   []*domain.TranCatBalRecord
	}
	var groups []group
	for _, row := range allRows {
		if len(groups) == 0 || groups[len(groups)-1].acctID != row.TrancatAcctID {
			groups = append(groups, group{acctID: row.TrancatAcctID})
		}
		groups[len(groups)-1].rows = append(groups[len(groups)-1].rows, row)
	}

	var s Summary
	tranIDSuffix := 0

	for _, g := range groups {
		acct, err := acctStore.Get(ctx, g.acctID)
		if err != nil {
			fmt.Fprintf(cfg.Out, "intcalc: ACCOUNT NOT FOUND: %d\n", g.acctID)
			continue
		}

		// Look up the card xref to get a card number for the interest transaction.
		xrefs, err := xrefStore.GetByAccount(ctx, g.acctID)
		cardNum := ""
		if err == nil && len(xrefs) > 0 {
			cardNum = xrefs[0].XrefCardNum
		}

		totalInterest := decimal.Zero

		for _, row := range g.rows {
			dgRec, err := discStore.Get(ctx, acct.AcctGroupID, row.TrancatTypeCD, row.TrancatCode)
			if err != nil {
				// Try default group as fallback (COBOL 1200-A-GET-DEFAULT-INT-RATE).
				dgRec, err = discStore.Get(ctx, discGroupDefault, row.TrancatTypeCD, row.TrancatCode)
				if err != nil {
					continue // no rate configured, skip this category
				}
			}
			if dgRec.DisIntRate.IsZero() {
				continue
			}

			// monthlyInt = (catBal * annualRate) / 1200 — COBOL truncates (no ROUNDED, CBACT04C.cbl:464-465).
			monthlyInt := row.TranCatBalance.Mul(dgRec.DisIntRate).Div(twelve).Truncate(2)
			totalInterest = totalInterest.Add(monthlyInt)

			// Write one interest transaction per category.
			tranIDSuffix++
			ts := now.Format("2006-01-02 15:04:05.000000")
			tranID := buildTranID(parmDate, tranIDSuffix)
			tran := &domain.TransactionRecord{
				TranID:           tranID,
				TranTypeCode:     "01",
				TranCatCode:      5,
				TranSource:       "System",
				TranDesc:         truncate(fmt.Sprintf("Int. for a/c %d", g.acctID), 100),
				TranAmt:          monthlyInt,
				TranMerchantID:   0,
				TranMerchantName: "",
				TranMerchantCity: "",
				TranMerchantZip:  "",
				TranCardNum:      cardNum,
				TranOrigTS:       ts,
				TranProcTS:       ts,
			}
			if err := tranStore.Create(ctx, tran); err != nil {
				if err == repo.ErrConflict {
					continue // idempotent on re-run
				}
				return s, fmt.Errorf("intcalc: write transaction: %w", err)
			}
			s.TransactionsWritten++
			fmt.Fprintf(cfg.Out, "INT TRAN: acct=%d type=%s cat=%d amt=%s\n",
				g.acctID, row.TrancatTypeCD, row.TrancatCode, monthlyInt.StringFixed(2))
		}

		if totalInterest.IsZero() {
			continue
		}

		// Update account: add total interest to current balance; zero cycle accumulators.
		acct.AcctCurrBal = acct.AcctCurrBal.Add(totalInterest)
		acct.AcctCurrCycCredit = decimal.Zero
		acct.AcctCurrCycDebit = decimal.Zero
		if err := acctStore.Update(ctx, acct); err != nil {
			return s, fmt.Errorf("intcalc: update account %d: %w", g.acctID, err)
		}
		s.AccountsProcessed++
		fmt.Fprintf(cfg.Out, "UPDATED ACCT %d: total-interest=%s new-bal=%s\n",
			g.acctID, totalInterest.StringFixed(2), acct.AcctCurrBal.StringFixed(2))
	}

	fmt.Fprintf(cfg.Out, "\nAccounts updated: %d, Interest transactions written: %d\n",
		s.AccountsProcessed, s.TransactionsWritten)
	return s, nil
}

// buildTranID creates a transaction ID matching the COBOL pattern:
// STRING PARM-DATE, WS-TRANID-SUFFIX into TRAN-ID (16 chars total).
func buildTranID(parmDate string, suffix int) string {
	raw := fmt.Sprintf("%s%06d", parmDate, suffix)
	if len(raw) > 16 {
		raw = raw[:16]
	}
	return raw + strings.Repeat(" ", 16-len(raw))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
