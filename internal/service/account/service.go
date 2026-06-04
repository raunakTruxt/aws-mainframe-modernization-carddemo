// Package account provides the business logic for the account view/update
// screens (COACTVWC / COACTUPC in the legacy COBOL system).
package account

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// View is the joined read model returned by GetByID.
type View struct {
	Account  *domain.AccountRecord
	Customer *domain.CustomerRecord
	CardNum  string // primary card number from xref
}

// UpdateRequest carries all editable fields from the update form.
// Field names mirror the BMS screen fields from COACTUPC / COACTUP.bms.
type UpdateRequest struct {
	AcctID int64

	// Account fields (ACCTDAT)
	ActiveStatus    string
	CreditLimit     decimal.Decimal
	CashCreditLimit decimal.Decimal
	CurrBal         decimal.Decimal
	CurrCycCredit   decimal.Decimal
	CurrCycDebit    decimal.Decimal
	OpenDate        string // YYYY-MM-DD
	ExpirationDate  string // YYYY-MM-DD
	ReissueDate     string // YYYY-MM-DD
	GroupID         string

	// Customer fields (CUSTDAT)
	FirstName        string
	MiddleName       string
	LastName         string
	AddrLine1        string
	AddrLine2        string
	AddrLine3        string // city
	AddrStateCode    string
	AddrZip          string // 5-digit
	AddrCountryCode  string
	PhoneNum1        string // "(NNN)NNN-NNNN", empty = not provided
	PhoneNum2        string
	SSN              int64 // 9-digit: area*1000000 + group*10000 + serial
	GovtIssuedID     string
	DOB              string // YYYY-MM-DD
	EFTAccountID     string
	PriCardHolderInd string // Y or N
	FICOCreditScore  int64  // 300–850
}

// ValidationError is returned when one or more fields fail validation.
// The message mirrors the COBOL WS-RETURN-MSG text (first error only).
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// IsValidationError reports whether err is a *ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// Service holds the business logic for account view and update operations.
type Service struct {
	accounts  repo.AccountRepository
	customers repo.CustomerRepository
	xref      repo.CardXrefRepository
}

// New constructs a Service.
func New(
	acctRepo repo.AccountRepository,
	custRepo repo.CustomerRepository,
	xrefRepo repo.CardXrefRepository,
) *Service {
	return &Service{accounts: acctRepo, customers: custRepo, xref: xrefRepo}
}

// GetByID fetches the account, its primary xref entry, and its associated
// customer. Returns repo.ErrNotFound (wrapped) if the account does not exist.
func (s *Service) GetByID(ctx context.Context, acctID int64) (*View, error) {
	acct, err := s.accounts.Get(ctx, acctID)
	if err != nil {
		return nil, fmt.Errorf("account: GetByID: %w", err)
	}

	var cardNum string
	var custID int64
	xrefs, err := s.xref.GetByAccount(ctx, acctID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, fmt.Errorf("account: GetByID: xref: %w", err)
	}
	if len(xrefs) > 0 {
		cardNum = xrefs[0].XrefCardNum
		custID = xrefs[0].XrefCustID
	}

	var cust *domain.CustomerRecord
	if custID > 0 {
		cust, err = s.customers.Get(ctx, custID)
		if err != nil && !errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("account: GetByID: customer: %w", err)
		}
	}

	return &View{Account: acct, Customer: cust, CardNum: cardNum}, nil
}

