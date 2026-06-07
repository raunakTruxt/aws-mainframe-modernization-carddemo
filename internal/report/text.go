package report

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"strings"
)

const (
	stmtWidth   = 80
	reportWidth = 133
	pageSize    = 20
)

// TextGenerator renders reports as fixed-width text matching the COBOL print
// layouts. The zero value is ready to use.
type TextGenerator struct{}

// NewTextGenerator constructs a TextGenerator.
func NewTextGenerator() *TextGenerator { return &TextGenerator{} }

// Generate dispatches on spec.Kind.
func (g *TextGenerator) Generate(ctx context.Context, spec ReportSpec, in Input, w io.Writer) error {
	switch spec.Kind {
	case KindStatement:
		return g.generateStatement(ctx, spec, in, w)
	case KindTranReport:
		return g.generateTranReport(ctx, spec, in, w)
	default:
		return fmt.Errorf("report: unknown kind %q", spec.Kind)
	}
}

// ── Statement (CBSTM03A / COSTM01.CPY) ────────────────────────────────────────

// generateStatement writes one statement per account in the account list,
// each in the 80-char STATEMENT-LINES layout. With spec.AccountID set, only
// that account is written.
func (g *TextGenerator) generateStatement(ctx context.Context, spec ReportSpec, in Input, w io.Writer) error {
	xrefByAcct := make(map[int64][]string, len(in.CardXrefs))
	custByID := make(map[int64]CustomerRow, len(in.Customers))
	for _, x := range in.CardXrefs {
		xrefByAcct[x.AcctID] = append(xrefByAcct[x.AcctID], x.CardNum)
	}
	for _, c := range in.Customers {
		custByID[c.CustID] = c
	}
	custByAcct := make(map[int64]CustomerRow, len(in.CardXrefs))
	for _, x := range in.CardXrefs {
		if c, ok := custByID[x.CustID]; ok {
			custByAcct[x.AcctID] = c
		}
	}
	txnByCard := make(map[string][]TransactionRow, len(in.Transactions))
	for _, t := range in.Transactions {
		txnByCard[t.TranCardNum] = append(txnByCard[t.TranCardNum], t)
	}

	bw := bufio.NewWriter(w)
	for _, acct := range in.Accounts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if spec.AccountID != "" && spec.AccountID != fmt.Sprintf("%d", acct.AcctID) {
			continue
		}
		cust := custByAcct[acct.AcctID]
		var txns []TransactionRow
		for _, card := range xrefByAcct[acct.AcctID] {
			txns = append(txns, txnByCard[card]...)
		}
		writeStatement(bw, acct, cust, txns)
	}
	return bw.Flush()
}

// writeStatement renders a single account's 80-char statement block.
func writeStatement(w *bufio.Writer, acct AccountRow, cust CustomerRow, txns []TransactionRow) {
	dash := strings.Repeat("-", stmtWidth)

	lines := []string{
		strings.Repeat("*", 31) + "START OF STATEMENT" + strings.Repeat("*", 31),
		stmtName(cust),
		fitLeft(cust.AddrLine1, 50) + strings.Repeat(" ", 30),
		fitLeft(cust.AddrLine2, 50) + strings.Repeat(" ", 30),
		stmtAddr3(cust),
		dash,
		strings.Repeat(" ", 33) + fitLeft("Basic Details", 14) + strings.Repeat(" ", 33),
		dash,
		"Account ID         :" + fitLeft(fmt.Sprintf("%011d", acct.AcctID), 20) + strings.Repeat(" ", 40),
		"Current Balance    :" + picBalance(acct.CurrBal) + strings.Repeat(" ", 7) + strings.Repeat(" ", 40),
		"FICO Score         :" + fitLeft(fmt.Sprintf("%d", cust.FICOScore), 20) + strings.Repeat(" ", 40),
		dash,
		strings.Repeat(" ", 30) + "TRANSACTION SUMMARY " + strings.Repeat(" ", 30),
		dash,
		fitLeft("Tran ID         ", 16) + fitLeft("Tran Details    ", 51) + "  Tran Amount",
		dash,
	}
	for _, l := range lines {
		writeLine(w, l, stmtWidth)
	}

	var total float64
	for _, t := range txns {
		writeLine(w, fitLeft(t.TranID, 16)+" "+fitLeft(t.TranDesc, 49)+"$"+picAmount(t.TranAmt), stmtWidth)
		total += t.TranAmt
	}

	writeLine(w, dash, stmtWidth)
	writeLine(w, "Total EXP:"+strings.Repeat(" ", 56)+"$"+picAmount(total), stmtWidth)
	writeLine(w, strings.Repeat("*", 32)+"END OF STATEMENT"+strings.Repeat("*", 32), stmtWidth)
}

