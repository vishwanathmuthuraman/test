package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/query"
	_ "github.com/lib/pq"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

func ScrapingWorker(jobs chan *VideoInfo, results chan<- *VideoInfoWithErr) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in ScrapingWorker", r)
		}
	}()
	for j := range jobs {

		func() {
			select {
			case <-j.BatchCtx.Done():
				fmt.Println("[Task Cancelled]", j.BatchTime, j.Url)
				return
			default:
			}

			if j.RetriedTimes >= 3 {
				fmt.Println("[Task Failed]", j.BatchTime, j.Url, " : ", "Retried too many times")
				j.Cancel()
				return
			}

			fmt.Println("[Task Submitted]", j.BatchTime, j.Url)

			data, err, retry := GetVideoStats(j.Url, false, true)
			if err != nil && retry {
				fmt.Println("[Retrying]", j.BatchTime, j.Url, " : ", err.Error())
				go func(j *VideoInfo) {
					j.RetriedTimes++
					jobs <- j
				}(j)
				return
			}

			go func(vi *VideoInfoWithErr) {
				results <- vi
			}(&VideoInfoWithErr{data, *j, err})
			// no need to cancel ctx here, it will be done in the processing worker
			return
		}()
	}
}

func ProcessingWorker(results chan *VideoInfoWithErr, client influxdb2.Client, db *sql.DB, discord *discordgo.Session, discordChannelId string) {
	for r := range results {
		func() {
			item := *r
			defer func() {
				if rec := recover(); rec != nil {
					fmt.Println("Recovered in ProcessingWorker", rec)
				}
			}()
			formattedError := sql.NullString{Valid: false}
			if item.Err != nil {
				fmt.Println("[Task Failed]", item.VideoInfo.BatchTime, item.VideoInfo.Url, " : ", item.Err.Error())
				if item.Err.Error() == "tt_10204" {
					formattedError = sql.NullString{Valid: true, String: "Video unavailable"}
				}
				if item.Err.Error() == "tt_classified" {
					formattedError = sql.NullString{Valid: true, String: "Video is restricted"}
				}
				if item.Err.Error() == "tt_takedown" {
					formattedError = sql.NullString{Valid: true, String: "Video was taken down"}
				}
			}
			_, err := db.ExecContext(r.VideoInfo.BatchCtx, "INSERT INTO statistics (video_id, error) VALUES ($1, $2) ON CONFLICT (video_id) DO UPDATE SET error = $2", item.VideoInfo.Id, formattedError)
			if err != nil {
				fmt.Println("[Db Error] while writing error to db:", err.Error())
				err = nil
			}

			if item.Err != nil {
				item.VideoInfo.Cancel()
				return
			}

			processVideo(client, db, item, discord, discordChannelId)
			fmt.Println("[Task Completed] [", item.VideoInfo.BatchTime, "]", item.VideoInfo.Url)
			item.VideoInfo.Cancel()
		}()
	}
}