// Update validates the request, re-reads both records, applies changes, and
// writes them back. Returns *ValidationError for field-level failures.
// The business rule "credit limit must be ≥ current balance" is enforced here.
func (s *Service) Update(ctx context.Context, req UpdateRequest) error {
	if err := validate(req); err != nil {
		return err
	}

	acct, err := s.accounts.Get(ctx, req.AcctID)
	if err != nil {
		return fmt.Errorf("account: Update: %w", err)
	}

	xrefs, err := s.xref.GetByAccount(ctx, req.AcctID)
	if err != nil || len(xrefs) == 0 {
		return fmt.Errorf("account: Update: xref not found for account %d", req.AcctID)
	}
	cust, err := s.customers.Get(ctx, xrefs[0].XrefCustID)
	if err != nil {
		return fmt.Errorf("account: Update: customer: %w", err)
	}

	// Apply updates to the account record.
	acct.AcctActiveStatus = strings.ToUpper(req.ActiveStatus)
	acct.AcctCreditLimit = req.CreditLimit
	acct.AcctCashCreditLimit = req.CashCreditLimit
	acct.AcctCurrBal = req.CurrBal
	acct.AcctCurrCycCredit = req.CurrCycCredit
	acct.AcctCurrCycDebit = req.CurrCycDebit
	acct.AcctOpenDate = req.OpenDate
	acct.AcctExpirationDate = req.ExpirationDate
	acct.AcctReissueDate = req.ReissueDate
	acct.AcctGroupID = req.GroupID

	// Apply updates to the customer record.
	cust.CustFirstName = req.FirstName
	cust.CustMiddleName = req.MiddleName
	cust.CustLastName = req.LastName
	cust.CustAddrLine1 = req.AddrLine1
	cust.CustAddrLine2 = req.AddrLine2
	cust.CustAddrLine3 = req.AddrLine3
	cust.CustAddrStateCode = strings.ToUpper(req.AddrStateCode)
	cust.CustAddrZip = req.AddrZip
	cust.CustAddrCountryCode = strings.ToUpper(req.AddrCountryCode)
	cust.CustPhoneNum1 = req.PhoneNum1
	cust.CustPhoneNum2 = req.PhoneNum2
	cust.CustSSN = req.SSN
	cust.CustGovtIssuedID = req.GovtIssuedID
	cust.CustDOB = req.DOB
	cust.CustEFTAccountID = req.EFTAccountID
	cust.CustPriCardHolderInd = strings.ToUpper(req.PriCardHolderInd)
	cust.CustFICOCreditScore = req.FICOCreditScore

	// Write account first; if customer write fails we cannot roll back the
	// account write in SQLite without a transaction, but both stores use the
	// same *sql.DB so a wrapper transaction is possible if the caller needs it.
	if err := s.accounts.Update(ctx, acct); err != nil {
		return fmt.Errorf("account: Update: write account: %w", err)
	}
	if err := s.customers.Update(ctx, cust); err != nil {
		return fmt.Errorf("account: Update: write customer: %w", err)
	}
	return nil
}

// validate mirrors COACTUPC 1200-EDIT-MAP-FIELDS.
// Returns the first *ValidationError found, mirroring COBOL single-error behaviour.
func validate(req UpdateRequest) error {
	// Account status: must be Y or N.
	status := strings.ToUpper(strings.TrimSpace(req.ActiveStatus))
	if status == "" {
		return &ValidationError{"Account Status must be supplied."}
	}
	if status != "Y" && status != "N" {
		return &ValidationError{"Account Status must be Y or N."}
	}

	if err := validateDate(req.OpenDate, "Open Date"); err != nil {
		return err
	}
	if err := validateSignedDecimal(req.CreditLimit, "Credit Limit"); err != nil {
		return err
	}
	if err := validateDate(req.ExpirationDate, "Expiry Date"); err != nil {
		return err
	}
	if err := validateSignedDecimal(req.CashCreditLimit, "Cash Credit Limit"); err != nil {
		return err
	}
	if err := validateDate(req.ReissueDate, "Reissue Date"); err != nil {
		return err
	}
	if err := validateSignedDecimal(req.CurrBal, "Current Balance"); err != nil {
		return err
	}

	// Credit limit must be ≥ current balance.
	if req.CreditLimit.LessThan(req.CurrBal) {
		return &ValidationError{"Credit Limit must be greater than or equal to current balance."}
	}

	if err := validateSignedDecimal(req.CurrCycCredit, "Current Cycle Credit Limit"); err != nil {
		return err
	}
	if err := validateSignedDecimal(req.CurrCycDebit, "Current Cycle Debit Limit"); err != nil {
		return err
	}
	if err := validateSSN(req.SSN); err != nil {
		return err
	}
	if err := validateDate(req.DOB, "Date of Birth"); err != nil {
		return err
	}
	if req.FICOCreditScore < 300 || req.FICOCreditScore > 850 {
		return &ValidationError{"FICO Score: should be between 300 and 850"}
	}

	if err := validateAlphaRequired(req.FirstName, "First Name"); err != nil {
		return err
	}
	if mn := strings.TrimSpace(req.MiddleName); mn != "" {
		if !isAlphaSpace(mn) {
			return &ValidationError{"Middle Name can have alphabets only."}
		}
	}
	if err := validateAlphaRequired(req.LastName, "Last Name"); err != nil {
		return err
	}
	if strings.TrimSpace(req.AddrLine1) == "" {
		return &ValidationError{"Address Line 1 must be supplied."}
	}

	stateUpper := strings.ToUpper(strings.TrimSpace(req.AddrStateCode))
	if stateUpper == "" {
		return &ValidationError{"State must be supplied."}
	}
	if !isAlphaSpace(stateUpper) {
		return &ValidationError{"State can have alphabets only."}
	}
	if !validUSStateCode[stateUpper] {
		return &ValidationError{"State: is not a valid state code"}
	}

	zip := strings.TrimSpace(req.AddrZip)
	if zip == "" {
		return &ValidationError{"Zip must be supplied."}
	}
	if len(zip) < 5 || !isNumeric(zip[:5]) {
		return &ValidationError{"Zip must be a 5 digit number."}
	}
	if zip[:5] == "00000" {
		return &ValidationError{"Zip must not be zero."}
	}

	if err := validateAlphaRequired(req.AddrLine3, "City"); err != nil {
		return err
	}

	countryUpper := strings.ToUpper(strings.TrimSpace(req.AddrCountryCode))
	if countryUpper == "" {
		return &ValidationError{"Country must be supplied."}
	}
	if !isAlphaSpace(countryUpper) {
		return &ValidationError{"Country can have alphabets only."}
	}

	if ph1 := strings.TrimSpace(req.PhoneNum1); ph1 != "" {
		if err := validatePhone(ph1, "Phone Number 1"); err != nil {
			return err
		}
	}
	if ph2 := strings.TrimSpace(req.PhoneNum2); ph2 != "" {
		if err := validatePhone(ph2, "Phone Number 2"); err != nil {
			return err
		}
	}

	eft := strings.TrimSpace(req.EFTAccountID)
	if eft == "" {
		return &ValidationError{"EFT Account Id must be supplied."}
	}
	if !isNumeric(eft) {
		return &ValidationError{"EFT Account Id must be a numeric number."}
	}
	if isAllZeros(eft) {
		return &ValidationError{"EFT Account Id must not be zero."}
	}

	phi := strings.ToUpper(strings.TrimSpace(req.PriCardHolderInd))
	if phi == "" {
		return &ValidationError{"Primary Card Holder must be supplied."}
	}
	if phi != "Y" && phi != "N" {
		return &ValidationError{"Primary Card Holder must be Y or N."}
	}

	// State + ZIP cross-edit.
	if len(zip) >= 2 {
		if !validStateZipCombo[stateUpper+zip[:2]] {
			return &ValidationError{"Invalid zip code for state"}
		}
	}

	return nil
}

