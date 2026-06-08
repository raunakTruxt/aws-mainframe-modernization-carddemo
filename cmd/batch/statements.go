// statements subcommand implements CBSTM03A / CREASTMT.JCL.
// It reads all accounts, customers, card-xrefs, and transactions from the
// SQLite database and writes one 80-char statement file per account.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/report"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

func cmdStatements(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("statements", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	outDir := fs.String("out", "out/statements", "output directory root")
	period := fs.String("period", "current", "statement period YYYY-MM; names the output subdirectory")
	acctID := fs.String("account", "", "limit to a single account ID (default: all)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	in, err := buildReportInput(ctx, db)
	if err != nil {
		return err
	}

	gen := report.NewTextGenerator()
	destDir := filepath.Join(*outDir, *period)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	count := 0
	for _, acct := range in.Accounts {
		acctIDStr := fmt.Sprintf("%d", acct.AcctID)
		if *acctID != "" && *acctID != acctIDStr {
			continue
		}
		path := filepath.Join(destDir, acctIDStr+".txt")
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create %s: %w", path, err)
		}
		spec := report.ReportSpec{
			Kind:      report.KindStatement,
			AccountID: acctIDStr,
			Period:    *period,
		}
		if err := gen.Generate(ctx, spec, in, f); err != nil {
			f.Close()
			return fmt.Errorf("generate %s: %w", path, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close %s: %w", path, err)
		}
		count++
	}
	fmt.Fprintf(os.Stderr, "statements: wrote %d statement(s) to %s\n", count, destDir)
	return nil
}

// buildReportInput loads all entities from db that the report generator needs.
func buildReportInput(ctx context.Context, db *sql.DB) (report.Input, error) {
	var in report.Input

	acctStore := sqlite.NewAccountStore(db)
	startAcct := int64(0)
	for {
		recs, err := acctStore.Browse(ctx, startAcct, 500)
		if err != nil {
			return in, fmt.Errorf("browse accounts: %w", err)
		}
		for _, r := range recs {
			in.Accounts = append(in.Accounts, report.AccountRow{
				AcctID:  r.AcctID,
				CurrBal: r.AcctCurrBal.InexactFloat64(),
			})
		}
		if len(recs) < 500 {
			break
		}
		startAcct = recs[len(recs)-1].AcctID + 1
	}

	custStore := sqlite.NewCustomerStore(db)
	startCust := int64(0)
	for {
		recs, err := custStore.Browse(ctx, startCust, 500)
		if err != nil {
			return in, fmt.Errorf("browse customers: %w", err)
		}
		for _, r := range recs {
			in.Customers = append(in.Customers, report.CustomerRow{
				CustID:      r.CustID,
				FirstName:   r.CustFirstName,
				MiddleName:  r.CustMiddleName,
				LastName:    r.CustLastName,
				AddrLine1:   r.CustAddrLine1,
				AddrLine2:   r.CustAddrLine2,
				AddrLine3:   r.CustAddrLine3,
				AddrStateCD: r.CustAddrStateCode,
				CountryCode: r.CustAddrCountryCode,
				Zip:         r.CustAddrZip,
				FICOScore:   r.CustFICOCreditScore,
			})
		}
		if len(recs) < 500 {
			break
		}
		startCust = recs[len(recs)-1].CustID + 1
	}

	xrefStore := sqlite.NewCardXrefStore(db)
	startXref := ""
	for {
		recs, err := xrefStore.Browse(ctx, startXref, 500)
		if err != nil {
			return in, fmt.Errorf("browse card_xrefs: %w", err)
		}
		for _, r := range recs {
			in.CardXrefs = append(in.CardXrefs, report.CardXrefRow{
				CardNum: r.XrefCardNum,
				CustID:  r.XrefCustID,
				AcctID:  r.XrefAcctID,
			})
		}
		if len(recs) < 500 {
			break
		}
		startXref = recs[len(recs)-1].XrefCardNum
	}

	tranStore := sqlite.NewTransactionStore(db)
	startTran := ""
	for {
		recs, err := tranStore.Browse(ctx, startTran, 500)
		if err != nil {
			return in, fmt.Errorf("browse transactions: %w", err)
		}
		for _, r := range recs {
			in.Transactions = append(in.Transactions, tranToReportRow(r))
		}
		if len(recs) < 500 {
			break
		}
		startTran = recs[len(recs)-1].TranID
	}

	return in, nil
}

func tranToReportRow(r *domain.TransactionRecord) report.TransactionRow {
	return report.TransactionRow{
		TranID:           r.TranID,
		TranTypeCode:     r.TranTypeCode,
		TranCatCode:      r.TranCatCode,
		TranSource:       r.TranSource,
		TranDesc:         r.TranDesc,
		TranAmt:          r.TranAmt.InexactFloat64(),
		TranCardNum:      r.TranCardNum,
		TranOrigTS:       r.TranOrigTS,
		TranProcTS:       r.TranProcTS,
		TranMerchantName: r.TranMerchantName,
		TranMerchantCity: r.TranMerchantCity,
		TranMerchantZip:  r.TranMerchantZip,
	}
}
