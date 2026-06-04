package domain

import (
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/cobolfmt"
	"github.com/shopspring/decimal"
)

// ExportRecord is the Go port of EXPORT-RECORD (CVEXPORT.cpy, 500 bytes).
//
// The copybook uses REDEFINES to overlay the 460-byte EXPORT-RECORD-DATA
// field with five mutually exclusive record types. Only one of Customer,
// Account, Transaction, CardXref, or Card is non-nil based on RecType.
//
// Common header layout:
//
//	05 EXPORT-REC-TYPE         PIC X(1)    offset   0 len  1
//	05 EXPORT-TIMESTAMP        PIC X(26)   offset   1 len 26
//	05 EXPORT-SEQUENCE-NUM     PIC 9(9)    COMP     offset  27 len  4
//	05 EXPORT-BRANCH-ID        PIC X(4)    offset  31 len  4
//	05 EXPORT-REGION-CODE      PIC X(5)    offset  36 len  5
//	05 EXPORT-RECORD-DATA      PIC X(460)  offset  40 len 460
type ExportRecord struct {
	RecType     string // EXPORT-REC-TYPE PIC X(1)
	Timestamp   string // EXPORT-TIMESTAMP PIC X(26)
	SequenceNum int64  // EXPORT-SEQUENCE-NUM PIC 9(9) COMP
	BranchID    string // EXPORT-BRANCH-ID PIC X(4)
	RegionCode  string // EXPORT-REGION-CODE PIC X(5)

	// Exactly one of the following is non-nil.
	Customer    *ExportCustomerData
	Account     *ExportAccountData
	Transaction *ExportTransactionData
	CardXref    *ExportCardXrefData
	Card        *ExportCardData
}

// ExportCustomerData is the EXPORT-CUSTOMER-DATA REDEFINES (460 bytes, RecType "C").
//
// Layout within EXPORT-RECORD-DATA:
//
//	10 EXP-CUST-ID                 PIC 9(09) COMP         offset   0 len  4
//	10 EXP-CUST-FIRST-NAME         PIC X(25)              offset   4 len 25
//	10 EXP-CUST-MIDDLE-NAME        PIC X(25)              offset  29 len 25
//	10 EXP-CUST-LAST-NAME          PIC X(25)              offset  54 len 25
//	10 EXP-CUST-ADDR-LINES (3×50)  PIC X(50)              offset  79 len 150
//	10 EXP-CUST-ADDR-STATE-CD      PIC X(02)              offset 229 len  2
//	10 EXP-CUST-ADDR-COUNTRY-CD    PIC X(03)              offset 231 len  3
//	10 EXP-CUST-ADDR-ZIP           PIC X(10)              offset 234 len 10
//	10 EXP-CUST-PHONE-NUMS (2×15)  PIC X(15)              offset 244 len 30
//	10 EXP-CUST-SSN                PIC 9(09)              offset 274 len  9
//	10 EXP-CUST-GOVT-ISSUED-ID     PIC X(20)              offset 283 len 20
//	10 EXP-CUST-DOB-YYYY-MM-DD     PIC X(10)              offset 303 len 10
//	10 EXP-CUST-EFT-ACCOUNT-ID     PIC X(10)              offset 313 len 10
//	10 EXP-CUST-PRI-CARD-HOLDER-IND PIC X(01)             offset 323 len  1
//	10 EXP-CUST-FICO-CREDIT-SCORE  PIC 9(03) COMP-3       offset 324 len  2
//	10 FILLER                      PIC X(134)             offset 326 len 134
type ExportCustomerData struct {
	CustID             int64           // PIC 9(09) COMP
	FirstName          string          // PIC X(25)
	MiddleName         string          // PIC X(25)
	LastName           string          // PIC X(25)
	AddrLine1          string          // PIC X(50) (OCCURS index 1)
	AddrLine2          string          // PIC X(50) (OCCURS index 2)
	AddrLine3          string          // PIC X(50) (OCCURS index 3)
	AddrStateCode      string          // PIC X(02)
	AddrCountryCode    string          // PIC X(03)
	AddrZip            string          // PIC X(10)
	PhoneNum1          string          // PIC X(15) (OCCURS index 1)
	PhoneNum2          string          // PIC X(15) (OCCURS index 2)
	SSN                int64           // PIC 9(09) zoned unsigned
	GovtIssuedID       string          // PIC X(20)
	DOB                string          // PIC X(10)
	EFTAccountID       string          // PIC X(10)
	PriCardHolderInd   string          // PIC X(01)
	FICOCreditScore    decimal.Decimal // PIC 9(03) COMP-3 (unsigned)
}

