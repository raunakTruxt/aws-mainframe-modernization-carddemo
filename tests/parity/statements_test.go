package parity_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/report"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

// TestParity_CBSTM03A_StatementsMatchGolden seeds a fresh DB from the EBCDIC
// fixtures and generates per-account statements (CBSTM03A / CREASTMT.JCL),
// comparing the concatenated output against a golden file.
//
// The golden file is the concatenation of all statements sorted by account ID.
func TestParity_CBSTM03A_StatementsMatchGolden(t *testing.T) {
	db := seedTestDB(t)
	ctx := context.Background()

	in, err := buildReportInput(t, ctx, db)
	if err != nil {
		t.Fatalf("buildReportInput: %v", err)
	}

	gen := report.NewTextGenerator()
	var all strings.Builder
	for _, acct := range in.Accounts {
		var buf bytes.Buffer
		spec := report.ReportSpec{
			Kind:      report.KindStatement,
			AccountID: fmt.Sprintf("%d", acct.AcctID),
			Period:    "2022-07",
		}
		if err := gen.Generate(ctx, spec, in, &buf); err != nil {
			t.Fatalf("generate statement for acct %d: %v", acct.AcctID, err)
		}
		all.Write(buf.Bytes())
	}
	compareGolden(t, "statements.txt", all.String())
}

// buildReportInput loads all entities from db needed for report generation.
func buildReportInput(t *testing.T, ctx context.Context, db *sql.DB) (report.Input, error) {
	t.Helper()
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
			in.Transactions = append(in.Transactions, report.TransactionRow{
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
			})
		}
		if len(recs) < 500 {
			break
		}
		startTran = recs[len(recs)-1].TranID
	}

	return in, nil
}
