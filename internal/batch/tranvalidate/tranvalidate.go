// Package tranvalidate replaces CBTRN01C / first-step of POSTTRAN.jcl (RAU-44).
//
// It reads a sequential flat file of 350-byte DailyTransactionRecords, validates
// each record exists in the card-xref / account / customer chain, and inserts
// valid records into the transactions table. Unlike tranpost it does not update
// account balances or category balances; it is a pure validation-and-insert pass.
package tranvalidate

import (
	"context"
	"fmt"
	"io"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the dependencies for the tran-validate subcommand.
type Config struct {
	Transactions repo.TransactionRepository
	CardXrefs    repo.CardXrefRepository
	Accounts     repo.AccountRepository
	Customers    repo.CustomerRepository
	Cards        repo.CardRepository
	Input        io.Reader // 350-byte record stream
	Out          io.Writer
}

// Summary reports counts of validated and skipped records.
type Summary struct {
	Validated int
	Skipped   int
}

// Run reads daily transactions from cfg.Input, validates each, and inserts
// valid ones into the transactions table.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Input == nil {
		return Summary{}, fmt.Errorf("tranvalidate: Input is required")
	}

	var s Summary
	buf := make([]byte, domain.DailyTransactionRecordLen)
	for {
		_, err := io.ReadFull(cfg.Input, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return s, fmt.Errorf("tranvalidate: read: %w", err)
		}

		dtr, err := domain.DecodeDailyTransactionRecord(buf)
		if err != nil {
			s.Skipped++
			continue
		}

		// Validate card-xref exists.
		xref, err := cfg.CardXrefs.Get(ctx, dtr.DalytranCardNum)
		if err != nil {
			s.Skipped++
			fmt.Fprintf(cfg.Out, "SKIP: card %s not in xref\n", dtr.DalytranCardNum)
			continue
		}

		// Validate account exists.
		if _, err := cfg.Accounts.Get(ctx, xref.XrefAcctID); err != nil {
			s.Skipped++
			fmt.Fprintf(cfg.Out, "SKIP: acct %d not found\n", xref.XrefAcctID)
			continue
		}

		// Validate customer exists.
		if _, err := cfg.Customers.Get(ctx, xref.XrefCustID); err != nil {
			s.Skipped++
			fmt.Fprintf(cfg.Out, "SKIP: cust %d not found\n", xref.XrefCustID)
			continue
		}

		// Insert into transactions table.
		tran := &domain.TransactionRecord{
			TranID:           dtr.DalytranID,
			TranTypeCode:     dtr.DalytranTypeCode,
			TranCatCode:      dtr.DalytranCatCode,
			TranSource:       dtr.DalytranSource,
			TranDesc:         dtr.DalytranDesc,
			TranAmt:          dtr.DalytranAmt,
			TranMerchantID:   dtr.DalytranMerchantID,
			TranMerchantName: dtr.DalytranMerchantName,
			TranMerchantCity: dtr.DalytranMerchantCity,
			TranMerchantZip:  dtr.DalytranMerchantZip,
			TranCardNum:      dtr.DalytranCardNum,
			TranOrigTS:       dtr.DalytranOrigTS,
			TranProcTS:       dtr.DalytranProcTS,
		}
		if err := cfg.Transactions.Create(ctx, tran); err != nil {
			if err == repo.ErrConflict {
				s.Skipped++
				continue
			}
			return s, fmt.Errorf("tranvalidate: insert %s: %w", dtr.DalytranID, err)
		}
		s.Validated++
	}

	fmt.Fprintf(cfg.Out, "\nValidated: %d, Skipped: %d\n", s.Validated, s.Skipped)
	return s, nil
}