// ExportAccountData is the EXPORT-ACCOUNT-DATA REDEFINES (460 bytes, RecType "A").
//
// Layout within EXPORT-RECORD-DATA:
//
//	10 EXP-ACCT-ID               PIC 9(11)              offset   0 len 11
//	10 EXP-ACCT-ACTIVE-STATUS    PIC X(01)              offset  11 len  1
//	10 EXP-ACCT-CURR-BAL         PIC S9(10)V99 COMP-3   offset  12 len  7
//	10 EXP-ACCT-CREDIT-LIMIT     PIC S9(10)V99          offset  19 len 12
//	10 EXP-ACCT-CASH-CREDIT-LIMIT PIC S9(10)V99 COMP-3  offset  31 len  7
//	10 EXP-ACCT-OPEN-DATE        PIC X(10)              offset  38 len 10
//	10 EXP-ACCT-EXPIRAION-DATE   PIC X(10)              offset  48 len 10
//	10 EXP-ACCT-REISSUE-DATE     PIC X(10)              offset  58 len 10
//	10 EXP-ACCT-CURR-CYC-CREDIT  PIC S9(10)V99          offset  68 len 12
//	10 EXP-ACCT-CURR-CYC-DEBIT   PIC S9(10)V99 COMP     offset  80 len  8
//	10 EXP-ACCT-ADDR-ZIP         PIC X(10)              offset  88 len 10
//	10 EXP-ACCT-GROUP-ID         PIC X(10)              offset  98 len 10
//	10 FILLER                    PIC X(352)             offset 108 len 352
type ExportAccountData struct {
	AcctID           int64           // PIC 9(11) zoned unsigned
	AcctActiveStatus string          // PIC X(01)
	AcctCurrBal      decimal.Decimal // PIC S9(10)V99 COMP-3 signed
	AcctCreditLimit  decimal.Decimal // PIC S9(10)V99 zoned signed
	AcctCashCredit   decimal.Decimal // PIC S9(10)V99 COMP-3 signed
	AcctOpenDate     string          // PIC X(10)
	AcctExpiryDate   string          // PIC X(10)
	AcctReissueDate  string          // PIC X(10)
	AcctCurrCycCredit decimal.Decimal // PIC S9(10)V99 zoned signed
	AcctCurrCycDebit  decimal.Decimal // PIC S9(10)V99 COMP signed scale=2
	AcctAddrZip      string          // PIC X(10)
	AcctGroupID      string          // PIC X(10)
}

// ExportTransactionData is the EXPORT-TRANSACTION-DATA REDEFINES (460 bytes, RecType "T").
//
// Layout within EXPORT-RECORD-DATA:
//
//	10 EXP-TRAN-ID               PIC X(16)              offset   0 len 16
//	10 EXP-TRAN-TYPE-CD          PIC X(02)              offset  16 len  2
//	10 EXP-TRAN-CAT-CD           PIC 9(04)              offset  18 len  4
//	10 EXP-TRAN-SOURCE           PIC X(10)              offset  22 len 10
//	10 EXP-TRAN-DESC             PIC X(100)             offset  32 len 100
//	10 EXP-TRAN-AMT              PIC S9(09)V99 COMP-3   offset 132 len  6
//	10 EXP-TRAN-MERCHANT-ID      PIC 9(09) COMP         offset 138 len  4
//	10 EXP-TRAN-MERCHANT-NAME    PIC X(50)              offset 142 len 50
//	10 EXP-TRAN-MERCHANT-CITY    PIC X(50)              offset 192 len 50
//	10 EXP-TRAN-MERCHANT-ZIP     PIC X(10)              offset 242 len 10
//	10 EXP-TRAN-CARD-NUM         PIC X(16)              offset 252 len 16
//	10 EXP-TRAN-ORIG-TS          PIC X(26)              offset 268 len 26
//	10 EXP-TRAN-PROC-TS          PIC X(26)              offset 294 len 26
//	10 FILLER                    PIC X(140)             offset 320 len 140
type ExportTransactionData struct {
	TranID          string          // PIC X(16)
	TranTypeCode    string          // PIC X(02)
	TranCatCode     int64           // PIC 9(04) zoned unsigned
	TranSource      string          // PIC X(10)
	TranDesc        string          // PIC X(100)
	TranAmt         decimal.Decimal // PIC S9(09)V99 COMP-3 signed scale=2
	TranMerchantID  int64           // PIC 9(09) COMP unsigned
	MerchantName    string          // PIC X(50)
	MerchantCity    string          // PIC X(50)
	MerchantZip     string          // PIC X(10)
	TranCardNum     string          // PIC X(16)
	TranOrigTS      string          // PIC X(26)
	TranProcTS      string          // PIC X(26)
}

