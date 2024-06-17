package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/anaskhan96/soup"
	"github.com/asticode/go-astisub"
	"io"
	"math/rand"
	"net/http"
	url2 "net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func GetVideoStats(url string, getCC, getPreview bool) (VideoStats, error, bool) {
	urlParsed, err := url2.Parse(url)
	if err != nil {
		return VideoStats{}, err, false
	}
	switch urlParsed.Host {
	case "tiktok.com":
		fallthrough
	case "vm.tiktok.com":
		fallthrough
	case "www.tiktok.com":
		res, err, retry := GetTiktokDetails(url, getCC, getPreview)
		res.Platform = VideoPlatformTiktok
		return res, err, retry
	case "youtube.com":
		fallthrough
	case "www.youtube.com":
		res, err, retry := GetYoutubeStats(url, getCC, getPreview)
		res.Platform = VideoPlatformYoutube
		return res, err, retry
	}
	return VideoStats{}, errors.New("unsupported platform. please enter a tiktok or youtube link"), false
}

// MARK: YouTube

func GetYoutubeStats(url string, getCC, getPreview bool) (VideoStats, error, bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic", r)
		}
	}()

	//var client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url2.URL{
	//	Scheme: "http",
	//	User:   url2.UserPassword("brd-customer-hl_545d83f9-zone-datacenter_proxies_ppu-country-us-session-rand"+strconv.Itoa(rand.Int()), "jwfu2f71rc9z"),
	//	Host:   "brd.superproxy.io:22225",
	//})}}
	var client = &http.Client{}
	client.Timeout = time.Second * 20
	//curl 'https://www.youtube.com/shorts/TMiIPIbfD2A' \
	//-X 'GET' \
	//-H 'Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8' \
	//-H 'Sec-Fetch-Site: none' \
	//-H 'Cookie: SIDCC=AKEyXzWoPzPefigTbZZvyG2A6yZZriWpmf4oyLa3hpuCO__Vv8zUe8h_oEsCGcbMLhdi9lpB; __Secure-1PSIDCC=AKEyXzUvi2PpEyrBc8vzmksTWY1IB8t6GlZE_XKecSb1kSQOPX5h8BnAPW5NoprMH2e07_opfQ; __Secure-3PSIDCC=AKEyXzXcEPzxJibyvkEm48gqbPVuV1NgkvVAtTFd7L4nXbw4dntpjELh3cMQckNMb4mjKc6U; PREF=f4=4000000&tz=America.Los_Angeles&f6=40000000&f7=100; CONSISTENCY=AKreu9s83Jht0KvW2ht7DQK8GoiZDmAB3e4kbytwHln-DDD9uDVDcFOO_r7DuWkArJZVhVabG1fN4oh6f8ue5mtIIo6XmyfI0IWSPD--InP5bvYTJ0tks0Hg7Ik; __Secure-1PSIDTS=sidts-CjIB7F1E_P2HqjwbKzkDbh70xx62RxYOJWTVmVu5AGXZzFHNYaYdc32LX9ekPruIWpLXvRAA; __Secure-3PSIDTS=sidts-CjIB7F1E_P2HqjwbKzkDbh70xx62RxYOJWTVmVu5AGXZzFHNYaYdc32LX9ekPruIWpLXvRAA; LOGIN_INFO=AFmmF2swRgIhAMRzcOls9upJSeW2iErfXADDlHI3pt2rRgv3PjCz1B78AiEA8UVUvhwssyQWR_P0xLXmJ6tnGhBMGhk-m4ZrPb4LjE8:QUQ3MjNmeU1rZkFuY01ORnhobzZ1Tk9uN3FtNGx6UHR4Q1RWTFVTZm41NzBYOFdQcEdmckpUS2FLTnQ5QmEyN3NPOE5DUXRoLXdHN0lCczNJOC1KOHlQNDRJcTBJb1c3dzE0N1FKM0VQVW9JNFl5UFJubDE0Q1JDckI0dEdEWXdhTGNWWW5jelFtd1dzNGotbHpzUjViMDNuZzlFSUppczVR; APISID=T_HBGvUHJjcUm4Jr/AaRQ1I7no2c9EcN4O; HSID=A65eJtNcdj6E4su5x; SAPISID=QwpaOX3eMsdBrimZ/Akpc4W52NNXFfEG0h; SID=g.a000iQhr6Gat3WIozvIb-rBp7__Oo8lyzAP6AhV_k-785UNzSpi6YEXK5pAGKxLt0oVp24rEJgACgYKAZESAQASFQHGX2MiRRIPNExXfO0d2xESDS1W5RoVAUF8yKqCtZrUPauAqBlsG3FkQLnX0076; SSID=AJgWnSVbbTpEc-GcE; __Secure-1PAPISID=QwpaOX3eMsdBrimZ/Akpc4W52NNXFfEG0h; __Secure-1PSID=g.a000iQhr6Gat3WIozvIb-rBp7__Oo8lyzAP6AhV_k-785UNzSpi6HqBdVr_MzwRKurIOIWtsgwACgYKAVsSAQASFQHGX2MiysLuB6H5Nspv41ROXHzAlhoVAUF8yKoDUqvxpsQd1bNdLFkZnaxY0076; __Secure-3PAPISID=QwpaOX3eMsdBrimZ/Akpc4W52NNXFfEG0h; __Secure-3PSID=g.a000iQhr6Gat3WIozvIb-rBp7__Oo8lyzAP6AhV_k-785UNzSpi68ougTNgojRY2XLetLwR_DQACgYKAY0SAQASFQHGX2MiE2kJ0sqkUOLocnYQnypC0RoVAUF8yKoGa4rNjrG6LyEH393ag_IO0076; VISITOR_INFO1_LIVE=l8WeCoR5ZY0; VISITOR_PRIVACY_METADATA=CgJVUxIEGgAgLQ%3D%3D; YSC=sBRhRK76n70' \
	//-H 'Accept-Encoding: gzip, deflate, br' \
	//-H 'Sec-Fetch-Mode: navigate' \
	//-H 'Host: www.youtube.com' \
	//-H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15' \
	//-H 'Accept-Language: en-US,en;q=0.9' \
	//-H 'Sec-Fetch-Dest: document' \
	//-H 'Connection: keep-alive'

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return VideoStats{}, err, false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Connection", "keep-alive")
	//req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Accept-Charset", "utf-8")
	req.Header.Set("Host", "www.youtube.com")

	resp, err := client.Do(req)
	fmt.Println(resp.Header.Get("Content-Type"))
	if err != nil {
		return VideoStats{}, err, true
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return VideoStats{}, err, true
	}

	htmlContent := soup.HTMLParse(string(respBody))

	head := htmlContent.Find("head")
	if head.Pointer == nil {
		return VideoStats{}, errors.New("nil pointer in search for head"), true
	}

	body := htmlContent.Find("body")
	if body.Pointer == nil {
		return VideoStats{}, errors.New("nil pointer in search for body"), true
	}
	var jsonData map[string]interface{}

	scripts := body.FindAll("script")
	var script soup.Root
	for _, result := range scripts {
		if result.Pointer != nil {
			if strings.Contains(result.Text(), "var ytInitialPlayerResponse") {
				script = result
				break
			}
		}
	}
	if script.Pointer == nil {
		return VideoStats{}, errors.New("nil pointer in search for script"), true
	}

	scriptText := script.FullText()
	err2 := json.Unmarshal([]byte(scriptText[strings.IndexByte(scriptText, '{'):strings.LastIndexByte(scriptText, '}')+1]), &jsonData)
	if err2 != nil {
		return VideoStats{}, err2, true
	}

	//playerResponse, ok := jsonData["ytInitialPlayerResponse"].(map[string]interface{})
	//if !ok {
	//	return VideoStats{}, errors.New("cannot find ytInitialPlayerResponse"), true
	//}

	videoDetails, ok := jsonData["videoDetails"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find videoDetails"), true
	}

	lengthSecondsStr, ok := videoDetails["lengthSeconds"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find lengthSeconds"), true
	}
	lengthSeconds, err := strconv.Atoi(lengthSecondsStr)
	if err != nil {
		return VideoStats{}, err, true
	}

	titleText, ok := videoDetails["title"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find title"), true
	}

	microformat, ok := jsonData["microformat"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find microformat"), true

	}

	playerMicroformatRenderer, ok := microformat["playerMicroformatRenderer"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find playerMicroformatRenderer"), true

	}

	viewCountStr, ok := playerMicroformatRenderer["viewCount"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find viewCount"), true
	}

	viewCount, err := strconv.Atoi(viewCountStr)
	if err != nil {
		return VideoStats{}, err, true
	}

	publishDate, ok := playerMicroformatRenderer["publishDate"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find publishDate"), true
	}

	publishDateTime, err := time.Parse(time.RFC3339, publishDate)
	if err != nil {
		return VideoStats{}, err, true
	}

	thumbnail, ok := playerMicroformatRenderer["thumbnail"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find thumbnail"), true
	}

	thumbnails, ok := thumbnail["thumbnails"].([]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find thumbnails"), true
	}

	thumbnailUrl := ""
	for _, thumb := range thumbnails {
		thumbnailFirst, ok := thumb.(map[string]interface{})
		if !ok {
			continue
		}

		thumbnailUrl, ok = thumbnailFirst["url"].(string)
		if ok {
			break
		} else {
			continue
		}
	}

	ownerProfileUrl, ok := playerMicroformatRenderer["ownerProfileUrl"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find ownerProfileUrl"), true
	}
	//

	closedCaptionsUrl := ""

	captions, ok := jsonData["captions"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find captions"), true
	}
	playerCaptionsTracklistRenderer, ok := captions["playerCaptionsTracklistRenderer"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find playerCaptionsTracklistRenderer"), true
	}
	captionTracks, ok := playerCaptionsTracklistRenderer["captionTracks"].([]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find captionTracks"), true
	}

	for _, track := range captionTracks {
		captionTrack, ok := track.(map[string]interface{})
		if !ok {
			continue
		}

		closedCaptionsUrl, ok = captionTrack["baseUrl"].(string)
		if ok {
			break
		} else {
			continue
		}
	}

	//overlay, ok := jsonData["overlay"].(map[string]interface{})
	//if !ok {
	//	return VideoStats{}, errors.New("cannot find overlay"), true
	//}
	//
	//reelPlayerOverlayRenderer, ok := overlay["reelPlayerOverlayRenderer"].(map[string]interface{})
	//if !ok {
	//	return VideoStats{}, errors.New("cannot find reelPlayerOverlayRenderer"), true
	//}
	//
	//likeButton, ok := reelPlayerOverlayRenderer["likeButton"].(map[string]interface{})
	//if !ok {
	//	return VideoStats{}, errors.New("cannot find likeButton"), true
	//}
	//
	//likeButtonRenderer, ok := likeButton["likeButtonRenderer"].(map[string]interface{})
	//if !ok {
	//	return VideoStats{}, errors.New("cannot find likeButtonRenderer"), true
	//}
	//
	//likeCount, ok := likeButtonRenderer["likeCount"].(float64)

	return VideoStats{
		Platform:   VideoPlatformYoutubeShorts,
		Caption:    titleText,
		Length:     lengthSeconds,
		Username:   ownerProfileUrl[strings.IndexByte(ownerProfileUrl, '@')+1:],
		PostedDate: publishDateTime,
		//Likes:    int(likeCount),
		Views:             viewCount,
		Preview:           thumbnailUrl,
		ClosedCaptionsUrl: closedCaptionsUrl,
	}, nil, false
}

