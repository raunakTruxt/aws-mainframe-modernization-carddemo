package account_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/account"
)

func makeTestTransactor(db *sql.DB) svc.Transactor {
	return func(ctx context.Context, fn func(repo.AccountRepository, repo.CustomerRepository) error) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := fn(sqlite.NewAccountStore(tx), sqlite.NewCustomerStore(tx)); err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit()
	}
}

// openSeededService returns a service + repos over the same in-memory DB.
func openSeededService(t *testing.T) (*svc.Service, *sqlite.AccountStore, *sqlite.CustomerStore, *sqlite.CardXrefStore) {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	acctStore := sqlite.NewAccountStore(db)
	custStore := sqlite.NewCustomerStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	ctx := context.Background()
	if err := acctStore.Create(ctx, sampleAccount(70000001001)); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if err := custStore.Create(ctx, sampleCustomer(1001)); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := xrefStore.Create(ctx, sampleXref(70000001001, 1001)); err != nil {
		t.Fatalf("seed xref: %v", err)
	}

	return svc.New(acctStore, custStore, xrefStore, makeTestTransactor(db)), acctStore, custStore, xrefStore
}

func sampleAccount(id int64) *domain.AccountRecord {
	return &domain.AccountRecord{
		AcctID:              id,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.RequireFromString("100.00"),
		AcctCreditLimit:     decimal.RequireFromString("5000.00"),
		AcctCashCreditLimit: decimal.RequireFromString("1000.00"),
		AcctOpenDate:        "2020-01-15",
		AcctExpirationDate:  "2025-12-31",
		AcctReissueDate:     "2023-01-15",
		AcctCurrCycCredit:   decimal.RequireFromString("0"),
		AcctCurrCycDebit:    decimal.RequireFromString("0"),
		AcctAddrZip:         "90210",
		AcctGroupID:         "GRP001",
	}
}

func sampleCustomer(id int64) *domain.CustomerRecord {
	return &domain.CustomerRecord{
		CustID:               id,
		CustFirstName:        "John",
		CustMiddleName:       "A",
		CustLastName:         "Doe",
		CustAddrLine1:        "123 Main St",
		CustAddrLine2:        "Apt 4",
		CustAddrLine3:        "Beverly Hills",
		CustAddrStateCode:    "CA",
		CustAddrCountryCode:  "USA",
		CustAddrZip:          "90210",
		CustPhoneNum1:        "(310)555-1234",
		CustPhoneNum2:        "",
		CustSSN:              123456789,
		CustGovtIssuedID:     "DL12345678",
		CustDOB:              "1980-06-15",
		CustEFTAccountID:     "1234567890",
		CustPriCardHolderInd: "Y",
		CustFICOCreditScore:  750,
	}
}

func sampleXref(acctID, custID int64) *domain.CardXrefRecord {
	return &domain.CardXrefRecord{
		XrefCardNum: "4111111111111111",
		XrefCustID:  custID,
		XrefAcctID:  acctID,
	}
}

