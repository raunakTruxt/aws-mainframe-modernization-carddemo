// Package export replaces CBEXPORT.cbl / CBEXPORT.jcl (RAU-44).
//
// It reads all customers, accounts, card-xrefs, transactions, and cards from
// the SQLite database and writes 500-byte ExportRecord entries to an output
// writer in the CVEXPORT format. Record types are:
//
//	C = Customer
//	A = Account
//	X = CardXref
//	T = Transaction
//	D = Card
package export

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

// Config holds the dependencies for the export subcommand.
type Config struct {
	Customers    repo.CustomerRepository
	Accounts     repo.AccountRepository
	CardXrefs    repo.CardXrefRepository
	Transactions repo.TransactionRepository
	Cards        repo.CardRepository
	BranchID     string // EXPORT-BRANCH-ID (4 chars)
	RegionCode   string // EXPORT-REGION-CODE (5 chars)
	Out          io.Writer // receives raw 500-byte records
	Log          io.Writer // progress messages; nil = discard
	Now          time.Time // injected for tests; zero means time.Now()
}

// Summary reports counts of exported records by type.
type Summary struct {
	Customers    int
	Accounts     int
	CardXrefs    int
	Transactions int
	Cards        int
}

func (s Summary) Total() int {
	return s.Customers + s.Accounts + s.CardXrefs + s.Transactions + s.Cards
}

// Run exports all entities to cfg.Out as 500-byte fixed records.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	if cfg.Out == nil {
		return Summary{}, fmt.Errorf("export: Out is required")
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ts := now.Format("2006-01-02 15:04:05.000000")

	var s Summary
	seq := int64(1)

	// Helper: encode and write one ExportRecord.
	write := func(rec *domain.ExportRecord) error {
		rec.Timestamp = ts
		rec.SequenceNum = seq
		rec.BranchID = cfg.BranchID
		rec.RegionCode = cfg.RegionCode
		seq++
		b, err := rec.Encode()
		if err != nil {
			return err
		}
		_, err = cfg.Out.Write(b)
		return err
	}

	// Customers.
	if err := browseCustomers(ctx, cfg.Customers, func(c *domain.CustomerRecord) error {
		rec := customerToExport(c)
		if err := write(rec); err != nil {
			return err
		}
		s.Customers++
		return nil
	}); err != nil {
		return s, fmt.Errorf("export: customers: %w", err)
	}

	// Accounts.
	if err := browseAccounts(ctx, cfg.Accounts, func(a *domain.AccountRecord) error {
		rec := accountToExport(a)
		if err := write(rec); err != nil {
			return err
		}
		s.Accounts++
		return nil
	}); err != nil {
		return s, fmt.Errorf("export: accounts: %w", err)
	}

	// Card-xrefs.
	if err := browseXrefs(ctx, cfg.CardXrefs, func(x *domain.CardXrefRecord) error {
		rec := xrefToExport(x)
		if err := write(rec); err != nil {
			return err
		}
		s.CardXrefs++
		return nil
	}); err != nil {
		return s, fmt.Errorf("export: xrefs: %w", err)
	}

	// Transactions.
	if err := browseTransactions(ctx, cfg.Transactions, func(t *domain.TransactionRecord) error {
		rec := transactionToExport(t)
		if err := write(rec); err != nil {
			return err
		}
		s.Transactions++
		return nil
	}); err != nil {
		return s, fmt.Errorf("export: transactions: %w", err)
	}

	// Cards.
	if err := browseCards(ctx, cfg.Cards, func(c *domain.CardRecord) error {
		rec := cardToExport(c)
		if err := write(rec); err != nil {
			return err
		}
		s.Cards++
		return nil
	}); err != nil {
		return s, fmt.Errorf("export: cards: %w", err)
	}

	fmt.Fprintf(cfg.Log,
		"Exported: customers=%d accounts=%d xrefs=%d transactions=%d cards=%d total=%d\n",
		s.Customers, s.Accounts, s.CardXrefs, s.Transactions, s.Cards, s.Total())
	return s, nil
}

// browseCustomers iterates all customers in batches.
func browseCustomers(ctx context.Context, r repo.CustomerRepository, fn func(*domain.CustomerRecord) error) error {
	const batchSize = 500
	var startID int64
	for {
		recs, err := r.Browse(ctx, startID, batchSize)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			if err := fn(rec); err != nil {
				return err
			}
			startID = rec.CustID + 1
		}
		if len(recs) < batchSize {
			break
		}
	}
	return nil
}

func browseAccounts(ctx context.Context, r repo.AccountRepository, fn func(*domain.AccountRecord) error) error {
	const batchSize = 500
	var startID int64
	for {
		recs, err := r.Browse(ctx, startID, batchSize)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			if err := fn(rec); err != nil {
				return err
			}
			startID = rec.AcctID + 1
		}
		if len(recs) < batchSize {
			break
		}
	}
	return nil
}

