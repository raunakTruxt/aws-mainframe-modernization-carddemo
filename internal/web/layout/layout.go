// Package layout provides the shared HTML base template and helpers used by all
// CardDemo web pages. Every handler calls layout.Render to wrap its page-specific
// content in the common header/footer shell, matching the 24×80 BMS screen frame.
package layout

import (
	"html/template"
	"net/http"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/auth"
)

// PageData is the data structure available to every page template.
// The "content" template block is the only part that varies per page.
type PageData struct {
	Title     string
	UserID    string
	IsAdmin   bool
	Flash     string // error or info message; shown in the red ERRMSG row
	CSRFToken string
	Now       time.Time
	Body      any // page-specific data; access in templates as .Body
}

// NewPage builds a PageData from the request context. It pulls the
// session (if any) to populate UserID / IsAdmin / CSRFToken.
func NewPage(r *http.Request, title string) PageData {
	pd := PageData{Title: title, Now: time.Now()}
	if sess, ok := auth.SessionFromContext(r.Context()); ok {
		pd.UserID = sess.UserID
		pd.IsAdmin = sess.IsAdmin()
		pd.CSRFToken = sess.CSRFToken
	}
	return pd
}

// Flash attaches an error/info message and returns the mutated PageData.
func (pd PageData) WithFlash(msg string) PageData {
	pd.Flash = msg
	return pd
}

// WithBody attaches page-specific data.
func (pd PageData) WithBody(body any) PageData {
	pd.Body = body
	return pd
}

// baseHTML is the shared 24-row page frame. Individual pages supply
// {{define "content"}} blocks that are rendered inside <main>.
const baseHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>CardDemo &#8211; {{.Title}}</title>
<link rel="stylesheet" href="/static/app.css">
</head>
<body>
<header class="app-header">
  <div class="header-left">
    <span class="label">Tran:</span><span class="value">CM00</span>
    <span class="label">Prog:</span><span class="value">CARDDEMO</span>
  </div>
  <div class="header-center"><span class="page-title">{{.Title}}</span></div>
  <div class="header-right">
    {{if .Now.IsZero}}{{else}}
    <span class="label">Date:</span><span class="value">{{.Now.Format "01/02/06"}}</span>
    <span class="label">Time:</span><span class="value">{{.Now.Format "15:04:05"}}</span>
    {{end}}
  </div>
</header>
{{if .UserID}}
<nav class="app-nav">
  <span class="nav-user">{{.UserID}}{{if .IsAdmin}}&nbsp;(Admin){{end}}</span>
  <form method="post" action="/logout" class="logout-form">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <button type="submit" class="btn-signoff">Sign off (F3)</button>
  </form>
</nav>
{{end}}
{{if .Flash}}<div class="app-flash">{{.Flash}}</div>{{end}}
<main class="app-main">
{{template "content" .}}
</main>
<footer class="app-footer">ENTER=Continue&nbsp;&nbsp;F3=Sign off</footer>
</body>
</html>`

// Render writes a complete HTML page. It clones the base template, parses
// the caller-supplied content block, and executes against pd.
//
// contentHTML must define a {{define "content"}}...{{end}} block.
func Render(w http.ResponseWriter, pd PageData, contentHTML string) {
	tpl := template.Must(template.Must(base.Clone()).Parse(contentHTML))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "base", pd); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// base is parsed once at startup; Render clones it for each page.
var base = template.Must(template.New("base").Parse(baseHTML))
