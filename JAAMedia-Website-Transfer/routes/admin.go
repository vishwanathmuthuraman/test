package routes

import (
	"bytes"
	"clicktrack/models"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/jackc/pgx/v5"
	"golang.org/x/exp/slices"
	"html/template"
	"net/http"
	url2 "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func ManageVideos(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || !(account.Type == "admin" || account.Type == "va") {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("routes/views/admin_videoslist.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_videoslist.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}

	var query string = "SELECT video.id, posted_date, COALESCE(sponsor.id, 0), COALESCE(sponsor.name, ''), COALESCE(account.id, 0), COALESCE(account.username, 'N/A'), account.platform, url, needs_entry, COALESCE(preview, ''), s.views_total, s.likes_total, s.comments_total, s.engagement_like, s.engagement_comment, s.error from video LEFT JOIN sponsor ON sponsor_id = sponsor.id LEFT JOIN account ON account_id = account.id LEFT JOIN statistics s on video.id = s.video_id"
	var clauses = make([]string, 0)
	url := r.FormValue("url")
	//if url != "" {
	clauses = append(clauses, "(url = $1 or $1 = '')")
	//}
	accountId, err := strconv.Atoi(r.FormValue("account_id"))
	//if err == nil {
	clauses = append(clauses, "(account_id = $2 or $2 = 0)")
	//} else {
	//	err = nil
	//}

	sponsorId, err := strconv.Atoi(r.FormValue("sponsor_id"))
	//if err == nil {
	clauses = append(clauses, "(sponsor_id = $3 or $3 = 0)")
	//} else {
	//	err = nil
	//}

	dateAfter, errDAfter := time.Parse("2006-01-02", r.FormValue("date_after"))
	dateBefore, errDBefore := time.Parse("2006-01-02", r.FormValue("date_before"))

	dateAfterDb := sql.NullTime{Time: dateAfter, Valid: errDAfter == nil}
	dateBeforeDb := sql.NullTime{Time: dateBefore, Valid: errDBefore == nil}

	//if err == nil {
	clauses = append(clauses, "((created BETWEEN $4 AND $5) OR ($4 IS NULL AND $5 IS NULL))")
	//} else {
	//	err = nil
	//}

	minViews, err := strconv.Atoi(r.FormValue("min_views"))
	clauses = append(clauses, "(s.views_total >= $6 or $6 = 0)")

	minLikes, err := strconv.Atoi(r.FormValue("min_likes"))
	clauses = append(clauses, "(s.likes_total >= $7 or $7 = 0)")

	if r.FormValue("sponsored_only") != "" {
		clauses = append(clauses, "(sponsor_id IS NOT NULL OR needs_entry is TRUE)")
	}

	if r.FormValue("non_sponsored_only") != "" {
		clauses = append(clauses, "(sponsor_id IS NULL AND needs_entry is FALSE)")
	}

	if r.FormValue("needs_entry") != "" {
		clauses = append(clauses, "(needs_entry)")
	}

	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		limit = 100
	}
	offset, err := strconv.Atoi(r.FormValue("offset"))
	if err != nil {
		offset = 0
	}

	var orderBy string = "created"
	var direction string = "desc"
	switch r.FormValue("sort") {
	case "date_desc":
		orderBy = "posted_date"
		direction = "desc"
	case "views_asc":
		orderBy = "s.views_total"
		direction = "asc"
	case "views_desc":
		orderBy = "s.views_total"
		direction = "desc"
	case "like_rate_desc":
		orderBy = "s.engagement_like"
		direction = "desc"
	case "like_rate_asc":
		orderBy = "s.engagement_like"
		direction = "asc"
	case "comment_rate_desc":
		orderBy = "s.engagement_comment"
		direction = "desc"
	case "comment_rate_asc":
		orderBy = "s.engagement_comment"
		direction = "asc"
	case "date_asc":
		orderBy = "posted_date"
		direction = "asc"
	default:
		orderBy = "posted_date"
		direction = "desc"
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	var videos = make([]models.VideoListItem, 0)

	res, err := db.Query(r.Context(), query+fmt.Sprintf(" order by %s %s nulls last limit $8 offset $9", orderBy, direction), url, accountId, sponsorId, dateAfterDb, dateBeforeDb, minViews, minLikes, limit, offset)
	defer res.Close()

	if err != nil {
		http.Error(w, "error getting videos", 500)
		return
	}
	for res.Next() {
		var video models.VideoListItem

		err := res.Scan(&video.Id, &video.PostedDate, &video.Sponsor.Id, &video.Sponsor.Name, &video.Account.Id, &video.Account.Username, &video.Account.Platform, &video.Url, &video.NeedsEntry, &video.Preview, &video.ViewCount, &video.LikeCount, &video.CommentCount, &video.LikeRate, &video.CommentRate, &video.Error)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")

			return
		}
		video.Graph = fmt.Sprintf("%s?from=now-6h&to=now&var-url=%s", "/grafana/d/filter-by-video/filter-by-video-url", url2.QueryEscape(video.Url))
		videos = append(videos, video)
	}

	var totalN int
	err = db.QueryRow(r.Context(), "SELECT count(*) from video LEFT JOIN statistics s on video.id = s.video_id"+" WHERE "+strings.Join(clauses, " AND "), url, accountId, sponsorId, dateAfter, dateBefore, minViews, minLikes).Scan(&totalN)
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	var accounts = make([]models.Account, 0)
	res2, err := db.Query(r.Context(), "SELECT id, username, platform from account")
	defer res2.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	for res2.Next() {
		var account models.Account
		err := res2.Scan(&account.Id, &account.Username, &account.Platform)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		accounts = append(accounts, account)
	}

	var sponsors = make([]models.Sponsor, 0)
	res3, err := db.Query(r.Context(), "SELECT id, name from sponsor")
	defer res3.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	for res3.Next() {
		var sponsor models.Sponsor
		err := res3.Scan(&sponsor.Id, &sponsor.Name)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		sponsors = append(sponsors, sponsor)
	}

	data := struct {
		Results             []models.VideoListItem
		Accounts            []models.Account
		Sponsors            []models.Sponsor
		FirstN              int
		LastN               int
		TotalN              int
		QueryAccount        int
		QuerySponsor        int
		QueryAfter          sql.NullTime
		QueryBefore         sql.NullTime
		QueryUrl            string
		QuerySort           string
		QueryMinViews       int
		QueryMinLikes       int
		Grid                bool
		QuerySponsorOnly    bool
		QueryNonSponsorOnly bool
		QueryNeedsEntryOnly bool
	}{
		QuerySponsorOnly:    r.FormValue("sponsored_only") != "",
		QueryNonSponsorOnly: r.FormValue("non_sponsored_only") != "",
		Results:             videos,
		Accounts:            accounts,
		Sponsors:            sponsors,
		FirstN:              offset + 1,
		LastN:               offset + len(videos),
		TotalN:              totalN,
		QueryAccount:        accountId,
		QuerySponsor:        sponsorId,
		QueryAfter:          dateAfterDb,
		QueryBefore:         dateBeforeDb,
		QueryMinViews:       minViews,
		QueryMinLikes:       minLikes,
		QueryUrl:            url,
		QuerySort:           r.FormValue("sort"),
		Grid:                r.FormValue("grid") != "",
		QueryNeedsEntryOnly: r.FormValue("needs_entry") != "",
	}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