func getYouTubeClosedCaptions(url string) (string, error) {
	//var client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url2.URL{
	//	Scheme: "http",
	//	User:   url2.UserPassword("brd-customer-hl_545d83f9-zone-datacenter_proxies_ppu-country-us-session-rand"+strconv.Itoa(rand.Int()), "jwfu2f71rc9z"),
	//	Host:   "brd.superproxy.io:22225",
	//})}}
	var client = &http.Client{}
	client.Timeout = time.Second * 20

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Connection", "keep-alive")
	//req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Accept-Charset", "utf-8")
	req.Header.Set("Host", "www.youtube.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var xmlCaptions Transcript
	err = xml.NewDecoder(resp.Body).Decode(&xmlCaptions)
	if err != nil {
		return "", err
	}
	var transcript string
	for _, textNode := range xmlCaptions.TextNodes {
		transcript += textNode.Text + " "
	}
	return transcript, nil
}

type Transcript struct {
	XMLName   xml.Name   `xml:"transcript"`
	TextNodes []TextNode `xml:"text"`
}
type TextNode struct {
	Start string `xml:"start,attr"`
	Dur   string `xml:"dur,attr"`
	Text  string `xml:",chardata"`
}

// MARK: Tiktok

func GetTiktokVideoId(url string) string {
	pattern := "\\/video\\/(\\w+)"
	pattern_compiled, _ := regexp.Compile(pattern)
	res := pattern_compiled.FindString(url)
	videoId := strings.Split(res, "/")[2]

	return videoId
}

func GetTiktokDetails(url string, getCC, getPreview bool) (VideoStats, error, bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic", r)
		}
	}()

	var client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url2.URL{
		Scheme: "http",
		User:   url2.UserPassword("brd-customer-hl_545d83f9-zone-datacenter_proxies_ppu-country-us-session-rand"+strconv.Itoa(rand.Int()), "jwfu2f71rc9z"),
		Host:   "brd.superproxy.io:22225",
	})}}
	//var client = &http.Client{}
	client.Timeout = time.Second * 20

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return VideoStats{}, err, false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
	req.Header.Set("X-Real-Ip", "1.1.1.1")
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	resp, err := client.Do(req)
	if err != nil {
		return VideoStats{}, err, true
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return VideoStats{}, err, true
	}

	htmlContent := soup.HTMLParse(string(body))

	err = checkIfAvailable(htmlContent)
	if err != nil {
		return VideoStats{}, err, false
	}

	defaultScope, err := tryDefaultScope(htmlContent)
	if err == nil {
		return defaultScope, nil, false
	} else {
		err = nil // reset error
	}

	sigi, err := trySigi(htmlContent)
	if err == nil {
		return sigi, nil, false
	} else {
		return VideoStats{}, err, false
	}
}

