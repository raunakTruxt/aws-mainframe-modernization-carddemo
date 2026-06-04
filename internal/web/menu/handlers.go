// Package menu handles the CardDemo main-menu (COMEN01C) and admin-menu
// (COADM01C) screens. Both BMS maps have the same shape: a numbered list of
// options, a selection input, and an ERRMSG row.
package menu

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/layout"
)

// userMenuOptions mirrors COMEN02Y.cpy (11 options, all type 'U').
var userMenuOptions = []menuOption{
	{1, "Account View", "/account/view"},
	{2, "Account Update", "/account/update"},
	{3, "Credit Card List", "/cards/list"},
	{4, "Credit Card View", "/cards/view"},
	{5, "Credit Card Update", "/cards/update"},
	{6, "Transaction List", "/transactions"},
	{7, "Transaction View", "/transactions/view"},
	{8, "Transaction Add", "/transactions/add"},
	{9, "Transaction Reports", "/reports"},
	{10, "Bill Payment", "/billing"},
	{11, "Pending Authorization View", "/pending"},
}

// adminMenuOptions mirrors COADM02Y.cpy (6 options, admin-only).
var adminMenuOptions = []menuOption{
	{1, "User List (Security)", "/admin/users"},
	{2, "User Add (Security)", "/admin/users/new"},
	{3, "User Update (Security)", "/admin/users"},
	{4, "User Delete (Security)", "/admin/users"},
	{5, "Transaction Type List/Update", "/admin/txn-types"},
	{6, "Transaction Type Maintenance", "/admin/txn-types/update"},
}

type menuOption struct {
	Num  int
	Name string
	URL  string
}

// menuPageBody is the template data for both menu pages.
type menuPageBody struct {
	Options   []menuOption
	PostURL   string
	CSRFToken string
}

const menuContent = `{{define "content"}}
<section class="menu-screen">
  <form method="post" action="{{.Body.PostURL}}">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <table class="menu-options">
      {{range .Body.Options}}
      <tr>
        <td class="opt-num">{{printf "%2d" .Num}}.</td>
        <td class="opt-name">{{.Name}}</td>
      </tr>
      {{end}}
    </table>
    <div class="menu-prompt">
      <label for="option">Please select an option :</label>
      <input id="option" name="option" type="number" min="1"
             max="{{len .Body.Options}}" maxlength="2" size="2" autofocus>
      <button type="submit">Enter</button>
    </div>
  </form>
</section>
{{end}}`

// GetMainMenu serves COMEN01 — the regular-user main menu.
func GetMainMenu(w http.ResponseWriter, r *http.Request) {
	pd := layout.NewPage(r, "Main Menu")
	pd = pd.WithBody(menuPageBody{
		Options:   userMenuOptions,
		PostURL:   "/",
		CSRFToken: pd.CSRFToken,
	})
	layout.Render(w, pd, menuContent)
}

// PostMainMenu processes the user's option selection and redirects.
func PostMainMenu(w http.ResponseWriter, r *http.Request) {
	opt, err := parseOption(r, len(userMenuOptions))
	if err != nil {
		pd := layout.NewPage(r, "Main Menu").WithFlash(err.Error())
		pd = pd.WithBody(menuPageBody{
			Options:   userMenuOptions,
			PostURL:   "/",
			CSRFToken: pd.CSRFToken,
		})
		layout.Render(w, pd, menuContent)
		return
	}
	http.Redirect(w, r, userMenuOptions[opt-1].URL, http.StatusSeeOther)
}

// GetAdminMenu serves COADM01 — the admin-only main menu.
func GetAdminMenu(w http.ResponseWriter, r *http.Request) {
	pd := layout.NewPage(r, "Admin Menu")
	pd = pd.WithBody(menuPageBody{
		Options:   adminMenuOptions,
		PostURL:   "/admin",
		CSRFToken: pd.CSRFToken,
	})
	layout.Render(w, pd, menuContent)
}

// PostAdminMenu processes the admin's option selection and redirects.
func PostAdminMenu(w http.ResponseWriter, r *http.Request) {
	opt, err := parseOption(r, len(adminMenuOptions))
	if err != nil {
		pd := layout.NewPage(r, "Admin Menu").WithFlash(err.Error())
		pd = pd.WithBody(menuPageBody{
			Options:   adminMenuOptions,
			PostURL:   "/admin",
			CSRFToken: pd.CSRFToken,
		})
		layout.Render(w, pd, menuContent)
		return
	}
	http.Redirect(w, r, adminMenuOptions[opt-1].URL, http.StatusSeeOther)
}

// parseOption reads the "option" form field and validates it is in [1, max].
func parseOption(r *http.Request, max int) (int, error) {
	raw := strings.TrimSpace(r.FormValue("option"))
	if raw == "" {
		return 0, fmt.Errorf("Please enter a valid option number...")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		return 0, fmt.Errorf("Please enter a valid option number...")
	}
	return n, nil
}
