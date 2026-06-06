package tranpost

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func TestRun_postValid(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	acctStore := sqlite.NewAccountStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	acct := &domain.AccountRecord{
		AcctID:              10000000001,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(500.00),
		AcctCreditLimit:     decimal.NewFromFloat(5000.00),
		AcctCashCreditLimit: decimal.NewFromFloat(1000.00),
		AcctExpirationDate:  "2030-12-31",
		AcctGroupID:         "GRP1      ",
	}
	if err := acctStore.Create(ctx, acct); err != nil {
		t.Fatalf("create acct: %v", err)
	}
	xref := &domain.CardXrefRecord{
		XrefCardNum: "4111111111111234",
		XrefCustID:  1,
		XrefAcctID:  10000000001,
	}
	if err := xrefStore.Create(ctx, xref); err != nil {
		t.Fatalf("create xref: %v", err)
	}

	input := makeDalytranInput(t, "TRAN00000001    ", "PR", 1,
		decimal.NewFromFloat(100.00), "4111111111111234", "2022-07-18 00:00:00.000000")

	fixedNow := time.Date(2022, 7, 18, 12, 0, 0, 0, time.UTC)
	s, err := Run(ctx, Config{
		DB:    db,
		Input: bytes.NewReader(input),
		Now:   fixedNow,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Posted != 1 || s.Rejected != 0 {
		t.Errorf("posted=%d rejected=%d, want 1,0", s.Posted, s.Rejected)
	}

	updated, err := acctStore.Get(ctx, 10000000001)
	if err != nil {
		t.Fatalf("get updated acct: %v", err)
	}
	wantBal := decimal.NewFromFloat(600.00)
	if !updated.AcctCurrBal.Equal(wantBal) {
		t.Errorf("balance = %s, want %s", updated.AcctCurrBal, wantBal)
	}
	// Cycle credit should include the transaction amount.
	wantCyc := decimal.NewFromFloat(100.00)
	if !updated.AcctCurrCycCredit.Equal(wantCyc) {
		t.Errorf("cyc-credit = %s, want %s", updated.AcctCurrCycCredit, wantCyc)
	}
}

func TestRun_rejectInvalidCard(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	input := makeDalytranInput(t, "TRAN00000002    ", "PR", 1,
		decimal.NewFromFloat(50.00), "9999999999999999", "2022-07-18 00:00:00.000000")

	var rejects bytes.Buffer
	s, err := Run(ctx, Config{
		DB:      db,
		Input:   bytes.NewReader(input),
		Rejects: &rejects,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Rejected != 1 || s.Posted != 0 {
		t.Errorf("rejected=%d posted=%d, want 1,0", s.Rejected, s.Posted)
	}
	if rejects.Len() != 430 {
		t.Errorf("reject buf = %d bytes, want 430", rejects.Len())
	}
	reason := string(rejects.Bytes()[350:354])
	if reason != "0100" {
		t.Errorf("reject reason = %q, want \"0100\"", reason)
	}
}

func TestRun_rejectOverlimit(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	acctStore := sqlite.NewAccountStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	if err := acctStore.Create(ctx, &domain.AccountRecord{
		AcctID:              10000000002,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(4900.00),
		AcctCreditLimit:     decimal.NewFromFloat(5000.00),
		AcctCashCreditLimit: decimal.NewFromFloat(1000.00),
		AcctCurrCycCredit:   decimal.NewFromFloat(4900.00),
		AcctExpirationDate:  "2030-12-31",
		AcctGroupID:         "GRP1      ",
	}); err != nil {
		t.Fatalf("create acct: %v", err)
	}
	if err := xrefStore.Create(ctx, &domain.CardXrefRecord{
		XrefCardNum: "4111111111119999",
		XrefCustID:  2,
		XrefAcctID:  10000000002,
	}); err != nil {
		t.Fatalf("create xref: %v", err)
	}

	// 4900 - 0 + 200 = 5100 > limit 5000
	input := makeDalytranInput(t, "TRAN00000003    ", "PR", 1,
		decimal.NewFromFloat(200.00), "4111111111119999", "2022-07-18 00:00:00.000000")

	var rejects bytes.Buffer
	s, err := Run(ctx, Config{DB: db, Input: bytes.NewReader(input), Rejects: &rejects})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Rejected != 1 {
		t.Errorf("rejected=%d, want 1", s.Rejected)
	}
	reason := string(rejects.Bytes()[350:354])
	if reason != "0102" {
		t.Errorf("reject reason = %q, want \"0102\"", reason)
	}
}

// Integration test: run against the EBCDIC DALYTRAN fixture if present.
func TestRun_fixtureIntegration(t *testing.T) {
	fixturePath := "../../../app/data/EBCDIC/AWS.M2.CARDDEMO.DALYTRAN.PS"
	acctFixture := "../../../app/data/EBCDIC/AWS.M2.CARDDEMO.ACCTDATA.PS"
	xrefFixture := "../../../app/data/EBCDIC/AWS.M2.CARDDEMO.CARDXREF.PS"

	dalytranData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("DALYTRAN fixture missing: %v", err)
	}
	acctData, err := os.ReadFile(acctFixture)
	if err != nil {
		t.Skipf("ACCTDATA fixture missing: %v", err)
	}
	xrefData, err := os.ReadFile(xrefFixture)
	if err != nil {
		t.Skipf("CARDXREF fixture missing: %v", err)
	}

	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	accts, err := domain.DecodeAccountRecords(acctData)
	if err != nil {
		t.Fatalf("decode accounts: %v", err)
	}
	xrefs, err := domain.DecodeCardXrefRecords(xrefData)
	if err != nil {
		t.Fatalf("decode xrefs: %v", err)
	}

	acctStore := sqlite.NewAccountStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)
	for _, a := range accts {
		if err := acctStore.Create(ctx, a); err != nil {
			t.Fatalf("seed acct %d: %v", a.AcctID, err)
		}
	}
	for _, x := range xrefs {
		if err := xrefStore.Create(ctx, x); err != nil {
			t.Fatalf("seed xref %s: %v", x.XrefCardNum, err)
		}
	}

	totalBalBefore := decimal.Zero
	acctsBefore, _ := acctStore.Browse(ctx, 0, 0)
	for _, a := range acctsBefore {
		totalBalBefore = totalBalBefore.Add(a.AcctCurrBal)
	}

	var rejects bytes.Buffer
	s, err := Run(ctx, Config{
		DB:      db,
		Input:   bytes.NewReader(dalytranData),
		Rejects: &rejects,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("Posted: %d, Rejected: %d", s.Posted, s.Rejected)

	// Invariant: total balance delta == sum of posted transaction amounts.
	tranStore := sqlite.NewTransactionStore(db)
	trans, _ := tranStore.Browse(ctx, "", 0)
	postedAmt := decimal.Zero
	for _, tr := range trans {
		postedAmt = postedAmt.Add(tr.TranAmt)
	}

	acctsAfter, _ := acctStore.Browse(ctx, 0, 0)
	totalBalAfter := decimal.Zero
	for _, a := range acctsAfter {
		totalBalAfter = totalBalAfter.Add(a.AcctCurrBal)
	}

	balDelta := totalBalAfter.Sub(totalBalBefore)
	if !balDelta.Equal(postedAmt) {
		t.Errorf("balance delta %s != sum of posted amounts %s", balDelta, postedAmt)
	}
}

// TestRun_rejectBothOverlimitAndExpiredIsCode103 verifies COBOL precedence: both IFs run
// independently and reason 103 (expired) overwrites 102 (overlimit) when both apply.
func TestRun_rejectBothOverlimitAndExpiredIsCode103(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	acctStore := sqlite.NewAccountStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	if err := acctStore.Create(ctx, &domain.AccountRecord{
		AcctID:              10000000099,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(4900.00),
		AcctCreditLimit:     decimal.NewFromFloat(5000.00),
		AcctCashCreditLimit: decimal.NewFromFloat(1000.00),
		AcctCurrCycCredit:   decimal.NewFromFloat(4900.00),
		AcctExpirationDate:  "2020-01-01", // expired
		AcctGroupID:         "GRP1      ",
	}); err != nil {
		t.Fatalf("create acct: %v", err)
	}
	if err := xrefStore.Create(ctx, &domain.CardXrefRecord{
		XrefCardNum: "4111111111118888",
		XrefCustID:  99,
		XrefAcctID:  10000000099,
	}); err != nil {
		t.Fatalf("create xref: %v", err)
	}

	// Both conditions apply: overlimit (4900+200>5000) AND expired (2022 > 2020).
	input := makeDalytranInput(t, "TRAN00000099    ", "PR", 1,
		decimal.NewFromFloat(200.00), "4111111111118888", "2022-07-18 00:00:00.000000")

	var rejects bytes.Buffer
	s, err := Run(ctx, Config{DB: db, Input: bytes.NewReader(input), Rejects: &rejects})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Rejected != 1 {
		t.Errorf("rejected=%d, want 1", s.Rejected)
	}
	reason := string(rejects.Bytes()[350:354])
	if reason != "0103" {
		t.Errorf("reject reason = %q, want \"0103\" (expired overwrites overlimit, CBTRN02C.cbl:407-420)", reason)
	}
}

// TestRun_rerunIdempotent verifies that re-running the same daily file does not double-apply balances.
func TestRun_rerunIdempotent(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	acctStore := sqlite.NewAccountStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	acct := &domain.AccountRecord{
		AcctID:              10000000050,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(100.00),
		AcctCreditLimit:     decimal.NewFromFloat(5000.00),
		AcctCashCreditLimit: decimal.NewFromFloat(1000.00),
		AcctExpirationDate:  "2030-12-31",
		AcctGroupID:         "GRP1      ",
	}
	if err := acctStore.Create(ctx, acct); err != nil {
		t.Fatalf("create acct: %v", err)
	}
	if err := xrefStore.Create(ctx, &domain.CardXrefRecord{
		XrefCardNum: "4111111111115050",
		XrefCustID:  50,
		XrefAcctID:  10000000050,
	}); err != nil {
		t.Fatalf("create xref: %v", err)
	}

	input := makeDalytranInput(t, "TRAN00000050    ", "PR", 1,
		decimal.NewFromFloat(50.00), "4111111111115050", "2022-07-18 00:00:00.000000")

	// First run: posts normally.
	s1, err := Run(ctx, Config{DB: db, Input: bytes.NewReader(input)})
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if s1.Posted != 1 {
		t.Errorf("first run posted=%d, want 1", s1.Posted)
	}

	// Second run with same data: dup detected before balance writes, no double-apply.
	s2, err := Run(ctx, Config{DB: db, Input: bytes.NewReader(input)})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if s2.Posted != 0 && s2.Rejected != 0 {
		t.Logf("second run: posted=%d rejected=%d (dup skipped silently)", s2.Posted, s2.Rejected)
	}

	final, err := acctStore.Get(ctx, 10000000050)
	if err != nil {
		t.Fatalf("get acct: %v", err)
	}
	wantBal := decimal.NewFromFloat(150.00) // only applied once
	if !final.AcctCurrBal.Equal(wantBal) {
		t.Errorf("balance after re-run = %s, want %s (must not double-apply)", final.AcctCurrBal, wantBal)
	}
}

// makeDalytranInput encodes a simple DailyTransactionRecord as 350 EBCDIC bytes.
func makeDalytranInput(t *testing.T, id, typeCD string, catCD int64, amt decimal.Decimal, cardNum, origTS string) []byte {
	t.Helper()
	tr := &domain.TransactionRecord{
		TranID:       padRight(id, 16),
		TranTypeCode: typeCD,
		TranCatCode:  catCD,
		TranAmt:      amt,
		TranCardNum:  padRight(cardNum, 16),
		TranOrigTS:   padRight(origTS, 26),
	}
	b, err := tr.Encode()
	if err != nil {
		t.Fatalf("encode tran: %v", err)
	}
	return b
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	b := make([]byte, n)
	copy(b, s)
	for i := len(s); i < n; i++ {
		b[i] = ' '
	}
	return string(b)
}
