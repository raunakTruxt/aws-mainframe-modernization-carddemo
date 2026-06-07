// Package reports is the HTTP edge for the transaction-reports screen.
// It is the Go replacement for CORPT00C → GET/POST /reports.
//
// Report generation runs asynchronously: POST enqueues a job on an in-memory
// queue and redirects to GET ?job=<id>, which polls the job status. This mirrors
// the original screen submitting a batch job (CBTRN03C) and the user checking
// back for the result.
package reports

import (
	"net/http"
	"strings"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/report"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/layout"
)

// Handlers exposes the HTTP entry points for the reports screen.
type Handlers struct {
	Queue *report.Queue
}

// NewHandlers constructs Handlers backed by queue.
func NewHandlers(queue *report.Queue) *Handlers {
	return &Handlers{Queue: queue}
}

type reportsBody struct {
	StartDate string
	EndDate   string
	AccountID string
	CSRFToken string
	Job       *report.Job
}

// GetReports renders the report request form, or — with ?job=<id> — the status
// and output of a previously submitted job.
func (h *Handlers) GetReports(w http.ResponseWriter, r *http.Request) {
	pd := layout.NewPage(r, "Transaction Reports")
	body := reportsBody{CSRFToken: pd.CSRFToken}

	if jobID := r.URL.Query().Get("job"); jobID != "" {
		if h.Queue == nil {
			layout.Render(w, pd.WithFlash("Reporting is not available.").WithBody(body), reportsContent)
			return
		}
		job, ok := h.Queue.Get(jobID)
		if !ok {
			layout.Render(w, pd.WithFlash("Unknown report job.").WithBody(body), reportsContent)
			return
		}
		body.Job = &job
	}

	layout.Render(w, pd.WithBody(body), reportsContent)
}

// PostReports validates the form, enqueues a report job, and redirects to the
// status page for that job.
func (h *Handlers) PostReports(w http.ResponseWriter, r *http.Request) {
	pd := layout.NewPage(r, "Transaction Reports")
	startDate := strings.TrimSpace(r.FormValue("start_date"))
	endDate := strings.TrimSpace(r.FormValue("end_date"))
	acctID := strings.TrimSpace(r.FormValue("account_id"))

	body := reportsBody{StartDate: startDate, EndDate: endDate, AccountID: acctID, CSRFToken: pd.CSRFToken}

	if h.Queue == nil {
		layout.Render(w, pd.WithFlash("Reporting is not available.").WithBody(body), reportsContent)
		return
	}

	var spec report.ReportSpec
	if acctID != "" {
		spec = report.ReportSpec{Kind: report.KindStatement, AccountID: acctID}
	} else {
		if startDate == "" || endDate == "" {
			layout.Render(w, pd.WithFlash("Enter a start and end date, or an account ID.").WithBody(body), reportsContent)
			return
		}
		spec = report.ReportSpec{Kind: report.KindTranReport, StartDate: startDate, EndDate: endDate}
	}

	id := h.Queue.Submit(spec)
	http.Redirect(w, r, "/reports?job="+id, http.StatusSeeOther)
}

const reportsContent = `{{define "content"}}
<section class="reports-screen">
  {{if .Body.Job}}
  <div class="report-status">
    <p>Job <strong>{{.Body.Job.ID}}</strong> — status: <strong>{{.Body.Job.Status}}</strong></p>
    {{if eq (printf "%s" .Body.Job.Status) "pending"}}
    <p><a href="/reports?job={{.Body.Job.ID}}">Refresh</a></p>
    {{else if eq (printf "%s" .Body.Job.Status) "failed"}}
    <p class="report-error">{{.Body.Job.Err}}</p>
    {{else}}
    <pre class="report-output">{{.Body.Job.Output}}</pre>
    {{end}}
    <p><a href="/reports">New report</a></p>
  </div>
  {{else}}
  <form method="post" action="/reports">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <fieldset>
      <legend>Daily Transaction Report</legend>
      <label for="start_date">Start date (YYYY-MM-DD):</label>
      <input id="start_date" name="start_date" type="text" maxlength="10" value="{{.Body.StartDate}}">
      <label for="end_date">End date (YYYY-MM-DD):</label>
      <input id="end_date" name="end_date" type="text" maxlength="10" value="{{.Body.EndDate}}">
    </fieldset>
    <fieldset>
      <legend>Account Statement</legend>
      <label for="account_id">Account ID:</label>
      <input id="account_id" name="account_id" type="text" maxlength="11" value="{{.Body.AccountID}}">
    </fieldset>
    <button type="submit">Submit (ENTER)</button>
  </form>
  {{end}}
</section>
{{end}}`
