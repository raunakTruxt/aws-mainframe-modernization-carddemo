// Package custdump replaces CBCUS01C / READCUST.jcl (RAU-44).
// It browses the customers table sequentially and writes a formatted report.
package custdump

import (
	"context"
	"fmt"
	"io"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Config holds the dependencies for the cust-dump subcommand.
type Config struct {
	Customers repo.CustomerRepository
	Out       io.Writer
}

// Summary reports how many records were processed.
type Summary struct {
	Count int
}

// Run browses all customers and writes a formatted report to cfg.Out.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	var s Summary
	const batchSize = 500
	var startID int64
	for {
		recs, err := cfg.Customers.Browse(ctx, startID, batchSize)
		if err != nil {
			return s, fmt.Errorf("custdump: browse: %w", err)
		}
		if len(recs) == 0 {
			break
		}
		for _, r := range recs {
			fmt.Fprintf(cfg.Out,
				"CUST-ID=%-9d NAME=%-25s %-25s %-25s\n"+
					"  ADDR=%-50s %-50s %-50s %s %s %-10s\n"+
					"  PHONE1=%-15s PHONE2=%-15s DOB=%-10s FICO=%d\n",
				r.CustID, r.CustFirstName, r.CustMiddleName, r.CustLastName,
				r.CustAddrLine1, r.CustAddrLine2, r.CustAddrLine3,
				r.CustAddrStateCode, r.CustAddrCountryCode, r.CustAddrZip,
				r.CustPhoneNum1, r.CustPhoneNum2, r.CustDOB, r.CustFICOCreditScore,
			)
			s.Count++
			startID = r.CustID + 1
		}
		if len(recs) < batchSize {
			break
		}
	}
	fmt.Fprintf(cfg.Out, "\nTotal customers: %d\n", s.Count)
	return s, nil
}
