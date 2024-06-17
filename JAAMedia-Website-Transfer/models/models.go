package models

import (
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/exp/constraints"
	"strconv"
	"time"
)

const WriterRate = 20

type Source struct {
	Name string
	Id   int
}
type Strategy struct {
	Name     string
	Id       int
	Variable string
	Value    string
}

type VaAddVideoViewPageData struct {
	VideosNeedEntry      []VideoListItem
	VideosAlreadyEntered []VideoListItem
}

type VideoListItem struct {
	Id              int
	Url             string
	Writer          Writer
	Sponsor         Sponsor
	SponsorRate     int
	WriterRate      int
	Created         time.Time
	Account         Account
	Va              VaWithDetail
	AccountRate     int
	AccountRateType string
	Graph           string
	ViewCount       pgtype.Int8
	LikeCount       pgtype.Int8
	LikeRate        pgtype.Float8
	CommentCount    pgtype.Int8
	CommentRate     pgtype.Float8
	Cost            int
	Title           string
	NeedsEntry      bool
	PostedDate      pgtype.Timestamp
	Preview         string
	ScrapedScript   string
	Error           pgtype.Text
}

type Writer struct {
	Id   int
	Name string
}

type Sponsor struct {
	Id   int
	Name string
}

type Audio struct {
	Id   int
	Name string
}

type VaListView struct {
	Vas []VaWithDetail
}

type VaWithDetail struct {
	Id            int
	Name          string
	Email         string
	VideosEntered int
}

type Account struct {
	Id           int
	Platform     string
	Username     string
	TotalViews   int
	Views24H     int
	Views1H      int
	RevenueTotal int
	Revenue24H   int
	Revenue1H    int
	VideoCount   int
}

type Voice struct {
	Id   int
	Name string
}

type SponsorWithDetail struct {
	Id         int
	Name       string
	Email      string
	VideoCount int
}

type WriterWithDetail struct {
	Id         int
	Name       string
	Email      string
	VideoCount int
}

type SponsorListView struct {
	Sponsors []SponsorWithDetail
}

type WriterListView struct {
	Writers []WriterWithDetail
}

type VideoPrefill struct {
	Id              int
	EnteredBy       int
	Created         time.Time
	SponsorId       sql.NullInt64
	SponsorRate     sql.NullInt64
	WriterId        int
	WriterRate      int
	AccountId       int
	Url             string
	VoiceId         int
	AudioId         int
	SourceId        int
	AccountRate     int
	AccountRateType string
	Strategies      StrategiesArray
	CoWriterId      sql.NullInt64
	CoWriterRate    sql.NullInt64
}

type VaEditVideoViewPageData struct {
	Video      VideoPrefill
	Sponsors   []Sponsor
	Audios     []Audio
	Voices     []Voice
	Writers    []Writer
	Sources    []Source
	Strategies StrategiesArray
}

type StrategiesArray []Strategy
type NullInt64 sql.NullInt64

type SheetsPluginData struct {
	Sponsors []SheetsPluginDataItem `json:"sponsors"`
	Audios   []SheetsPluginDataItem `json:"audios"`
	Voices   []SheetsPluginDataItem `json:"voices"`
	Writers  []SheetsPluginDataItem `json:"writers"`
	Sources  []SheetsPluginDataItem `json:"sources"`
}

type SheetsPluginDataItem struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func (strategies StrategiesArray) Contains(strat Strategy) bool {
	for _, v := range strategies {
		if v.Id == strat.Id {
			return true
		}
	}
	return false
}

func (s NullInt64) HtmlValue() string {
	if s.Valid {
		return strconv.Itoa(int(s.Int64))
	} else {
		return ""
	}
}

type Instant struct {
	Percent int
	Seconds int
}

type TrackingError struct {
	Url       string
	Message   string
	LastSeen  time.Time
	Frequency int
}

type Number interface {
	constraints.Integer | constraints.Float
}

func FormattedWithSuffix(s int64) string {
	suffixes := [4]string{"", "K", "M", "B"} // Add more suffixes as needed
	var i int
	for i = 0; s >= 1000 && i < len(suffixes)-1; i++ {
		s /= 1000
	}
	return fmt.Sprintf("%.1d%s", s, suffixes[i])
}

type VideoEntryPage struct {
	NeedsEntry         bool
	VarStrategyOptions map[string][]Strategy
	PresetUrl          string
	PresetStoryLink    string
	PresetStoryCode    string
	PresetId           int
	PresetSponsorId    int
	PresetWriterId     int
	PresetVoiceId      int
	PresetAudioId      int
	PresetSourceId     int
	PresetSponsorRate  int
	PresetWriterRate   int
	PresetCoWriterId   int
	PresetCoWriterRate int
	PresetStrategies   StrategiesArray
	Sponsors           []Sponsor
	Audios             []Audio
	Voices             []Voice
	Writers            []Writer
	Sources            []Source
	ShowMetrics        bool
	ScrapedScript      string
	ViewCount          int64
	LikeCount          int64
	CommentCount       int64
	SaveCount          int64
	ShareCount         int64
}
