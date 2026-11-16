package console

import (
	"net/http"
	"time"
)

func (s *Server) password(w http.ResponseWriter, r *http.Request, id string, v Session) {
	p := Page{Title: "Change password", Kind: "account", CSRF: v.CSRF, User: v.User}
	if v.RequiresPasswordChange {
		p.Notice = "You must change your password before using the console."
	}
	if r.Method == http.MethodPost {
		current, next := r.PostFormValue("current_password"), r.PostFormValue("new_password")
		switch {
		case current == "":
			p.Error = "Current password is required."
		case len(next) < 8 || len(next) > 72 || next == "admin":
			p.Error = "New password must be 8–72 bytes and must not be the bootstrap default."
		case next != r.PostFormValue("confirm_password"):
			p.Error = "New passwords do not match."
		default:
			_, _, err := s.request(r.Context(), v.Token, http.MethodPost, "/auth/password", map[string]string{"current_password": current, "new_password": next})
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
