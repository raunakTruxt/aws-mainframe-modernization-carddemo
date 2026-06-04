package transaction_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/transaction"
	webtxn "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/transaction"
	"github.com/shopspring/decimal"

	_ "modernc.org/sqlite"
)

// ── test fixtures ─────────────────────────────────────────────────────────────

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newHandlers(t *testing.T, db *sql.DB) *webtxn.Handlers {
	t.Helper()
	s := svc.New(
		sqlite.NewTransactionStore(db),
		sqlite.NewAccountStore(db),
		sqlite.NewCardStore(db),
		sqlite.NewTranTypeStore(db),
		sqlite.NewTranCatStore(db),
	)
	fixedNow := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return fixedNow }
	return &webtxn.Handlers{Service: s}
}

const (
	testCardNum = "4111111111111111"
	testAcctID  = int64(11111111111)
)

func seedData(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()

	acct := &domain.AccountRecord{
		AcctID:              testAcctID,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.RequireFromString("500.00"),
		AcctCreditLimit:     decimal.RequireFromString("1000.00"),
		AcctCashCreditLimit: decimal.RequireFromString("200.00"),
		AcctOpenDate:        "2020-01-01",
		AcctExpirationDate:  "2030-01-01",
		AcctReissueDate:     "2025-01-01",
		AcctCurrCycCredit:   decimal.Zero,
		AcctCurrCycDebit:    decimal.Zero,
		AcctAddrZip:         "90210",
		AcctGroupID:         "GRP001",
	}
	if err := sqlite.NewAccountStore(db).Create(ctx, acct); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	card := &domain.CardRecord{
		CardNum:            testCardNum,
		CardAcctID:         testAcctID,
		CardCVVCode:        123,
		CardEmbossedName:   "USER0001",
		CardExpirationDate: "2030-01-01",
		CardActiveStatus:   "Y",
	}
	if err := sqlite.NewCardStore(db).Create(ctx, card); err != nil {
		t.Fatalf("seed card: %v", err)
	}

	if err := sqlite.NewTranTypeStore(db).Create(ctx, &domain.TranTypeRecord{TranType: "01", TranTypeDesc: "Purchase"}); err != nil {
		t.Fatalf("seed tran type: %v", err)
	}
	if err := sqlite.NewTranCatStore(db).Create(ctx, &domain.TranCatRecord{TranTypeCode: "01", TranCatCode: 1000, TranCatTypeDesc: "Retail"}); err != nil {
		t.Fatalf("seed tran cat: %v", err)
	}
}

// newRequest builds an http.Request. Handlers render correctly without a
// session; layout.NewPage returns an empty PageData when no session is set.
func newRequest(t *testing.T, method, path string, body *strings.Reader) *http.Request {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, path, body)
	} else {
		req, err = http.NewRequest(method, path, nil)
	}
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestListTxns_Empty(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	req := newRequest(t, http.MethodGet, "/transactions", nil)
	rec := httptest.NewRecorder()
	h.ListTxns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No transactions found") {
		t.Fatalf("expected empty-state message, body:\n%s", rec.Body.String())
	}
}

func TestListTxns_WithResults(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	// Seed a transaction directly.
	ctx := context.Background()
	tx := &domain.TransactionRecord{
		TranID: "T0000000000000001", TranTypeCode: "01", TranCatCode: 1000,
		TranSource: "POS", TranDesc: "COFFEE", TranAmt: decimal.RequireFromString("5.00"),
		TranCardNum: testCardNum, TranOrigTS: "2024-01-15-12.00.00.000000",
		TranProcTS: "2024-01-15-12.00.01.000000",
	}
	if err := sqlite.NewTransactionStore(db).Create(ctx, tx); err != nil {
		t.Fatalf("create txn: %v", err)
	}

	req := newRequest(t, http.MethodGet, "/transactions?card="+testCardNum, nil)
	rec := httptest.NewRecorder()
	h.ListTxns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "T0000000000000001") {
		t.Fatalf("expected tran id in body, got:\n%s", body)
	}
	if !strings.Contains(body, "COFFEE") {
		t.Fatalf("expected description in body")
	}
}

func TestViewTxn_Found(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	ctx := context.Background()
	tx := &domain.TransactionRecord{
		TranID: "T0000000000000001", TranTypeCode: "01", TranCatCode: 1000,
		TranSource: "POS", TranDesc: "LUNCH", TranAmt: decimal.RequireFromString("12.50"),
		TranCardNum: testCardNum, TranOrigTS: "2024-01-15-12.00.00.000000",
		TranProcTS: "2024-01-15-12.00.01.000000",
	}
	if err := sqlite.NewTransactionStore(db).Create(ctx, tx); err != nil {
		t.Fatalf("create txn: %v", err)
	}

	req := newRequest(t, http.MethodGet, "/transactions/view?id=T0000000000000001", nil)
	rec := httptest.NewRecorder()
	h.ViewTxn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "LUNCH") {
		t.Fatalf("expected description in body")
	}
}

