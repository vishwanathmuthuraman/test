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

func WriterDashboard(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "writer" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/writer/login", 302)
		return
	}

	tmpl := template.Must(template.New("writer_dash.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/writer_dash.gohtml", "routes/views/global_layout.gohtml"))

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", account.Id))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func WriterLogin(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	tmpl := template.Must(template.New("routes/views/writer_login.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/writer_login.gohtml", "routes/views/global_layout.gohtml"))

	err := tmpl.ExecuteTemplate(w, "base", WrapGlobal(nil, "", "", struct{ Redirect string }{r.FormValue("redirect")}))
	if err != nil {
		w.Write([]byte(err.Error()))
	}
}

func WriterLoginPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	var login Login
	pwHash := fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw"))))
	row := db.QueryRow(r.Context(), "SELECT email, pw_hash from writer where email = $1 and pw_hash = $2", r.FormValue("email"), pwHash)
	err := row.Scan(&login.Username, &login.Hash)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorBack(w, r, "Bad username or password, please contact an admin if you have forgotten your password", "")
			return
		}
		ErrorBack(w, r, err.Error(), "")
		return
	}
	login.Type = "writer"
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
		SuccessBack(w, r, "Logged in", "/writer/dash")
	} else {
		SuccessBack(w, r, "Logged in", r.FormValue("redirect"))
	}
	return
}