// ExportCardXrefData is the EXPORT-CARD-XREF-DATA REDEFINES (460 bytes, RecType "X").
//
// Layout within EXPORT-RECORD-DATA:
//
//	10 EXP-XREF-CARD-NUM   PIC X(16)      offset  0 len 16
//	10 EXP-XREF-CUST-ID    PIC 9(09)      offset 16 len  9
//	10 EXP-XREF-ACCT-ID    PIC 9(11) COMP offset 25 len  8
//	10 FILLER              PIC X(427)     offset 33 len 427
type ExportCardXrefData struct {
	XrefCardNum string // PIC X(16)
	XrefCustID  int64  // PIC 9(09) zoned unsigned
	XrefAcctID  int64  // PIC 9(11) COMP unsigned
}

// ExportCardData is the EXPORT-CARD-DATA REDEFINES (460 bytes, RecType "D").
//
// Layout within EXPORT-RECORD-DATA:
//
//	10 EXP-CARD-NUM              PIC X(16)        offset  0 len 16
//	10 EXP-CARD-ACCT-ID          PIC 9(11) COMP   offset 16 len  8
//	10 EXP-CARD-CVV-CD           PIC 9(03) COMP   offset 24 len  2
//	10 EXP-CARD-EMBOSSED-NAME    PIC X(50)         offset 26 len 50
//	10 EXP-CARD-EXPIRAION-DATE   PIC X(10)         offset 76 len 10
//	10 EXP-CARD-ACTIVE-STATUS    PIC X(01)         offset 86 len  1
//	10 FILLER                    PIC X(373)        offset 87 len 373
type ExportCardData struct {
	CardNum        string // PIC X(16)
	CardAcctID     int64  // PIC 9(11) COMP unsigned
	CardCVVCode    int64  // PIC 9(03) COMP unsigned
	CardEmbossed   string // PIC X(50)
	CardExpiryDate string // PIC X(10)
	CardActive     string // PIC X(01)
}

// ExportRecordLen is the fixed length of an EXPORT-RECORD in bytes.
const ExportRecordLen = 500

// exportHeaderLen is the number of bytes before EXPORT-RECORD-DATA.
const exportHeaderLen = 40

// exportDataLen is the length of the EXPORT-RECORD-DATA section.
const exportDataLen = 460

// DecodeExportRecord decodes one 500-byte record from the multi-record export file.
func DecodeExportRecord(b []byte) (*ExportRecord, error) {
	if len(b) < ExportRecordLen {
		return nil, fmt.Errorf("domain: export: record too short: got %d, want %d", len(b), ExportRecordLen)
	}
	b = b[:ExportRecordLen]

	var r ExportRecord
	var err error

	if r.RecType, err = cobolfmt.DecodeEBCDICTrimmed(b[0:1]); err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-REC-TYPE: %w", err)
	}
	if r.Timestamp, err = cobolfmt.DecodeEBCDICTrimmed(b[1:27]); err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-TIMESTAMP: %w", err)
	}
	r.SequenceNum, err = cobolfmt.DecodeComp(b[27:31], false)
	if err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-SEQUENCE-NUM: %w", err)
	}
	if r.BranchID, err = cobolfmt.DecodeEBCDICTrimmed(b[31:35]); err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-BRANCH-ID: %w", err)
	}
	if r.RegionCode, err = cobolfmt.DecodeEBCDICTrimmed(b[35:40]); err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-REGION-CODE: %w", err)
	}

	data := b[exportHeaderLen:]

	switch r.RecType {
	case "C":
		r.Customer, err = decodeExportCustomer(data)
	case "A":
		r.Account, err = decodeExportAccount(data)
	case "T":
		r.Transaction, err = decodeExportTransaction(data)
	case "X":
		r.CardXref, err = decodeExportCardXref(data)
	case "D":
		r.Card, err = decodeExportCard(data)
	default:
		return nil, fmt.Errorf("domain: export: unknown record type %q", r.RecType)
	}
	if err != nil {
		return nil, fmt.Errorf("domain: export: type %q data: %w", r.RecType, err)
	}

	return &r, nil
}

