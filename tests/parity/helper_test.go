// Package parity_test verifies that the Go port produces output that matches
// the legacy COBOL programs when run against the canonical EBCDIC sample data.
//
// Golden files live under tests/parity/golden/. Re-generate them with:
//
//	go test ./tests/parity/... -args -update
package parity_test

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

var update = flag.Bool("update", false, "regenerate golden files")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// dataDir returns the path to the EBCDIC fixture files relative to this
// package (tests/parity/ → ../../app/data/EBCDIC).
func dataDir() string {
	return filepath.Join("..", "..", "app", "data", "EBCDIC")
}

// goldenPath returns the path of a golden file by name.
func goldenPath(name string) string {
	return filepath.Join("golden", name)
}

// compareGolden diffs got against the named golden file.
// When -update is set it writes got to the golden file instead of comparing.
func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("compareGolden: mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("compareGolden: write %s: %v", path, err)
		}
		t.Logf("golden updated: %s (%d bytes)", path, len(got))
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("compareGolden: read %s: %v (run with -update to generate)", path, err)
	}
	if string(want) != got {
		// Show a brief diff (first 10 lines that differ).
		wantLines := splitLines(string(want))
		gotLines := splitLines(got)
		var diff bytes.Buffer
		maxLine := len(wantLines)
		if len(gotLines) > maxLine {
			maxLine = len(gotLines)
		}
		shown := 0
		for i := 0; i < maxLine && shown < 10; i++ {
			wl, gl := "", ""
			if i < len(wantLines) {
				wl = wantLines[i]
			}
			if i < len(gotLines) {
				gl = gotLines[i]
			}
			if wl != gl {
				fmt.Fprintf(&diff, "line %d:\n  want: %q\n  got:  %q\n", i+1, wl, gl)
				shown++
			}
		}
		if len(gotLines) != len(wantLines) {
			fmt.Fprintf(&diff, "line count: want %d, got %d\n", len(wantLines), len(gotLines))
		}
		t.Errorf("golden mismatch for %s:\n%s(run with -update to regenerate)", name, diff.String())
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// seedTestDB opens a fresh in-memory SQLite DB and loads all EBCDIC fixtures
// into it, mirroring the logic in cmd/seed.
func seedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("seedTestDB: open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	dir := dataDir()

	if err := loadAccounts(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.ACCTDATA.PS")); err != nil {
		t.Fatalf("seedTestDB accounts: %v", err)
	}
	if err := loadCustomers(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.CUSTDATA.PS")); err != nil {
		t.Fatalf("seedTestDB customers: %v", err)
	}
	if err := loadCards(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.CARDDATA.PS")); err != nil {
		t.Fatalf("seedTestDB cards: %v", err)
	}
	if err := loadCardXrefs(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.CARDXREF.PS")); err != nil {
		t.Fatalf("seedTestDB cardxrefs: %v", err)
	}
	if err := loadTransactions(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.DALYTRAN.PS")); err != nil {
		t.Fatalf("seedTestDB transactions: %v", err)
	}
	if err := loadTranTypes(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.TRANTYPE.PS")); err != nil {
		t.Fatalf("seedTestDB tran_types: %v", err)
	}
	if err := loadTranCats(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.TRANCATG.PS")); err != nil {
		t.Fatalf("seedTestDB tran_cats: %v", err)
	}
	if err := loadDiscGroups(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.DISCGRP.PS")); err != nil {
		t.Fatalf("seedTestDB disc_groups: %v", err)
	}
	if err := loadTranCatBals(ctx, db, filepath.Join(dir, "AWS.M2.CARDDEMO.TCATBALF.PS")); err != nil {
		t.Fatalf("seedTestDB tran_cat_bals: %v", err)
	}
	return db
}

func loadAccounts(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeAccountRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewAccountStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func loadCustomers(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeCustomerRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewCustomerStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func loadCards(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeCardRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewCardStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func loadCardXrefs(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeCardXrefRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewCardXrefStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func loadTransactions(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data)%domain.DailyTransactionRecordLen != 0 {
		return fmt.Errorf("data length %d is not a multiple of %d",
			len(data), domain.DailyTransactionRecordLen)
	}
	count := len(data) / domain.DailyTransactionRecordLen
	s := sqlite.NewTransactionStore(db)
	for i := 0; i < count; i++ {
		d, err := domain.DecodeDailyTransactionRecord(
			data[i*domain.DailyTransactionRecordLen : (i+1)*domain.DailyTransactionRecordLen])
		if err != nil {
			return err
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
			return err
		}
	}
	return nil
}

func loadTranTypes(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeTranTypeRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewTranTypeStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func loadTranCats(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeTranCatRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewTranCatStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func loadDiscGroups(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeDiscGroupRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewDiscGroupStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func loadTranCatBals(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recs, err := domain.DecodeTranCatBalRecords(data)
	if err != nil {
		return err
	}
	s := sqlite.NewTranCatBalStore(db)
	for _, r := range recs {
		if err := s.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}
