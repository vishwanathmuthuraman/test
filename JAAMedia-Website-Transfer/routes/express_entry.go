package routes

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"html/template"
	"net/http"
	url2 "net/url"
	"time"
)

func ExpressEntry(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	tmpl := template.Must(template.New("routes/views/express_video_entry.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/express_video_entry.gohtml", "routes/views/global_layout.gohtml"))
	account, _ := GetLogin(r, db)
	err := tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", ""))
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func ExpressEntryPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	err := r.ParseForm()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	url := r.FormValue("video_url")
	u, _ := url2.Parse(url)
	u.RawQuery = ""
	u.Fragment = ""
	videoUrl := u.String()
	password := r.FormValue("password")

	if password != "1234" {
		ErrorBack(w, r, "Access Denied", "")
		return
	}

	_, err = db.Exec(r.Context(), "INSERT INTO video (url, created, needs_entry, posted_date) VALUES ($1, $2, $3, $4, $5, $6, $7)", videoUrl, time.Now(), true)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				ErrorBack(w, r, "Video already entered", "")
				return
			}
		}
		ErrorBack(w, r, err.Error(), "")
		return
	}

	SuccessBack(w, r, "Video added", "/express_entry")
}