func main() {
	fmt.Println("Hello World")

	token := os.Getenv("INFLUX")
	url := os.Getenv("INFLUX_URL")
	influxClient := influxdb2.NewClientWithOptions(url, token,
		influxdb2.DefaultOptions().
			SetBatchSize(100).
			SetUseGZip(true).
			SetTLSConfig(&tls.Config{
				InsecureSkipVerify: true,
			}))

	// open database
	db, err := sql.Open("postgres", os.Getenv("PG_STRING"))
	if err != nil {
		log.Fatal(err)
	}

	discord, err := discordgo.New(os.Getenv("DISCORD"))
	if err != nil {
		log.Fatal(err)
	}

	guilds, err := discord.UserGuilds(5, "", "")
	if err != nil {
		log.Fatal(err)
	}

	var discordChannelId string

	for _, guild := range guilds {
		if discordChannelId != "" {
			// already found
			break
		}
		channels, err := discord.GuildChannels(guild.ID)
		if err != nil {
			continue
		}
		for _, channel := range channels {
			if channel.Name == "notifications" {
				discordChannelId = channel.ID
				break
			}
		}
	}

	//var videosList = make(map[int]VideoInfo)

	fetch := func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered from panic in main fetch routine", r)
			}
		}()

		jobs := make(chan *VideoInfo)
		results := make(chan *VideoInfoWithErr)
		// start up 50 workers
		for w := 1; w <= 50; w++ {
			go ScrapingWorker(jobs, results)
		}

		for w := 1; w <= 50; w++ {
			go ProcessingWorker(results, influxClient, db, discord, discordChannelId)
		}

		go func() {
			time.Sleep(10 * time.Minute)
			close(jobs)
			close(results)
		}()

		now := time.Now()
		batchTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), int(math.Floor(float64(now.Minute())/30.0))*30, 0, 0, now.Location())
		fmt.Println("Fetching batch", batchTime, "at", now)

		// refresh the list of videos
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		q := "SELECT video.id, acc.platform, acc.username, url, COALESCE(video.source_id, -1), COALESCE(video.audio_id,-1) ,COALESCE(video.voice_id, -1), COALESCE(video.writer_id, -1), COALESCE(video.sponsor_id, -1), video.sponsor_rate, COALESCE(video.writer_rate, 0), COALESCE(video.co_writer_id, -1), COALESCE(video.co_writer_rate, -1), video.posted_date, COALESCE(video.needs_entry, true), last_updated, last_emailed, (script_text IS NULL) as needs_script FROM video JOIN account acc on video.account_id = acc.id "
		if !(batchTime.Hour()%4 == 0 && batchTime.Minute() == 0) {
			q += "where (sponsor_id is not null OR needs_entry = TRUE)"
		}
		q += "ORDER BY created desc"
		rows, err := db.QueryContext(ctx, q)
		if err != nil {
			fmt.Println("[Batch Terminated]", err.Error())
			return
		}
		defer rows.Close()
		for rows.Next() {

			info := VideoInfo{}
			var lastEmailed sql.NullInt64

			err = rows.Scan(&info.Id, &info.Platform, &info.Username, &info.Url, &info.SourceId, &info.AudioId, &info.VoiceId, &info.WriterId, &info.SponsorId, &info.SponsorRate, &info.WriterRate, &info.CoWriterId, &info.CoWriterRate, &info.PostedDate, &info.NeedsEntry, &info.LastUpdated, &lastEmailed, &info.NeedsScript)
			if err != nil {
				fmt.Println("[Batch Terminated]", rows.Err().Error())
				return
			}

			if lastEmailed.Valid {
				info.LastEmailed = int(lastEmailed.Int64)
			} else {
				info.LastEmailed = -50_000
			}
			defaultStrategy := struct {
				Id   int
				Name string
			}{
				Id: -1, Name: "None",
			}
			info.Strategies = append(info.Strategies, defaultStrategy)

			//if inf || info.NeedsEntry {
			info.BatchTime = batchTime
			info.BatchCtx, info.Cancel = context.WithTimeout(context.Background(), 10*time.Minute)
			info.RetriedTimes = 0
			jobs <- &info
			//}
		}
		if rows.Err() != nil {
			fmt.Println("[Batch Terminated]", rows.Err().Error())
		}

	}

	go func() {
		for {
			go fetch()
			time.Sleep(10 * time.Minute)
		}
	}()

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("auth") != "AUTHSECRET!!!" {
			return
		}
		fmt.Fprintf(w, "pong")
	})
	//manualTicker <- true
	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatal("while starting http server", err)
	}
}

type Point struct {
	OldRecord *query.FluxRecord
	Views     float64
	Likes     float64
	Comments  float64
	Revenue   float64
}