func decodeExportCustomer(d []byte) (*ExportCustomerData, error) {
	var c ExportCustomerData
	var err error

	c.CustID, err = cobolfmt.DecodeComp(d[0:4], false)
	if err != nil {
		return nil, fmt.Errorf("EXP-CUST-ID: %w", err)
	}
	for _, af := range []struct {
		dst    *string
		off, n int
		label  string
	}{
		{&c.FirstName, 4, 25, "EXP-CUST-FIRST-NAME"},
		{&c.MiddleName, 29, 25, "EXP-CUST-MIDDLE-NAME"},
		{&c.LastName, 54, 25, "EXP-CUST-LAST-NAME"},
		{&c.AddrLine1, 79, 50, "EXP-CUST-ADDR-LINE(1)"},
		{&c.AddrLine2, 129, 50, "EXP-CUST-ADDR-LINE(2)"},
		{&c.AddrLine3, 179, 50, "EXP-CUST-ADDR-LINE(3)"},
		{&c.AddrStateCode, 229, 2, "EXP-CUST-ADDR-STATE-CD"},
		{&c.AddrCountryCode, 231, 3, "EXP-CUST-ADDR-COUNTRY-CD"},
		{&c.AddrZip, 234, 10, "EXP-CUST-ADDR-ZIP"},
		{&c.PhoneNum1, 244, 15, "EXP-CUST-PHONE-NUM(1)"},
		{&c.PhoneNum2, 259, 15, "EXP-CUST-PHONE-NUM(2)"},
		{&c.GovtIssuedID, 283, 20, "EXP-CUST-GOVT-ISSUED-ID"},
		{&c.DOB, 303, 10, "EXP-CUST-DOB-YYYY-MM-DD"},
		{&c.EFTAccountID, 313, 10, "EXP-CUST-EFT-ACCOUNT-ID"},
		{&c.PriCardHolderInd, 323, 1, "EXP-CUST-PRI-CARD-HOLDER-IND"},
	} {
		*af.dst, err = cobolfmt.DecodeEBCDICTrimmed(d[af.off : af.off+af.n])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", af.label, err)
		}
	}
	c.SSN, err = cobolfmt.DecodeZonedUint(d[274:283])
	if err != nil {
		return nil, fmt.Errorf("EXP-CUST-SSN: %w", err)
	}
	// PIC 9(03) COMP-3 — unsigned, sign nibble 0xF.
	c.FICOCreditScore, err = cobolfmt.DecodePacked(d[324:326], 0)
	if err != nil {
		return nil, fmt.Errorf("EXP-CUST-FICO-CREDIT-SCORE: %w", err)
	}
	return &c, nil
}

func decodeExportAccount(d []byte) (*ExportAccountData, error) {
	var a ExportAccountData
	var err error

	a.AcctID, err = cobolfmt.DecodeZonedUint(d[0:11])
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-ID: %w", err)
	}
	a.AcctActiveStatus, err = cobolfmt.DecodeEBCDICTrimmed(d[11:12])
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-ACTIVE-STATUS: %w", err)
	}
	// PIC S9(10)V99 COMP-3 (12 digits, scale=2, signed).
	a.AcctCurrBal, err = cobolfmt.DecodePacked(d[12:19], 2)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CURR-BAL: %w", err)
	}
	a.AcctCreditLimit, err = cobolfmt.DecodeZoned(d[19:31], 2, true)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CREDIT-LIMIT: %w", err)
	}
	// PIC S9(10)V99 COMP-3 (12 digits, scale=2, signed).
	a.AcctCashCredit, err = cobolfmt.DecodePacked(d[31:38], 2)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CASH-CREDIT-LIMIT: %w", err)
	}
	for _, af := range []struct {
		dst    *string
		off, n int
		label  string
	}{
		{&a.AcctOpenDate, 38, 10, "EXP-ACCT-OPEN-DATE"},
		{&a.AcctExpiryDate, 48, 10, "EXP-ACCT-EXPIRAION-DATE"},
		{&a.AcctReissueDate, 58, 10, "EXP-ACCT-REISSUE-DATE"},
	} {
		*af.dst, err = cobolfmt.DecodeEBCDICTrimmed(d[af.off : af.off+af.n])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", af.label, err)
		}
	}
	a.AcctCurrCycCredit, err = cobolfmt.DecodeZoned(d[68:80], 2, true)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CURR-CYC-CREDIT: %w", err)
	}
	// PIC S9(10)V99 COMP (12 digits, scale=2, signed binary).
	// The stored integer represents value × 100.
	rawDebit, err := cobolfmt.DecodeComp(d[80:88], true)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CURR-CYC-DEBIT: %w", err)
	}
	a.AcctCurrCycDebit = decimal.NewFromInt(rawDebit).Shift(-2)
	for _, af := range []struct {
		dst    *string
		off, n int
		label  string
	}{
		{&a.AcctAddrZip, 88, 10, "EXP-ACCT-ADDR-ZIP"},
		{&a.AcctGroupID, 98, 10, "EXP-ACCT-GROUP-ID"},
	} {
		*af.dst, err = cobolfmt.DecodeEBCDICTrimmed(d[af.off : af.off+af.n])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", af.label, err)
		}
	}
	return &a, nil
}

