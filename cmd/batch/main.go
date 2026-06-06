// Command batch is the CardDemo batch CLI (RAU-44).
//
// Each subcommand replaces one JCL step (or orchestrated JCL job) from the
// legacy CardDemo system. All subcommands accept -db to specify the SQLite
// database path. Run 'carddemo-batch <subcommand> -help' for per-subcommand flags.
//
// Subcommands:
//
//	acct-dump      CBACT01C / READACCT.jcl  — print all accounts
//	card-dump      CBACT02C / READCARD.jcl  — print all cards
//	cust-dump      CBCUS01C / READCUST.jcl  — print all customers
//	xref-dump      CBACT03C / READXREF.jcl  — print all card-xrefs
//	intcalc        CBACT04C / INTCALC.jcl   — compute monthly interest
//	tran-validate  CBTRN01C / POSTTRAN.jcl  — validate and post daily transactions
//	tran-post      CBTRN02C / POSTTRAN.jcl  — post daily transactions with reject file
//	tran-report    CBTRN03C / TRANREPT.jcl  — produce transaction report
//	export         CBEXPORT / CBEXPORT.jcl  — export all entities to flat file
//	import         CBIMPORT / CBIMPORT.jcl  — import entities from export flat file
//	wait           COBSWAIT / WAITSTEP.jcl  — sleep for N centiseconds
//	combtran       COMBTRAN.jcl             — combine two transaction sources
//	tranbkp        TRANBKP.jcl              — backup and optionally reset transactions
//	run-job        orchestrator             — run a named job from batch/jobs.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/acctdump"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/batchimport"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/carddump"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/combtran"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/custdump"
	batchexport "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/export"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/intcalc"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/tranbkp"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/tranpost"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/tranreport"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/tranvalidate"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/wait"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/xrefdump"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	if err := dispatch(context.Background(), os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "batch: %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, subcmd string, args []string) error {
	switch subcmd {
	case "acct-dump":
		return cmdAcctDump(ctx, args)
	case "card-dump":
		return cmdCardDump(ctx, args)
	case "cust-dump":
		return cmdCustDump(ctx, args)
	case "xref-dump":
		return cmdXrefDump(ctx, args)
	case "intcalc":
		return cmdIntCalc(ctx, args)
	case "tran-validate":
		return cmdTranValidate(ctx, args)
	case "tran-post":
		return cmdTranPost(ctx, args)
	case "tran-report":
		return cmdTranReport(ctx, args)
	case "export":
		return cmdExport(ctx, args)
	case "import":
		return cmdImport(ctx, args)
	case "wait":
		return cmdWait(ctx, args)
	case "combtran":
		return cmdCombTran(ctx, args)
	case "tranbkp":
		return cmdTranBkp(ctx, args)
	case "run-job":
		return cmdRunJob(ctx, args)
	case "-help", "--help", "-h", "help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q; run with no args for help", subcmd)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: carddemo-batch <subcommand> [flags]

Subcommands:
  acct-dump      Print all accounts           (CBACT01C / READACCT.jcl)
  card-dump      Print all cards              (CBACT02C / READCARD.jcl)
  cust-dump      Print all customers          (CBCUS01C / READCUST.jcl)
  xref-dump      Print all card-xrefs         (CBACT03C / READXREF.jcl)
  intcalc        Compute monthly interest     (CBACT04C / INTCALC.jcl)
  tran-validate  Validate + post daily trans  (CBTRN01C / POSTTRAN.jcl)
  tran-post      Post daily transactions      (CBTRN02C / POSTTRAN.jcl)
  tran-report    Transaction report           (CBTRN03C / TRANREPT.jcl)
  export         Export all to flat file      (CBEXPORT / CBEXPORT.jcl)
  import         Import from flat file        (CBIMPORT / CBIMPORT.jcl)
  wait           Sleep N centiseconds         (COBSWAIT / WAITSTEP.jcl)
  combtran       Combine transactions         (COMBTRAN.jcl)
  tranbkp        Backup + reset transactions  (TRANBKP.jcl)
  run-job        Run a named job from batch/jobs.yaml

Run 'carddemo-batch <subcommand> -help' for per-subcommand flags.
`)
}

func cmdAcctDump(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("acct-dump", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	s, err := acctdump.Run(ctx, acctdump.Config{
		Accounts: sqlite.NewAccountStore(db),
		Out:      os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "acct-dump: %d accounts\n", s.Count)
	return nil
}

func cmdCardDump(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("card-dump", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	s, err := carddump.Run(ctx, carddump.Config{
		Cards: sqlite.NewCardStore(db),
		Out:   os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "card-dump: %d cards\n", s.Count)
	return nil
}

func cmdCustDump(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("cust-dump", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	s, err := custdump.Run(ctx, custdump.Config{
		Customers: sqlite.NewCustomerStore(db),
		Out:       os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "cust-dump: %d customers\n", s.Count)
	return nil
}

func cmdXrefDump(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("xref-dump", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	s, err := xrefdump.Run(ctx, xrefdump.Config{
		CardXrefs: sqlite.NewCardXrefStore(db),
		Out:       os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "xref-dump: %d xref records\n", s.Count)
	return nil
}

func cmdIntCalc(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("intcalc", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	parmDate := fs.String("parm-date", "", "YYYYMMDDNN parameter date (default: today)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	s, err := intcalc.Run(ctx, intcalc.Config{
		DB:       db,
		ParmDate: *parmDate,
		Out:      os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "intcalc: accounts=%d transactions=%d\n",
		s.AccountsProcessed, s.TransactionsWritten)
	return nil
}

func cmdTranValidate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tran-validate", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	inputPath := fs.String("input", "-", "daily transaction flat file; '-' for stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	in, cleanIn, err := openInput(*inputPath)
	if err != nil {
		return err
	}
	defer cleanIn()

	s, err := tranvalidate.Run(ctx, tranvalidate.Config{
		Transactions: sqlite.NewTransactionStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
		Accounts:     sqlite.NewAccountStore(db),
		Customers:    sqlite.NewCustomerStore(db),
		Cards:        sqlite.NewCardStore(db),
		Input:        in,
		Out:          os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "tran-validate: validated=%d skipped=%d\n", s.Validated, s.Skipped)
	return nil
}

func cmdTranPost(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tran-post", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	inputPath := fs.String("input", "-", "daily transaction flat file; '-' for stdin")
	rejectsPath := fs.String("rejects", "", "output path for 430-byte reject records")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	in, cleanIn, err := openInput(*inputPath)
	if err != nil {
		return err
	}
	defer cleanIn()

	rejects, cleanRej, err := openOutput(*rejectsPath)
	if err != nil {
		return err
	}
	defer cleanRej()

	s, err := tranpost.Run(ctx, tranpost.Config{
		DB:      db,
		Input:   in,
		Rejects: rejects,
		Out:     os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "tran-post: posted=%d rejected=%d\n", s.Posted, s.Rejected)
	if s.Rejected > 0 {
		// Mirror COBOL RETURN-CODE=4 on any rejects.
		os.Exit(4)
	}
	return nil
}

func cmdTranReport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tran-report", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	dateFrom := fs.String("from", "", "start date filter YYYY-MM-DD")
	dateTo := fs.String("to", "", "end date filter YYYY-MM-DD")
	outputPath := fs.String("output", "-", "report output file; '-' for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	out, cleanOut, err := openOutput(*outputPath)
	if err != nil {
		return err
	}
	defer cleanOut()

	s, err := tranreport.Run(ctx, tranreport.Config{
		Transactions: sqlite.NewTransactionStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
		TranTypes:    sqlite.NewTranTypeStore(db),
		TranCats:     sqlite.NewTranCatStore(db),
		DateFrom:     *dateFrom,
		DateTo:       *dateTo,
		Out:          out,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "tran-report: pages=%d transactions=%d\n", s.Pages, s.Transactions)
	return nil
}

func cmdExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	outputPath := fs.String("output", "carddemo-export.dat", "output file path; '-' for stdout")
	branchID := fs.String("branch-id", "    ", "4-char branch ID")
	regionCode := fs.String("region-code", "     ", "5-char region code")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	out, cleanOut, err := openOutput(*outputPath)
	if err != nil {
		return err
	}
	defer cleanOut()

	s, err := batchexport.Run(ctx, batchexport.Config{
		Customers:    sqlite.NewCustomerStore(db),
		Accounts:     sqlite.NewAccountStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
		Transactions: sqlite.NewTransactionStore(db),
		Cards:        sqlite.NewCardStore(db),
		BranchID:     *branchID,
		RegionCode:   *regionCode,
		Out:          out,
		Log:          os.Stderr,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "export: total=%d\n", s.Total())
	return nil
}

func cmdImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	inputPath := fs.String("input", "-", "export flat file; '-' for stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	in, cleanIn, err := openInput(*inputPath)
	if err != nil {
		return err
	}
	defer cleanIn()

	s, err := batchimport.Run(ctx, batchimport.Config{
		Input:        in,
		Customers:    sqlite.NewCustomerStore(db),
		Accounts:     sqlite.NewAccountStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
		Transactions: sqlite.NewTransactionStore(db),
		Cards:        sqlite.NewCardStore(db),
		Log:          os.Stderr,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "import: total=%d errors=%d\n", s.Total(), s.Errors)
	return nil
}

func cmdWait(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("wait", flag.ContinueOnError)
	cs := fs.Int("centiseconds", 0, "duration in centiseconds (1/100 second)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Accept positional arg as centiseconds (SYSIN-style).
	if *cs == 0 && fs.NArg() > 0 {
		if _, err := fmt.Sscanf(fs.Arg(0), "%d", cs); err != nil {
			return fmt.Errorf("wait: invalid centiseconds %q", fs.Arg(0))
		}
	}
	s, err := wait.Run(ctx, wait.Config{Centiseconds: *cs})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wait: slept %v\n", s.Waited)
	return nil
}

func cmdCombTran(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("combtran", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	secondaryPath := fs.String("secondary", "", "secondary transaction flat file (350-byte records)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var secondary io.Reader
	if *secondaryPath != "" && *secondaryPath != "-" {
		f, err := os.Open(*secondaryPath)
		if err != nil {
			return fmt.Errorf("combtran: open secondary %q: %w", *secondaryPath, err)
		}
		defer f.Close()
		secondary = f
	} else if *secondaryPath == "-" {
		secondary = os.Stdin
	}

	tranStore := sqlite.NewTransactionStore(db)
	s, err := combtran.Run(ctx, combtran.Config{
		Primary:   tranStore,
		Secondary: secondary,
		Target:    tranStore,
		Out:       os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "combtran: primary=%d secondary=%d written=%d duplicates=%d\n",
		s.FromPrimary, s.FromSecondary, s.Written, s.Duplicates)
	return nil
}

func cmdTranBkp(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tranbkp", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	outputPath := fs.String("output", "transact-bkup.dat", "backup flat file path")
	reset := fs.Bool("reset", false, "delete all transactions after backup")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	f, err := os.Create(*outputPath)
	if err != nil {
		return fmt.Errorf("tranbkp: create backup file: %w", err)
	}
	defer f.Close()

	s, err := tranbkp.Run(ctx, tranbkp.Config{
		Transactions: sqlite.NewTransactionStore(db),
		Backup:       f,
		Reset:        *reset,
		Log:          os.Stderr,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "tranbkp: backed-up=%d deleted=%d\n", s.BackedUp, s.Deleted)
	return nil
}

func cmdRunJob(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run-job", flag.ContinueOnError)
	dbPath := fs.String("db", "carddemo.sqlite", "SQLite database path")
	jobsYAML := fs.String("jobs", "batch/jobs.yaml", "path to jobs.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("run-job: job name required; see %s for available jobs", *jobsYAML)
	}
	return runOrchestrator(ctx, *dbPath, *jobsYAML, fs.Arg(0))
}

func runOrchestrator(ctx context.Context, dbPath, jobsYAML, jobName string) error {
	cfg, err := loadJobsYAML(jobsYAML)
	if err != nil {
		return fmt.Errorf("run-job: load %q: %w", jobsYAML, err)
	}
	job, ok := cfg.Jobs[jobName]
	if !ok {
		var names []string
		for k := range cfg.Jobs {
			names = append(names, k)
		}
		return fmt.Errorf("run-job: job %q not found; available: %v", jobName, names)
	}

	fmt.Fprintf(os.Stderr, "run-job: executing %q (%d steps)\n", jobName, len(job.Steps))
	for i, step := range job.Steps {
		if step.Skip {
			fmt.Fprintf(os.Stderr, "  step %d: %s — SKIPPED (%s)\n", i+1, step.Name, step.SkipReason)
			continue
		}
		fmt.Fprintf(os.Stderr, "  step %d: %s (%s)\n", i+1, step.Name, step.Command)
		stepArgs := append([]string{"-db", dbPath}, step.Args...)
		if err := dispatch(ctx, step.Command, stepArgs); err != nil {
			return fmt.Errorf("run-job: step %q failed: %w", step.Name, err)
		}
	}
	fmt.Fprintf(os.Stderr, "run-job: %q completed\n", jobName)
	return nil
}

// jobsConfig is the top-level structure of batch/jobs.yaml.
type jobsConfig struct {
	Jobs map[string]jobDef
}

type jobDef struct {
	Description string
	Steps       []stepDef
}

type stepDef struct {
	Name       string
	Command    string
	Args       []string
	Skip       bool
	SkipReason string
}

// loadJobsYAML parses batch/jobs.yaml with a minimal line-oriented parser.
// This avoids adding a yaml library dependency for a simple config file.
func loadJobsYAML(path string) (*jobsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseJobsYAML(string(data)), nil
}

func parseJobsYAML(src string) *jobsConfig {
	cfg := &jobsConfig{Jobs: map[string]jobDef{}}
	var currentJob string
	var currentStep *stepDef
	inArgs := false

	for _, raw := range splitLines(src) {
		stripped := trimLeft(raw)
		if stripped == "" || stripped[0] == '#' {
			continue
		}
		indent := len(raw) - len(stripped)

		if indent == 0 {
			continue // "jobs:" header
		}

		// Job name at indent 2
		if indent == 2 && len(stripped) > 1 && stripped[len(stripped)-1] == ':' {
			if currentStep != nil && currentJob != "" {
				j := cfg.Jobs[currentJob]
				j.Steps = append(j.Steps, *currentStep)
				cfg.Jobs[currentJob] = j
				currentStep = nil
			}
			currentJob = stripped[:len(stripped)-1]
			cfg.Jobs[currentJob] = jobDef{}
			inArgs = false
			continue
		}

		// Job-level keys at indent 4
		if indent == 4 && currentJob != "" {
			if stripped == "steps:" {
				if currentStep != nil {
					j := cfg.Jobs[currentJob]
					j.Steps = append(j.Steps, *currentStep)
					cfg.Jobs[currentJob] = j
					currentStep = nil
				}
				inArgs = false
				continue
			}
			k, v := splitKV(stripped)
			if k == "description" {
				j := cfg.Jobs[currentJob]
				j.Description = v
				cfg.Jobs[currentJob] = j
			}
			continue
		}

		// Step item start at indent 6: "- <field>: <val>"
		if indent == 6 {
			if len(stripped) > 2 && stripped[:2] == "- " {
				if currentStep != nil {
					j := cfg.Jobs[currentJob]
					j.Steps = append(j.Steps, *currentStep)
					cfg.Jobs[currentJob] = j
				}
				currentStep = &stepDef{}
				inArgs = false
				rest := stripped[2:]
				k, v := splitKV(rest)
				applyStepField(currentStep, k, v)
				continue
			}
			// Field continuation at indent 6
			if currentStep != nil {
				k, v := splitKV(stripped)
				if k == "args" {
					inArgs = true
				} else {
					applyStepField(currentStep, k, v)
					inArgs = false
				}
			}
			continue
		}

		// Args list items at indent 8
		if indent == 8 && currentStep != nil && inArgs {
			if len(stripped) > 2 && stripped[:2] == "- " {
				currentStep.Args = append(currentStep.Args, stripQuotes(stripped[2:]))
			}
			continue
		}

		// Other indent 8 fields
		if indent == 8 && currentStep != nil {
			k, v := splitKV(stripped)
			applyStepField(currentStep, k, v)
		}
	}

	if currentStep != nil && currentJob != "" {
		j := cfg.Jobs[currentJob]
		j.Steps = append(j.Steps, *currentStep)
		cfg.Jobs[currentJob] = j
	}
	return cfg
}

func applyStepField(s *stepDef, k, v string) {
	switch k {
	case "name":
		s.Name = v
	case "command":
		s.Command = v
	case "skip":
		s.Skip = v == "true" || v == "True" || v == "TRUE" || v == "yes" || v == "Yes" || v == "YES"
	case "skip_reason":
		s.SkipReason = v
	}
}

func splitKV(s string) (string, string) {
	for i, c := range s {
		if c == ':' {
			return s[:i], stripQuotes(trimLeft(s[i+1:]))
		}
	}
	return s, ""
}

// stripQuotes removes a single layer of surrounding double or single quotes.
func stripQuotes(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
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

func trimLeft(s string) string {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return s[i:]
		}
	}
	return ""
}

// openInput opens a file for reading; returns os.Stdin if path is "-".
func openInput(path string) (io.Reader, func(), error) {
	if path == "" || path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open input %q: %w", path, err)
	}
	return f, func() { f.Close() }, nil
}

// openOutput opens a file for writing; returns os.Stdout if path is "-" or "".
func openOutput(path string) (io.Writer, func(), error) {
	if path == "" || path == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output %q: %w", path, err)
	}
	return f, func() { f.Close() }, nil
}
