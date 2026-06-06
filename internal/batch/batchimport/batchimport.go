// Package batchimport replaces CBIMPORT.cbl / CBIMPORT.jcl (RAU-44).
//
// It reads the 500-byte multi-record export file produced by CBEXPORT and
// routes each record to the appropriate output writer by record type:
//
//	C = Customer  → customer output
//	A = Account   → account output
//	X = CardXref  → xref output
//	T = Transaction → transaction output
//	D = Card      → card output
package batchimport

import (
	"context"
	"fmt"
	"io"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the I/O for the import subcommand.
type Config struct {
	Input        io.Reader // 500-byte ExportRecord stream
	Customers    repo.CustomerRepository
	Accounts     repo.AccountRepository
	CardXrefs    repo.CardXrefRepository
	Transactions repo.TransactionRepository
	Cards        repo.CardRepository
	Log          io.Writer // progress messages; nil = discard
}

// Summary reports counts of imported records by type.
type Summary struct {
	Customers    int
	Accounts     int
	CardXrefs    int
	Transactions int
	Cards        int
	Errors       int
}

func (s Summary) Total() int {
	return s.Customers + s.Accounts + s.CardXrefs + s.Transactions + s.Cards
}

// Run reads ExportRecords from cfg.Input and inserts them into the DB.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	if cfg.Input == nil {
		return Summary{}, fmt.Errorf("batchimport: Input is required")
	}

	var s Summary
	buf := make([]byte, domain.ExportRecordLen)

	for {
		_, err := io.ReadFull(cfg.Input, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return s, fmt.Errorf("batchimport: read: %w", err)
		}

		rec, err := domain.DecodeExportRecord(buf)
		if err != nil {
			s.Errors++
			fmt.Fprintf(cfg.Log, "batchimport: decode error: %v\n", err)
			continue
		}

		switch rec.RecType {
		case "C":
			if rec.Customer == nil {
				s.Errors++
				continue
			}
			cust := exportCustomerToDomain(rec.Customer)
			if err := cfg.Customers.Create(ctx, cust); err != nil && err != repo.ErrConflict {
				s.Errors++
				fmt.Fprintf(cfg.Log, "batchimport: insert customer %d: %v\n", rec.Customer.CustID, err)
				continue
			}
			s.Customers++

		case "A":
			if rec.Account == nil {
				s.Errors++
				continue
			}
			acct := exportAccountToDomain(rec.Account)
			if err := cfg.Accounts.Create(ctx, acct); err != nil && err != repo.ErrConflict {
				s.Errors++
				fmt.Fprintf(cfg.Log, "batchimport: insert account %d: %v\n", rec.Account.AcctID, err)
				continue
			}
			s.Accounts++

		case "X":
			if rec.CardXref == nil {
				s.Errors++
				continue
			}
			xref := &domain.CardXrefRecord{
				XrefCardNum: rec.CardXref.XrefCardNum,
				XrefCustID:  rec.CardXref.XrefCustID,
				XrefAcctID:  rec.CardXref.XrefAcctID,
			}
			if err := cfg.CardXrefs.Create(ctx, xref); err != nil && err != repo.ErrConflict {
				s.Errors++
				fmt.Fprintf(cfg.Log, "batchimport: insert xref %s: %v\n", xref.XrefCardNum, err)
				continue
			}
			s.CardXrefs++

		case "T":
			if rec.Transaction == nil {
				s.Errors++
				continue
			}
			tran := exportTransactionToDomain(rec.Transaction)
			if err := cfg.Transactions.Create(ctx, tran); err != nil && err != repo.ErrConflict {
				s.Errors++
				fmt.Fprintf(cfg.Log, "batchimport: insert tran %s: %v\n", tran.TranID, err)
				continue
			}
			s.Transactions++

		case "D":
			if rec.Card == nil {
				s.Errors++
				continue
			}
			card := exportCardToDomain(rec.Card)
			if err := cfg.Cards.Create(ctx, card); err != nil && err != repo.ErrConflict {
				s.Errors++
				fmt.Fprintf(cfg.Log, "batchimport: insert card %s: %v\n", card.CardNum, err)
				continue
			}
			s.Cards++

		default:
			s.Errors++
			fmt.Fprintf(cfg.Log, "batchimport: unknown rec type %q\n", rec.RecType)
		}
	}

	fmt.Fprintf(cfg.Log,
		"Imported: customers=%d accounts=%d xrefs=%d transactions=%d cards=%d errors=%d\n",
		s.Customers, s.Accounts, s.CardXrefs, s.Transactions, s.Cards, s.Errors)
	return s, nil
}

