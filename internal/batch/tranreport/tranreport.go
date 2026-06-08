// Package tranreport replaces CBTRN03C / TRANREPT.jcl (RAU-44).
//
// It browses the transactions table (optionally filtered by date range),
// resolves type and category descriptions, and writes a formatted 133-char
// wide report to the output writer with page breaks, account subtotals,
// and a grand total.
package tranreport

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

const (
	pageWidth = 133
	pageLines = 20
)

// Config holds the dependencies for the tran-report subcommand.
type Config struct {
	Transactions repo.TransactionRepository
	CardXrefs    repo.CardXrefRepository
	TranTypes    repo.TranTypeRepository
	TranCats     repo.TranCatRepository
	DateFrom     string // YYYY-MM-DD optional; filter procTS >= DateFrom (CBTRN03C.cbl:173)
	DateTo       string // YYYY-MM-DD optional; filter procTS <= DateTo
	Out          io.Writer
}

// Summary reports counts of pages and transactions written.
type Summary struct {
	Pages        int
	Transactions int
}

// Run produces a formatted transaction report on cfg.Out.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}

	// Build lookup caches for tran-type and tran-cat descriptions.
	typeDescs := map[string]string{}
	catDescs := map[string]string{}

	if tts, err := cfg.TranTypes.List(ctx); err == nil {
		for _, t := range tts {
			typeDescs[t.TranType] = t.TranTypeDesc
		}
	}
	if cats, err := cfg.TranCats.List(ctx); err == nil {
		for _, c := range cats {
			catDescs[fmt.Sprintf("%s:%d", c.TranTypeCode, c.TranCatCode)] = c.TranCatTypeDesc
		}
	}

	// Load all transactions, then filter by date range.
	all, err := cfg.Transactions.Browse(ctx, "", 0)
	if err != nil {
		return Summary{}, fmt.Errorf("tranreport: browse transactions: %w", err)
	}

	var filtered []*domain.TransactionRecord
	for _, t := range all {
		procDate := ""
		if len(t.TranProcTS) >= 10 {
			procDate = t.TranProcTS[:10]
		}
		if cfg.DateFrom != "" && procDate < cfg.DateFrom {
			continue
		}
		if cfg.DateTo != "" && procDate > cfg.DateTo {
			continue
		}
		filtered = append(filtered, t)
	}

	var s Summary
	var lineCount int
	var grandTotal decimal.Decimal

	printHeader := func(page int) {
		fmt.Fprintf(cfg.Out, "%s\n", centerPad("CARDDEMO TRANSACTION REPORT", pageWidth))
		fmt.Fprintf(cfg.Out, "%s\n", centerPad(fmt.Sprintf("Page %d", page), pageWidth))
		fmt.Fprintf(cfg.Out, "%s\n", strings.Repeat("-", pageWidth))
		fmt.Fprintf(cfg.Out, "%-16s %-2s %-4s %-16s %-26s %11s  %-40s\n",
			"TRAN-ID", "TY", "CAT", "CARD-NUM", "PROC-TS", "AMOUNT", "DESCRIPTION")
		fmt.Fprintf(cfg.Out, "%s\n", strings.Repeat("-", pageWidth))
		lineCount = 5
	}

	s.Pages = 1
	printHeader(s.Pages)

	// Group by card number for subtotals (mirrors COBOL: breaks on TRNX-CARD-NUM).
	var lastCardNum string
	var cardTotal decimal.Decimal

	flushCardSubtotal := func() {
		if lastCardNum != "" {
			fmt.Fprintf(cfg.Out, "%s\n", strings.Repeat("-", pageWidth))
			fmt.Fprintf(cfg.Out, "  Card %-16s SUBTOTAL: %11s\n",
				lastCardNum, cardTotal.StringFixed(2))
			lineCount += 2
		}
	}

	for _, t := range filtered {
		if lineCount >= pageLines {
			flushCardSubtotal()
			s.Pages++
			fmt.Fprintf(cfg.Out, "\f")
			printHeader(s.Pages)
			lastCardNum = ""
			cardTotal = decimal.Zero
		}

		if t.TranCardNum != lastCardNum {
			if lastCardNum != "" {
				flushCardSubtotal()
				lineCount++
			}
			lastCardNum = t.TranCardNum
			cardTotal = decimal.Zero
		}

		typeDesc := typeDescs[strings.TrimSpace(t.TranTypeCode)]
		if typeDesc == "" {
			typeDesc = t.TranTypeCode
		}
		catKey := fmt.Sprintf("%s:%d", strings.TrimSpace(t.TranTypeCode), t.TranCatCode)
		catDesc := catDescs[catKey]
		if catDesc == "" {
			catDesc = fmt.Sprintf("CAT%04d", t.TranCatCode)
		}

		desc := t.TranDesc
		if desc == "" {
			desc = catDesc
		}
		if len(desc) > 40 {
			desc = desc[:40]
		}

		procTS := t.TranProcTS
		if len(procTS) > 26 {
			procTS = procTS[:26]
		}

		fmt.Fprintf(cfg.Out, "%-16s %-2s %4d %-16s %-26s %11s  %-40s\n",
			strings.TrimSpace(t.TranID),
			strings.TrimSpace(t.TranTypeCode),
			t.TranCatCode,
			t.TranCardNum,
			procTS,
			t.TranAmt.StringFixed(2),
			desc,
		)

		cardTotal = cardTotal.Add(t.TranAmt)
		grandTotal = grandTotal.Add(t.TranAmt)
		lineCount++
		s.Transactions++
	}

	flushCardSubtotal()
	fmt.Fprintf(cfg.Out, "%s\n", strings.Repeat("=", pageWidth))
	fmt.Fprintf(cfg.Out, "  GRAND TOTAL: %11s  (%d transactions)\n",
		grandTotal.StringFixed(2), s.Transactions)

	return s, nil
}

func centerPad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-pad-len(s))
}