type Point struct {
	Url      string
	Views    float64
	Likes    float64
	Comments float64
}

func ExportVideoData(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {

	csvWriter := csv.NewWriter(w)
	r.ParseForm()
	videoUrls := r.Form["export_url"]
	startDate := r.Form.Get("date_after")
	if startDate == "" {
		startDate = "-7d"
	}
	//for i, url := range videoUrls {
	//decoded, _ := base64.URLEncoding.DecodeString(url)
	//videoUrls[i] = string(decoded)
	//}

	token := os.Getenv("INFLUX")
	url := os.Getenv("INFLUX_URL")
	influxClient := influxdb2.NewClientWithOptions(url, token,
		influxdb2.DefaultOptions().
			SetUseGZip(true).
			SetTLSConfig(&tls.Config{
				InsecureSkipVerify: true,
			}))

	queryApi := influxClient.QueryAPI("jaamedia")

	constraintString := ""
	for i, videoUrl := range videoUrls {
		if i > 0 {
			constraintString += " or "
		}
		constraintString += fmt.Sprintf("r[\"url\"] == \"%s\"", videoUrl)
	}

	timesViews := make(map[time.Time]float64)
	timesLikes := make(map[time.Time]float64)
	timesComments := make(map[time.Time]float64)

	result, err := queryApi.Query(context.Background(), `

import "experimental/aggregate"

import "date"

from(bucket: "views")
  |> range(start: `+startDate+`, stop: -1s)
  |> filter(fn: (r) => r["_measurement"] == "views")
  |> filter(fn: (r) => r["_field"] == "views")
  |> filter(fn: (r) => `+constraintString+`)
  |> aggregateWindow(every: 1h, fn: max, createEmpty: true)
  |> fill(usePrevious: true)
    |> derivative(unit: 1h, nonNegative: true)
|> group()
  |> aggregateWindow(every: 1h, fn: sum, createEmpty: false)
`)

	loc, _ := time.LoadLocation("America/New_York")
	if err == nil {
		for result.Next() {
			float, _ := result.Record().Value().(float64)
			timesViews[result.Record().Time().In(loc)] = float
		}
	}

	result, err = queryApi.Query(context.Background(), `

import "experimental/aggregate"

import "date"

from(bucket: "views")
  |> range(start: `+startDate+`, stop: -1s)
  |> filter(fn: (r) => r["_measurement"] == "views")
  |> filter(fn: (r) => r["_field"] == "likes")
  |> filter(fn: (r) => `+constraintString+`)
  |> aggregateWindow(every: 1h, fn: max, createEmpty: true)
  |> fill(usePrevious: true)
    |> derivative(unit: 1h, nonNegative: true)
|> group()
  |> aggregateWindow(every: 1h, fn: sum, createEmpty: false)
`)

	if err == nil {
		for result.Next() {
			float, _ := result.Record().Value().(float64)
			timesLikes[result.Record().Time().In(loc)] = float
		}
	}

	result, err = queryApi.Query(context.Background(), `

import "experimental/aggregate"

import "date"

from(bucket: "views")
  |> range(start: `+startDate+`, stop: -1s)
  |> filter(fn: (r) => r["_measurement"] == "views")
  |> filter(fn: (r) => r["_field"] == "comments")
  |> filter(fn: (r) => `+constraintString+`)
  |> aggregateWindow(every: 1h, fn: max, createEmpty: true)
  |> fill(usePrevious: true)
    |> derivative(unit: 1h, nonNegative: true)
|> group()
  |> aggregateWindow(every: 1h, fn: sum, createEmpty: false)
`)

	if err == nil {
		for result.Next() {
			float, _ := result.Record().Value().(float64)
			timesComments[result.Record().Time().In(loc)] = float
		}
	}
	csvWriter.Write([]string{"Timestamp", "Views", "Likes", "Comments"})
	w.Header().Set("Content-Type", "text/csv")
	sortedTimes := make([]time.Time, 0)
	for t := range timesViews {
		sortedTimes = append(sortedTimes, t)
	}
	sort.Slice(sortedTimes, func(i, j int) bool {
		return sortedTimes[i].Before(sortedTimes[j])
	})
	for _, t := range sortedTimes {
		csvWriter.Write([]string{t.Format(time.DateTime), fmt.Sprintf("%.2f", timesViews[t]), fmt.Sprintf("%.2f", timesLikes[t]), fmt.Sprintf("%.2f", timesComments[t])})
		csvWriter.Flush()
	}

}

func ManageVaVideos(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("manage_videos.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_manage_videos.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var videos = make([]models.VideoListItem, 0)

	res, err := db.Query(r.Context(), "SELECT video.id, entered_by, va.name, created, sponsor.id, sponsor.name, sponsor_rate, writer.id, writer.name, writer_rate, account.id, account.username, account.platform, url from video JOIN va ON entered_by = va.id JOIN sponsor ON sponsor_id = sponsor.id JOIN writer ON writer_id = writer.id JOIN account ON account_id = account.id where entered_by = $1 order by created desc", r.FormValue("va_id"))
	if err != nil {
		http.Error(w, "error getting VA videos", 500)
		return
	}
	for res.Next() {
		var video models.VideoListItem

		err := res.Scan(&video.Id, &video.Va.Id, &video.Va.Name, &video.Created, &video.Sponsor.Id, &video.Sponsor.Name, &video.SponsorRate, &video.Writer.Id, &video.Writer.Name, &video.WriterRate, &video.Account.Id, &video.Account.Username, &video.Account.Platform, &video.Url)
		if err != nil {
			http.Error(w, "error getting VA videos", 500)

			return
		}
		video.Graph = fmt.Sprintf("%s?from=now-6h&to=now&var-url=%s", "/grafana/d/filter-by-video/filter-by-video-url", url2.QueryEscape(video.Url))
		videos = append(videos, video)
	}
	data := videos

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func AdminFinances(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("routes/views/admin_finances.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_finances.gohtml", "routes/views/global_layout.gohtml"))

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", ""))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func AdminSponsorDetails(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	sponsorId, err := strconv.Atoi(r.FormValue("sponsor_id"))

	tmpl := template.Must(template.New("routes/views/admin_sponsor_details.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_sponsor_details.gohtml", "routes/views/global_layout.gohtml"))

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", sponsorId))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func AdminCreativity(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		http.Redirect(w, r, "/admin/login", 302)
		return
	}
	nDays, err := strconv.Atoi(r.FormValue("n_days"))
	if err != nil {
		http.Error(w, "Bad number of days (must be a number)", 400)

		return
	}
	tmpl := template.Must(template.New("routes/views/admin_creativity.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_creativity.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var accounts = make([]models.Account, 0)

	res, err := db.Query(r.Context(), "SELECT id,platform,username from account")

	if err != nil {
		http.Error(w, "error getting accounts", 500)

		return
	}
	for res.Next() {
		var account models.Account

		err := res.Scan(&account.Id, &account.Platform, &account.Username)
		if err != nil {
			http.Error(w, "error getting accounts", 500)

			return
		}
		accounts = append(accounts, account)
	}

	days := make([]time.Time, 0)
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), now.Day()-nDays, now.Hour(), now.Minute(), 0, 0, now.Location())
	for i := 0; i < nDays; i++ {
		d := time.Date(first.Year(), first.Month(), first.Day()+i, first.Hour(), first.Minute(), 0, 0, first.Location())
		days = append(days, d)
	}

	values := make(map[string]models.NullInt64, 0)
	res, err = db.Query(r.Context(), "SELECT account_id, day, amount from creativity where day >= $1", first)

	if err != nil {
		http.Error(w, "error getting creativity", 500)

		return
	}
	for res.Next() {
		var accountId int
		var day time.Time
		var value sql.NullInt64

		err := res.Scan(&accountId, &day, &value)
		if err != nil {
			http.Error(w, "error getting creativity", 500)

			return
		}

		//if value.Valid {
		values[fmt.Sprintf("%d_%s", accountId, day.Format("2006-01-02"))] = models.NullInt64(value)
		//}
	}

	data := struct {
		Accounts []models.Account
		Days     []time.Time
		Prefill  map[string]models.NullInt64
	}{
		accounts, days, values,
	}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func AdminCreativityPost(writer http.ResponseWriter, request *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	items := make([]struct {
		Day       time.Time
		AccountId int
		Amount    int
	}, 0)
	err := request.ParseForm()
	if err != nil {
		ErrorBack(writer, request, err.Error(), "")
	}

	for key, values := range request.Form { // range over map
		for _, value := range values { // range over []string
			if value == "" {
				continue
			}
			fmt.Println(key, value)
			var item struct {
				Day       time.Time
				AccountId int
				Amount    int
			}
			amt, err := strconv.Atoi(value)
			if err != nil {
				ErrorBack(writer, request, "bad value: "+err.Error(), "")
				return
			}
			item.Amount = amt
			year := 0
			month := 0
			day := 0
			if _, err := fmt.Sscanf(key, "%d_%d-%d-%d", &item.AccountId, &year, &month, &day); err != nil {
				ErrorBack(writer, request, "could not enter creativity: "+err.Error(), "")
				return
			}
			item.Day = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			items = append(items, item)
		}
	}
	for _, item := range items {
		_, err := db.Exec(request.Context(), "INSERT INTO creativity values ($1,$2,$3) on conflict on constraint creativity_pk do update set amount = $3 ", item.Day, item.AccountId, item.Amount)
		if err != nil {
			ErrorBack(writer, request, "could not enter creativity: "+err.Error(), "")
			return
		}
	}

	SuccessBack(writer, request, "Updated successfully", "")
}

func AdminDashboard(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("manage_accounts.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_manage_accounts.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	//var accounts = make([]models.Account, 0)
	//
	//res, err := db.Query(r.Context(), "SELECT username, platform from account")
	//if err != nil {
	//	http.Error(w, "error getting accounts", 500)
	//
	//	return
	//}
	//for res.Next() {
	//	var account models.Account
	//
	//	err := res.Scan(&account.Username, &account.Platform)
	//	if err != nil {
	//		http.Error(w, "error getting accounts", 500)
	//
	//		return
	//	}
	//	accounts = append(accounts, account)
	//}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", ""))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func AdminRetention(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	tmpl := template.Must(template.New("routes/views/admin_retention.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_retention.gohtml", "routes/views/global_layout.gohtml"))

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", ""))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func AdminLogin(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	tmpl := template.Must(template.New("routes/views/admin_login.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_login.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	err := tmpl.ExecuteTemplate(w, "base", WrapGlobal(nil, "", "", struct{ Redirect string }{r.FormValue("redirect")}))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")

	}
}

func AdminLoginPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	var login Login
	pwHash := fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw"))))
	row := db.QueryRow(r.Context(), "SELECT email, password_hash from admin where email = $1 and password_hash = $2", r.FormValue("email"), pwHash)
	err := row.Scan(&login.Username, &login.Hash)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorBack(w, r, "Bad username or password, please contact an admin if you have forgotten your password", "")
			return
		}
		ErrorBack(w, r, err.Error(), "")
		return
	}
	login.Type = "admin"
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
		SuccessBack(w, r, "Logged in", "/admin/dashboard")
	} else {
		SuccessBack(w, r, "Logged in", r.FormValue("redirect"))
	}
	return
}

