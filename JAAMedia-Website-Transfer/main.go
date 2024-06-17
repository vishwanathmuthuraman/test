package main

import (
	//"bytes"
	//"clicktrack/graphs"
	"clicktrack/models"
	routes "clicktrack/routes"
	"context"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	//"crypto/sha256"
	"database/sql"
	//"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	_ "github.com/jackc/pgx/v5/stdlib"
	"html/template"
	"log"
	"net/http"
	url2 "net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	//token := os.Getenv("INFLUX")
	//url := os.Getenv("INFLUX_URL")
	//influxClient := influxdb2.NewClient(url, token)
	pqStr := os.Getenv("PG_STRING") + " pool_max_conns=50"
	// open database
	//db, err := sql.Open("pgx", pqStr)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//// close database at the end of program
	//defer db.Close()

	db, err := pgxpool.New(context.Background(), pqStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// check db
	//err = db.Ping()
	//if err != nil {
	//	log.Fatal(err)
	//}
	//var client = http.Client{
	//	Transport: &http.Transport{
	//		DialContext: (&net.Dialer{
	//			Timeout: 2 * time.Second,
	//		}).DialContext,
	//	},
	//}
	templateFuncs := template.FuncMap{
		"comma": func(x int) string {
			if x < 10000 {
				return humanize.Comma(int64(x))
			} else if x < 1000000 {
				return humanize.Comma(int64(x/1000)) + "K"
			} else {
				return humanize.Comma(int64(x/1000000)) + "M"
			}
		},
		"multiply": func(x int, y int) int {
			return x * y
		},
		"format": models.FormattedWithSuffix,
	}

	grafanaProxy := ProxyHandler(db)
	if err != nil {
		log.Fatal("while setting up grafana proxy:", err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.New("index.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/index.gohtml"))

		redirect := r.URL.Query().Get("redirect")
		err = tmpl.ExecuteTemplate(w, "base", routes.WrapGlobal(nil, "", "", struct{ Redirect string }{redirect}))
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	http.HandleFunc("/grafana/", grafanaProxy.ServeHTTP)
	http.HandleFunc("/shopify", func(w http.ResponseWriter, r *http.Request) {
		conn, err := db.Acquire(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		GetOrderCounts(w, r, conn.Conn())
	})
	http.HandleFunc("/logo", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/JaaMediaLogo-1.png")
	})
	http.HandleFunc("/retention-demo", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/retention_demo.png")
	})
	http.HandleFunc("/stylesheet", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		http.ServeFile(w, r, "static/style.css")
	})
	//http.HandleFunc("/csv.js", func(w http.ResponseWriter, r *http.Request) {
	//	w.Header().Set("Content-Type", "application/javascript")
	//	http.ServeFile(w, r, "static/csv.js")
	//})
	http.HandleFunc("/csv_sync.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "static/csv_sync.js")
	})

	http.HandleFunc("/wheel.gif", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		http.ServeFile(w, r, "static/wheel.gif")
	})
	http.HandleFunc("/preview.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "script/javascript")
		http.ServeFile(w, r, "static/preview.js")
	})

	http.HandleFunc("/sheets_plugin", func(w http.ResponseWriter, r *http.Request) {
		var results models.SheetsPluginData
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// sponsors

		res, err := db.Query(r.Context(), "SELECT id, name from sponsor")
		if err != nil {
			http.Error(w, "error getting sponsors", 500)
			return
		}
		for res.Next() {
			var sponsor models.SheetsPluginDataItem

			err := res.Scan(&sponsor.Id, &sponsor.Name)
			if err != nil {
				http.Error(w, "error getting sponsors", 500)
				return
			}
			results.Sponsors = append(results.Sponsors, sponsor)
		}

		// audios
		res, err = db.Query(r.Context(), "SELECT id, name from audio")
		if err != nil {
			http.Error(w, "error getting audios", 500)
			return
		}
		for res.Next() {
			var sponsor models.SheetsPluginDataItem

			err := res.Scan(&sponsor.Id, &sponsor.Name)
			if err != nil {
				http.Error(w, "error getting audios", 500)
				return
			}
			results.Audios = append(results.Audios, sponsor)
		}

		// voices
		res, err = db.Query(r.Context(), "SELECT id, name from voice")
		if err != nil {
			http.Error(w, "error getting voice", 500)
			return
		}
		for res.Next() {
			var sponsor models.SheetsPluginDataItem

			err := res.Scan(&sponsor.Id, &sponsor.Name)
			if err != nil {
				http.Error(w, "error getting voice", 500)
				return
			}
			results.Voices = append(results.Voices, sponsor)
		}

		// writers
		res, err = db.Query(r.Context(), "SELECT id, name from writer")
		if err != nil {
			http.Error(w, "error getting writers", 500)
			return
		}
		for res.Next() {
			var sponsor models.SheetsPluginDataItem

			err := res.Scan(&sponsor.Id, &sponsor.Name)
			if err != nil {
				http.Error(w, "error getting writers", 500)
				return
			}
			results.Writers = append(results.Writers, sponsor)
		}

		// sources
		res, err = db.Query(r.Context(), "SELECT id, name from source")
		if err != nil {
			http.Error(w, "error getting sources", 500)
			return
		}
		for res.Next() {
			var sponsor models.SheetsPluginDataItem

			err := res.Scan(&sponsor.Id, &sponsor.Name)
			if err != nil {
				http.Error(w, "error getting sources", 500)
				return
			}
			results.Sources = append(results.Sources, sponsor)
		}

		json.NewEncoder(w).Encode(results)
	})
	http.HandleFunc("/upload_csv", func(w http.ResponseWriter, r *http.Request) {
		conn, err := db.Acquire(r.Context())
		user, err := routes.GetLogin(r, conn.Conn())
		if err != nil || !(user.Type == "admin" || user.Type == "va") {
			http.Redirect(w, r, "/admin/login?redirect=/upload_csv", 302)
			return
		}

		tmpl := template.Must(template.New("upload_csv.gohtml").Funcs(templateFuncs).ParseFiles("routes/views/upload_csv.gohtml", "routes/views/global_layout.gohtml"))
		err = tmpl.ExecuteTemplate(w, "base", routes.WrapGlobal(user, "", "", ""))
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	http.HandleFunc("/admin/post_csv", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		conn, err := db.Acquire(ctx)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		user, err := routes.GetLogin(r, conn.Conn())
		if err != nil || !(user.Type == "admin" || user.Type == "va") {
			w.WriteHeader(401)
			return
		}

		upgrader := websocket.Upgrader{} // use default options
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}

		ws.SetCloseHandler(func(code int, text string) error {
			fmt.Println("Closing websocket")
			cancel()
			return nil
		})

		_, file, err := ws.NextReader()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Create a new reader.
		reader := csv.NewReader(file)

		records, err := reader.ReadAll()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		ws.WriteMessage(websocket.TextMessage, []byte("COUNT "+strconv.Itoa(len(records))+"\n"))

		go func() {
			for {
				time.Sleep(5 * time.Second)
				_, _, err := ws.ReadMessage()
				if err != nil {
					cancel()
				}
			}
		}()

		var writersMap = make(map[string]int)
		var sponsorsMap = make(map[string]int)
		var voicesMap = make(map[string]int)
		var audiosMap = make(map[string]int)
		var sourcesMap = make(map[string]int)

		//var strategiesMap = make(map[string]int)

		// populate pre-query text>id maps
		for _, record := range records {
			if len(record) < 20 {
				http.Error(w, "invalid format (need 20 columns)", 400)
				return
			}
			writerName := strings.ToLower(strings.TrimSpace(record[8]))
			coWriter := strings.ToLower(strings.TrimSpace(record[14]))
			audio := strings.ToLower(strings.TrimSpace(record[15]))
			voice := strings.ToLower(strings.TrimSpace(record[10]))
			source := strings.ToLower(strings.TrimSpace(record[16]))
			sponsor := strings.ToLower(strings.TrimSpace(record[17]))

			if writersMap[writerName] == 0 {
				res := conn.QueryRow(ctx, "INSERT into writer (name) values ($1) ON CONFLICT ON CONSTRAINT writer_pk2 DO UPDATE SET name = writer.name RETURNING id;", writerName)
				var id int
				err := res.Scan(&id)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				writersMap[writerName] = id
			}

			if writersMap[coWriter] == 0 {
				res := conn.QueryRow(ctx, "INSERT into writer (name) values ($1) ON CONFLICT ON CONSTRAINT writer_pk2 DO UPDATE SET name = writer.name RETURNING id;", writerName)

				var id int
				err := res.Scan(&id)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				writersMap[coWriter] = id
			}

			if audiosMap[audio] == 0 {
				res := conn.QueryRow(ctx, "INSERT into audio (name) values ($1) ON CONFLICT ON CONSTRAINT audio_pk2 DO UPDATE SET name = audio.name RETURNING id;", audio)
				var id int
				err := res.Scan(&id)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				audiosMap[audio] = id
			}

			if voicesMap[voice] == 0 {
				res := conn.QueryRow(ctx, "INSERT into voice (name) values ($1) ON CONFLICT ON CONSTRAINT voice_pk2 DO UPDATE SET name = voice.name RETURNING id;", voice)
				var id int
				err := res.Scan(&id)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				voicesMap[voice] = id
			}

			if sourcesMap[source] == 0 {
				res := conn.QueryRow(ctx, "INSERT into source (name) values ($1) ON CONFLICT ON CONSTRAINT source_pk2 DO UPDATE SET name = source.name RETURNING id;", source)
				var id int
				err := res.Scan(&id)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				sourcesMap[source] = id
			}

			if sponsorsMap[sponsor] == 0 && sponsor != "" {
				res := conn.QueryRow(ctx, "INSERT into sponsor (name) values ($1) ON CONFLICT ON CONSTRAINT sponsor_pk2 DO UPDATE SET name = sponsor.name RETURNING id;", sponsor)
				var id int
				err := res.Scan(&id)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				sponsorsMap[sponsor] = id
			}
		}

		rowN := 0
		for _, record := range records {
			rowN++

			writeWs, err := ws.NextWriter(websocket.TextMessage)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			writer := csv.NewWriter(writeWs)
			flush := func() {
				writer.Flush()
				writeWs.Close()
			}
			// check if video already in db
			var caption string
			var length int
			var postedDate time.Time

			var previousSponsor pgtype.Int8

			err = conn.QueryRow(ctx, "SELECT title, duration, posted_date, sponsor_id FROM video WHERE url = $1 and posted_date is not null and duration is not null and title is not null", record[3]).Scan(&caption, &length, &postedDate, &previousSponsor)
			//if err != nil {
			//	writer.Write(append(record, strconv.Itoa(rowN), "[Skipped] failed to check if video already entered"))
			//	flush()
			//	continue
			//}
			//if exists {
			//	writer.Write(append(record, strconv.Itoa(rowN), "[Skipped] video already entered"))
			//	flush()
			//	continue
			//}

			fmt.Println(record)

			// insert into db
			url := record[3]
			u, err := url2.Parse(url)
			if err != nil {
				writer.Write(append(record, strconv.Itoa(rowN), "[Failed] invalid url "+err.Error()))
				flush()
				continue
			}
			u.RawQuery = ""
			u.Fragment = ""
			url = u.String()

			var accountId = 1

			res := conn.QueryRow(ctx, "INSERT into account (platform, username) values ($1, $2) ON CONFLICT ON CONSTRAINT account_pk DO UPDATE SET platform = account.platform, username = account.username RETURNING id;", "", "")

			err = res.Scan(&accountId)
			if err != nil {
				writer.Write(append(record, strconv.Itoa(rowN), "[Failed] "+err.Error()))
				flush()
				return
			}

			storyUrl := record[2]
			u2, _ := url2.Parse(storyUrl)
			u2.RawQuery = ""
			u2.Fragment = ""
			storyUrl = u2.String()

			storyCode := record[13]
			writerId := writersMap[strings.ToLower(strings.TrimSpace(record[8]))]
			writerRate, err := strconv.Atoi("0") // TODO: actual

			coWriter := writersMap[strings.ToLower(strings.TrimSpace(record[14]))]
			coWriterRate, err := strconv.Atoi("0") // TODO: actual
			coWriterDb := sql.NullInt64{Valid: coWriter != 0}
			if coWriter != 0 {
				coWriterDb = sql.NullInt64{Int64: int64(coWriter), Valid: true}
			}
			coWriterRateDb := sql.NullInt64{Valid: coWriter != 0}
			if coWriter != 0 {
				coWriterRateDb = sql.NullInt64{Int64: int64(coWriterRate), Valid: true}
			}

			audio := audiosMap[strings.ToLower(strings.TrimSpace(record[15]))]
			voice := voicesMap[strings.ToLower(strings.TrimSpace(record[10]))]
			source := sourcesMap[strings.ToLower(strings.TrimSpace(record[16]))]
			sponsor := sponsorsMap[strings.ToLower(strings.TrimSpace(record[17]))]
			sponsorDb := sql.NullInt64{Valid: sponsor != 0}
			if sponsor != 0 {
				sponsorDb = sql.NullInt64{Int64: int64(sponsor), Valid: true}
			}
			sponsorRate, err := strconv.Atoi(record[18])
			sponsorRateDb := sql.NullInt64{Valid: sponsor != 0}
			if sponsor != 0 {
				sponsorRateDb = sql.NullInt64{Int64: int64(sponsorRate), Valid: true}
			}

			strategiesString := record[19]
			strategiesSep := strings.Split(strategiesString, ",")
			for i, _ := range strategiesSep {
				strategiesSep[i] = strings.TrimSpace(strategiesSep[i])
				strategiesSep[i] = strings.ToLower(strategiesSep[i])
			}

			tx, err := conn.Begin(ctx)
			defer tx.Rollback(ctx)

			if err != nil {
				writer.Write(append(record, strconv.Itoa(rowN), "[Skipped] could not begin transaction: "+err.Error()))
				flush()
				return
			}

			res = tx.QueryRow(ctx, `INSERT INTO video 
    (created, sponsor_id, sponsor_rate, writer_id, writer_rate, account_id, url, voice_id, audio_id, source_id, co_writer_id, co_writer_rate, title, story_code, story_link, duration, posted_date, needs_entry, preview)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)

ON CONFLICT (url) DO UPDATE
	SET (created, sponsor_id, sponsor_rate, writer_id, writer_rate, account_id, url, voice_id, audio_id, source_id, co_writer_id, co_writer_rate, title, story_code, story_link, duration, posted_date, needs_entry, preview)
= ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)

RETURNING id;`,
				time.Now(),
				sponsorDb,
				sponsorRateDb,
				writerId,
				writerRate,
				accountId,
				url,
				voice,
				audio,
				source,
				coWriterDb,
				coWriterRateDb,
				caption,
				storyCode, // story code
				storyUrl,
				length,
				postedDate,
				false,
			)

			var insertedVideoId int
			err = res.Scan(&insertedVideoId)
			if err != nil {
				writer.Write(append(record, strconv.Itoa(rowN), "[Failed] "+err.Error()))
				flush()
				return
			}

			_, err = tx.Exec(ctx, `DELETE from video_strategies where video_id = $1`, insertedVideoId)
			if err != nil {
				writer.Write(append(record, strconv.Itoa(rowN), "[Failed] while deleting existing strategies "+err.Error()))
				flush()
				return
			}

			if len(strategiesSep) > 0 {
				// add strategies
				sqlStr := "INSERT INTO video_strategies(video_id, strategy_id) VALUES "
				vals := make([]any, 0)

				for i, row := range strategiesSep {
					id, err := strconv.Atoi(row)
					if err != nil {
						varName := sql.NullString{Valid: false}
						valName := sql.NullString{Valid: false}

						parts := strings.Split(row, ":")
						if len(parts) == 2 {
							varName = sql.NullString{String: parts[0], Valid: true}
							valName = sql.NullString{String: parts[1], Valid: true}
						}
						// create new source if needed

						res := conn.QueryRow(ctx, "INSERT into strategy (name, variable, value) values ($1, $2, $3) ON CONFLICT ON CONSTRAINT strategy_pk2  DO UPDATE SET name = strategy.name RETURNING id;", row, varName, valName)
						err := res.Scan(&id)
						if err != nil {
							writer.Write(append(record, strconv.Itoa(rowN), "[Failed] "+err.Error()))
							flush()
							return
						}
					}
					sqlStr += fmt.Sprintf("($%d, $%d),", 2*i+1, 2*i+2)
					vals = append(vals, insertedVideoId, id)
				}

				//trim the last ,
				sqlStr = strings.TrimSuffix(sqlStr, ",")

				//prepare the statement
				_, err := tx.Exec(ctx, sqlStr, vals...)
				if err != nil {
					writer.Write(append(record, strconv.Itoa(rowN), "[Failed] "+err.Error()))
					flush()
					return
				}
				// TODO: close!
			}

			err = tx.Commit(ctx)
			if err != nil {
				writer.Write(append(record, strconv.Itoa(rowN), "[Skipped] Could not commit transaction: "+err.Error()))
				flush()
				return
			}

			writer.Write(append(record, strconv.Itoa(rowN)))
			flush()

		}

		w.Write([]byte("DONE"))
	})
	http.HandleFunc("/express_entry", routes.Handler(db, templateFuncs, routes.ExpressEntry))
	http.HandleFunc("/express_entry_post", routes.Handler(db, templateFuncs, routes.ExpressEntryPost))
	http.HandleFunc("/video_details", routes.Handler(db, templateFuncs, routes.VideoDetails))
	http.HandleFunc("/update_video_details", routes.Handler(db, templateFuncs, routes.UpdateVideoDetails))
	http.HandleFunc("/admin/manage_va", routes.Handler(db, templateFuncs, routes.ManageVa))
	http.HandleFunc("/admin/manage_writers", routes.Handler(db, templateFuncs, routes.ManageWriters))
	http.HandleFunc("/admin/manage_sponsors", routes.Handler(db, templateFuncs, routes.ManageSponsors))
	http.HandleFunc("/admin/create_writer_post", routes.Handler(db, templateFuncs, routes.CreateWriterPost))
	http.HandleFunc("/admin/change_writer_pw", routes.Handler(db, templateFuncs, routes.ChangeWriterPw))

	http.HandleFunc("/admin/videos", routes.Handler(db, templateFuncs, routes.ManageVideos))
	http.HandleFunc("/export_videos", routes.Handler(db, templateFuncs, routes.ExportVideoData))
	http.HandleFunc("/admin/tracking_status", routes.Handler(db, templateFuncs, routes.AdminErrors))
	http.HandleFunc("/admin/va_videos", routes.Handler(db, templateFuncs, routes.ManageVaVideos))
	http.HandleFunc("/admin/finances", routes.Handler(db, templateFuncs, routes.AdminFinances))
	http.HandleFunc("/admin/sponsor_details", routes.Handler(db, templateFuncs, routes.AdminSponsorDetails))
	http.HandleFunc("/admin/creativity", routes.Handler(db, templateFuncs, routes.AdminCreativity))
	http.HandleFunc("/admin/creativity_post", routes.Handler(db, templateFuncs, routes.AdminCreativityPost))
	http.HandleFunc("/admin/dashboard", routes.Handler(db, templateFuncs, routes.AdminDashboard))
	http.HandleFunc("/admin/retention", routes.Handler(db, templateFuncs, routes.AdminRetention))
	http.HandleFunc("/strategies", routes.Handler(db, templateFuncs, routes.StrategiesView))
	http.HandleFunc("/admin/create_sponsor_post", routes.Handler(db, templateFuncs, routes.CreateSponsorPost))
	http.HandleFunc("/admin/change_sponsor_pw", routes.Handler(db, templateFuncs, routes.ChangeSponsorPw))
	http.HandleFunc("/admin/create_va_post", routes.Handler(db, templateFuncs, routes.CreateVaPost))
	http.HandleFunc("/admin/change_va_pw", routes.Handler(db, templateFuncs, routes.ChangeVaPw))
	http.HandleFunc("/admin/enter_payment", routes.Handler(db, templateFuncs, routes.EnterPayment))
	http.HandleFunc("/admin/enter_payment_post", routes.Handler(db, templateFuncs, routes.EnterPaymentPost))

	http.HandleFunc("/admin/login", routes.Handler(db, templateFuncs, routes.AdminLogin))
	http.HandleFunc("/admin/login_post", routes.Handler(db, templateFuncs, routes.AdminLoginPost))
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		c := &http.Cookie{
			Name:    "login",
			Value:   "",
			Path:    "/",
			Expires: time.Unix(0, 0),
		}

		http.SetCookie(w, c)
		http.Redirect(w, r, "/", 302)
	})

	//http.HandleFunc("/va/add_video", routes.Handler(db, templateFuncs, routes.AddVideo))
	//http.HandleFunc("/va/add_video_post", routes.Handler(db, templateFuncs, routes.AddVideoPost))

	http.HandleFunc("/va/login", routes.Handler(db, templateFuncs, routes.VaLogin))
	http.HandleFunc("/va/login_post", routes.Handler(db, templateFuncs, routes.VaLoginPost))

	http.HandleFunc("/retention", routes.Handler(db, templateFuncs, routes.VaRetention))
	http.HandleFunc("/post_retention_graph", routes.Handler(db, templateFuncs, routes.VaRetentionPost))

	http.HandleFunc("/sponsor/login", routes.Handler(db, templateFuncs, routes.SponsorLogin))
	http.HandleFunc("/sponsor/login_post", routes.Handler(db, templateFuncs, routes.SponsorLoginPost))
	http.HandleFunc("/sponsor/dash", routes.Handler(db, templateFuncs, routes.SponsorDash))

	http.HandleFunc("/writer/login", routes.Handler(db, templateFuncs, routes.WriterLogin))
	http.HandleFunc("/writer/login_post", routes.Handler(db, templateFuncs, routes.WriterLoginPost))
	http.HandleFunc("/writer/dash", routes.Handler(db, templateFuncs, routes.WriterDashboard))

	http.HandleFunc("/api/video_urls", func(w http.ResponseWriter, r *http.Request) {
		conn, err := db.Acquire(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer conn.Release()
		r.ParseForm()
		sponsorId := r.FormValue("sponsor_id")
		sponsorIdInt, _ := strconv.Atoi(sponsorId)
		sponsorNameLower := strings.ToLower(r.FormValue("sponsor_name"))

		res, err := conn.Query(r.Context(), "SELECT url from video join sponsor s on s.id = video.sponsor_id where  s.id = $1 OR LOWER(s.name) = $2 ", sponsorIdInt, sponsorNameLower)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		var urls = make([]string, 0)
		for res.Next() {
			var url string
			err := res.Scan(&url)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			urls = append(urls, url)
		}
		json.NewEncoder(w).Encode(urls)
	})

	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatal("Error starting server: ", err)
	}
}

