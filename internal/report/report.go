// Package report is the Go port of the CardDemo batch reporting programs.
// It produces account statements (CBSTM03A / CREASTMT.JCL) and the daily
// transaction report (CBTRN03C / TRANREPT.JCL) from decoded domain records.
//
// The text generator reproduces the fixed-width COBOL print layouts exactly:
// statement lines are 80 chars (COSTM01.CPY / STATEMENT-LINES in CBSTM03A),
// transaction-report lines are 133 chars (CVTRA07Y.cpy).
package report

import (
	"context"
	"io"
)

// ReportKind selects which report a Generator produces.
type ReportKind string

const (
	// KindStatement is the per-account statement (CBSTM03A).
	KindStatement ReportKind = "statement"
	// KindTranReport is the daily transaction report (CBTRN03C).
	KindTranReport ReportKind = "tran-report"
)

// ReportSpec describes what to generate.
type ReportSpec struct {
	Kind ReportKind
	// AccountID, for KindStatement; empty means all accounts.
	AccountID string
	// Period, for KindStatement, as "YYYY-MM".
	Period string
	// StartDate, for KindTranReport, as "YYYY-MM-DD".
	StartDate string
	// EndDate, for KindTranReport, as "YYYY-MM-DD".
	EndDate string
}

// Generator produces a report from a spec, writing the rendered output to w.
type Generator interface {
	Generate(ctx context.Context, spec ReportSpec, in Input, w io.Writer) error
}

// Input is the decoded data a Generator draws on. All fields are optional;
// the generator uses what is available for the requested ReportKind.
type Input struct {
	Transactions []TransactionRow
	CardXrefs    []CardXrefRow
	Customers    []CustomerRow
	Accounts     []AccountRow
	TranTypes    map[string]string // typeCode -> description
	TranCats     map[string]string // "typeCode:catCode" -> description
}

// TransactionRow is one transaction in report-ready form.
type TransactionRow struct {
	TranID           string
	TranTypeCode     string
	TranCatCode      int64
	TranSource       string
	TranDesc         string
	TranAmt          float64 // positive = debit, negative = credit
	TranCardNum      string
	TranOrigTS       string
	TranProcTS       string
	TranMerchantName string
	TranMerchantCity string
	TranMerchantZip  string
}

// CardXrefRow links a card number to its customer and account.
type CardXrefRow struct {
	CardNum string
	CustID  int64
	AcctID  int64
}

// CustomerRow is the customer detail printed on a statement.
type CustomerRow struct {
	CustID      int64
	FirstName   string
	MiddleName  string
	LastName    string
	AddrLine1   string
	AddrLine2   string
	AddrLine3   string
	AddrStateCD string
	CountryCode string
	Zip         string
	FICOScore   int64
}

// AccountRow is the account detail printed on a statement.
type AccountRow struct {
	AcctID  int64
	CurrBal float64
}
