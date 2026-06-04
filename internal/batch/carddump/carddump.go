// Package carddump replaces CBACT02C / READCARD.jcl (RAU-44).
// It browses the cards table sequentially and writes a formatted report.
package carddump

import (
	"context"
	"fmt"
	"io"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the dependencies for the card-dump subcommand.
type Config struct {
	Cards repo.CardRepository
	Out   io.Writer
}

// Summary reports how many records were processed.
type Summary struct {
	Count int
}

// Run browses all cards and writes a formatted report to cfg.Out.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	var s Summary
	const batchSize = 500
	var startNum string
	for {
		recs, err := cfg.Cards.Browse(ctx, startNum, batchSize)
		if err != nil {
			return s, fmt.Errorf("carddump: browse: %w", err)
		}
		if len(recs) == 0 {
			break
		}
		for _, r := range recs {
			fmt.Fprintf(cfg.Out,
				"CARD-NUM=%-16s ACCT-ID=%-11d CVV=%03d STATUS=%-1s\n"+
					"  NAME=%-50s EXP=%-10s\n",
				r.CardNum, r.CardAcctID, r.CardCVVCode,
				r.CardActiveStatus, r.CardEmbossedName, r.CardExpirationDate,
			)
			s.Count++
			startNum = r.CardNum + "\x01"
		}
		if len(recs) < batchSize {
			break
		}
	}
	fmt.Fprintf(cfg.Out, "\nTotal cards: %d\n", s.Count)
	return s, nil
}