func AdminErrors(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}
	data := struct {
		Errors []models.TrackingError
	}{}

	res, err := db.Query(r.Context(), "SELECT url, message, last_timestamp, count from errors order by last_timestamp desc")

	for res.Next() {
		var row models.TrackingError
		err := res.Scan(&row.Url, &row.Message, &row.LastSeen, &row.Frequency)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		data.Errors = append(data.Errors, row)
	}
	tmpl := template.Must(template.New("routes/views/admin_errors.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/admin_errors.gohtml", "routes/views/global_layout.gohtml"))

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

type VideoStatistics struct {
	ViewsTotal        int
	Views24h          int
	Views12h          int
	Views6h           int
	Views1h           int
	Views30m          int
	Retention3        int
	Retention5        int
	Retention10       int
	EngagementLike    float64
	EngagementComment float64
	LikeTotal         int
	CommentTotal      int
}

type ByPercentile struct {
	Percentile25 int
	Percentile50 int
	Percentile75 int
}

type SummaryStatistic struct {
	Strategy    models.Strategy
	ViewsTotal  ByPercentile
	Views24h    ByPercentile
	Views12h    ByPercentile
	Views6h     ByPercentile
	Views1h     ByPercentile
	Views30m    ByPercentile
	Retention3  ByPercentile
	Retention5  ByPercentile
	Retention10 ByPercentile
}