// stmtName builds ST-NAME (75 chars) as first+" "+middle+" "+last+" ", matching
// the COBOL STRING ... DELIMITED BY ' ' concatenation in 5000-CREATE-STATEMENT.
func stmtName(c CustomerRow) string {
	name := delimSpace(c.FirstName) + " " + delimSpace(c.MiddleName) + " " + delimSpace(c.LastName) + " "
	return fitLeft(name, 75) + strings.Repeat(" ", 5)
}

// stmtAddr3 builds ST-ADD3 (80 chars) as addr3+" "+state+" "+country+" "+zip+" ".
func stmtAddr3(c CustomerRow) string {
	s := delimSpace(c.AddrLine3) + " " + delimSpace(c.AddrStateCD) + " " +
		delimSpace(c.CountryCode) + " " + delimSpace(c.Zip) + " "
	return fitLeft(s, 80)
}

// delimSpace returns s up to its first space, mirroring COBOL DELIMITED BY ' '.
func delimSpace(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}

// ── Transaction report (CBTRN03C / CVTRA07Y.cpy) ──────────────────────────────

// generateTranReport writes the 133-char daily transaction report. Transactions
// are grouped by card number; account totals print at each card change, page
// totals every pageSize detail lines, a grand total at the end.
func (g *TextGenerator) generateTranReport(ctx context.Context, spec ReportSpec, in Input, w io.Writer) error {
	bw := bufio.NewWriter(w)
	header := tranReportNameHeader(spec.StartDate, spec.EndDate)
	dash := strings.Repeat("-", reportWidth)

	writeHeaders := func() {
		writeLine(bw, header, reportWidth)
		writeLine(bw, "", reportWidth)
		writeLine(bw, tranReportColHeader(), reportWidth)
		writeLine(bw, dash, reportWidth)
	}

	writeHeaders()

	var pageTotal, accountTotal, grandTotal float64
	var currCard string
	first := true
	lineCount := 0

	for _, t := range in.Transactions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !first && t.TranCardNum != currCard {
			writeLine(bw, tranTotalLine("Account Total", 13, accountTotal), reportWidth)
			writeLine(bw, dash, reportWidth)
			accountTotal = 0
		}
		first = false
		currCard = t.TranCardNum

		if lineCount > 0 && lineCount%pageSize == 0 {
			writeLine(bw, tranTotalLine("Page Total", 11, pageTotal), reportWidth)
			writeLine(bw, dash, reportWidth)
			grandTotal += pageTotal
			pageTotal = 0
			writeHeaders()
		}

		acctID := xrefAcctID(in.CardXrefs, t.TranCardNum)
		typeDesc := in.TranTypes[t.TranTypeCode]
		catDesc := in.TranCats[fmt.Sprintf("%s:%d", t.TranTypeCode, t.TranCatCode)]
		writeLine(bw, tranDetailLine(t, acctID, typeDesc, catDesc), reportWidth)

		pageTotal += t.TranAmt
		accountTotal += t.TranAmt
		lineCount++
	}

	if !first {
		writeLine(bw, tranTotalLine("Account Total", 13, accountTotal), reportWidth)
		writeLine(bw, dash, reportWidth)
	}
	writeLine(bw, tranTotalLine("Page Total", 11, pageTotal), reportWidth)
	writeLine(bw, dash, reportWidth)
	grandTotal += pageTotal
	writeLine(bw, tranTotalLine("Grand Total", 11, grandTotal), reportWidth)

	return bw.Flush()
}

// tranReportNameHeader builds REPORT-NAME-HEADER.
func tranReportNameHeader(start, end string) string {
	return fitLeft("DALYREPT", 38) +
		fitLeft("Daily Transaction Report", 41) +
		fitLeft("Date Range: ", 12) +
		fitLeft(start, 10) + " to " + fitLeft(end, 10)
}

// tranReportColHeader builds TRANSACTION-HEADER-1.
func tranReportColHeader() string {
	return fitLeft("Transaction ID", 17) +
		fitLeft("Account ID", 12) +
		fitLeft("Transaction Type", 19) +
		fitLeft("Tran Category", 35) +
		fitLeft("Tran Source", 14) +
		" " +
		fitLeft("        Amount", 16)
}

