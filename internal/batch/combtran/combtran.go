// Package combtran replaces COMBTRAN.jcl (RAU-44).
//
// COMBTRAN.jcl combines two transaction flat files (a GDG backup + a
// system-generated batch), sorts by TRAN-ID, and repopulates the VSAM master.
//
// The Go equivalent reads transactions from two sources (DB + flat file),
// merges and deduplicates by TranID, and writes the combined set back to the DB.
// If both readers are nil, it is a no-op (idempotent empty run).
package combtran

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the dependencies for the combtran subcommand.
type Config struct {
	// Primary is the existing transaction repository (replaces backup GDG).
	Primary repo.TransactionRepository
	// Secondary is an optional flat-file reader of 350-byte TransactionRecords
	// (replaces the system-generated transactions input).
	Secondary io.Reader
	// Target receives the merged result (usually the same repo as Primary
	// after a wipe, or a different one for testing).
	Target repo.TransactionRepository
	Out    io.Writer
}

// Summary reports merge counts.
type Summary struct {
	FromPrimary   int
	FromSecondary int
	Written       int
	Duplicates    int
}

// Run merges transactions from primary + secondary, deduplicates by TranID,
// sorts by TranID, and writes to target.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}

	seen := map[string]*domain.TransactionRecord{}

	// Read from primary.
	var s Summary
	if cfg.Primary != nil {
		all, err := cfg.Primary.Browse(ctx, "", 0)
		if err != nil {
			return s, fmt.Errorf("combtran: read primary: %w", err)
		}
		for _, t := range all {
			seen[t.TranID] = t
			s.FromPrimary++
		}
	}

	// Read from secondary flat file (350-byte records).
	if cfg.Secondary != nil {
		buf := make([]byte, domain.TransactionRecordLen)
		for {
			_, err := io.ReadFull(cfg.Secondary, buf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				return s, fmt.Errorf("combtran: read secondary: %w", err)
			}
			tran, err := domain.DecodeTransactionRecord(buf)
			if err != nil {
				continue
			}
			if _, dup := seen[tran.TranID]; dup {
				s.Duplicates++
				continue
			}
			seen[tran.TranID] = tran
			s.FromSecondary++
		}
	}

	// Sort by TranID.
	merged := make([]*domain.TransactionRecord, 0, len(seen))
	for _, t := range seen {
		merged = append(merged, t)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].TranID < merged[j].TranID
	})

	// Write to target.
	if cfg.Target != nil {
		for _, tran := range merged {
			if err := cfg.Target.Create(ctx, tran); err != nil {
				if err == repo.ErrConflict {
					continue
				}
				return s, fmt.Errorf("combtran: write %s: %w", tran.TranID, err)
			}
			s.Written++
		}
	} else {
		s.Written = len(merged)
	}

	fmt.Fprintf(cfg.Out, "Combined: primary=%d secondary=%d duplicates=%d written=%d\n",
		s.FromPrimary, s.FromSecondary, s.Duplicates, s.Written)
	return s, nil
}
