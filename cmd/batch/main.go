// Command batch is the CardDemo batch CLI. Each subcommand replaces one JCL
// step in the original job stream.
//
// JCL → subcommand mapping:
//
//	CBSTM03A / CREASTMT.JCL → batch statements
//	CBTRN03C / TRANREPT.JCL → batch tran-report
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/report"
)

const defaultDataDir = "app/data/EBCDIC"

// EBCDIC fixture file names, matching the seed loader.
const (
	fileAccounts  = "AWS.M2.CARDDEMO.ACCTDATA.PS"
	fileCustomers = "AWS.M2.CARDDEMO.CUSTDATA.PS"
	fileCardXref  = "AWS.M2.CARDDEMO.CARDXREF.PS"
	fileDalyTran  = "AWS.M2.CARDDEMO.DALYTRAN.PS"
	fileTranType  = "AWS.M2.CARDDEMO.TRANTYPE.PS"
	fileTranCatg  = "AWS.M2.CARDDEMO.TRANCATG.PS"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "statements":
		err = runStatements(os.Args[2:])
	case "tran-report":
		err = runTranReport(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "carddemo-batch: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: carddemo-batch <subcommand> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  statements   Generate account statements (CBSTM03A/CREASTMT.JCL)\n")
	fmt.Fprintf(os.Stderr, "  tran-report  Generate daily transaction report (CBTRN03C/TRANREPT.JCL)\n")
}

// runStatements implements CBSTM03A: one 80-char statement file per account.
func runStatements(args []string) error {
	fs := flag.NewFlagSet("statements", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir, "directory holding the EBCDIC fixture files")
	outDir := fs.String("out", "out/statements", "output directory root")
	period := fs.String("period", "current", "statement period (YYYY-MM); names the output subdirectory")
	acctID := fs.String("account", "", "limit to a single account ID (default: all accounts)")
	fs.Parse(args)

	in, err := loadStatementInput(*dataDir)
	if err != nil {
		return err
	}

	gen := report.NewTextGenerator()
	destDir := filepath.Join(*outDir, *period)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	ctx := context.Background()
	count := 0
	for _, acct := range in.Accounts {
		if *acctID != "" && *acctID != fmt.Sprintf("%d", acct.AcctID) {
			continue
		}
		path := filepath.Join(destDir, fmt.Sprintf("%d.txt", acct.AcctID))
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create %s: %w", path, err)
		}
		spec := report.ReportSpec{Kind: report.KindStatement, AccountID: fmt.Sprintf("%d", acct.AcctID), Period: *period}
		if err := gen.Generate(ctx, spec, in, f); err != nil {
			f.Close()
			return fmt.Errorf("generate %s: %w", path, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close %s: %w", path, err)
		}
		count++
	}

	fmt.Fprintf(os.Stderr, "wrote %d statement(s) to %s\n", count, destDir)
	return nil
}

// runTranReport implements CBTRN03C: the 133-char daily transaction report.
func runTranReport(args []string) error {
	fs := flag.NewFlagSet("tran-report", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir, "directory holding the EBCDIC fixture files")
	startDate := fs.String("start-date", "", "report start date (YYYY-MM-DD)")
	endDate := fs.String("end-date", "", "report end date (YYYY-MM-DD)")
	output := fs.String("output", "", "output file (default: stdout)")
	fs.Parse(args)

	in, err := loadTranReportInput(*dataDir)
	if err != nil {
		return err
	}

	w := os.Stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			return fmt.Errorf("create %s: %w", *output, err)
		}
		defer f.Close()
		w = f
	}

	spec := report.ReportSpec{Kind: report.KindTranReport, StartDate: *startDate, EndDate: *endDate}
	return report.NewTextGenerator().Generate(context.Background(), spec, in, w)
}

// loadStatementInput decodes the accounts, customers, card xrefs and daily
// transactions needed to render statements.
func loadStatementInput(dataDir string) (report.Input, error) {
	var in report.Input

	accts, err := readAccounts(filepath.Join(dataDir, fileAccounts))
	if err != nil {
		return in, err
	}
	in.Accounts = accts

	custs, err := readCustomers(filepath.Join(dataDir, fileCustomers))
	if err != nil {
		return in, err
	}
	in.Customers = custs

	xrefs, err := readCardXrefs(filepath.Join(dataDir, fileCardXref))
	if err != nil {
		return in, err
	}
	in.CardXrefs = xrefs

	txns, err := readDailyTransactions(filepath.Join(dataDir, fileDalyTran))
	if err != nil {
		return in, err
	}
	in.Transactions = txns

	return in, nil
}