func decodeExportTransaction(d []byte) (*ExportTransactionData, error) {
	var t ExportTransactionData
	var err error

	for _, af := range []struct {
		dst    *string
		off, n int
		label  string
	}{
		{&t.TranID, 0, 16, "EXP-TRAN-ID"},
		{&t.TranTypeCode, 16, 2, "EXP-TRAN-TYPE-CD"},
		{&t.TranSource, 22, 10, "EXP-TRAN-SOURCE"},
		{&t.TranDesc, 32, 100, "EXP-TRAN-DESC"},
	} {
		*af.dst, err = cobolfmt.DecodeEBCDICTrimmed(d[af.off : af.off+af.n])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", af.label, err)
		}
	}
	t.TranCatCode, err = cobolfmt.DecodeZonedUint(d[18:22])
	if err != nil {
		return nil, fmt.Errorf("EXP-TRAN-CAT-CD: %w", err)
	}
	// PIC S9(09)V99 COMP-3 (11 digits, scale=2, signed).
	t.TranAmt, err = cobolfmt.DecodePacked(d[132:138], 2)
	if err != nil {
		return nil, fmt.Errorf("EXP-TRAN-AMT: %w", err)
	}
	t.TranMerchantID, err = cobolfmt.DecodeComp(d[138:142], false)
	if err != nil {
		return nil, fmt.Errorf("EXP-TRAN-MERCHANT-ID: %w", err)
	}
	for _, af := range []struct {
		dst    *string
		off, n int
		label  string
	}{
		{&t.MerchantName, 142, 50, "EXP-TRAN-MERCHANT-NAME"},
		{&t.MerchantCity, 192, 50, "EXP-TRAN-MERCHANT-CITY"},
		{&t.MerchantZip, 242, 10, "EXP-TRAN-MERCHANT-ZIP"},
		{&t.TranCardNum, 252, 16, "EXP-TRAN-CARD-NUM"},
		{&t.TranOrigTS, 268, 26, "EXP-TRAN-ORIG-TS"},
		{&t.TranProcTS, 294, 26, "EXP-TRAN-PROC-TS"},
	} {
		*af.dst, err = cobolfmt.DecodeEBCDICTrimmed(d[af.off : af.off+af.n])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", af.label, err)
		}
	}
	return &t, nil
}

func decodeExportCardXref(d []byte) (*ExportCardXrefData, error) {
	var x ExportCardXrefData
	var err error

	x.XrefCardNum, err = cobolfmt.DecodeEBCDICTrimmed(d[0:16])
	if err != nil {
		return nil, fmt.Errorf("EXP-XREF-CARD-NUM: %w", err)
	}
	x.XrefCustID, err = cobolfmt.DecodeZonedUint(d[16:25])
	if err != nil {
		return nil, fmt.Errorf("EXP-XREF-CUST-ID: %w", err)
	}
	x.XrefAcctID, err = cobolfmt.DecodeComp(d[25:33], false)
	if err != nil {
		return nil, fmt.Errorf("EXP-XREF-ACCT-ID: %w", err)
	}
	return &x, nil
}

func decodeExportCard(d []byte) (*ExportCardData, error) {
	var c ExportCardData
	var err error

	c.CardNum, err = cobolfmt.DecodeEBCDICTrimmed(d[0:16])
	if err != nil {
		return nil, fmt.Errorf("EXP-CARD-NUM: %w", err)
	}
	c.CardAcctID, err = cobolfmt.DecodeComp(d[16:24], false)
	if err != nil {
		return nil, fmt.Errorf("EXP-CARD-ACCT-ID: %w", err)
	}
	c.CardCVVCode, err = cobolfmt.DecodeComp(d[24:26], false)
	if err != nil {
		return nil, fmt.Errorf("EXP-CARD-CVV-CD: %w", err)
	}
	for _, af := range []struct {
		dst    *string
		off, n int
		label  string
	}{
		{&c.CardEmbossed, 26, 50, "EXP-CARD-EMBOSSED-NAME"},
		{&c.CardExpiryDate, 76, 10, "EXP-CARD-EXPIRAION-DATE"},
		{&c.CardActive, 86, 1, "EXP-CARD-ACTIVE-STATUS"},
	} {
		*af.dst, err = cobolfmt.DecodeEBCDICTrimmed(d[af.off : af.off+af.n])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", af.label, err)
		}
	}
	return &c, nil
}