func TestViewTxn_NotFound(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)

	req := newRequest(t, http.MethodGet, "/transactions/view?id=NOTEXIST1234567", nil)
	rec := httptest.NewRecorder()
	h.ViewTxn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with flash, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Transaction not found") {
		t.Fatalf("expected not-found flash in body")
	}
}

func TestViewTxn_NoID_Redirects(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)

	req := newRequest(t, http.MethodGet, "/transactions/view", nil)
	rec := httptest.NewRecorder()
	h.ViewTxn(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d", rec.Code)
	}
}

func TestGetAddTxn(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)

	req := newRequest(t, http.MethodGet, "/transactions/add", nil)
	rec := httptest.NewRecorder()
	h.GetAddTxn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Transaction Add") {
		t.Fatalf("expected form in body")
	}
}

func TestPostAddTxn_Valid(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	form := url.Values{
		"card_num":      {testCardNum},
		"tran_type":     {"01"},
		"tran_cat":      {"1000"},
		"tran_source":   {"POS"},
		"tran_desc":     {"COFFEE"},
		"tran_amt":      {"5.00"},
		"merchant_name": {"ACME"},
	}
	req := newRequest(t, http.MethodPost, "/transactions/add", strings.NewReader(form.Encode()))
	rec := httptest.NewRecorder()
	h.PostAddTxn(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/transactions/view?id=") {
		t.Fatalf("expected redirect to view, got %q", loc)
	}
}

func TestPostAddTxn_ZeroAmount(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	form := url.Values{
		"card_num":  {testCardNum},
		"tran_type": {"01"},
		"tran_cat":  {"1000"},
		"tran_amt":  {"0.00"},
	}
	req := newRequest(t, http.MethodPost, "/transactions/add", strings.NewReader(form.Encode()))
	rec := httptest.NewRecorder()
	h.PostAddTxn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with flash, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Amount must not be zero") {
		t.Fatalf("expected error message in body: %s", rec.Body.String())
	}
}

func TestPostAddTxn_InvalidType(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	form := url.Values{
		"card_num":  {testCardNum},
		"tran_type": {"ZZ"},
		"tran_cat":  {"1000"},
		"tran_amt":  {"10.00"},
	}
	req := newRequest(t, http.MethodPost, "/transactions/add", strings.NewReader(form.Encode()))
	rec := httptest.NewRecorder()
	h.PostAddTxn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with flash, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid transaction type") {
		t.Fatalf("expected error in body: %s", rec.Body.String())
	}
}

func TestGetBillPay(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)

	req := newRequest(t, http.MethodGet, "/billing", nil)
	rec := httptest.NewRecorder()
	h.GetBillPay(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Bill Payment") {
		t.Fatalf("expected Bill Payment form")
	}
}

func TestPostBillPay_Valid(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	form := url.Values{
		"card_num": {testCardNum},
		"amt":      {"100.00"},
	}
	req := newRequest(t, http.MethodPost, "/billing", strings.NewReader(form.Encode()))
	rec := httptest.NewRecorder()
	h.PostBillPay(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/transactions/view?id=") {
		t.Fatalf("expected redirect to view, got %q", loc)
	}
}

func TestPostBillPay_ExceedsBalance(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	form := url.Values{
		"card_num": {testCardNum},
		"amt":      {"9999.00"},
	}
	req := newRequest(t, http.MethodPost, "/billing", strings.NewReader(form.Encode()))
	rec := httptest.NewRecorder()
	h.PostBillPay(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with flash, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Payment amount exceeds current balance") {
		t.Fatalf("expected error in body: %s", rec.Body.String())
	}
}

func TestPostBillPay_NegativeAmount(t *testing.T) {
	db := newTestDB(t)
	h := newHandlers(t, db)
	seedData(t, db)

	form := url.Values{
		"card_num": {testCardNum},
		"amt":      {"-50.00"},
	}
	req := newRequest(t, http.MethodPost, "/billing", strings.NewReader(form.Encode()))
	rec := httptest.NewRecorder()
	h.PostBillPay(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with flash, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "positive") {
		t.Fatalf("expected error in body: %s", rec.Body.String())
	}
}
