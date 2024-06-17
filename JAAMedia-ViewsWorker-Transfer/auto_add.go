package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/anaskhan96/soup"
	"io"
	"math/rand"
	"net/http"
	url2 "net/url"
	"strconv"
	"time"
)

func AutoAdd(username string, platform string, db *sql.DB) error {
	url := "https://www.tiktok.com/@" + username

	var client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url2.URL{
		Scheme: "http",
		User:   url2.UserPassword("brd-customer-hl_545d83f9-zone-datacenter_proxies_ppu-session-rand"+strconv.Itoa(rand.Int()), "jwfu2f71rc9z"),
		Host:   "brd.superproxy.io:22225",
	})},
		Timeout: 5 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
	//req.Header.Set("X-Real-Ip", "1.1.1.1")
	//req.Header.Set("X-Forwarded-For", "1.1.1.1")
	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	htmlContent := soup.HTMLParse(string(body))

	content := htmlContent.Find("script", "id", "SIGI_STATE")
	if content.Pointer == nil {
		return errors.New("nil pointer in search for SIGI_STATE")
	}
	var jsonData map[string]interface{}

	err2 := json.Unmarshal([]byte(content.FullText()), &jsonData)
	if err2 != nil {
		return err2
	}

	itemModuleList, ok := jsonData["ItemModule"].(map[string]interface{})
	if !ok {
		return errors.New("cannot find ItemModule")
	}

	itemList, ok := jsonData["ItemList"].(map[string]interface{})
	if !ok {
		return errors.New("cannot find ItemList")
	}

	userPost, ok := itemList["user-post"].(map[string]interface{})
	if !ok {
		return errors.New("cannot find userPost")
	}

	list, ok := userPost["list"].([]interface{}) // TODO: or browserList?
	if !ok {
		return errors.New("cannot find list")
	}

	videosCollected := make([]VideoStats, 0)

	for _, itemId := range list {
		i, ok := itemId.(string)
		item, ok := itemModuleList[i].(map[string]interface{})
		if !ok {
			continue // skip this one (could be private etc) TODO: report error
		}
		parsed, err := processItemModule(item)
		if err != nil {
			continue // skip this one (could be private etc) TODO: report error
		}
		videosCollected = append(videosCollected, parsed)
	}
	for _, video := range videosCollected {
		var accountId = -1

		res := db.QueryRow("INSERT into account (platform, username) values ($1, $2) ON CONFLICT ON CONSTRAINT account_pk DO UPDATE SET platform = account.platform, username = account.username RETURNING id;", "tiktok", video.Username)
		if res.Err() != nil {
			continue
		}
		err = res.Scan(&accountId)
		if err != nil {
			continue
		}

		_, err = db.Exec("INSERT INTO video (url, created, needs_entry, posted_date , title, account_id, duration) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (url) DO NOTHING", video.FullUrl, time.Now(), true, video.PostedDate, video.Caption, accountId, video.Length)
		if err != nil {
			continue
		}
	}

	return nil
}
