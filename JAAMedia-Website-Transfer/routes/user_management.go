package routes

import (
	"clicktrack/models"
	"crypto/sha256"
	"fmt"
	"github.com/jackc/pgx/v5"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

func ManageVa(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("manage_va.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_manage_va.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var vas = make([]models.VaWithDetail, 0)

	res, err := db.Query(r.Context(), "SELECT id, name, email, vids_entered from va")
	if err != nil {
		http.Error(w, "error getting VAs", 500)
		return
	}
	for res.Next() {
		var va models.VaWithDetail

		err := res.Scan(&va.Id, &va.Name, &va.Email, &va.VideosEntered)
		if err != nil {
			http.Error(w, "error getting VAs", 500)

			return
		}
		vas = append(vas, va)
	}
	data := models.VaListView{Vas: vas}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		w.Write([]byte(err.Error()))
	}
}

func ManageWriters(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("manage_writers.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_manage_writers.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var vas = make([]models.WriterWithDetail, 0)

	res, err := db.Query(r.Context(), "SELECT id, COALESCE(name, ''), COALESCE(email, ''), COALESCE(video_count, 0) from writer")
	if err != nil {
		http.Error(w, "error getting writers", 500)

		return
	}
	for res.Next() {
		var va models.WriterWithDetail

		err := res.Scan(&va.Id, &va.Name, &va.Email, &va.VideoCount)
		if err != nil {
			http.Error(w, "error getting writers", 500)

			return
		}
		vas = append(vas, va)
	}
	data := models.WriterListView{Writers: vas}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		w.Write([]byte(err.Error()))
	}
}

func ManageSponsors(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("manage_sponsor.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_manage_sponsor.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var sponsors = make([]models.SponsorWithDetail, 0)

	res, err := db.Query(r.Context(), "SELECT id, name, COALESCE(email, '') from sponsor")
	if err != nil {
		http.Error(w, "error getting sponsors", 500)

		return
	}
	for res.Next() {
		var sponsor models.SponsorWithDetail

		err := res.Scan(&sponsor.Id, &sponsor.Name, &sponsor.Email)
		if err != nil {
			http.Error(w, "error getting sponsors", 500)
			return
		}
		sponsors = append(sponsors, sponsor)
	}
	data := models.SponsorListView{Sponsors: sponsors}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func CreateWriterPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	_, err = db.Exec(r.Context(), "INSERT INTO writer (name, email, pw_hash) VALUES ($1, $2, $3)", r.FormValue("name"), r.FormValue("email"), fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw")))))

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	SuccessBack(w, r, "Writer created", "/admin/manage_writers")

}

func ChangeWriterPw(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	_, err = db.Exec(r.Context(), "UPDATE writer SET pw_hash = $1 WHERE id = $2", fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw")))), r.FormValue("id"))

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	http.Redirect(w, r, "/admin/manage_writers", 302)
}

func CreateSponsorPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	_, err = db.Exec(r.Context(), "INSERT INTO sponsor (name, email, password_hash, video_count) VALUES ($1, $2, $3, $4)", r.FormValue("name"), r.FormValue("email"), fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw")))), 0)

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	SuccessBack(w, r, "Sponsor created", "/admin/manage_sponsors")
}

func ChangeSponsorPw(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	pwHash := fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw"))))
	_, err = db.Exec(r.Context(), "UPDATE sponsor SET password_hash = $1 WHERE id = $2", pwHash, r.FormValue("id"))

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	SuccessBack(w, r, "Sponsor password changed", "/admin/manage_sponsors")
}

func CreateVaPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	_, err = db.Exec(r.Context(), "INSERT INTO va (name, email, password_hash) VALUES ($1, $2, $3)", r.FormValue("name"), r.FormValue("email"), fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw")))))

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	SuccessBack(w, r, "VA created", "/admin/manage_va")
}

func ChangeVaPw(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	pwHash := fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw"))))
	_, err = db.Exec(r.Context(), "UPDATE va SET password_hash = $1 WHERE id = $2", pwHash, r.FormValue("id"))

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	http.Redirect(w, r, "/admin/dashboard", 302)
}

func EnterPayment(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}
	tmpl := template.Must(template.New("routes/views/admin_enter_payment.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_enter_payment.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var sponsors = make([]models.Sponsor, 0)

	res, err := db.Query(r.Context(), "SELECT id, name from sponsor")
	if err != nil {
		http.Error(w, "error getting sponsors", 500)
		return
	}
	for res.Next() {
		var sponsor models.Sponsor

		err := res.Scan(&sponsor.Id, &sponsor.Name)
		if err != nil {
			http.Error(w, "error getting sponsors", 500)
			return
		}
		sponsors = append(sponsors, sponsor)
	}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", struct{ Sponsors []models.Sponsor }{sponsors}))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func EnterPaymentPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	sponsorId, err := strconv.Atoi(r.FormValue("sponsor_id"))
	if err != nil {
		ErrorBack(w, r, "Invalid Sponsor ID", "")
		return
	}
	amount_dollar, err := strconv.Atoi(r.FormValue("amount_dollars"))
	if err != nil {
		ErrorBack(w, r, "Invalid Amount", "")
		return

	}

	amount_cent, err := strconv.Atoi(r.FormValue("amount_cents"))
	if err != nil {
		ErrorBack(w, r, "Invalid Amount", "")
		return

	}

	amount := amount_dollar*100 + amount_cent

	date, err := time.Parse("2006-01-02", r.FormValue("date"))
	if err != nil {
		ErrorBack(w, r, "Invalid Date", "")
		return
	}

	_, err = db.Exec(r.Context(), "INSERT INTO sponsor_payment (date, sponsor_id, amount, details) VALUES ($1, $2, $3, $4)", date, sponsorId, amount, r.FormValue("details"))

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	SuccessBack(w, r, "Payment entered successfully", "/admin/finances")
}
