// Package tranpost replaces CBTRN02C / POSTTRAN.jcl (RAU-44).
//
// It reads a sequential file of 350-byte DailyTransactionRecords (same binary
// layout as TransactionRecord), validates each against the card-xref and
// account tables, and either posts the transaction (insert + update account +
// upsert tran-cat-bal, atomically inside a DB transaction) or writes a
// 430-byte reject record (350-byte tran data + 80-byte trailer).
//
// Validation reason codes mirror the COBOL:
//
//	100  invalid card number (not in card_xrefs)
//	101  account not found
//	102  overlimit transaction
//	103  transaction after account expiration
package tranpost

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

// RejectReasonLen is the fixed width of the validation-trailer appended to rejected records.
const RejectReasonLen = 80

// Config holds all I/O and dependencies for the tran-post subcommand.
type Config struct {
	DB      *sql.DB
	Input   io.Reader // 350-byte EBCDIC DailyTransactionRecord stream (or ASCII flat file)
	Rejects io.Writer // receives 430-byte reject records; nil = discard
	Out     io.Writer // progress messages; nil = discard
	Now     time.Time // injected for tests; zero means time.Now()
}

// Summary reports how many records were posted vs rejected.
type Summary struct {
	Posted   int
	Rejected int
}

// Run reads daily transactions from cfg.Input, validates, and posts valid ones
// to the SQLite DB atomically. Returns with non-zero Rejected if any rejects
// occurred (mirrors COBOL RETURN-CODE=4).
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Rejects == nil {
		cfg.Rejects = io.Discard
	}
	if cfg.DB == nil {
		return Summary{}, fmt.Errorf("tranpost: DB is required")
	}
	if cfg.Input == nil {
		return Summary{}, fmt.Errorf("tranpost: Input is required")
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var s Summary
	buf := make([]byte, domain.DailyTransactionRecordLen)

	for {
		_, err := io.ReadFull(cfg.Input, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return s, fmt.Errorf("tranpost: read input: %w", err)
		}

		dtr, err := domain.DecodeDailyTransactionRecord(buf)
		if err != nil {
			return s, fmt.Errorf("tranpost: decode record: %w", err)
		}

		reason, desc := validate(ctx, cfg.DB, dtr)
		if reason != 0 {
			s.Rejected++
			writeReject(cfg.Rejects, buf, reason, desc)
			fmt.Fprintf(cfg.Out, "REJECT tran=%s reason=%04d %s\n",
				dtr.DalytranID, reason, desc)
			continue
		}

		if err := post(ctx, cfg.DB, dtr, now); err != nil {
			return s, fmt.Errorf("tranpost: post %s: %w", dtr.DalytranID, err)
		}
		s.Posted++
		fmt.Fprintf(cfg.Out, "POSTED tran=%s card=%s amt=%s\n",
			dtr.DalytranID, dtr.DalytranCardNum, dtr.DalytranAmt.StringFixed(2))
	}

	fmt.Fprintf(cfg.Out, "\nPosted: %d, Rejected: %d\n", s.Posted, s.Rejected)
	return s, nil
}

// validate checks a daily transaction against the DB, returning a non-zero
// reason code and description if validation fails (mirrors 1500-VALIDATE-TRAN).
func validate(ctx context.Context, db *sql.DB, dtr *domain.DailyTransactionRecord) (int, string) {
	xrefStore := sqlite.NewCardXrefStore(db)
	xref, err := xrefStore.Get(ctx, dtr.DalytranCardNum)
	if err != nil {
		return 100, "INVALID CARD NUMBER FOUND"
	}

	acctStore := sqlite.NewAccountStore(db)
	acct, err := acctStore.Get(ctx, xref.XrefAcctID)
	if err != nil {
		return 101, "ACCOUNT RECORD NOT FOUND"
	}

	// Both checks run independently; 103 overwrites 102 when both apply (CBTRN02C.cbl:407-420).
	reason := 0
	desc := ""

	tempBal := acct.AcctCurrCycCredit.Sub(acct.AcctCurrCycDebit).Add(dtr.DalytranAmt)
	if acct.AcctCreditLimit.LessThan(tempBal) {
		reason = 102
		desc = "OVERLIMIT TRANSACTION"
	}

	origDate := ""
	if len(dtr.DalytranOrigTS) >= 10 {
		origDate = dtr.DalytranOrigTS[:10]
	}
	if acct.AcctExpirationDate < origDate {
		reason = 103
		desc = "TRANSACTION RECEIVED AFTER ACCT EXPIRATION"
	}

	return reason, desc
}

