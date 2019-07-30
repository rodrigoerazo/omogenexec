// Handler for logging in a user
package users

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/jsannemo/omogenjudge/frontend/paths"
	"github.com/jsannemo/omogenjudge/frontend/request"
	"github.com/jsannemo/omogenjudge/storage/users"
)

var loginTemplates = template.Must(template.ParseFiles(
	"frontend/users/login.tpl",
	"frontend/templates/header.tpl",
	"frontend/templates/nav.tpl",
	"frontend/templates/footer.tpl",
))

// LoginHandler handles login requests
func LoginHandler(r *request.Request) (request.Response, error) {
	rootUrl, err := paths.Route(paths.Home).URL()
	if err != nil {
		return nil, err
	}
	root := rootUrl.String()
	if r.Context.UserId != 0 {
		return request.Redirect(root), nil
	}
	if r.Request.Method == http.MethodPost {
		username := r.Request.FormValue("username")
		password := r.Request.FormValue("password")

		user, err := users.AuthenticateUser(r.Request.Context(), username, password)
		if err == users.ErrInvalidLogin {
      // TODO show an error message for this instead on the registration page
			return request.Error(errors.New("Incorrect login details")), nil
		} else if err != nil {
			return request.Error(err), nil
		}
		r.Context.UserId = user.AccountId
		return request.Redirect(root), nil
	}
	return request.Template(loginTemplates, "page", nil), nil
}