func browseXrefs(ctx context.Context, r repo.CardXrefRepository, fn func(*domain.CardXrefRecord) error) error {
	const batchSize = 500
	var startNum string
	for {
		recs, err := r.Browse(ctx, startNum, batchSize)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			if err := fn(rec); err != nil {
				return err
			}
			startNum = rec.XrefCardNum + "\x01"
		}
		if len(recs) < batchSize {
			break
		}
	}
	return nil
}

func browseTransactions(ctx context.Context, r repo.TransactionRepository, fn func(*domain.TransactionRecord) error) error {
	const batchSize = 500
	var startID string
	for {
		recs, err := r.Browse(ctx, startID, batchSize)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			if err := fn(rec); err != nil {
				return err
			}
			startID = rec.TranID + "\x01"
		}
		if len(recs) < batchSize {
			break
		}
	}
	return nil
}

func browseCards(ctx context.Context, r repo.CardRepository, fn func(*domain.CardRecord) error) error {
	const batchSize = 500
	var startNum string
	for {
		recs, err := r.Browse(ctx, startNum, batchSize)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			if err := fn(rec); err != nil {
				return err
			}
			startNum = rec.CardNum + "\x01"
		}
		if len(recs) < batchSize {
			break
		}
	}
	return nil
}

func customerToExport(c *domain.CustomerRecord) *domain.ExportRecord {
	return &domain.ExportRecord{
		RecType: "C",
		Customer: &domain.ExportCustomerData{
			CustID:           c.CustID,
			FirstName:        c.CustFirstName,
			MiddleName:       c.CustMiddleName,
			LastName:         c.CustLastName,
			AddrLine1:        c.CustAddrLine1,
			AddrLine2:        c.CustAddrLine2,
			AddrLine3:        c.CustAddrLine3,
			AddrStateCode:    c.CustAddrStateCode,
			AddrCountryCode:  c.CustAddrCountryCode,
			AddrZip:          c.CustAddrZip,
			PhoneNum1:        c.CustPhoneNum1,
			PhoneNum2:        c.CustPhoneNum2,
			SSN:              c.CustSSN,
			GovtIssuedID:     c.CustGovtIssuedID,
			DOB:              c.CustDOB,
			EFTAccountID:     c.CustEFTAccountID,
			PriCardHolderInd: c.CustPriCardHolderInd,
			FICOCreditScore:  decimal.NewFromInt(c.CustFICOCreditScore),
		},
	}
}

func accountToExport(a *domain.AccountRecord) *domain.ExportRecord {
	return &domain.ExportRecord{
		RecType: "A",
		Account: &domain.ExportAccountData{
			AcctID:            a.AcctID,
			AcctActiveStatus:  a.AcctActiveStatus,
			AcctCurrBal:       a.AcctCurrBal,
			AcctCreditLimit:   a.AcctCreditLimit,
			AcctCashCredit:    a.AcctCashCreditLimit,
			AcctOpenDate:      a.AcctOpenDate,
			AcctExpiryDate:    a.AcctExpirationDate,
			AcctReissueDate:   a.AcctReissueDate,
			AcctCurrCycCredit: a.AcctCurrCycCredit,
			AcctCurrCycDebit:  a.AcctCurrCycDebit,
			AcctAddrZip:       a.AcctAddrZip,
			AcctGroupID:       a.AcctGroupID,
		},
	}
}

func xrefToExport(x *domain.CardXrefRecord) *domain.ExportRecord {
	return &domain.ExportRecord{
		RecType: "X",
		CardXref: &domain.ExportCardXrefData{
			XrefCardNum: x.XrefCardNum,
			XrefCustID:  x.XrefCustID,
			XrefAcctID:  x.XrefAcctID,
		},
	}
}

func transactionToExport(t *domain.TransactionRecord) *domain.ExportRecord {
	return &domain.ExportRecord{
		RecType: "T",
		Transaction: &domain.ExportTransactionData{
			TranID:         t.TranID,
			TranTypeCode:   t.TranTypeCode,
			TranCatCode:    t.TranCatCode,
			TranSource:     t.TranSource,
			TranDesc:       t.TranDesc,
			TranAmt:        t.TranAmt,
			TranMerchantID: t.TranMerchantID,
			MerchantName:   t.TranMerchantName,
			MerchantCity:   t.TranMerchantCity,
			MerchantZip:    t.TranMerchantZip,
			TranCardNum:    t.TranCardNum,
			TranOrigTS:     t.TranOrigTS,
			TranProcTS:     t.TranProcTS,
		},
	}
}

func cardToExport(c *domain.CardRecord) *domain.ExportRecord {
	return &domain.ExportRecord{
		RecType: "D",
		Card: &domain.ExportCardData{
			CardNum:      c.CardNum,
			CardAcctID:   c.CardAcctID,
			CardCVVCode:  c.CardCVVCode,
			CardEmbossed: c.CardEmbossedName,
			CardExpiryDate: c.CardExpirationDate,
			CardActive:   c.CardActiveStatus,
		},
	}
}
