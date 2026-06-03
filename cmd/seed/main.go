// Command seed loads the EBCDIC fixture files from app/data/EBCDIC into a
// SQLite database via the repo interfaces, then prints per-table row counts.
//
// By default (wipe=true) all tables are cleared and reloaded atomically inside
// a single transaction; the entire load succeeds or the DB is left unchanged.
// Pass -wipe=false to insert without clearing — this fails on duplicate keys.
//
// User passwords: the USRSEC fixture carries legacy plaintext passwords.
// The seed stores them verbatim in the pwd_hash column. This is intentional
// for development/fixture purposes only; production user provisioning goes
// through internal/auth, which uses bcrypt. A warning is printed at startup.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

func main() {
	dbPath := flag.String("db", "carddemo.sqlite", "SQLite database file to create or update")
	dataDir := flag.String("data", filepath.Join("app", "data", "EBCDIC"), "directory holding the EBCDIC fixture files")
	wipe := flag.Bool("wipe", true, "delete all rows before inserting (atomic wipe-and-reload; safe to re-run)")
	flag.Parse()

	if err := run(*dbPath, *dataDir, *wipe); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

// wipeTables is the ordered list of tables to clear when -wipe=true.
// Order is leaf-first to respect any future FK constraints.
var wipeTables = []string{
	"tran_cat_bals", "disc_groups", "tran_cats", "tran_types",
	"transactions", "card_xrefs", "cards", "customers", "accounts", "users",
}

func run(dbPath, dataDir string, wipe bool) error {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Fprintln(os.Stderr, "WARNING: USRSEC passwords are stored verbatim (plaintext) — dev fixture only, never use in production")

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if wipe {
		for _, table := range wipeTables {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return fmt.Errorf("wipe %s: %w", table, err)
			}
		}
	}

	files := map[string]string{
		"accounts":      "AWS.M2.CARDDEMO.ACCTDATA.PS",
		"customers":     "AWS.M2.CARDDEMO.CUSTDATA.PS",
		"cards":         "AWS.M2.CARDDEMO.CARDDATA.PS",
		"card_xrefs":    "AWS.M2.CARDDEMO.CARDXREF.PS",
		"transactions":  "AWS.M2.CARDDEMO.DALYTRAN.PS",
		"tran_types":    "AWS.M2.CARDDEMO.TRANTYPE.PS",
		"tran_cats":     "AWS.M2.CARDDEMO.TRANCATG.PS",
		"disc_groups":   "AWS.M2.CARDDEMO.DISCGRP.PS",
		"tran_cat_bals": "AWS.M2.CARDDEMO.TCATBALF.PS",
		"users":         "AWS.M2.CARDDEMO.USRSEC.PS",
	}

	type loader struct {
		table string
		fn    func(context.Context, string) (int, error)
	}
	loaders := []loader{
		{"accounts", func(c context.Context, p string) (int, error) {
			return seedAccounts(c, sqlite.NewAccountStore(tx), p)
		}},
		{"customers", func(c context.Context, p string) (int, error) {
			return seedCustomers(c, sqlite.NewCustomerStore(tx), p)
		}},
		{"cards", func(c context.Context, p string) (int, error) {
			return seedCards(c, sqlite.NewCardStore(tx), p)
		}},
		{"card_xrefs", func(c context.Context, p string) (int, error) {
			return seedCardXrefs(c, sqlite.NewCardXrefStore(tx), p)
		}},
		{"transactions", func(c context.Context, p string) (int, error) {
			return seedTransactions(c, sqlite.NewTransactionStore(tx), p)
		}},
		{"tran_types", func(c context.Context, p string) (int, error) {
			return seedTranTypes(c, sqlite.NewTranTypeStore(tx), p)
		}},
		{"tran_cats", func(c context.Context, p string) (int, error) {
			return seedTranCats(c, sqlite.NewTranCatStore(tx), p)
		}},
		{"disc_groups", func(c context.Context, p string) (int, error) {
			return seedDiscGroups(c, sqlite.NewDiscGroupStore(tx), p)
		}},
		{"tran_cat_bals", func(c context.Context, p string) (int, error) {
			return seedTranCatBals(c, sqlite.NewTranCatBalStore(tx), p)
		}},
		{"users", func(c context.Context, p string) (int, error) {
			return seedUsers(c, sqlite.NewUserSecStore(tx), p)
		}},
	}

	counts := make(map[string]int, len(loaders))
	for _, l := range loaders {
		n, err := l.fn(ctx, filepath.Join(dataDir, files[l.table]))
		if err != nil {
			return fmt.Errorf("%s: %w", l.table, err)
		}
		counts[l.table] = n
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	for _, l := range loaders {
		fmt.Printf("%-14s %d\n", l.table, counts[l.table])
	}
	return nil
}

func readFixture(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func seedAccounts(ctx context.Context, s *sqlite.AccountStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeAccountRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

func seedCustomers(ctx context.Context, s *sqlite.CustomerStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeCustomerRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

func seedCards(ctx context.Context, s *sqlite.CardStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeCardRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

func seedCardXrefs(ctx context.Context, s *sqlite.CardXrefStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeCardXrefRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

// seedTransactions loads the daily-transaction fixture (DALYTRAN-RECORD) into
// the transactions table. The daily and posted layouts are identical apart from
// the field-name prefix, so each DailyTransactionRecord maps field-for-field.
func seedTransactions(ctx context.Context, s *sqlite.TransactionStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	if len(data)%domain.DailyTransactionRecordLen != 0 {
		return 0, fmt.Errorf("data length %d is not a multiple of %d", len(data), domain.DailyTransactionRecordLen)
	}
	count := len(data) / domain.DailyTransactionRecordLen
	for i := 0; i < count; i++ {
		d, err := domain.DecodeDailyTransactionRecord(data[i*domain.DailyTransactionRecordLen : (i+1)*domain.DailyTransactionRecordLen])
		if err != nil {
			return 0, err
		}
		r := &domain.TransactionRecord{
			TranID:           d.DalytranID,
			TranTypeCode:     d.DalytranTypeCode,
			TranCatCode:      d.DalytranCatCode,
			TranSource:       d.DalytranSource,
			TranDesc:         d.DalytranDesc,
			TranAmt:          d.DalytranAmt,
			TranMerchantID:   d.DalytranMerchantID,
			TranMerchantName: d.DalytranMerchantName,
			TranMerchantCity: d.DalytranMerchantCity,
			TranMerchantZip:  d.DalytranMerchantZip,
			TranCardNum:      d.DalytranCardNum,
			TranOrigTS:       d.DalytranOrigTS,
			TranProcTS:       d.DalytranProcTS,
		}
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func seedTranTypes(ctx context.Context, s *sqlite.TranTypeStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeTranTypeRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

func seedTranCats(ctx context.Context, s *sqlite.TranCatStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeTranCatRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

func seedDiscGroups(ctx context.Context, s *sqlite.DiscGroupStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeDiscGroupRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

func seedTranCatBals(ctx context.Context, s *sqlite.TranCatBalStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeTranCatBalRecords(data)
	if err != nil {
		return 0, err
	}
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

// seedUsers loads the USRSEC fixture. The on-disk record carries the legacy
// plaintext password; the seed stores it verbatim in PwdHash so the fixture is
// loadable without re-hashing. A warning is printed at startup.
// Production user provisioning runs through internal/auth (bcrypt).
func seedUsers(ctx context.Context, s *sqlite.UserSecStore, path string) (int, error) {
	data, err := readFixture(path)
	if err != nil {
		return 0, err
	}
	recs, err := domain.DecodeUserSecRecords(data)
	if err != nil {
		return 0, err
	}
	for _, rec := range recs {
		u := domain.UserSec{
			UserID:    rec.UserID,
			FirstName: rec.FirstName,
			LastName:  rec.LastName,
			PwdHash:   rec.Password,
		}
		if rec.UserType != "" {
			u.Type = domain.UserType(rec.UserType[0])
		}
		if err := s.Create(ctx, u); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}