// validateDate checks that s is a non-empty YYYY-MM-DD calendar date.
func validateDate(s, label string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return &ValidationError{label + " must be supplied."}
	}
	parts := strings.Split(s, "-")
	if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		return &ValidationError{label + " must be in YYYY-MM-DD format."}
	}
	y, errY := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	d, errD := strconv.Atoi(parts[2])
	if errY != nil || errM != nil || errD != nil || y < 1 || m < 1 || m > 12 || d < 1 || d > 31 {
		return &ValidationError{label + " is not a valid date."}
	}
	return nil
}

// validateSignedDecimal checks that d is a valid signed decimal value.
// The COBOL validator only rejects non-numeric input; any numeric value is OK.
func validateSignedDecimal(d decimal.Decimal, label string) error {
	// decimal.Decimal is always valid in Go; only the string parse can fail,
	// which happens in the handler before calling service. No-op here.
	_ = d
	_ = label
	return nil
}

// validateSSN checks SSN parts extracted from the 9-digit int64.
func validateSSN(ssn int64) error {
	area := ssn / 1_000_000
	group := (ssn / 10_000) % 100
	serial := ssn % 10_000

	if area == 0 || area == 666 || (area >= 900 && area <= 999) {
		return &ValidationError{"SSN: First 3 chars should not be 000, 666, or between 900 and 999"}
	}
	if group == 0 {
		return &ValidationError{"SSN 4th & 5th chars must not be zero."}
	}
	if serial == 0 {
		return &ValidationError{"SSN Last 4 chars must not be zero."}
	}
	return nil
}

// validateAlphaRequired checks a mandatory alphabets-and-spaces field.
func validateAlphaRequired(s, label string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return &ValidationError{label + " must be supplied."}
	}
	if !isAlphaSpace(s) {
		return &ValidationError{label + " can have alphabets only."}
	}
	return nil
}

// validatePhone checks (NNN)NNN-NNNN format — valid North American phone.
func validatePhone(phone, label string) error {
	// Accept "(NNN)NNN-NNNN" (13 chars).
	phone = strings.TrimSpace(phone)
	if len(phone) != 13 || phone[0] != '(' || phone[4] != ')' || phone[8] != '-' {
		return &ValidationError{label + ": must be in (NNN)NNN-NNNN format."}
	}
	area := phone[1:4]
	prefix := phone[5:8]
	line := phone[9:13]

	if !isNumeric(area) {
		return &ValidationError{label + ": Area code must be numeric."}
	}
	areaInt, _ := strconv.Atoi(area)
	if areaInt == 0 {
		return &ValidationError{label + ": Area code must be supplied."}
	}
	// Valid North American area codes don't start with 0 or 1.
	if area[0] == '0' || area[0] == '1' {
		return &ValidationError{label + ": Not valid North America general purpose area code"}
	}

	if !isNumeric(prefix) {
		return &ValidationError{label + ": Prefix code must be numeric."}
	}
	prefixInt, _ := strconv.Atoi(prefix)
	if prefixInt == 0 {
		return &ValidationError{label + ": Prefix code cannot be zero"}
	}

	if !isNumeric(line) || len(line) != 4 {
		return &ValidationError{label + ": Line number code must be a 4 digit number."}
	}
	return nil
}