func validUpdateRequest(acctID int64) svc.UpdateRequest {
	return svc.UpdateRequest{
		AcctID:           acctID,
		ActiveStatus:     "Y",
		CreditLimit:      decimal.RequireFromString("5000.00"),
		CashCreditLimit:  decimal.RequireFromString("1000.00"),
		CurrBal:          decimal.RequireFromString("100.00"),
		CurrCycCredit:    decimal.RequireFromString("0"),
		CurrCycDebit:     decimal.RequireFromString("0"),
		OpenDate:         "2020-01-15",
		ExpirationDate:   "2025-12-31",
		ReissueDate:      "2023-01-15",
		GroupID:          "GRP001",
		FirstName:        "John",
		MiddleName:       "A",
		LastName:         "Doe",
		AddrLine1:        "123 Main St",
		AddrLine2:        "Apt 4",
		AddrLine3:        "Beverly Hills",
		AddrStateCode:    "CA",
		AddrZip:          "90210",
		AddrCountryCode:  "USA",
		PhoneNum1:        "(310)555-1234",
		PhoneNum2:        "",
		SSN:              123456789,
		GovtIssuedID:     "DL12345678",
		DOB:              "1980-06-15",
		EFTAccountID:     "1234567890",
		PriCardHolderInd: "Y",
		FICOCreditScore:  750,
	}
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestGetByID_HappyPath(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	v, err := s.GetByID(context.Background(), 70000001001)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if v.Account == nil || v.Account.AcctID != 70000001001 {
		t.Errorf("expected account 70000001001, got %v", v.Account)
	}
	if v.Customer == nil || v.Customer.CustID != 1001 {
		t.Errorf("expected customer 1001, got %v", v.Customer)
	}
	if v.CardNum != "4111111111111111" {
		t.Errorf("expected card 4111111111111111, got %q", v.CardNum)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	_, err := s.GetByID(context.Background(), 99999999999)
	if !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ── Update happy path ─────────────────────────────────────────────────────────

func TestUpdate_HappyPath(t *testing.T) {
	s, acctStore, custStore, _ := openSeededService(t)
	ctx := context.Background()

	req := validUpdateRequest(70000001001)
	req.FirstName = "Jane"
	req.CreditLimit = decimal.RequireFromString("6000.00")

	if err := s.Update(ctx, req); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify account persisted.
	acct, err := acctStore.Get(ctx, 70000001001)
	if err != nil {
		t.Fatalf("re-read account: %v", err)
	}
	if !acct.AcctCreditLimit.Equal(decimal.RequireFromString("6000.00")) {
		t.Errorf("credit limit not updated: got %s", acct.AcctCreditLimit)
	}

	// Verify customer persisted.
	cust, err := custStore.Get(ctx, 1001)
	if err != nil {
		t.Fatalf("re-read customer: %v", err)
	}
	if cust.CustFirstName != "Jane" {
		t.Errorf("first name not updated: got %q", cust.CustFirstName)
	}
}

// ── Validation errors ─────────────────────────────────────────────────────────

func mustFailValidation(t *testing.T, s *svc.Service, req svc.UpdateRequest, expectedFragment string) {
	t.Helper()
	err := s.Update(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var ve *svc.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if expectedFragment != "" && !containsString(ve.Msg, expectedFragment) {
		t.Errorf("expected message containing %q, got %q", expectedFragment, ve.Msg)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findString(s, substr))
}

func findString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestUpdate_InvalidStatus(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.ActiveStatus = "X"
	mustFailValidation(t, s, req, "Account Status must be Y or N")
}

func TestUpdate_EmptyStatus(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.ActiveStatus = ""
	mustFailValidation(t, s, req, "Account Status must be supplied")
}

func TestUpdate_CreditLimitBelowBalance(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.CurrBal = decimal.RequireFromString("500.00")
	req.CreditLimit = decimal.RequireFromString("100.00") // less than balance
	mustFailValidation(t, s, req, "Credit Limit must be greater than or equal to current balance")
}

func TestUpdate_InvalidFICO_TooLow(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.FICOCreditScore = 299
	mustFailValidation(t, s, req, "FICO Score")
}

func TestUpdate_InvalidFICO_TooHigh(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.FICOCreditScore = 851
	mustFailValidation(t, s, req, "FICO Score")
}

func TestUpdate_SSN_InvalidArea_000(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.SSN = 0*1_000_000 + 12*10_000 + 3456 // area=000
	mustFailValidation(t, s, req, "SSN")
}

func TestUpdate_SSN_InvalidArea_666(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.SSN = 666*1_000_000 + 12*10_000 + 3456
	mustFailValidation(t, s, req, "SSN")
}

func TestUpdate_SSN_InvalidArea_900(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.SSN = 901*1_000_000 + 12*10_000 + 3456
	mustFailValidation(t, s, req, "SSN")
}

func TestUpdate_SSN_ZeroGroup(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.SSN = 123*1_000_000 + 0*10_000 + 4567 // group=00
	mustFailValidation(t, s, req, "SSN")
}

func TestUpdate_SSN_ZeroSerial(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.SSN = 123*1_000_000 + 45*10_000 + 0 // serial=0000
	mustFailValidation(t, s, req, "SSN")
}

func TestUpdate_EmptyFirstName(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.FirstName = ""
	mustFailValidation(t, s, req, "First Name")
}

func TestUpdate_NonAlphaFirstName(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.FirstName = "John123"
	mustFailValidation(t, s, req, "First Name")
}

func TestUpdate_InvalidState(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.AddrStateCode = "XX"
	mustFailValidation(t, s, req, "valid state code")
}

func TestUpdate_InvalidStateZipCombo(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.AddrStateCode = "NY"
	req.AddrZip = "90001" // 90 is not valid for NY
	mustFailValidation(t, s, req, "zip code for state")
}

func TestUpdate_ZeroZip(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.AddrZip = "00000"
	mustFailValidation(t, s, req, "Zip must not be zero")
}

func TestUpdate_InvalidPhone(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PhoneNum1 = "(011)555-1234" // area starts with 0, invalid NANP
	mustFailValidation(t, s, req, "Phone Number 1")
}

func TestUpdate_EmptyEFT(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.EFTAccountID = ""
	mustFailValidation(t, s, req, "EFT Account Id")
}

func TestUpdate_InvalidPriCardHolder(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PriCardHolderInd = "Z"
	mustFailValidation(t, s, req, "Primary Card Holder")
}

func TestUpdate_InvalidDate_OpenDate(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.OpenDate = "not-a-date"
	mustFailValidation(t, s, req, "Open Date")
}

func TestIsValidationError(t *testing.T) {
	ve := &svc.ValidationError{Msg: "test"}
	if !svc.IsValidationError(ve) {
		t.Error("expected IsValidationError to return true")
	}
	if svc.IsValidationError(errors.New("other")) {
		t.Error("expected IsValidationError to return false for generic error")
	}
}

// ── Date validation ───────────────────────────────────────────────────────────

func TestUpdate_Date_Feb31_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.OpenDate = "2021-02-31"
	mustFailValidation(t, s, req, "Open Date")
}

func TestUpdate_Date_Apr31_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.OpenDate = "2021-04-31"
	mustFailValidation(t, s, req, "Open Date")
}

func TestUpdate_Date_NonLeapFeb29_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.OpenDate = "2021-02-29" // 2021 is not a leap year
	mustFailValidation(t, s, req, "Open Date")
}

func TestUpdate_Date_LeapFeb29_Accepted(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.OpenDate = "2020-02-29" // 2020 is a leap year
	if err := s.Update(context.Background(), req); err != nil {
		t.Fatalf("expected leap-year Feb 29 to be accepted: %v", err)
	}
}

func TestUpdate_Date_YearBefore1900_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.OpenDate = "1850-01-01"
	mustFailValidation(t, s, req, "Open Date")
}