func StrategiesView(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || account.Type != "admin" {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/admin/login", 302)
		return
	}

	var accounts = make([]models.Account, 0)
	{
		rows, err := db.Query(r.Context(), "SELECT id, username, platform from account")
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		for rows.Next() {
			var account models.Account
			err := rows.Scan(&account.Id, &account.Username, &account.Platform)
			if err != nil {
				ErrorBack(w, r, err.Error(), "")
				return
			}
			accounts = append(accounts, account)
		}
	}

	var sponsors = make([]models.Sponsor, 0)
	{
		rows, err := db.Query(r.Context(), "SELECT id, name from sponsor")
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		for rows.Next() {
			var sponsor models.Sponsor
			err := rows.Scan(&sponsor.Id, &sponsor.Name)
			if err != nil {
				ErrorBack(w, r, err.Error(), "")
				return
			}
			sponsors = append(sponsors, sponsor)
		}
	}

	var controlOptions = make([]models.Strategy, 0)
	{
		rows, err := db.Query(r.Context(), "SELECT id, name from strategy")
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		for rows.Next() {
			var strategy models.Strategy
			err := rows.Scan(&strategy.Id, &strategy.Name)
			if err != nil {
				ErrorBack(w, r, err.Error(), "")
				return
			}
			controlOptions = append(controlOptions, strategy)
		}
	}

	var strategiesMap = make(map[models.Strategy][]VideoStatistics)

	var query string = "SELECT s2.id, s2.name, s2.variable, s2.value, s.* from video_strategies join statistics s on video_strategies.video_id = s.video_id join strategy s2 on video_strategies.strategy_id = s2.id join video v on s.video_id = v.id"
	var clauses = make([]string, 0)
	//}
	accountId, err := strconv.Atoi(r.FormValue("account_id"))
	clauses = append(clauses, "(v.account_id = $1 or $1 = 0)")

	sponsorId, err := strconv.Atoi(r.FormValue("sponsor_id"))
	clauses = append(clauses, "(v.sponsor_id = $2 or $2 = 0)")

	dateAfter, err := time.Parse("2006-01-02", r.FormValue("date_after"))
	dateBefore, err := time.Parse("2006-01-02", r.FormValue("date_before"))
	clauses = append(clauses, "((v.created BETWEEN $3 AND $4) OR ($3 = '0001-01-01' AND $4 = '0001-01-01'))")

	onlyVariable := r.FormValue("variable")
	clauses = append(clauses, "(s2.variable = $5 or $5 = '')")

	if r.FormValue("sponsored_only") != "" {
		clauses = append(clauses, "(v.sponsor_id IS NOT NULL)")
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := db.Query(r.Context(), query, accountId, sponsorId, dateAfter, dateBefore, onlyVariable)

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for rows.Next() {
		var strategy models.Strategy
		var stats VideoStatistics
		var variable sql.NullString
		var value sql.NullString
		var videoId int
		err := rows.Scan(&strategy.Id, &strategy.Name, &variable, &value, &videoId, &stats.ViewsTotal, &stats.Views24h, &stats.Views12h, &stats.Views6h, &stats.Views1h, &stats.Views30m, &stats.Retention3, &stats.Retention5, &stats.Retention10, &stats.EngagementLike, &stats.EngagementComment, &stats.LikeTotal, &stats.CommentTotal)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}

		if variable.Valid && value.Valid {
			strategy.Variable = variable.String
			strategy.Value = value.String
		}

		strategiesMap[strategy] = append(strategiesMap[strategy], stats)
	}

	var res struct {
		QueryAccount   int
		QuerySponsor   int
		QueryBefore    time.Time
		QueryAfter     time.Time
		QuerySortBy    string
		QueryVariable  string
		Strategies     []SummaryStatistic
		Accounts       []models.Account
		Sponsors       []models.Sponsor
		ControlOptions []models.Strategy
		Variables      map[string]int
	}
	res.Strategies = make([]SummaryStatistic, 0)
	res.Variables = make(map[string]int)

	rows, err = db.Query(r.Context(), "SELECT DISTINCT(variable) from strategy")
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	for rows.Next() {
		var v sql.NullString
		err := rows.Scan(&v)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		res.Variables[v.String] = 1
	}

	for strategy, stats := range strategiesMap {

		var statistic SummaryStatistic
		statistic.Strategy = strategy

		p25 := len(stats) / 4
		p50 := len(stats) / 2
		p75 := len(stats) / 4 * 3

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.ViewsTotal < j.ViewsTotal })
		statistic.ViewsTotal = ByPercentile{
			Percentile25: stats[p25].ViewsTotal,
			Percentile50: stats[p50].ViewsTotal,
			Percentile75: stats[p75].ViewsTotal,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Views24h < j.Views24h })
		statistic.Views24h = ByPercentile{
			Percentile25: stats[p25].Views24h,
			Percentile50: stats[p50].Views24h,
			Percentile75: stats[p75].Views24h,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Views12h < j.Views12h })
		statistic.Views12h = ByPercentile{
			Percentile25: stats[p25].Views12h,
			Percentile50: stats[p50].Views12h,
			Percentile75: stats[p75].Views12h,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Views6h < j.Views6h })
		statistic.Views6h = ByPercentile{
			Percentile25: stats[p25].Views6h,
			Percentile50: stats[p50].Views6h,
			Percentile75: stats[p75].Views6h,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Views1h < j.Views1h })
		statistic.Views1h = ByPercentile{
			Percentile25: stats[p25].Views1h,
			Percentile50: stats[p50].Views1h,
			Percentile75: stats[p75].Views1h,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Views30m < j.Views30m })
		statistic.Views30m = ByPercentile{
			Percentile25: stats[p25].Views30m,
			Percentile50: stats[p50].Views30m,
			Percentile75: stats[p75].Views30m,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Retention3 < j.Retention3 })
		statistic.Retention3 = ByPercentile{
			Percentile25: stats[p25].Retention3,
			Percentile50: stats[p50].Retention3,
			Percentile75: stats[p75].Retention3,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Retention5 < j.Retention5 })
		statistic.Retention5 = ByPercentile{
			Percentile25: stats[p25].Retention5,
			Percentile50: stats[p50].Retention5,
			Percentile75: stats[p75].Retention5,
		}

		slices.SortFunc(stats, func(i, j VideoStatistics) bool { return i.Retention10 < j.Retention10 })
		statistic.Retention10 = ByPercentile{
			Percentile25: stats[p25].Retention10,
			Percentile50: stats[p50].Retention10,
			Percentile75: stats[p75].Retention10,
		}

		res.Strategies = append(res.Strategies, statistic)
	}

	sortBy := r.FormValue("sort_by")

	if sortBy == "views_total" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].ViewsTotal.Percentile50 > res.Strategies[j].ViewsTotal.Percentile50
		})
	} else if sortBy == "views_24h" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Views24h.Percentile50 > res.Strategies[j].Views24h.Percentile50
		})
	} else if sortBy == "views_12h" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Views12h.Percentile50 > res.Strategies[j].Views12h.Percentile50
		})
	} else if sortBy == "views_6h" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Views6h.Percentile50 > res.Strategies[j].Views6h.Percentile50
		})
	} else if sortBy == "views_1h" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Views1h.Percentile50 > res.Strategies[j].Views1h.Percentile50
		})
	} else if sortBy == "views_30m" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Views30m.Percentile50 > res.Strategies[j].Views30m.Percentile50
		})
	} else if sortBy == "retention_3" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Retention3.Percentile50 > res.Strategies[j].Retention3.Percentile50
		})
	} else if sortBy == "retention_5" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Retention5.Percentile50 > res.Strategies[j].Retention5.Percentile50
		})
	} else if sortBy == "retention_10" {
		sort.Slice(res.Strategies, func(i, j int) bool {
			return res.Strategies[i].Retention10.Percentile50 > res.Strategies[j].Retention10.Percentile50
		})
	}

	res.QueryAccount = accountId
	res.QuerySponsor = sponsorId
	res.QueryAfter = dateAfter
	res.QueryBefore = dateBefore
	res.QuerySortBy = sortBy
	res.QueryVariable = onlyVariable
	res.Accounts = accounts
	res.Sponsors = sponsors
	res.ControlOptions = controlOptions
	tmpl := template.Must(template.New("routes/views/strategies.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/strategies.gohtml", "routes/views/global_layout.gohtml"))

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", res))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func VideoDetails(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil {
		http.Redirect(w, r, "/admin/login", 302)
		return
	}
	if !(account.Type == "admin" || account.Type == "va") {
		http.Redirect(w, r, "/admin/login", 302)
		return
	}
	var needsEntry bool
	var videoUrl string
	//var videoId int
	var sponsorId sql.NullInt64
	var writerId sql.NullInt64
	var voiceId sql.NullInt64
	var audioId sql.NullInt64
	var sourceId sql.NullInt64
	var sponsorRate sql.NullInt64
	var writerRate sql.NullInt64
	var coWriterId sql.NullInt64
	var coWriterRate sql.NullInt64
	var storyLink sql.NullString
	var storyCode sql.NullString
	var scrapedScript sql.NullString
	var viewCount int64
	var likeCount int64
	var commentCount int64
	var shareCount int64
	var saveCount int64

	videoId, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	row := db.QueryRow(r.Context(), "SELECT url, sponsor_id, writer_id, voice_id, audio_id, source_id, sponsor_rate, writer_rate, co_writer_id, co_writer_rate, needs_entry, story_link, story_code, convert_from(script_text, 'UTF8') from video where id = $1", videoId)
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	err = row.Scan(&videoUrl, &sponsorId, &writerId, &voiceId, &audioId, &sourceId, &sponsorRate, &writerRate, &coWriterId, &coWriterRate, &needsEntry, &storyLink, &storyCode, &scrapedScript)
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	row = db.QueryRow(r.Context(), "SELECT views_total, likes_total, comments_total, shares_total, saves_total from statistics where video_id = $1", videoId)
	err = row.Scan(&viewCount, &likeCount, &commentCount, &shareCount, &saveCount)
	if err != nil {
		fmt.Println("failed to get stats for video ID", videoId)
	}

	var sponsors []models.Sponsor

	res, err := db.Query(r.Context(), "SELECT id, name FROM sponsor")
	defer res.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for res.Next() {
		var sponsor models.Sponsor
		err = res.Scan(&sponsor.Id, &sponsor.Name)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		sponsors = append(sponsors, sponsor)
	}

	var audios []models.Audio

	res, err = db.Query(r.Context(), "SELECT id, name FROM audio")
	defer res.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for res.Next() {
		var audio models.Audio
		err = res.Scan(&audio.Id, &audio.Name)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		audios = append(audios, audio)
	}

	var voices []models.Voice

	res, err = db.Query(r.Context(), "SELECT id, name FROM voice")
	defer res.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for res.Next() {
		var voice models.Voice
		err = res.Scan(&voice.Id, &voice.Name)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		voices = append(voices, voice)
	}

	var writers []models.Writer

	res, err = db.Query(r.Context(), "SELECT id, name FROM writer")
	defer res.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for res.Next() {
		var writer models.Writer
		err = res.Scan(&writer.Id, &writer.Name)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		writers = append(writers, writer)
	}

	var sources []models.Source

	res, err = db.Query(r.Context(), "SELECT id, name FROM source")
	defer res.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for res.Next() {
		var voice models.Source
		err = res.Scan(&voice.Id, &voice.Name)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		sources = append(sources, voice)
	}

	strategyOptions := make(map[string][]models.Strategy)

	res, err = db.Query(r.Context(), "SELECT id, name, COALESCE(variable,''), COALESCE(value,'') FROM strategy order by variable ")
	defer res.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for res.Next() {
		var strategy models.Strategy
		err = res.Scan(&strategy.Id, &strategy.Name, &strategy.Variable, &strategy.Value)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		strategyOptions[strategy.Variable] = append(strategyOptions[strategy.Variable], strategy)
	}

	strategies := make(models.StrategiesArray, 0)

	res, err = db.Query(r.Context(), "SELECT s.id, s.name, COALESCE(s.variable,''), COALESCE(s.value,'') FROM video_strategies join strategy s on s.id = video_strategies.strategy_id where video_strategies.video_id = $1 order by variable", videoId)
	defer res.Close()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	for res.Next() {
		var strategy models.Strategy
		err = res.Scan(&strategy.Id, &strategy.Name, &strategy.Variable, &strategy.Value)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		strategies = append(strategies, strategy)
	}

	var resp = models.VideoEntryPage{
		NeedsEntry:         needsEntry,
		VarStrategyOptions: strategyOptions,
		PresetUrl:          videoUrl,
		PresetId:           videoId,
		PresetStoryLink:    storyLink.String,
		PresetStoryCode:    storyCode.String,
		PresetSponsorId:    int(sponsorId.Int64),
		PresetWriterId:     int(writerId.Int64),
		PresetVoiceId:      int(voiceId.Int64),
		PresetAudioId:      int(audioId.Int64),
		PresetSourceId:     int(sourceId.Int64),
		PresetSponsorRate:  int(sponsorRate.Int64),
		PresetWriterRate:   int(writerRate.Int64),
		PresetCoWriterId:   int(coWriterId.Int64),
		PresetCoWriterRate: int(coWriterRate.Int64),
		PresetStrategies:   strategies,
		Sponsors:           sponsors,
		Audios:             audios,
		Voices:             voices,
		Writers:            writers,
		Sources:            sources,
		ShowMetrics:        account.Type == "admin",
		ScrapedScript:      scrapedScript.String,
		ViewCount:          viewCount,
		LikeCount:          likeCount,
		CommentCount:       commentCount,
		ShareCount:         shareCount,
		SaveCount:          saveCount,
	}
	tmpl := template.Must(template.New("video_details.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/video_details.gohtml", "routes/views/global_layout.gohtml"))
	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", resp))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func UpdateVideoDetails(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {

	account, err := GetLogin(r, db)
	if err != nil || !(account.Type == "admin" || account.Type == "va") {
		//ErrorBack(w, r, err.Error(), 401)
		http.Redirect(w, r, "/va/login", 302)
		return
	}

	err = r.ParseForm()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	videoId, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	url := r.FormValue("url")
	u, _ := url2.Parse(url)
	u.RawQuery = ""
	u.Fragment = ""
	url = u.String()

	storyUrl := r.FormValue("story_link")
	u2, _ := url2.Parse(storyUrl)
	u2.RawQuery = ""
	u2.Fragment = ""
	storyUrl = u2.String()

	strategies := r.Form["strategy"]

	//vidInfo := VideoInfo{
	//	Platform: vidInfoBasic.Platform,
	//	Username: vidInfoBasic.Username,
	//	Url:      r.FormValue("url"),
	//	Id:       0,
	//}

	voiceId, err := strconv.Atoi(r.FormValue("voice_id"))
	if err != nil {
		// create new voice if needed
		res := db.QueryRow(r.Context(), "INSERT into voice (name) values ($1) ON CONFLICT ON CONSTRAINT voice_pk2  DO UPDATE SET name = voice.name RETURNING id;", r.FormValue("voice_custom"))

		err2 := res.Scan(&voiceId)
		if err2 != nil {
			ErrorBack(w, r, err2.Error(), "")
			return
		}
	}
	audioId, err := strconv.Atoi(r.FormValue("audio_id"))
	if err != nil {

		// create new audio if needed
		res := db.QueryRow(r.Context(), "INSERT into audio (name) values ($1) ON CONFLICT ON CONSTRAINT audio_pk2  DO UPDATE SET name = audio.name RETURNING id;", r.FormValue("audio_custom"))

		err2 := res.Scan(&audioId)
		if err2 != nil {
			ErrorBack(w, r, err2.Error(), "")
			return
		}
	}
	sourceId, err := strconv.Atoi(r.FormValue("source_id"))
	if err != nil {
		// create new source if needed
		res := db.QueryRow(r.Context(), "INSERT into source (name) values ($1) ON CONFLICT ON CONSTRAINT source_pk2  DO UPDATE SET name = source.name RETURNING id;", r.FormValue("source_custom"))

		err2 := res.Scan(&sourceId)
		if err2 != nil {
			ErrorBack(w, r, err2.Error(), "")
			return
		}
	}

	writerId, err := strconv.Atoi(r.FormValue("writer_id"))
	writerRate, err := strconv.Atoi(r.FormValue("writer_rate"))
	//writerRate, err := strconv.Atoi(r.FormValue("writer_rate"))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	coWriterId, coWriterIdErr := strconv.Atoi(r.FormValue("co_writer_id"))
	coWriterIdDb := sql.NullInt64{Int64: int64(coWriterId), Valid: true}
	coWriterRate, coWriterRateErr := strconv.Atoi(r.FormValue("co_writer_rate"))
	coWriterRateDb := sql.NullInt64{Int64: int64(coWriterRate), Valid: true}
	//writerRate, err := strconv.Atoi(r.FormValue("writer_rate"))
	if coWriterIdErr != nil || coWriterRateErr != nil {
		coWriterIdDb = sql.NullInt64{Int64: 0, Valid: false}
		coWriterRateDb = sql.NullInt64{Int64: 0, Valid: false}
	}

	sponsorId, sponsorIdErr := strconv.Atoi(r.FormValue("sponsor_id"))
	sponsorIdDb := sql.NullInt64{Int64: int64(sponsorId), Valid: true}
	sponsorRate, sponsorRateErr := strconv.Atoi(r.FormValue("sponsor_rate"))
	sponsorRateDb := sql.NullInt64{Int64: int64(sponsorRate), Valid: true}
	if sponsorIdErr != nil || sponsorRateErr != nil {
		sponsorIdDb = sql.NullInt64{Int64: 0, Valid: false}
		sponsorRateDb = sql.NullInt64{Int64: 0, Valid: false}
	}

	_, err = db.Exec(r.Context(), `UPDATE video 
		SET (entered_by, created, sponsor_id, sponsor_rate, writer_id, writer_rate, url, voice_id, audio_id, source_id, co_writer_id, co_writer_rate, story_code, story_link, needs_entry)
	= ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	where id = $16`,
		nil,
		time.Now(),
		sponsorIdDb,
		sponsorRateDb,
		writerId,
		writerRate,
		url,
		voiceId,
		audioId,
		sourceId,
		coWriterIdDb,
		coWriterRateDb,
		r.FormValue("story_code"),
		storyUrl,
		false,
		videoId)

	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	_, err = db.Exec(r.Context(), `DELETE from video_strategies where video_id = $1`, videoId)
	if err != nil {
		ErrorBack(w, r, "[Failed] while deleting existing strategies "+err.Error(), "")
		return
	}

	if len(strategies) > 0 {
		// add strategies
		sqlStr := "INSERT INTO video_strategies(video_id, strategy_id) VALUES "
		vals := make([]any, 0)

		for i, sName := range strategies {

			varName := sql.NullString{Valid: false}
			valName := sql.NullString{Valid: false}

			parts := strings.Split(sName, ":")
			if len(parts) == 2 {
				varName = sql.NullString{String: parts[0], Valid: true}
				valName = sql.NullString{String: parts[1], Valid: true}
			}

			var id int
			res := db.QueryRow(r.Context(), "INSERT into strategy (name, variable, value) values ($1, $2, $3) ON CONFLICT ON CONSTRAINT strategy_pk2  DO UPDATE SET name = strategy.name RETURNING id;", sName, varName, valName)

			err := res.Scan(&id)
			if err != nil {
				ErrorBack(w, r, err.Error(), "")
				return
			}
			sqlStr += fmt.Sprintf("($%d, $%d),", 2*i+1, 2*i+2)
			vals = append(vals, videoId, id)
		}

		//trim the last ,
		sqlStr = strings.TrimSuffix(sqlStr, ",")
		sqlStr += " ON CONFLICT DO NOTHING"
		//prepare the statement
		_, err := db.Exec(r.Context(), sqlStr, vals...)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		// TODO: close!
	}

	_, err = db.Exec(r.Context(), `UPDATE va SET vids_entered = vids_entered + 1 WHERE id = $1`,
		account.Id)
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	_, err = db.Exec(r.Context(), `UPDATE sponsor SET video_count = sponsor.video_count + 1 WHERE id = $1`,
		sponsorId)
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}

	// issue an http request to the endpoint in path variable "WORKER"
	//url := os.Getenv("WORKER")
	//req, err := http.NewRequest("GET", url, nil)
	//_, err = client.Do(req)
	//resp.Body.Close()

	SuccessBack(w, r, "Video updated", "")

}