// post writes the transaction, updates the account, and upserts tran-cat-bal
// inside a single SQLite transaction for atomicity.
func post(ctx context.Context, db *sql.DB, dtr *domain.DailyTransactionRecord, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	xrefStore := sqlite.NewCardXrefStore(tx)
	xref, err := xrefStore.Get(ctx, dtr.DalytranCardNum)
	if err != nil {
		return fmt.Errorf("lookup xref: %w", err)
	}

	acctStore := sqlite.NewAccountStore(tx)
	acct, err := acctStore.Get(ctx, xref.XrefAcctID)
	if err != nil {
		return fmt.Errorf("lookup account: %w", err)
	}

	// Check for duplicate before any balance writes so re-running the file is idempotent.
	tranStore := sqlite.NewTransactionStore(tx)
	if _, err := tranStore.Get(ctx, dtr.DalytranID); err == nil {
		return tx.Rollback()
	}

	ts := now.Format("2006-01-02 15:04:05.000000")

	tran := &domain.TransactionRecord{
		TranID:           dtr.DalytranID,
		TranTypeCode:     dtr.DalytranTypeCode,
		TranCatCode:      dtr.DalytranCatCode,
		TranSource:       dtr.DalytranSource,
		TranDesc:         dtr.DalytranDesc,
		TranAmt:          dtr.DalytranAmt,
		TranMerchantID:   dtr.DalytranMerchantID,
		TranMerchantName: dtr.DalytranMerchantName,
		TranMerchantCity: dtr.DalytranMerchantCity,
		TranMerchantZip:  dtr.DalytranMerchantZip,
		TranCardNum:      dtr.DalytranCardNum,
		TranOrigTS:       dtr.DalytranOrigTS,
		TranProcTS:       ts,
	}

	// Update tran-cat-bal.
	tcatbalStore := sqlite.NewTranCatBalStore(tx)
	tcatbalRec := &domain.TranCatBalRecord{
		TrancatAcctID:  xref.XrefAcctID,
		TrancatTypeCD:  dtr.DalytranTypeCode,
		TrancatCode:    dtr.DalytranCatCode,
		TranCatBalance: dtr.DalytranAmt,
	}
	existing, err := tcatbalStore.Get(ctx, xref.XrefAcctID, dtr.DalytranTypeCode, dtr.DalytranCatCode)
	if err != nil {
		if err != repo.ErrNotFound {
			return fmt.Errorf("get tcatbal: %w", err)
		}
		// Create new record.
		if err := tcatbalStore.Create(ctx, tcatbalRec); err != nil {
			return fmt.Errorf("create tcatbal: %w", err)
		}
	} else {
		existing.TranCatBalance = existing.TranCatBalance.Add(dtr.DalytranAmt)
		if err := tcatbalStore.Update(ctx, existing); err != nil {
			return fmt.Errorf("update tcatbal: %w", err)
		}
	}

	// Update account balances.
	acct.AcctCurrBal = acct.AcctCurrBal.Add(dtr.DalytranAmt)
	if dtr.DalytranAmt.GreaterThanOrEqual(decimal.Zero) {
		acct.AcctCurrCycCredit = acct.AcctCurrCycCredit.Add(dtr.DalytranAmt)
	} else {
		acct.AcctCurrCycDebit = acct.AcctCurrCycDebit.Add(dtr.DalytranAmt)
	}
	if err := acctStore.Update(ctx, acct); err != nil {
		return fmt.Errorf("update account: %w", err)
	}

	// Write transaction record (dup already excluded by pre-check above).
	if err := tranStore.Create(ctx, tran); err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	return tx.Commit()
}

// writeReject writes a 430-byte reject record: 350-byte tran + 80-byte trailer.
// The trailer is: 4-byte ASCII reason code + 76-byte description (space-padded).
func writeReject(w io.Writer, tranData []byte, reason int, desc string) {
	trailer := make([]byte, RejectReasonLen)
	// Reason code as 4-byte big-endian ASCII digits.
	reasonStr := fmt.Sprintf("%04d", reason)
	copy(trailer[0:4], []byte(reasonStr))
	// Description: up to 76 bytes, space-padded.
	descBytes := []byte(desc)
	if len(descBytes) > 76 {
		descBytes = descBytes[:76]
	}
	copy(trailer[4:80], descBytes)
	for i := 4 + len(descBytes); i < 80; i++ {
		trailer[i] = 0x20
	}

	raw := make([]byte, domain.DailyTransactionRecordLen)
	copy(raw, tranData)
	_, _ = w.Write(raw)
	_, _ = w.Write(trailer)
}