func checkIfAvailable(htmlContent soup.Root) error {
	content := htmlContent.Find("script", "id", "__UNIVERSAL_DATA_FOR_REHYDRATION__")
	if content.Pointer == nil {
		return nil
	}
	var jsonData map[string]interface{}

	err2 := json.Unmarshal([]byte(content.FullText()), &jsonData)
	if err2 != nil {
		return nil
	}
	lvl1, ok := jsonData["__DEFAULT_SCOPE__"].(map[string]interface{})
	if !ok {
		return nil
	}

	videoDetail, ok := lvl1["webapp.video-detail"].(map[string]interface{})
	if !ok {
		return nil
	}

	statusCode := videoDetail["statusCode"].(float64)
	if statusCode == 10204 {
		return errors.New("tt_10204")
	}

	itemInfo, ok := videoDetail["itemInfo"].(map[string]interface{})
	if !ok {
		return nil
	}
	itemStruct, ok := itemInfo["itemStruct"].(map[string]interface{})
	if !ok {
		return nil
	}
	takedown, ok := itemStruct["takedown"].(float64)
	if ok && takedown != 0 {
		return errors.New("tt_takedown")
	}

	isContentClassified, ok := itemStruct["isContentClassified"].(bool)
	if ok && isContentClassified {
		return errors.New("tt_classified")
	}

	return nil
}
func trySigi(htmlContent soup.Root) (VideoStats, error) {
	content := htmlContent.Find("script", "id", "SIGI_STATE")
	if content.Pointer == nil {
		return VideoStats{}, errors.New("nil pointer in search for SIGI_STATE")
	}
	var jsonData map[string]interface{}

	err2 := json.Unmarshal([]byte(content.FullText()), &jsonData)
	if err2 != nil {
		return VideoStats{}, err2
	}
	lvl1, ok := jsonData["ItemModule"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find ItemModule")
	}

	videoUrlEl := htmlContent.Find("meta", "property", "og:url")
	// IMPORTANT: check for nil ptr
	if videoUrlEl.Pointer == nil {
		return VideoStats{}, errors.New("cannot find the tiktok FULL URL")
	}
	videoUrl := videoUrlEl.Attrs()["content"]
	tiktokVideoId := GetTiktokVideoId(videoUrl)

	itemModule, ok := lvl1[tiktokVideoId].(map[string]interface{})

	if !ok {
		return VideoStats{}, errors.New("cannot find video id in ItemModule")
	}

	return processItemModule(itemModule)
}
func tryDefaultScope(htmlContent soup.Root) (VideoStats, error) {
	content := htmlContent.Find("script", "id", "__UNIVERSAL_DATA_FOR_REHYDRATION__")
	if content.Pointer == nil {
		return VideoStats{}, errors.New("nil pointer in search for __UNIVERSAL_DATA_FOR_REHYDRATION__")
	}
	var jsonData map[string]interface{}

	err2 := json.Unmarshal([]byte(content.FullText()), &jsonData)
	if err2 != nil {
		return VideoStats{}, err2
	}
	lvl1, ok := jsonData["__DEFAULT_SCOPE__"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find __DEFAULT_SCOPE__")
	}

	videoDetail, ok := lvl1["webapp.video-detail"].(map[string]interface{})
	if !ok {
		//fmt.Println("\n\n\n" + string(body) + "\n\n\n")
		return VideoStats{}, errors.New("cannot find webapp.video-detail")
	}

	shareMeta, ok := videoDetail["shareMeta"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find shareMeta")
	}

	desc, ok := shareMeta["desc"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find desc")
	}

	lvl3, ok := videoDetail["itemInfo"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find itemInfo")
	}

	itemStruct, ok := lvl3["itemStruct"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find itemStruct")
	}

	video, ok := itemStruct["video"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find video inside itemstruct")
	}

	dynamicCover, ok := video["dynamicCover"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find dynamicCover")
	}

	// 	// /__DEFAULT_SCOPE__/webapp.video-detail/itemInfo/itemStruct/video/subtitleInfos/2/LanguageCodeName

	subtitleInfos, subtitlesOk := video["subtitleInfos"].([]interface{})
	if !subtitlesOk {
		fmt.Println("cannot find subtitleInfos")
	}

	captionsUrl := ""
	for _, v := range subtitleInfos {
		v, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if v["LanguageCodeName"] == "eng-US" {
			captionsUrl = v["Url"].(string)
			break
		}
	}

	duration, ok := video["duration"].(float64)
	if !ok {
		return VideoStats{}, errors.New("cannot find duration")
	}

	rawStatData, ok := itemStruct["stats"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find stats")
	}

	author, ok := itemStruct["author"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find author")
	}

	username, ok := author["uniqueId"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find username")
	}

	count, ok := rawStatData["playCount"].(float64)
	if !ok {
		return VideoStats{}, errors.New("playCount not found")
	}
	viewCount := int(count)

	count, ok = rawStatData["commentCount"].(float64)
	if !ok {
		return VideoStats{}, errors.New("commentCount not found")
	}
	commentCount := int(count)

	count, ok = rawStatData["diggCount"].(float64)
	if !ok {
		return VideoStats{}, errors.New("diggCount not found")
	}
	likeCount := int(count)

	// shareCount
	count, ok = rawStatData["shareCount"].(float64)
	if !ok {
		return VideoStats{}, errors.New("shareCount not found")
	}
	shareCount := int(count)

	// saveCount
	countStr, ok := rawStatData["collectCount"].(string)
	if !ok {
		return VideoStats{}, errors.New("collectCount not found or was not string")
	}
	saveCount, err := strconv.Atoi(countStr)

	var postedAt int
	postedAtString, ok := itemStruct["createTime"].(string)
	if !ok {
		postedAtFloat := itemStruct["createTime"].(float64)
		postedAt = int(postedAtFloat)
	}
	postedAt, err = strconv.Atoi(postedAtString)
	if err != nil {
		err = nil
	}

	id, ok := itemStruct["id"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find id ")
	}

	return VideoStats{
		Platform:          VideoPlatformTiktok,
		Username:          username,
		Caption:           desc,
		FullUrl:           "https://tiktok.com/@" + username + "/video/" + id,
		Likes:             likeCount,
		Comments:          commentCount,
		Views:             viewCount,
		PostedDate:        time.Unix(int64(postedAt), 0),
		Length:            int(duration),
		Preview:           dynamicCover,
		ClosedCaptionsUrl: captionsUrl,
		Shares:            shareCount,
		Saves:             saveCount,
	}, nil
}
func processItemModule(itemModule map[string]interface{}) (VideoStats, error) {
	video, ok := itemModule["video"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find sub video inside item module")
	}

	dynamicCover, ok := video["dynamicCover"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find dynamicCover")
	}

	duration, ok := video["duration"].(float64)

	if !ok {
		return VideoStats{}, errors.New("cannot find duration")
	}

	tiktokVideoId, ok := itemModule["id"].(string)

	desc, ok := itemModule["desc"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find desc")
	}

	rawStatData, ok := itemModule["stats"].(map[string]interface{})
	if !ok {
		return VideoStats{}, errors.New("cannot find stats")
	}

	username, ok := itemModule["author"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find author")
	}

	count, ok := rawStatData["playCount"].(float64)
	if !ok {
		return VideoStats{}, errors.New("playCount not found")
	}
	viewCount := int(count)

	count, ok = rawStatData["commentCount"].(float64)
	if !ok {
		return VideoStats{}, errors.New("commentCount not found")
	}
	commentCount := int(count)

	count, ok = rawStatData["diggCount"].(float64)
	if !ok {
		return VideoStats{}, errors.New("diggCount not found")
	}
	likeCount := int(count)

	postedAtString := itemModule["createTime"].(string)
	if !ok {
		return VideoStats{}, errors.New("cannot find createTime ")
	}

	postedAt, err := strconv.Atoi(postedAtString)
	if err != nil {
		return VideoStats{}, err
	}

	return VideoStats{
		Platform:   VideoPlatformTiktok,
		Username:   username,
		Caption:    desc,
		FullUrl:    "https://tiktok.com/@" + username + "/video/" + tiktokVideoId,
		Likes:      likeCount,
		Comments:   commentCount,
		Views:      viewCount,
		PostedDate: time.Unix(int64(postedAt), 0),
		Length:     int(duration),
		Preview:    dynamicCover,
		//ClosedCaption: captions,
	}, nil
}
func getTiktokCaptions(url string) (string, error) {
	var client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url2.URL{
		Scheme: "http",
		User:   url2.UserPassword("brd-customer-hl_545d83f9-zone-datacenter_proxies_ppu-country-us-session-rand"+strconv.Itoa(rand.Int()), "jwfu2f71rc9z"),
		Host:   "brd.superproxy.io:22225",
	})}}
	//var client = &http.Client{}
	client.Timeout = time.Second * 20
	//
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
	req.Header.Set("X-Real-Ip", "1.1.1.1")
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	vtt, err := astisub.ReadFromWebVTT(resp.Body)
	if err != nil {
		return "", err
	}

	vtt.RemoveStyling()
	vtt.Unfragment()

	vttString := ""
	for _, item := range vtt.Items {
		vttString += item.String() + " "
	}

	return vttString, nil
}