// Encode serialises an ExportRecord back to its 500-byte representation.
func (r *ExportRecord) Encode() ([]byte, error) {
	out := make([]byte, ExportRecordLen)

	// Header fields.
	ftype, err := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(r.RecType, 1), 1)
	if err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-REC-TYPE: %w", err)
	}
	copy(out[0:1], ftype)

	fts, err := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(r.Timestamp, 26), 26)
	if err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-TIMESTAMP: %w", err)
	}
	copy(out[1:27], fts)

	fseq, err := cobolfmt.EncodeComp(r.SequenceNum, 4)
	if err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-SEQUENCE-NUM: %w", err)
	}
	copy(out[27:31], fseq)

	fbranch, err := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(r.BranchID, 4), 4)
	if err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-BRANCH-ID: %w", err)
	}
	copy(out[31:35], fbranch)

	fregion, err := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(r.RegionCode, 5), 5)
	if err != nil {
		return nil, fmt.Errorf("domain: export: EXPORT-REGION-CODE: %w", err)
	}
	copy(out[35:40], fregion)

	// Data section.
	var dataSec []byte
	switch r.RecType {
	case "C":
		if r.Customer == nil {
			return nil, fmt.Errorf("domain: export: Customer data is nil for type C")
		}
		dataSec, err = r.Customer.encode()
	case "A":
		if r.Account == nil {
			return nil, fmt.Errorf("domain: export: Account data is nil for type A")
		}
		dataSec, err = r.Account.encode()
	case "T":
		if r.Transaction == nil {
			return nil, fmt.Errorf("domain: export: Transaction data is nil for type T")
		}
		dataSec, err = r.Transaction.encode()
	case "X":
		if r.CardXref == nil {
			return nil, fmt.Errorf("domain: export: CardXref data is nil for type X")
		}
		dataSec, err = r.CardXref.encode()
	case "D":
		if r.Card == nil {
			return nil, fmt.Errorf("domain: export: Card data is nil for type D")
		}
		dataSec, err = r.Card.encode()
	default:
		return nil, fmt.Errorf("domain: export: unknown record type %q", r.RecType)
	}
	if err != nil {
		return nil, fmt.Errorf("domain: export: type %q: %w", r.RecType, err)
	}
	copy(out[exportHeaderLen:], dataSec)
	return out, nil
}

func (c *ExportCustomerData) encode() ([]byte, error) {
	out := make([]byte, exportDataLen)

	fid, err := cobolfmt.EncodeComp(c.CustID, 4)
	if err != nil {
		return nil, fmt.Errorf("EXP-CUST-ID: %w", err)
	}
	copy(out[0:4], fid)

	for _, af := range []struct {
		val    string
		off, n int
		label  string
	}{
		{c.FirstName, 4, 25, "EXP-CUST-FIRST-NAME"},
		{c.MiddleName, 29, 25, "EXP-CUST-MIDDLE-NAME"},
		{c.LastName, 54, 25, "EXP-CUST-LAST-NAME"},
		{c.AddrLine1, 79, 50, "EXP-CUST-ADDR-LINE(1)"},
		{c.AddrLine2, 129, 50, "EXP-CUST-ADDR-LINE(2)"},
		{c.AddrLine3, 179, 50, "EXP-CUST-ADDR-LINE(3)"},
		{c.AddrStateCode, 229, 2, "EXP-CUST-ADDR-STATE-CD"},
		{c.AddrCountryCode, 231, 3, "EXP-CUST-ADDR-COUNTRY-CD"},
		{c.AddrZip, 234, 10, "EXP-CUST-ADDR-ZIP"},
		{c.PhoneNum1, 244, 15, "EXP-CUST-PHONE-NUM(1)"},
		{c.PhoneNum2, 259, 15, "EXP-CUST-PHONE-NUM(2)"},
		{c.GovtIssuedID, 283, 20, "EXP-CUST-GOVT-ISSUED-ID"},
		{c.DOB, 303, 10, "EXP-CUST-DOB-YYYY-MM-DD"},
		{c.EFTAccountID, 313, 10, "EXP-CUST-EFT-ACCOUNT-ID"},
		{c.PriCardHolderInd, 323, 1, "EXP-CUST-PRI-CARD-HOLDER-IND"},
	} {
		fb, ferr := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(af.val, af.n), af.n)
		if ferr != nil {
			return nil, fmt.Errorf("%s: %w", af.label, ferr)
		}
		copy(out[af.off:af.off+af.n], fb)
	}

	fssn, err := cobolfmt.EncodeZonedUint(c.SSN, 9)
	if err != nil {
		return nil, fmt.Errorf("EXP-CUST-SSN: %w", err)
	}
	copy(out[274:283], fssn)

	// PIC 9(03) COMP-3 — unsigned, emit 0xF sign nibble.
	ffico, err := cobolfmt.EncodePacked(c.FICOCreditScore, 3, 0, true)
	if err != nil {
		return nil, fmt.Errorf("EXP-CUST-FICO-CREDIT-SCORE: %w", err)
	}
	copy(out[324:326], ffico)

	copy(out[326:460], cobolfmt.EBCDICSpacePad(134))
	return out, nil
}

