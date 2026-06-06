// Package tranbkp replaces TRANBKP.jcl (RAU-44).
//
// TRANBKP.jcl backs up the VSAM transaction master to a new GDG generation,
// deletes the VSAM cluster, and recreates it empty.
//
// The Go equivalent writes all current transactions to a flat output file
// (350-byte TransactionRecords, in TranID order), then optionally deletes
// all rows from the transactions table (the "reset" step).
package tranbkp

import (
	"context"
	"fmt"
	"io"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the dependencies for the tranbkp subcommand.
type Config struct {
	Transactions repo.TransactionRepository
	Backup       io.Writer // receives 350-byte TransactionRecord stream
	Reset        bool      // if true, delete all transactions after backup
	Log          io.Writer
}

// Summary reports backup counts.
type Summary struct {
	BackedUp int
	Deleted  int
}

// Run backs up all transactions to cfg.Backup (as 350-byte flat records) and,
// if cfg.Reset is true, deletes all transaction rows from the DB.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	if cfg.Backup == nil {
		return Summary{}, fmt.Errorf("tranbkp: Backup writer is required")
	}

	all, err := cfg.Transactions.Browse(ctx, "", 0)
	if err != nil {
		return Summary{}, fmt.Errorf("tranbkp: browse: %w", err)
	}

	var s Summary
	for _, tran := range all {
		b, err := tran.Encode()
		if err != nil {
			return s, fmt.Errorf("tranbkp: encode %s: %w", tran.TranID, err)
		}
		if _, err := cfg.Backup.Write(b); err != nil {
			return s, fmt.Errorf("tranbkp: write backup: %w", err)
		}
		s.BackedUp++
	}
	fmt.Fprintf(cfg.Log, "Backed up %d transactions\n", s.BackedUp)

	if cfg.Reset {
		for _, tran := range all {
			if err := cfg.Transactions.Delete(ctx, tran.TranID); err != nil {
				if err == repo.ErrNotFound {
					continue
				}
				return s, fmt.Errorf("tranbkp: delete %s: %w", tran.TranID, err)
			}
			s.Deleted++
		}
		fmt.Fprintf(cfg.Log, "Deleted %d transactions (reset)\n", s.Deleted)
	}

	return s, nil
}

// Restore reads 350-byte TransactionRecords from r and inserts them into the DB.
// This is the inverse operation used to restore from a backup.
func Restore(ctx context.Context, r io.Reader, transactions repo.TransactionRepository, log io.Writer) (int, error) {
	if log == nil {
		log = io.Discard
	}
	buf := make([]byte, domain.TransactionRecordLen)
	var count int
	for {
		_, err := io.ReadFull(r, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("tranbkp restore: read: %w", err)
		}
		tran, err := domain.DecodeTransactionRecord(buf)
		if err != nil {
			continue
		}
		if err := transactions.Create(ctx, tran); err != nil {
			if err == repo.ErrConflict {
				continue
			}
			return count, fmt.Errorf("tranbkp restore: insert %s: %w", tran.TranID, err)
		}
		count++
	}
	fmt.Fprintf(log, "Restored %d transactions\n", count)
	return count, nil
}