type VideoPlatform string

const VideoPlatformTiktok VideoPlatform = "tiktok"
const VideoPlatformYoutube VideoPlatform = "youtube"
const VideoPlatformYoutubeShorts VideoPlatform = "shorts"

type VideoStats struct {
	Platform          VideoPlatform
	Username          string
	Caption           string
	FullUrl           string
	Likes             int
	Comments          int
	Views             int
	Shares            int
	Saves             int
	PostedDate        time.Time
	Length            int // seconds
	Preview           string
	ClosedCaptionsUrl string
	ClosedCaption     string
}
type VideoInfo struct {
	BatchTime    time.Time
	BatchCtx     context.Context
	Cancel       context.CancelFunc
	RetriedTimes int
	//Entered      time.Time
	Platform     VideoPlatform
	Username     string
	Url          string
	Caption      string
	Id           int
	SourceId     int
	SourceName   string
	AudioId      int
	AudioName    string
	VoiceId      int
	VoiceName    string
	WriterId     int
	CoWriterId   sql.NullInt64
	CoWriterName string
	WriterName   string
	SponsorId    sql.NullInt64
	SponsorName  string
	Strategies   []struct {
		Id   int
		Name string
	}
	WriterRate   int
	CoWriterRate sql.NullInt64
	SponsorRate  sql.NullInt64
	PostedDate   time.Time
	NeedsEntry   bool
	LastEmailed  int
	LastUpdated  sql.NullTime
	NeedsScript  bool
}
type WorkerError struct {
	Err   error
	Retry bool
}
type VideoInfoWithErr struct {
	VideoMetrics VideoStats
	VideoInfo    VideoInfo
	Err          error
}