func processVideo(influxClient influxdb2.Client, db *sql.DB, vidInfo VideoInfoWithErr, discord *discordgo.Session, discordChannelId string) {
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	// get non-blocking write client
	writeAPI := influxClient.WriteAPI(os.Getenv("INFLUX_ORG"), "views")

	sponsorRevenue := float64(float64(vidInfo.VideoInfo.SponsorRate.Int64) * (float64(vidInfo.VideoMetrics.Views) / 1_000_000.0))

	select {
	case <-vidInfo.VideoInfo.BatchCtx.Done():
		fmt.Println("[Batch Cancelled inside Processing]", vidInfo.VideoInfo.BatchTime, vidInfo.VideoInfo.Url)
		return
	case <-ctx.Done():
		fmt.Println("[Individual task Cancelled inside Processing]", vidInfo.VideoInfo.Url)
		return
	default:
	}
	for _, s := range vidInfo.VideoInfo.Strategies {
		p := influxdb2.NewPointWithMeasurement("views").
			AddTag("username", vidInfo.VideoMetrics.Username).
			AddTag("url", vidInfo.VideoInfo.Url).
			AddTag("platform", string(vidInfo.VideoInfo.Platform)).
			AddField("views", float64(vidInfo.VideoMetrics.Views)).
			AddField("comments", float64(vidInfo.VideoMetrics.Comments)).
			AddField("likes", float64(vidInfo.VideoMetrics.Likes)).
			AddField("shares", float64(vidInfo.VideoMetrics.Shares)).
			AddField("saves", float64(vidInfo.VideoMetrics.Saves)).
			AddField("revenue", sponsorRevenue).
			AddField("engagement_like", float64(vidInfo.VideoMetrics.Likes)/float64(vidInfo.VideoMetrics.Views)).
			AddField("engagement_comment", float64(vidInfo.VideoMetrics.Comments)/float64(vidInfo.VideoMetrics.Views)).
			//AddField("writer_pay", writerPay).
			//AddField("co_writer_pay", coWriterPay).
			//AddTag("strategy_id", strconv.Itoa(s.Id)).
			AddTag("strategy_name", s.Name).
			AddTag("source_id", strconv.Itoa(vidInfo.VideoInfo.SourceId)).
			AddTag("audio_id", strconv.Itoa(vidInfo.VideoInfo.AudioId)).
			AddTag("sponsor_id", strconv.Itoa(int(vidInfo.VideoInfo.SponsorId.Int64))).
			AddTag("writer_id", strconv.Itoa(vidInfo.VideoInfo.WriterId)).
			AddTag("voice_id", strconv.Itoa(vidInfo.VideoInfo.VoiceId)).
			//AddTag("entered", vidInfo.VideoInfo.Entered.UTC().Format(time.UnixDate)).
			AddTag("posted", vidInfo.VideoInfo.PostedDate.Format(time.RFC3339)).
			SetTime(vidInfo.VideoInfo.BatchTime)

		writeAPI.WritePoint(p) // Flush writes s
	}
	select {
	case <-vidInfo.VideoInfo.BatchCtx.Done():
		fmt.Println("[Batch Cancelled inside Processing]", vidInfo.VideoInfo.BatchTime, vidInfo.VideoInfo.Url)
		return
	case <-ctx.Done():
		fmt.Println("[Individual task Cancelled inside Processing]", vidInfo.VideoInfo.Url)
		return
	default:
	}

	if (vidInfo.VideoInfo.SponsorId.Int64 != -1 || vidInfo.VideoInfo.NeedsEntry) && (vidInfo.VideoInfo.LastEmailed+100_000 < vidInfo.VideoMetrics.Views) {

		threshold := vidInfo.VideoMetrics.Views - (vidInfo.VideoMetrics.Views % 50_000) // round down to nearest 50k

		//if vidInfo.VideoMetrics.ViewCount < 100_000 && vidInfo.VideoMetrics.ViewCount >= 50_000 {
		//	threshold = 50_000
		//}

		err := SendEmail(threshold, vidInfo.VideoInfo.Url, discord, discordChannelId)
		if err != nil {
			fmt.Println("while sending email:", err)
			return
		}
		_, err = db.ExecContext(ctx, "UPDATE video SET last_emailed = $1, last_updated = $2 WHERE id = $3", threshold, time.Now(), vidInfo.VideoInfo.Id)
		if err != nil {
			fmt.Println("[E-Mail Error] while updating last sent in db:", err)
			return
		}
	}
	select {
	case <-vidInfo.VideoInfo.BatchCtx.Done():
		fmt.Println("[Batch Cancelled inside Processing]", vidInfo.VideoInfo.BatchTime, vidInfo.VideoInfo.Url)
		return
	case <-ctx.Done():
		fmt.Println("[Individual task Cancelled inside Processing]", vidInfo.VideoInfo.Url)
		return
	default:
	}

	err := writeStatistics(influxClient, db, vidInfo, discord, discordChannelId, ctx)
	if err != nil {
		fmt.Println("[Statistics Error] failed to get stats from influx: ", err.Error())
	}
}

