package console

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]{1,64}$`)

// password serves the single-admin account page: change the username, the
// password, or both. Any successful change signs the operator out.
func (s *Server) password(w http.ResponseWriter, r *http.Request, id string, v Session) {
	p := Page{Title: "Account", Kind: "account", CSRF: v.CSRF, User: v.User, Item: Record{"username": v.User}}
	p.ForceChange = v.RequiresPasswordChange
	if v.RequiresPasswordChange {
		p.Notice = "You must change your password before using the console. You can also change the username here; you will sign in again afterwards."
	}
	if r.Method == http.MethodPost {
		current := r.PostFormValue("current_password")
		next := r.PostFormValue("new_password")
		user := strings.TrimSpace(r.PostFormValue("new_username"))
		if user == "" {
			user = v.User // blank keeps the current username
		}
		p.Item["username"] = user
		switch {
		case current == "":
			p.Error = "Current password is required."
		case !usernamePattern.MatchString(user):
			p.Error = "Username must be 1–64 characters: letters, numbers, _ . @ -"
		case next == "" && v.RequiresPasswordChange:
			p.Error = "The bootstrap password must be replaced with a new password."
		case next == "" && user == v.User:
			p.Error = "Change the username, the password, or both."
		case next != "" && (len(next) < 8 || len(next) > 72 || next == "admin"):
			p.Error = "New password must be 8–72 bytes and must not be the bootstrap default."
		case next != r.PostFormValue("confirm_password"):
			p.Error = "New passwords do not match."
		default:
			path, body := "/auth/password", map[string]string{"current_password": current, "new_password": next}
			if user != v.User {
				path, body = "/auth/credentials", map[string]string{"current_password": current, "new_username": user}
				if next != "" {
					body["new_password"] = next
				}
			}
			_, _, err := s.request(r.Context(), v.Token, http.MethodPost, path, body)
			if err == nil {
				s.remove(id)
				s.cookie(w, "", time.Unix(1, 0))
				w.Header().Set("HX-Redirect", "/login")
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if s.expired(w, r, id, err) {
				return
			}
			p.Error = err.Error()
		}
	}
	render(w, r, p)
}