// loadTranReportInput decodes the daily transactions, card xrefs, and the
// transaction type/category lookups needed for the report.
func loadTranReportInput(dataDir string) (report.Input, error) {
	var in report.Input

	txns, err := readDailyTransactions(filepath.Join(dataDir, fileDalyTran))
	if err != nil {
		return in, err
	}
	in.Transactions = txns

	xrefs, err := readCardXrefs(filepath.Join(dataDir, fileCardXref))
	if err != nil {
		return in, err
	}
	in.CardXrefs = xrefs

	types, err := readTranTypes(filepath.Join(dataDir, fileTranType))
	if err != nil {
		return in, err
	}
	in.TranTypes = types

	cats, err := readTranCats(filepath.Join(dataDir, fileTranCatg))
	if err != nil {
		return in, err
	}
	in.TranCats = cats

	return in, nil
}

func readAccounts(path string) ([]report.AccountRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	recs, err := domain.DecodeAccountRecords(data)
	if err != nil {
		return nil, err
	}
	rows := make([]report.AccountRow, 0, len(recs))
	for _, r := range recs {
		rows = append(rows, report.AccountRow{AcctID: r.AcctID, CurrBal: r.AcctCurrBal.InexactFloat64()})
	}
	return rows, nil
}

func readCustomers(path string) ([]report.CustomerRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	recs, err := domain.DecodeCustomerRecords(data)
	if err != nil {
		return nil, err
	}
	rows := make([]report.CustomerRow, 0, len(recs))
	for _, r := range recs {
		rows = append(rows, report.CustomerRow{
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
	return rows, nil
}

func readCardXrefs(path string) ([]report.CardXrefRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	recs, err := domain.DecodeCardXrefRecords(data)
	if err != nil {
		return nil, err
	}
	rows := make([]report.CardXrefRow, 0, len(recs))
	for _, r := range recs {
		rows = append(rows, report.CardXrefRow{CardNum: r.XrefCardNum, CustID: r.XrefCustID, AcctID: r.XrefAcctID})
	}
	return rows, nil
}

// readDailyTransactions decodes the daily-transaction fixture (DALYTRAN-RECORD),
// mapping each field to a report TransactionRow.
func readDailyTransactions(path string) ([]report.TransactionRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data)%domain.DailyTransactionRecordLen != 0 {
		return nil, fmt.Errorf("read %s: data length %d is not a multiple of %d", path, len(data), domain.DailyTransactionRecordLen)
	}
	count := len(data) / domain.DailyTransactionRecordLen
	rows := make([]report.TransactionRow, 0, count)
	for i := 0; i < count; i++ {
		d, err := domain.DecodeDailyTransactionRecord(data[i*domain.DailyTransactionRecordLen : (i+1)*domain.DailyTransactionRecordLen])
		if err != nil {
			return nil, err
		}
		rows = append(rows, report.TransactionRow{
			TranID:           d.DalytranID,
			TranTypeCode:     d.DalytranTypeCode,
			TranCatCode:      d.DalytranCatCode,
			TranSource:       d.DalytranSource,
			TranDesc:         d.DalytranDesc,
			TranAmt:          d.DalytranAmt.InexactFloat64(),
			TranCardNum:      d.DalytranCardNum,
			TranOrigTS:       d.DalytranOrigTS,
			TranProcTS:       d.DalytranProcTS,
			TranMerchantName: d.DalytranMerchantName,
			TranMerchantCity: d.DalytranMerchantCity,
			TranMerchantZip:  d.DalytranMerchantZip,
		})
	}
	return rows, nil
}

func readTranTypes(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	recs, err := domain.DecodeTranTypeRecords(data)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(recs))
	for _, r := range recs {
		m[r.TranType] = r.TranTypeDesc
	}
	return m, nil
}

func readTranCats(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	recs, err := domain.DecodeTranCatRecords(data)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(recs))
	for _, r := range recs {
		m[fmt.Sprintf("%s:%d", r.TranTypeCode, r.TranCatCode)] = r.TranCatTypeDesc
	}
	return m, nil
}