func writeStatistics(influxClient influxdb2.Client, db *sql.DB, vidInfo VideoInfoWithErr, discord *discordgo.Session, discordChannelId string, ctx context.Context) error {
	q := `

import "date"

from(bucket: "views")
    |> range(start: date.add(d: -6h, to: now()), stop: date.add(d: -30m, to: now()))
    |> filter(fn: (r) => r._measurement == "views")
    |> filter(fn: (r) => r._field == "views")
    |> filter(fn: (r) => r["url"] == "%s")
    |> derivative(unit: %s)
	|> derivative(unit: %s)
    |> map( fn: (r) => ({r with _time: if exists r._time then r._time else r._time })  )
    |> last()
`
	//res, err := influxClient.QueryAPI(os.Getenv("INFLUX_ORG")).Query(vidInfo.VideoInfo.BatchCtx, fmt.Sprintf(q, vidInfo.VideoInfo.Url, "30m"))
	//if err != nil {
	//	return err
	//}
	//var views30m float64
	//for res.Next() {
	//	views30m = res.Record().Value().(float64)
	//}

	if vidInfo.VideoInfo.SponsorId.Int64 != -1 || vidInfo.VideoInfo.NeedsEntry {

		res, err := influxClient.QueryAPI(os.Getenv("INFLUX_ORG")).Query(vidInfo.VideoInfo.BatchCtx, fmt.Sprintf(q, vidInfo.VideoInfo.Url, "1h", "1h"))
		if err != nil {
			return err
		}
		var acceleration float64
		for res.Next() {
			acceleration = res.Record().Value().(float64)
		}

		if acceleration > 2000 {
			_, err := discord.ChannelMessageSend(discordChannelId, "Video with url "+vidInfo.VideoInfo.Url+" is going viral.", discordgo.WithContext(ctx))
			if err != nil {
				fmt.Println("[Discord acceleration Error]", err.Error())
			}
		}
	}
	//res, err = influxClient.QueryAPI(os.Getenv("INFLUX_ORG")).Query(vidInfo.VideoInfo.BatchCtx, fmt.Sprintf(q, vidInfo.VideoInfo.Url, "6h"))
	//if err != nil {
	//	return err
	//}
	//var views6h float64
	//for res.Next() {
	//	views6h = res.Record().Value().(float64)
	//}
	//
	//res, err = influxClient.QueryAPI(os.Getenv("INFLUX_ORG")).Query(vidInfo.VideoInfo.BatchCtx, fmt.Sprintf(q, vidInfo.VideoInfo.Url, "12h"))
	//if err != nil {
	//	return err
	//}
	//var views12h float64
	//for res.Next() {
	//	views12h = res.Record().Value().(float64)
	//}
	//
	//res, err = influxClient.QueryAPI(os.Getenv("INFLUX_ORG")).Query(vidInfo.VideoInfo.BatchCtx, fmt.Sprintf(q, vidInfo.VideoInfo.Url, "24h"))
	//if err != nil {
	//	return err
	//}
	//var views24h float64
	//for res.Next() {
	//	views24h = res.Record().Value().(float64)
	//}

	// engagement %
	//	q = `
	//import "date"
	//
	//from(bucket: "views")
	//    |> range(start: date.add(d: -48h, to: now()), stop: date.add(d: -30m, to: now()))
	//    |> filter(fn: (r) => r._measurement == "views")
	//    |> filter(fn: (r) => r._field == "engagement_like")
	//    |> filter(fn: (r) => r["url"] == "%s")
	//|> map( fn: (r) => ({r with _time: if exists r._time then r._time else r._time })  )
	//    |> last()
	//`
	//	res, err = influxClient.QueryAPI(os.Getenv("INFLUX_ORG")).Query(vidInfo.VideoInfo.BatchCtx, fmt.Sprintf(q, vidInfo.VideoInfo.Url))
	//	if err != nil {
	//		return err
	//	}
	//	var engagementLike float64
	//	for res.Next() {
	//		engagementLike = res.Record().Value().(float64)
	//	}
	//
	//	q = `
	//import "date"
	//
	//from(bucket: "views")
	//    |> range(start: date.add(d: -48h, to: now()), stop: date.add(d: -30m, to: now()))
	//    |> filter(fn: (r) => r._measurement == "views")
	//    |> filter(fn: (r) => r._field == "engagement_comment")
	//    |> filter(fn: (r) => r["url"] == "%s")
	//|> map( fn: (r) => ({r with _time: if exists r._time then r._time else r._time })  )
	//    |> last()
	//`
	//	res, err = influxClient.QueryAPI(os.Getenv("INFLUX_ORG")).Query(ctx, fmt.Sprintf(q, vidInfo.VideoInfo.Url))
	//	if err != nil {
	//		return err
	//	}
	//	var engagementComment float64
	//	for res.Next() {
	//		engagementComment = res.Record().Value().(float64)
	//	}

	if vidInfo.VideoInfo.NeedsScript && vidInfo.VideoMetrics.ClosedCaptionsUrl != "" {
		captions := ""
		var err2 error
		if vidInfo.VideoInfo.Platform == VideoPlatformTiktok {
			captions, err2 = getTiktokCaptions(vidInfo.VideoMetrics.ClosedCaptionsUrl)
		} else if vidInfo.VideoInfo.Platform == VideoPlatformYoutube {
			captions, err2 = getYouTubeClosedCaptions(vidInfo.VideoMetrics.ClosedCaptionsUrl)
		}
		if err2 != nil {
			fmt.Println("error while getting captions:", err2)
		}
		vidInfo.VideoMetrics.ClosedCaption = captions
	}

	// TODO: retention

	{
		//_, err := db.ExecContext(ctx, "INSERT INTO statistics (video_id, views_total, views_24h, views_12h, views_6h, views_1h, views_30m, retention_3, retention_5, retention_10, engagement_like, engagement_comment, likes_total, comments_total) VALUES ($1, $2,$3,$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT (video_id) DO UPDATE SET views_total = $2, views_24h = $3, views_12h = $4, views_6h = $5, views_1h = $6, views_30m = $7, retention_3 = $8, retention_5 = $9, retention_10 = $10, engagement_like = $11, engagement_comment = $12, likes_total = $13, comments_total = $14", vidInfo.VideoInfo.Id, vidInfo.VideoMetrics.Views, int(views24h), int(views12h), int(views6h), int(views1h), int(views30m), 0, 0, 0, engagementLike, engagementComment, vidInfo.VideoMetrics.Likes, vidInfo.VideoMetrics.Comments)
		_, err := db.ExecContext(ctx, "INSERT INTO statistics (video_id, views_total, views_24h, views_12h, views_6h, views_1h, views_30m, retention_3, retention_5, retention_10, engagement_like, engagement_comment, likes_total, comments_total, shares_total, saves_total) VALUES ($1, $2,$3,$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) ON CONFLICT (video_id) DO UPDATE SET views_total = $2, views_24h = $3, views_12h = $4, views_6h = $5, views_1h = $6, views_30m = $7, retention_3 = $8, retention_5 = $9, retention_10 = $10, engagement_like = $11, engagement_comment = $12, likes_total = $13, comments_total = $14, shares_total = $15, saves_total = $16", vidInfo.VideoInfo.Id, vidInfo.VideoMetrics.Views, int(0), int(0), int(0), int(0), int(0), 0, 0, 0, 0, 0, vidInfo.VideoMetrics.Likes, vidInfo.VideoMetrics.Comments, vidInfo.VideoMetrics.Shares, vidInfo.VideoMetrics.Saves)

		if err != nil {
			return fmt.Errorf("while writing statistics to db: %w", err)
		}
	}

	{
		_, err := db.ExecContext(ctx, "UPDATE video SET preview = $1 WHERE id = $2", vidInfo.VideoMetrics.Preview, vidInfo.VideoInfo.Id)
		if err != nil {
			return fmt.Errorf("while writing preview to db: %w", err)
		}
	}

	if vidInfo.VideoInfo.NeedsScript && vidInfo.VideoMetrics.ClosedCaption != "" {
		_, err := db.ExecContext(ctx, "UPDATE video SET script_text = $1 WHERE id = $2", []byte(vidInfo.VideoMetrics.ClosedCaption), vidInfo.VideoInfo.Id)
		if err != nil {
			return fmt.Errorf("while writing closed caption to db: %w", err)
		}
	}

	return nil
}