func (a *ExportAccountData) encode() ([]byte, error) {
	out := make([]byte, exportDataLen)

	fid, err := cobolfmt.EncodeZonedUint(a.AcctID, 11)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-ID: %w", err)
	}
	copy(out[0:11], fid)

	fstatus, err := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(a.AcctActiveStatus, 1), 1)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-ACTIVE-STATUS: %w", err)
	}
	copy(out[11:12], fstatus)

	// PIC S9(10)V99 COMP-3 (12 digits, scale=2, signed).
	fbal, err := cobolfmt.EncodePacked(a.AcctCurrBal, 12, 2, false)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CURR-BAL: %w", err)
	}
	copy(out[12:19], fbal)

	fcredit, err := cobolfmt.EncodeZoned(a.AcctCreditLimit, 12, 2, true)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CREDIT-LIMIT: %w", err)
	}
	copy(out[19:31], fcredit)

	// PIC S9(10)V99 COMP-3 (12 digits, scale=2, signed).
	fcash, err := cobolfmt.EncodePacked(a.AcctCashCredit, 12, 2, false)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CASH-CREDIT-LIMIT: %w", err)
	}
	copy(out[31:38], fcash)

	for _, af := range []struct {
		val    string
		off, n int
		label  string
	}{
		{a.AcctOpenDate, 38, 10, "EXP-ACCT-OPEN-DATE"},
		{a.AcctExpiryDate, 48, 10, "EXP-ACCT-EXPIRAION-DATE"},
		{a.AcctReissueDate, 58, 10, "EXP-ACCT-REISSUE-DATE"},
	} {
		fb, ferr := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(af.val, af.n), af.n)
		if ferr != nil {
			return nil, fmt.Errorf("%s: %w", af.label, ferr)
		}
		copy(out[af.off:af.off+af.n], fb)
	}

	fccredit, err := cobolfmt.EncodeZoned(a.AcctCurrCycCredit, 12, 2, true)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CURR-CYC-CREDIT: %w", err)
	}
	copy(out[68:80], fccredit)

	// PIC S9(10)V99 COMP — stored as binary integer (value × 100).
	rawDebit := a.AcctCurrCycDebit.Shift(2).IntPart()
	fdebit, err := cobolfmt.EncodeComp(rawDebit, 8)
	if err != nil {
		return nil, fmt.Errorf("EXP-ACCT-CURR-CYC-DEBIT: %w", err)
	}
	copy(out[80:88], fdebit)

	for _, af := range []struct {
		val    string
		off, n int
		label  string
	}{
		{a.AcctAddrZip, 88, 10, "EXP-ACCT-ADDR-ZIP"},
		{a.AcctGroupID, 98, 10, "EXP-ACCT-GROUP-ID"},
	} {
		fb, ferr := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(af.val, af.n), af.n)
		if ferr != nil {
			return nil, fmt.Errorf("%s: %w", af.label, ferr)
		}
		copy(out[af.off:af.off+af.n], fb)
	}

	copy(out[108:460], cobolfmt.EBCDICSpacePad(352))
	return out, nil
}