//func populate(db *pgx.Conn) {
//	urls := make([]string, 0)
//	res, err := db.Query(r.Context(), "SELECT url from video order by created desc")
//	if err != nil {
//		log.Fatal("Error getting videos: ", res.Err())
//		return
//	}
//
//	for res.Next() {
//		var url string
//		err := res.Scan(&url)
//		if err != nil {
//			log.Fatal("Error getting videos: ", err)
//			return
//		}
//		urls = append(urls, url)
//	}
//
//	for _, url := range urls {
//		_, _, durationS, _, err := tiktok.GetTiktokDetails(url)
//		if err != nil {
//			fmt.Println("Trying again, url:", url)
//			var err2 error
//			_, _, durationS, _, err2 = tiktok.GetTiktokDetails(url)
//			if err2 != nil {
//				log.Println("Error getting tiktok details: ", err2)
//				continue
//			}
//		}
//
//		//var accountId = 1
//
//		//res := db.QueryRow(r.Context(), "INSERT into account (platform, username) values ($1, $2) ON CONFLICT ON CONSTRAINT account_pk DO UPDATE SET platform = account.platform, username = account.username RETURNING id;", "tiktok", username)
//		//if res.Err() != nil {
//		//	log.Fatal("Error getting account id: ", res.Err())
//		//	return
//		//}
//		//err = res.Scan(&accountId)
//		//if err != nil {
//		//	if err == sql.ErrNoRows {
//		//		log.Fatal("No matching account was found")
//		//		return
//		//	}
//		//	log.Fatal("Error getting account id: ", err)
//		//	return
//		//}
//		fmt.Println(url, "test")
//		_, err = db.Exec(r.Context(), "UPDATE video set duration = $1 where url = $2", durationS, url)
//		if err != nil {
//			log.Fatal("Error updating video: ", err)
//			return
//		}
//	}
//
//}