func isAlphaSpace(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && r != ' ' {
			return false
		}
	}
	return true
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isAllZeros(s string) bool {
	for _, r := range s {
		if r != '0' {
			return false
		}
	}
	return true
}

// validUSStateCode is the set of valid 2-letter US state/territory codes
// from CSLKPCDY.cpy (VALID-US-STATE-CODE).
var validUSStateCode = map[string]bool{
	"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true,
	"CO": true, "CT": true, "DE": true, "FL": true, "GA": true,
	"HI": true, "ID": true, "IL": true, "IN": true, "IA": true,
	"KS": true, "KY": true, "LA": true, "ME": true, "MD": true,
	"MA": true, "MI": true, "MN": true, "MS": true, "MO": true,
	"MT": true, "NE": true, "NV": true, "NH": true, "NJ": true,
	"NM": true, "NY": true, "NC": true, "ND": true, "OH": true,
	"OK": true, "OR": true, "PA": true, "RI": true, "SC": true,
	"SD": true, "TN": true, "TX": true, "UT": true, "VT": true,
	"VA": true, "WA": true, "WV": true, "WI": true, "WY": true,
	"DC": true, "AS": true, "GU": true, "MP": true, "PR": true, "VI": true,
}

// validStateZipCombo is the set of valid state+2-digit-ZIP-prefix combinations
// from CSLKPCDY.cpy (VALID-US-STATE-ZIP-CD2-COMBO). Key = state + first 2 digits of ZIP.
var validStateZipCombo = func() map[string]bool {
	combos := map[string][]string{
		"AA": {"34"},
		"AE": {"90", "91", "92", "93", "94", "95", "96", "97", "98"},
		"AK": {"99"},
		"AL": {"35", "36"},
		"AP": {"96"},
		"AR": {"71", "72"},
		"AS": {"96"},
		"AZ": {"85", "86"},
		"CA": {"90", "91", "92", "93", "94", "95", "96"},
		"CO": {"80", "81"},
		"CT": {"60", "61", "62", "63", "64", "65", "66", "67", "68", "69"},
		"DC": {"20", "56", "88"},
		"DE": {"19"},
		"FL": {"32", "33", "34"},
		"FM": {"96"},
		"GA": {"30", "31", "39"},
		"GU": {"96"},
		"HI": {"96"},
		"IA": {"50", "51", "52"},
		"ID": {"83"},
		"IL": {"60", "61", "62"},
		"IN": {"46", "47"},
		"KS": {"66", "67"},
		"KY": {"40", "41", "42"},
		"LA": {"70", "71"},
		"MA": {"10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "55"},
		"MD": {"20", "21"},
		"ME": {"39", "40", "41", "42", "43", "44", "45", "46", "47", "48", "49"},
		"MH": {"96"},
		"MI": {"48", "49"},
		"MN": {"55", "56"},
		"MO": {"63", "64", "65", "72"},
		"MP": {"96"},
		"MS": {"38", "39"},
		"MT": {"59"},
		"NC": {"27", "28"},
		"ND": {"58"},
		"NE": {"68", "69"},
		"NH": {"30", "31", "32", "33", "34", "35", "36", "37", "38"},
		"NJ": {"70", "71", "72", "73", "74", "75", "76", "77", "78", "79", "80", "81", "82", "83", "84", "85", "86", "87", "88", "89"},
		"NM": {"87", "88"},
		"NV": {"88", "89"},
		"NY": {"10", "11", "12", "13", "14", "50", "54", "63"},
		"OH": {"43", "44", "45"},
		"OK": {"73", "74"},
		"OR": {"97"},
		"PA": {"15", "16", "17", "18", "19"},
		"PR": {"60", "61", "62", "63", "64", "65", "66", "67", "68", "69", "70", "71", "72", "73", "74", "75", "76", "77", "78", "79", "90", "91", "92", "93", "94", "95", "96", "97", "98"},
		"PW": {"96"},
		"RI": {"28", "29"},
		"SC": {"29"},
		"SD": {"57"},
		"TN": {"37", "38"},
		"TX": {"73", "75", "76", "77", "78", "79", "88"},
		"UT": {"84"},
		"VA": {"20", "22", "23", "24"},
		"VI": {"80", "82", "83", "84", "85"},
		"VT": {"50", "51", "52", "53", "54", "56", "57", "58", "59"},
		"WA": {"98", "99"},
		"WI": {"53", "54"},
		"WV": {"24", "25", "26"},
		"WY": {"82", "83"},
	}
	m := make(map[string]bool, 300)
	for state, prefixes := range combos {
		for _, p := range prefixes {
			m[state+p] = true
		}
	}
	return m
}()