func TestUpdate_Date_YearAfter2099_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.OpenDate = "2200-01-01"
	mustFailValidation(t, s, req, "Open Date")
}

func TestUpdate_DOB_FutureRejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.DOB = "2099-12-31" // far future
	mustFailValidation(t, s, req, "Date of Birth")
}

// ── Phone area code allowlist ─────────────────────────────────────────────────

func TestUpdate_Phone_Area211_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PhoneNum1 = "(211)555-0100" // 211 is not in VALID-GENERAL-PURP-CODE
	mustFailValidation(t, s, req, "Phone Number 1")
}

func TestUpdate_Phone_Area411_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PhoneNum1 = "(411)555-0100"
	mustFailValidation(t, s, req, "Phone Number 1")
}

func TestUpdate_Phone_Area555_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PhoneNum1 = "(555)555-0100" // 555 is NOT in VALID-GENERAL-PURP-CODE
	mustFailValidation(t, s, req, "Phone Number 1")
}

func TestUpdate_Phone_Area911_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PhoneNum1 = "(911)555-0100"
	mustFailValidation(t, s, req, "Phone Number 1")
}

func TestUpdate_Phone_Area201_Accepted(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PhoneNum1 = "(201)555-0100"
	if err := s.Update(context.Background(), req); err != nil {
		t.Fatalf("expected area 201 to be accepted: %v", err)
	}
}

func TestUpdate_Phone_Area212_Accepted(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PhoneNum1 = "(212)555-0100"
	if err := s.Update(context.Background(), req); err != nil {
		t.Fatalf("expected area 212 to be accepted: %v", err)
	}
}

// ── Status case-sensitivity ───────────────────────────────────────────────────

func TestUpdate_Status_Lowercase_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.ActiveStatus = "y"
	mustFailValidation(t, s, req, "Account Status must be Y or N")
}

// ── PriCardHolderInd case-sensitivity ─────────────────────────────────────────

func TestUpdate_PriCardHolder_Lowercase_Rejected(t *testing.T) {
	s, _, _, _ := openSeededService(t)
	req := validUpdateRequest(70000001001)
	req.PriCardHolderInd = "y"
	mustFailValidation(t, s, req, "Primary Card Holder")
}
