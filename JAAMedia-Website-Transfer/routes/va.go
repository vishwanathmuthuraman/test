package routes

import (
	"bytes"
	"clicktrack/graphs"
	"clicktrack/models"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"html/template"
	"io"
	"net/http"
	url2 "net/url"
	"os"
	"strconv"
	"time"
)

func AddVideo(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || !(account.Type == "va" || account.Type == "admin") {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/va/login", 302)
		return
	}

	tmpl := template.Must(template.New("va_addvideo.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/populate_video.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var videosNeedEntry = make([]models.VideoListItem, 0)
	var videosAlreadyEntered = make([]models.VideoListItem, 0)

	res, err := db.Query(r.Context(), "SELECT video.id, url, a.username, posted_date, needs_entry FROM video join account a on a.id = video.account_id where needs_entry = true order by created desc")
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	for res.Next() {
		var v models.VideoListItem
		err = res.Scan(&v.Id, &v.Url, &v.Account.Username, &v.PostedDate, &v.NeedsEntry)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		videosNeedEntry = append(videosNeedEntry, v)
	}

	res, err = db.Query(r.Context(), "SELECT id, url FROM video where needs_entry = false order by created desc limit 100")
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	for res.Next() {
		var v models.VideoListItem
		err = res.Scan(&v.Id, &v.Url)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		videosNeedEntry = append(videosNeedEntry, v)
	}

	data := models.VaAddVideoViewPageData{
		VideosNeedEntry:      videosNeedEntry,
		VideosAlreadyEntered: videosAlreadyEntered,
	}
	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", data))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

//func AddVideoPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
//
//	account, err := GetLogin(r, db)
//	if err != nil || account.Type != "va" {
//		//ErrorBack(w, r, err.Error(), 401)
//		http.Redirect(w, r, "/va/login", 302)
//		return
//	}
//
//	err = r.ParseForm()
//	if err != nil {
//		ErrorBack(w, r, err.Error(), "")
//		return
//	}
//
//	url := r.FormValue("url")
//	u, _ := url2.Parse(url)
//	u.RawQuery = ""
//	u.Fragment = ""
//	url = u.String()
//
//	scraped, err, _ := tiktok.GetTiktokDetails(url)
//	if err != nil {
//		fmt.Println("Trying again, url:", url)
//		var err2 error
//		scraped, err2, _ = tiktok.GetTiktokDetails(url)
//		if err2 != nil {
//			ErrorBack(w, r, "While scraping tiktok for title, got error: "+err.Error(), "")
//			return
//		}
//	}
//
//	storyUrl := r.FormValue("story_link")
//	u2, _ := url2.Parse(storyUrl)
//	u2.RawQuery = ""
//	u2.Fragment = ""
//	storyUrl = u2.String()
//
//	strategies := r.Form["strategy"]
//	//vidInfoBasic := ProcessUrl(r.FormValue("url"))
//
//	//vidInfo := VideoInfo{
//	//	Platform: vidInfoBasic.Platform,
//	//	Username: vidInfoBasic.Username,
//	//	Url:      r.FormValue("url"),
//	//	Id:       0,
//	//}
//
//	voiceId, err := strconv.Atoi(r.FormValue("voice_id"))
//	if err != nil {
//		// create new voice if needed
//		res := db.QueryRow(r.Context(), "INSERT into voice (name) values ($1) ON CONFLICT ON CONSTRAINT voice_pk2  DO UPDATE SET name = voice.name RETURNING id;", r.FormValue("voice_custom"))
//		if res.Err() != nil {
//			ErrorBack(w, r, res.Err().Error(), "")
//			return
//		}
//		res.Scan(&voiceId)
//	}
//	audioId, err := strconv.Atoi(r.FormValue("audio_id"))
//	if err != nil {
//
//		// create new audio if needed
//		res := db.QueryRow(r.Context(), "INSERT into audio (name) values ($1) ON CONFLICT ON CONSTRAINT audio_pk2  DO UPDATE SET name = audio.name RETURNING id;", r.FormValue("audio_custom"))
//		if res.Err() != nil {
//			ErrorBack(w, r, res.Err().Error(), "")
//			return
//		}
//		res.Scan(&audioId)
//	}
//	sourceId, err := strconv.Atoi(r.FormValue("source_id"))
//	if err != nil {
//		// create new source if needed
//		res := db.QueryRow(r.Context(), "INSERT into source (name) values ($1) ON CONFLICT ON CONSTRAINT source_pk2  DO UPDATE SET name = source.name RETURNING id;", r.FormValue("source_custom"))
//		if res.Err() != nil {
//			ErrorBack(w, r, res.Err().Error(), "")
//			return
//		}
//		res.Scan(&sourceId)
//	}
//
//	// get account ID
//	var accountId = 1
//
//	res := db.QueryRow(r.Context(), "INSERT into account (platform, username) values ($1, $2) ON CONFLICT ON CONSTRAINT account_pk DO UPDATE SET platform = account.platform, username = account.username RETURNING id;", "tiktok", scraped.Username)
//	if res.Err() != nil {
//		ErrorBack(w, r, res.Err().Error(), "")
//		return
//	}
//	err = res.Scan(&accountId)
//	if err != nil {
//		if err == sql.ErrNoRows {
//			ErrorBack(w, r, "No matching account was found", "")
//			return
//		}
//		ErrorBack(w, r, err.Error(), "")
//		return
//	}
//
//	writerId, err := strconv.Atoi(r.FormValue("writer_id"))
//	writerRate, err := strconv.Atoi(r.FormValue("writer_rate"))
//	//writerRate, err := strconv.Atoi(r.FormValue("writer_rate"))
//	if err != nil {
//		ErrorBack(w, r, err.Error(), "")
//		return
//	}
//
//	coWriterId, err := strconv.Atoi(r.FormValue("co_writer_id"))
//	coWriterIdDb := sql.NullInt64{Int64: int64(coWriterId), Valid: true}
//	coWriterRate, err := strconv.Atoi(r.FormValue("co_writer_rate"))
//	coWriterRateDb := sql.NullInt64{Int64: int64(coWriterRate), Valid: true}
//	//writerRate, err := strconv.Atoi(r.FormValue("writer_rate"))
//	if err != nil {
//		coWriterIdDb = sql.NullInt64{Int64: 0, Valid: false}
//		coWriterRateDb = sql.NullInt64{Int64: 0, Valid: false}
//	}
//
//	sponsorId, err := strconv.Atoi(r.FormValue("sponsor_id"))
//	sponsorIdDb := sql.NullInt64{Int64: int64(sponsorId), Valid: true}
//	sponsorRate, err := strconv.Atoi(r.FormValue("sponsor_rate"))
//	sponsorRateDb := sql.NullInt64{Int64: int64(sponsorRate), Valid: true}
//	if err != nil {
//		sponsorIdDb = sql.NullInt64{Int64: 0, Valid: false}
//		sponsorRateDb = sql.NullInt64{Int64: 0, Valid: false}
//	}
//
//	res = db.QueryRow(`INSERT INTO video
//	   (entered_by, created, sponsor_id, sponsor_rate, writer_id, writer_rate, account_id, url, voice_id, audio_id, source_id, co_writer_id, co_writer_rate, title, story_code, story_link, duration, posted_date, needs_entry)
//	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
//	ON CONFLICT (url) DO UPDATE
//		SET (entered_by, created, sponsor_id, sponsor_rate, writer_id, writer_rate, account_id, url, voice_id, audio_id, source_id, co_writer_id, co_writer_rate, title, story_code, story_link, duration, posted_date, needs_entry)
//	= ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
//	RETURNING id`,
//		account.Id,
//		time.Now(),
//		sponsorIdDb,
//		sponsorRateDb,
//		writerId,
//		writerRate,
//		accountId,
//		url,
//		voiceId,
//		audioId,
//		sourceId,
//		coWriterIdDb,
//		coWriterRateDb,
//		scraped.Caption,
//		r.FormValue("story_code"),
//		storyUrl,
//		scraped.Length,
//		scraped.PostedDate,
//		false)
//
//	if res.Err() != nil {
//		switch res.Err().(type) {
//		case *pq.Error:
//			{
//				if res.Err().(*pq.Error).Code == "23505" {
//					ErrorBack(w, r, "Video already entered", "")
//					return
//				}
//				if res.Err().(*pq.Error).Code == "23503" {
//					ErrorBack(w, r, "For content source, audio, and voice: select an existing number from the dropdown, OR manually enter a new NAME ", "")
//					return
//				}
//			}
//		}
//
//		ErrorBack(w, r, res.Err().Error(), "")
//		return
//	}
//
//	var insertedVideoId int
//	err = res.Scan(&insertedVideoId)
//	if err != nil {
//		ErrorBack(w, r, err.Error(), "")
//		return
//	}
//
//	scripts.RewritePoints(url, strconv.Itoa(sponsorId))
//
//	if len(strategies) > 0 {
//		// add strategies
//		sqlStr := "INSERT INTO video_strategies(video_id, strategy_id) VALUES "
//		vals := make([]any, 0)
//
//		for i, row := range strategies {
//			id, err := strconv.Atoi(row)
//			if err != nil {
//				varName := sql.NullString{Valid: false}
//				valName := sql.NullString{Valid: false}
//
//				parts := strings.Split(row, ":")
//				if len(parts) == 2 {
//					varName = sql.NullString{String: parts[0], Valid: true}
//					valName = sql.NullString{String: parts[1], Valid: true}
//				}
//				// create new source if needed
//
//				res := db.QueryRow(r.Context(), "INSERT into strategy (name, variable, value) values ($1, $2, $3) ON CONFLICT ON CONSTRAINT strategy_pk2  DO UPDATE SET name = strategy.name RETURNING id;", row, varName, valName)
//				if res.Err() != nil {
//					ErrorBack(w, r, res.Err().Error(), "")
//					return
//				}
//				err := res.Scan(&id)
//				if err != nil {
//					ErrorBack(w, r, err.Error(), "")
//					return
//				}
//			}
//			sqlStr += fmt.Sprintf("($%d, $%d),", 2*i+1, 2*i+2)
//			vals = append(vals, insertedVideoId, id)
//		}
//
//		//trim the last ,
//		sqlStr = strings.TrimSuffix(sqlStr, ",")
//
//		//prepare the statement
//		stmt, _ := db.Prepare(sqlStr)
//
//		//format all vals at once
//		_, err = stmt.Exec(vals...)
//		if err != nil {
//			ErrorBack(w, r, err.Error(), "")
//			return
//		}
//	}
//
//	res = db.QueryRow(`UPDATE va SET vids_entered = vids_entered + 1 WHERE id = $1`,
//		account.Id)
//
//	if res.Err() != nil {
//		ErrorBack(w, r, res.Err().Error(), "")
//		return
//	}
//
//	res = db.QueryRow(`UPDATE sponsor SET video_count = sponsor.video_count + 1 WHERE id = $1`,
//		sponsorId)
//
//	if res.Err() != nil {
//		ErrorBack(w, r, res.Err().Error(), "")
//		return
//	}
//	// issue an http request to the endpoint in path variable "WORKER"
//	//url := os.Getenv("WORKER")
//	//req, err := http.NewRequest("GET", url, nil)
//	//_, err = client.Do(req)
//	//resp.Body.Close()
//
//	SuccessBack(w, r, "Video added", "/va/add_video")
//}

func VaRetention(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {

	account, err := GetLogin(r, db)
	if err != nil || !(account.Type == "va" || account.Type == "admin") {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/va/login", 302)
		return
	}
	err = r.ParseForm()
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	_, sponsoredOnly := r.Form["sponsored"]

	rows, err := db.Query(r.Context(), "SELECT id, username FROM account")
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
	var accounts []models.Account
	for rows.Next() {
		var account models.Account
		err = rows.Scan(&account.Id, &account.Username)
		if err != nil {
			ErrorBack(w, r, err.Error(), "")
			return
		}
		accounts = append(accounts, account)
	}

	tmpl := template.Must(template.New("va_retention_rate.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/va_retention.gohtml", "routes/views/global_layout.gohtml"))
	//if err != nil {
	//	panic(err)
	//}
	var pageData struct {
		Accounts         []models.Account
		Videos           []models.VideoListItem
		DatePreset       string
		SponsoredPreset  bool
		AccountPreset    int
		ManualLinkPreset string
	}
	pageData.Accounts = accounts

	if r.FormValue("manual_link") != "" {
		// use the manual link entry
		url := r.FormValue("manual_link")
		u, _ := url2.Parse(url)
		u.RawQuery = ""
		u.Fragment = ""
		url = u.String()
		// use the manual link instead
		row := db.QueryRow(r.Context(), "SELECT video.id, created, account_id, a.username, a.platform, url from video join account a on video.account_id = a.id where url = $1", url)
		var video models.VideoListItem

		err := row.Scan(&video.Id, &video.Created, &video.Account.Id, &video.Account.Username, &video.Account.Platform, &video.Url)
		if err != nil {
			ErrorBack(w, r, "That video is not in the system. Try filtering instead.", "/va/retention")
			return
		}
		pageData.Videos = []models.VideoListItem{video}
		pageData.ManualLinkPreset = url
	} else {
		var videos = make([]models.VideoListItem, 0)
		dateStr := r.URL.Query().Get("date")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			err = nil
		}
		accountId := r.URL.Query().Get("account_id")
		if accountId == "" {
			err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", pageData))
			if err != nil {
				ErrorBack(w, r, err.Error(), "/va/retention")
				return
			}
			return
		}
		accountIdParsed, err := strconv.Atoi(accountId)
		if err != nil {
			ErrorBack(w, r, "Failed to parse account ID: "+err.Error(), "/va/retention")
			return
		}

		query := "SELECT video.id, video.created, account.id, account.username, account.platform, url, COALESCE(video.preview, ''), posted_date from statistics JOIN video on statistics.video_id = video.id JOIN account ON video.account_id = account.id where (retention_entered_by IS NULL) and (posted_date = $1) and (account_id = $2) and duration is not null"

		// include all days if necessary
		if dateStr == "" {
			query = "SELECT video.id, created, account.id, account.username, account.platform, url, COALESCE(video.preview, ''), posted_date from video JOIN account ON account_id = account.id where (retention_entered_by IS NULL) and (posted_date > $1) and (account_id = $2) and duration is not null "
		}

		if sponsoredOnly {
			query += " and (sponsor_id is not null or needs_entry = true)"
		}
		query += " order by created desc limit 100"
		res, err := db.Query(r.Context(), query, date, accountIdParsed)
		if err != nil {
			ErrorBack(w, r, "Error querying videos", "/va/retention")
			return
		}
		for res.Next() {
			var video models.VideoListItem

			err := res.Scan(&video.Id, &video.Created, &video.Account.Id, &video.Account.Username, &video.Account.Platform, &video.Url, &video.Preview, &video.PostedDate)
			if err != nil {
				ErrorBack(w, r, "Error scanning videos. Try filtering instead.", "/va/retention")

				return
			}
			//video.Graph = fmt.Sprintf("%s&from=now-6h&to=now&var-url=%s", os.Getenv("GRAFANA_HOST"), url2.QueryEscape(video.Url))
			videos = append(videos, video)
		}
		pageData.Videos = videos
		pageData.DatePreset = dateStr
		pageData.SponsoredPreset = sponsoredOnly
		pageData.AccountPreset = accountIdParsed
	}

	err = tmpl.ExecuteTemplate(w, "base", WrapGlobal(account, "", "", pageData))
	if err != nil {
		ErrorBack(w, r, err.Error(), "")
		return
	}
}

func VaRetentionPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	account, err := GetLogin(r, db)
	if err != nil || !(account.Type == "va" || account.Type == "admin") {
		//ErrorBack(w, r, err.Error(), 401)

		http.Redirect(w, r, "/va/login", 302)
		return
	}

	err = r.ParseMultipartForm(5 << 20) // 5 MB limit for uploaded files
	if err != nil {
		ErrorBack(w, r, "Error parsing form data", "")
		return
	}

	videoID := r.FormValue("video_id")
	videoIdParsed, err := strconv.Atoi(videoID)
	if err != nil {
		ErrorBack(w, r, "Error parsing video ID / Did you select a video?", "")
		return
	}

	//minutes, err := strconv.Atoi(r.FormValue("minutes"))
	//seconds, err := strconv.Atoi(r.FormValue("seconds"))

	imageFile, _, err := r.FormFile("image")
	if err != nil {
		ErrorBack(w, r, "Error retrieving image", "")
		return
	}
	defer imageFile.Close()

	// Create a temporary file to save the uploaded image
	tempImageFile, err := os.CreateTemp("", "temp_image_*.png")
	if err != nil {
		ErrorBack(w, r, "Error creating temporary image file", "")
		return
	}
	defer os.Remove(tempImageFile.Name())
	defer tempImageFile.Close()

	// Save the uploaded image to the temporary file
	_, err = io.Copy(tempImageFile, imageFile)
	if err != nil {
		ErrorBack(w, r, "Error saving image", "")
		return
	}

	row := db.QueryRow(r.Context(), "SELECT duration from video where id = $1", videoIdParsed)
	durationS := 0
	err = row.Scan(&durationS)
	if err != nil {
		ErrorBack(w, r, "Video duration not available", "")
		return
	}

	// Decode the line graph values from the uploaded image
	values, err := graphs.DecodeLineGraphValues(tempImageFile.Name(), durationS)
	if err != nil {
		ErrorBack(w, r, "[PNG only!] "+err.Error(), "")
		return
	}

	stmt, err := db.Prepare(r.Context(), "insert_retention", "INSERT INTO retention_rate (video_id, instant_s, value) VALUES ($1, $2, $3) ON CONFLICT (video_id, instant_s) DO UPDATE SET value = $3")
	if err != nil {
		ErrorBack(w, r, "Error preparing SQL statement", "")
		return
	}

	tx, err := db.Begin(r.Context())
	if err != nil {
		ErrorBack(w, r, "Error starting transaction", "")
		return
	}

	for second, value := range values {
		_, err = tx.Exec(r.Context(), stmt.Name, videoIdParsed, second, value)
		fmt.Println(second)
		if err != nil {
			tx.Rollback(r.Context())
			ErrorBack(w, r, "Error inserting data into database", "")
			return
		}
	}

	// Update the video's retention_entered to TRUE
	_, err = tx.Exec(r.Context(), "UPDATE video SET retention_entered_by = $1 WHERE id = $2", account.Id, videoIdParsed)
	if err != nil {
		tx.Rollback(r.Context()) // Rollback if the update fails
		ErrorBack(w, r, err.Error(), "")
		return
	}

	err = tx.Commit(r.Context())
	if err != nil {
		ErrorBack(w, r, "Error committing transaction", "")
		return
	}

	SuccessBack(w, r, "Retention graph uploaded", "/va/retention")
}

func VaLogin(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	tmpl := template.Must(template.New("va_login.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/va_login.gohtml", "routes/views/global_layout.gohtml"))

	err := tmpl.ExecuteTemplate(w, "base", WrapGlobal(nil, "", "", struct{ Redirect string }{r.FormValue("redirect")}))
	if err != nil {
		w.Write([]byte(err.Error()))
	}
}

func VaLoginPost(w http.ResponseWriter, r *http.Request, db *pgx.Conn, templateFuncs template.FuncMap) {
	var login Login
	pwHash := fmt.Sprintf("%x", sha256.Sum256([]byte(r.FormValue("pw"))))
	row := db.QueryRow(r.Context(), "SELECT email, password_hash from va where email = $1 and password_hash = $2", r.FormValue("email"), pwHash)

	err := row.Scan(&login.Username, &login.Hash)
	if err != nil {
		if err == sql.ErrNoRows {
			ErrorBack(w, r, "Bad username or password, please contact an admin if you have forgotten your password", "")
			return
		}
		ErrorBack(w, r, err.Error(), "")
		return
	}
	login.Type = "va"
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
		SuccessBack(w, r, "Logged in", "/admin/videos")
	} else {
		SuccessBack(w, r, "Logged in", r.FormValue("redirect"))
	}
	return
}
