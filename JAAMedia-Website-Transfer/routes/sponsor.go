package routes

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"html/template"
	"net/http"
)

func SponsorLogin(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	tmpl := template.Must(template.New("routes/views/sponsor_login.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/sponsor_login.gohtml", "routes/views/global_layout.gohtml"))

	err := tmpl.ExecuteTemplate(w, "base", WrapGlobal(nil, "", "", struct{ Redirect string }{r.FormValue("redirect")}))
	if err != nil {
		w.Write([]byte(err.Error()))
	}
}

func SponsorLoginPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	var login Login
	pwHash := fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw"))))
	row := db.QueryRow(r.Context(), "SELECT email, password_hash from sponsor where email = $1 and password_hash = $2", r.FormValue("email"), pwHash)

	err := row.Scan(&login.Username, &login.Hash)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorBack(w, r, "Bad username or password, please contact an admin if you have forgotten your password", "")
			return
		}
		ErrorBack(w, r, err.Error(), "")
		return
	}
	login.Type = "sponsor"
	cookieValue := bytes.NewBuffer([]byte{})
	err = json.NewEncoder(cookieValue).Encode(login)
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	cookie := http.Cookie{
		Name:  "login",
		Path:  "/",
		Value: base64.URLEncoding.EncodeToString(cookieValue.Bytes()),
	}
	http.SetCookie(w, &cookie)
	if r.FormValue("redirect") == "" {
		SuccessBack(w, r, "Logged in", "/sponsor/dash")
	} else {
		SuccessBack(w, r, "Logged in", r.FormValue("redirect"))
	}
	return
}

func SponsorDash(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "sponsor" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/sponsor/login", 302)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/grafana/d/sponsor-dash?var-sponsor_id=%d", account.Id), 302)
	return
}