// tranDetailLine builds TRANSACTION-DETAIL-REPORT.
func tranDetailLine(t TransactionRow, acctID, typeDesc, catDesc string) string {
	return fitLeft(t.TranID, 16) + " " +
		fitLeft(acctID, 11) + " " +
		fitLeft(t.TranTypeCode, 2) + "-" + fitLeft(typeDesc, 15) + " " +
		fmt.Sprintf("%04d", t.TranCatCode) + "-" + fitLeft(catDesc, 29) + " " +
		fitLeft(t.TranSource, 10) + strings.Repeat(" ", 4) +
		picReportAmt(t.TranAmt, false) + strings.Repeat(" ", 2)
}

// tranTotalLine builds a REPORT-*-TOTALS line: label, label-width-padded dots
// to column 97, then a +ZZZ,ZZZ,ZZZ.ZZ amount.
func tranTotalLine(label string, labelWidth int, amt float64) string {
	dots := 97 - labelWidth
	return fitLeft(label, labelWidth) + strings.Repeat(".", dots) + picReportAmt(amt, true)
}

// xrefAcctID returns the account ID (11-digit) for a card number, or "" if the
// card has no cross-reference entry.
func xrefAcctID(xrefs []CardXrefRow, card string) string {
	for _, x := range xrefs {
		if x.CardNum == card {
			return fmt.Sprintf("%011d", x.AcctID)
		}
	}
	return ""
}

// ── Numeric edited-picture formatters ─────────────────────────────────────────

// picBalance formats v as PIC 9(9).99- (13 chars): 9 leading-zero integer
// digits, '.', 2 decimals, trailing '-' if negative else ' '.
func picBalance(v float64) string {
	neg := v < 0
	cents := int64(math.Round(math.Abs(v) * 100))
	intPart := cents / 100
	frac := cents % 100
	sign := " "
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%09d.%02d%s", intPart, frac, sign)
}

// picAmount formats v as PIC Z(9).99- (13 chars): 9 zero-suppressed integer
// digits, '.', 2 decimals, trailing '-' if negative else ' '.
func picAmount(v float64) string {
	neg := v < 0
	cents := int64(math.Round(math.Abs(v) * 100))
	intPart := cents / 100
	frac := cents % 100
	sign := " "
	if neg {
		sign = "-"
	}
	intStr := suppressZeros(fmt.Sprintf("%09d", intPart))
	return fmt.Sprintf("%9s.%02d%s", intStr, frac, sign)
}

// picReportAmt formats v as PIC -ZZZ,ZZZ,ZZZ.ZZ (sign) or PIC +ZZZ,ZZZ,ZZZ.ZZ
// (signed) in a 15-char field: a leading sign position, then zero-suppressed
// comma-grouped integer digits, '.', 2 decimals, right-justified.
func picReportAmt(v float64, signed bool) string {
	neg := v < 0
	cents := int64(math.Round(math.Abs(v) * 100))
	intPart := cents / 100
	frac := cents % 100

	digits := suppressZerosCommas(fmt.Sprintf("%09d", intPart))
	sign := " "
	if neg {
		sign = "-"
	} else if signed {
		sign = "+"
	}
	body := sign + digits + fmt.Sprintf(".%02d", frac)
	return fmt.Sprintf("%15s", body)
}

// suppressZeros strips leading zeros from s, stopping at the first non-zero
// digit (COBOL Z-picture). An all-zero value collapses to the empty string,
// leaving the integer field blank.
func suppressZeros(s string) string {
	return strings.TrimLeft(s, "0")
}

// suppressZerosCommas zero-suppresses a 9-digit integer string and inserts
// thousands separators only between surviving digits, matching the COBOL
// ZZZ,ZZZ,ZZZ comma-suppression rule.
func suppressZerosCommas(s string) string {
	n := len(s)
	var b strings.Builder
	started := false
	for i := 0; i < n; i++ {
		c := s[i]
		if c != '0' {
			started = true
		}
		pos := n - i // digits remaining including current
		if started {
			b.WriteByte(c)
			if pos > 1 && pos%3 == 1 {
				b.WriteByte(',')
			}
		}
	}
	if !started {
		return "0"
	}
	return b.String()
}

// ── line helpers ──────────────────────────────────────────────────────────────

// fitLeft left-justifies s in width n: padded with spaces if short, truncated
// if long. This mirrors a COBOL MOVE into a fixed alphanumeric field.
func fitLeft(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

// writeLine pads/truncates line to width and writes it with a trailing newline.
func writeLine(w *bufio.Writer, line string, width int) {
	w.WriteString(fitLeft(line, width))
	w.WriteByte('\n')
}