func (t *ExportTransactionData) encode() ([]byte, error) {
	out := make([]byte, exportDataLen)

	for _, af := range []struct {
		val    string
		off, n int
		label  string
	}{
		{t.TranID, 0, 16, "EXP-TRAN-ID"},
		{t.TranTypeCode, 16, 2, "EXP-TRAN-TYPE-CD"},
		{t.TranSource, 22, 10, "EXP-TRAN-SOURCE"},
		{t.TranDesc, 32, 100, "EXP-TRAN-DESC"},
		{t.MerchantName, 142, 50, "EXP-TRAN-MERCHANT-NAME"},
		{t.MerchantCity, 192, 50, "EXP-TRAN-MERCHANT-CITY"},
		{t.MerchantZip, 242, 10, "EXP-TRAN-MERCHANT-ZIP"},
		{t.TranCardNum, 252, 16, "EXP-TRAN-CARD-NUM"},
		{t.TranOrigTS, 268, 26, "EXP-TRAN-ORIG-TS"},
		{t.TranProcTS, 294, 26, "EXP-TRAN-PROC-TS"},
	} {
		fb, ferr := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(af.val, af.n), af.n)
		if ferr != nil {
			return nil, fmt.Errorf("%s: %w", af.label, ferr)
		}
		copy(out[af.off:af.off+af.n], fb)
	}

	fcat, err := cobolfmt.EncodeZonedUint(t.TranCatCode, 4)
	if err != nil {
		return nil, fmt.Errorf("EXP-TRAN-CAT-CD: %w", err)
	}
	copy(out[18:22], fcat)

	// PIC S9(09)V99 COMP-3 (11 digits, scale=2, signed).
	famt, err := cobolfmt.EncodePacked(t.TranAmt, 11, 2, false)
	if err != nil {
		return nil, fmt.Errorf("EXP-TRAN-AMT: %w", err)
	}
	copy(out[132:138], famt)

	fmid, err := cobolfmt.EncodeComp(t.TranMerchantID, 4)
	if err != nil {
		return nil, fmt.Errorf("EXP-TRAN-MERCHANT-ID: %w", err)
	}
	copy(out[138:142], fmid)

	copy(out[320:460], cobolfmt.EBCDICSpacePad(140))
	return out, nil
}

func (x *ExportCardXrefData) encode() ([]byte, error) {
	out := make([]byte, exportDataLen)

	fnum, err := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(x.XrefCardNum, 16), 16)
	if err != nil {
		return nil, fmt.Errorf("EXP-XREF-CARD-NUM: %w", err)
	}
	copy(out[0:16], fnum)

	fcust, err := cobolfmt.EncodeZonedUint(x.XrefCustID, 9)
	if err != nil {
		return nil, fmt.Errorf("EXP-XREF-CUST-ID: %w", err)
	}
	copy(out[16:25], fcust)

	facct, err := cobolfmt.EncodeComp(x.XrefAcctID, 8)
	if err != nil {
		return nil, fmt.Errorf("EXP-XREF-ACCT-ID: %w", err)
	}
	copy(out[25:33], facct)

	copy(out[33:460], cobolfmt.EBCDICSpacePad(427))
	return out, nil
}

func (c *ExportCardData) encode() ([]byte, error) {
	out := make([]byte, exportDataLen)

	fnum, err := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(c.CardNum, 16), 16)
	if err != nil {
		return nil, fmt.Errorf("EXP-CARD-NUM: %w", err)
	}
	copy(out[0:16], fnum)

	facct, err := cobolfmt.EncodeComp(c.CardAcctID, 8)
	if err != nil {
		return nil, fmt.Errorf("EXP-CARD-ACCT-ID: %w", err)
	}
	copy(out[16:24], facct)

	fcvv, err := cobolfmt.EncodeComp(c.CardCVVCode, 2)
	if err != nil {
		return nil, fmt.Errorf("EXP-CARD-CVV-CD: %w", err)
	}
	copy(out[24:26], fcvv)

	for _, af := range []struct {
		val    string
		off, n int
		label  string
	}{
		{c.CardEmbossed, 26, 50, "EXP-CARD-EMBOSSED-NAME"},
		{c.CardExpiryDate, 76, 10, "EXP-CARD-EXPIRAION-DATE"},
		{c.CardActive, 86, 1, "EXP-CARD-ACTIVE-STATUS"},
	} {
		fb, ferr := cobolfmt.EncodeEBCDIC(cobolfmt.PadRight(af.val, af.n), af.n)
		if ferr != nil {
			return nil, fmt.Errorf("%s: %w", af.label, ferr)
		}
		copy(out[af.off:af.off+af.n], fb)
	}

	copy(out[87:460], cobolfmt.EBCDICSpacePad(373))
	return out, nil
}

// DecodeExportRecords reads all ExportRecords from a flat 500-byte-per-record file.
func DecodeExportRecords(data []byte) ([]*ExportRecord, error) {
	if len(data)%ExportRecordLen != 0 {
		return nil, fmt.Errorf("domain: export: data length %d is not a multiple of %d", len(data), ExportRecordLen)
	}
	count := len(data) / ExportRecordLen
	records := make([]*ExportRecord, 0, count)
	for i := 0; i < count; i++ {
		rec, err := DecodeExportRecord(data[i*ExportRecordLen : (i+1)*ExportRecordLen])
		if err != nil {
			return nil, fmt.Errorf("domain: export: record %d: %w", i, err)
		}
		records = append(records, rec)
	}
	return records, nil
}
