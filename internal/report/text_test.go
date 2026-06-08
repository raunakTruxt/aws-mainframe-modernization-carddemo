package report

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// goldenInput is the fixed dataset shared by the statement golden test and the
// generator. One account, one customer, three transactions on one card.
func goldenInput() Input {
	return Input{
		Accounts: []AccountRow{
			{AcctID: 1, CurrBal: 1234.56},
		},
		Customers: []CustomerRow{
			{
				CustID:      100,
				FirstName:   "John",
				MiddleName:  "Q",
				LastName:    "Public",
				AddrLine1:   "123 Main St",
				AddrLine2:   "Apt 4",
				AddrLine3:   "Springfield",
				AddrStateCD: "IL",
				CountryCode: "USA",
				Zip:         "62704",
				FICOScore:   780,
			},
		},
		CardXrefs: []CardXrefRow{
			{CardNum: "4111111111111111", CustID: 100, AcctID: 1},
		},
		Transactions: []TransactionRow{
			{TranID: "TXN0000000000001", TranTypeCode: "01", TranCatCode: 1, TranDesc: "Grocery Store Purchase", TranAmt: 84.20, TranCardNum: "4111111111111111"},
			{TranID: "TXN0000000000002", TranTypeCode: "02", TranCatCode: 2, TranDesc: "Online Refund", TranAmt: -15.00, TranCardNum: "4111111111111111"},
			{TranID: "TXN0000000000003", TranTypeCode: "01", TranCatCode: 3, TranDesc: "Gas Station", TranAmt: 45.99, TranCardNum: "4111111111111111"},
		},
	}
}

func Test_StatementGolden(t *testing.T) {
	var buf bytes.Buffer
	err := NewTextGenerator().Generate(context.Background(), ReportSpec{Kind: KindStatement, Period: "2025-06"}, goldenInput(), &buf)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	want, err := os.ReadFile("testdata/statement_golden.txt")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if got := buf.String(); got != string(want) {
		t.Errorf("statement output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	for i, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if len(line) != stmtWidth {
			t.Errorf("line %d width = %d, want %d: %q", i, len(line), stmtWidth, line)
		}
	}
}

func Test_TranReportGolden(t *testing.T) {
	in := Input{
		CardXrefs: []CardXrefRow{
			{CardNum: "4111111111111111", CustID: 100, AcctID: 1},
			{CardNum: "4222222222222222", CustID: 200, AcctID: 2},
		},
		TranTypes: map[string]string{"01": "Purchase", "02": "Refund"},
		TranCats:  map[string]string{"01:1": "Groceries", "01:3": "Fuel", "02:2": "Online"},
		Transactions: []TransactionRow{
			{TranID: "TXN0000000000001", TranTypeCode: "01", TranCatCode: 1, TranSource: "POS", TranDesc: "Groceries", TranAmt: 84.20, TranCardNum: "4111111111111111"},
			{TranID: "TXN0000000000003", TranTypeCode: "01", TranCatCode: 3, TranSource: "POS", TranDesc: "Fuel", TranAmt: 45.99, TranCardNum: "4111111111111111"},
			{TranID: "TXN0000000000010", TranTypeCode: "02", TranCatCode: 2, TranSource: "WEB", TranDesc: "Online", TranAmt: -15.00, TranCardNum: "4222222222222222"},
		},
	}

	var buf bytes.Buffer
	err := NewTextGenerator().Generate(context.Background(), ReportSpec{Kind: KindTranReport, StartDate: "2025-06-01", EndDate: "2025-06-30"}, in, &buf)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	for i, line := range lines {
		if len(line) != reportWidth {
			t.Errorf("line %d width = %d, want %d: %q", i, len(line), reportWidth, line)
		}
	}

	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "Grand Total") {
		t.Fatalf("last line is not the grand total: %q", last)
	}
	// 84.20 + 45.99 - 15.00 = 115.19
	if !strings.Contains(last, "+115.19") {
		t.Errorf("grand total line missing +115.19: %q", last)
	}
}