func exportCustomerToDomain(c *domain.ExportCustomerData) *domain.CustomerRecord {
	return &domain.CustomerRecord{
		CustID:               c.CustID,
		CustFirstName:        c.FirstName,
		CustMiddleName:       c.MiddleName,
		CustLastName:         c.LastName,
		CustAddrLine1:        c.AddrLine1,
		CustAddrLine2:        c.AddrLine2,
		CustAddrLine3:        c.AddrLine3,
		CustAddrStateCode:    c.AddrStateCode,
		CustAddrCountryCode:  c.AddrCountryCode,
		CustAddrZip:          c.AddrZip,
		CustPhoneNum1:        c.PhoneNum1,
		CustPhoneNum2:        c.PhoneNum2,
		CustSSN:              c.SSN,
		CustGovtIssuedID:     c.GovtIssuedID,
		CustDOB:              c.DOB,
		CustEFTAccountID:     c.EFTAccountID,
		CustPriCardHolderInd: c.PriCardHolderInd,
		CustFICOCreditScore:  c.FICOCreditScore.IntPart(),
	}
}

func exportAccountToDomain(a *domain.ExportAccountData) *domain.AccountRecord {
	return &domain.AccountRecord{
		AcctID:              a.AcctID,
		AcctActiveStatus:    a.AcctActiveStatus,
		AcctCurrBal:         a.AcctCurrBal,
		AcctCreditLimit:     a.AcctCreditLimit,
		AcctCashCreditLimit: a.AcctCashCredit,
		AcctOpenDate:        a.AcctOpenDate,
		AcctExpirationDate:  a.AcctExpiryDate,
		AcctReissueDate:     a.AcctReissueDate,
		AcctCurrCycCredit:   a.AcctCurrCycCredit,
		AcctCurrCycDebit:    a.AcctCurrCycDebit,
		AcctAddrZip:         a.AcctAddrZip,
		AcctGroupID:         a.AcctGroupID,
	}
}

func exportTransactionToDomain(t *domain.ExportTransactionData) *domain.TransactionRecord {
	return &domain.TransactionRecord{
		TranID:           t.TranID,
		TranTypeCode:     t.TranTypeCode,
		TranCatCode:      t.TranCatCode,
		TranSource:       t.TranSource,
		TranDesc:         t.TranDesc,
		TranAmt:          t.TranAmt,
		TranMerchantID:   t.TranMerchantID,
		TranMerchantName: t.MerchantName,
		TranMerchantCity: t.MerchantCity,
		TranMerchantZip:  t.MerchantZip,
		TranCardNum:      t.TranCardNum,
		TranOrigTS:       t.TranOrigTS,
		TranProcTS:       t.TranProcTS,
	}
}

func exportCardToDomain(c *domain.ExportCardData) *domain.CardRecord {
	return &domain.CardRecord{
		CardNum:            c.CardNum,
		CardAcctID:         c.CardAcctID,
		CardCVVCode:        c.CardCVVCode,
		CardEmbossedName:   c.CardEmbossed,
		CardExpirationDate: c.CardExpiryDate,
		CardActiveStatus:   c.CardActive,
	}
}
