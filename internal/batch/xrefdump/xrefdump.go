// Package xrefdump replaces CBACT03C / READXREF.jcl (RAU-44).
// It browses the card_xrefs table sequentially and writes a formatted report.
package xrefdump

import (
	"context"
	"fmt"
	"io"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the dependencies for the xref-dump subcommand.
type Config struct {
	CardXrefs repo.CardXrefRepository
	Out       io.Writer
}

// Summary reports how many records were processed.
type Summary struct {
	Count int
}

// Run browses all card-xref records and writes a formatted report to cfg.Out.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	var s Summary
	const batchSize = 500
	var startNum string
	for {
		recs, err := cfg.CardXrefs.Browse(ctx, startNum, batchSize)
		if err != nil {
			return s, fmt.Errorf("xrefdump: browse: %w", err)
		}
		if len(recs) == 0 {
			break
		}
		for _, r := range recs {
			fmt.Fprintf(cfg.Out,
				"XREF-CARD-NUM=%-16s CUST-ID=%-9d ACCT-ID=%-11d\n",
				r.XrefCardNum, r.XrefCustID, r.XrefAcctID,
			)
			s.Count++
			startNum = r.XrefCardNum + "\x01"
		}
		if len(recs) < batchSize {
			break
		}
	}
	fmt.Fprintf(cfg.Out, "\nTotal xref records: %d\n", s.Count)
	return s, nil
}
