package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"golang.org/x/sync/singleflight"
)

var ErrInvalidRankingsPeriod = errors.New("period must be today, week, month, quarter or year")
var ErrRankingsYearUnavailable = errors.New("year rankings are not available until a full year has been recorded")

const (
	rankingsMaxModels      = 100
	rankingsHistoryModels  = 8
	rankingsHistoryVendors = 8
	rankingsQueryTimeout   = 15 * time.Second
)

// RankingsRange uses half-open bounds. Comparisons end at the same local time
// in the previous period, so an incomplete current day is compared fairly.
type RankingsRange struct {
	Start, End, ComparisonStart, ComparisonEnd time.Time
	Timezone, Granularity                      string
	UseArchive                                 bool
	ArchiveCutoff                              time.Time
}

type RankingsBucket struct {
	Bucket                                                                    time.Time
	Model, CreatorModel                                                       string
	Previous                                                                  bool
	InputTokens, OutputTokens, CacheReadTokens, CacheCreationTokens, Requests int64
}

type RankingsRepository interface {
	Aggregate(context.Context, RankingsRange) ([]RankingsBucket, error)
}

// RankingsArchiveCoverage is optional so lightweight test repositories and
// deployments without the history migration fail closed for the year view.
type RankingsArchiveCoverage interface {
	RankingsArchiveCoverage(context.Context) (startedAt time.Time, lastCompletedDate time.Time, err error)
}

type RankingsModel struct {
	Rank                int      `json:"rank"`
	PreviousRank        *int     `json:"previous_rank"`
	ModelName           string   `json:"model_name"`
	Vendor              string   `json:"vendor"`
	VendorID            string   `json:"vendor_id"`
	TotalTokens         int64    `json:"total_tokens"`
	InputTokens         int64    `json:"input_tokens"`
	OutputTokens        int64    `json:"output_tokens"`
	CacheReadTokens     int64    `json:"cache_read_tokens"`
	CacheCreationTokens int64    `json:"cache_creation_tokens"`
	Requests            int64    `json:"requests"`
	Share               float64  `json:"share"`
	GrowthPct           *float64 `json:"growth_pct"`
	RankDelta           *int     `json:"rank_delta"`
}

type RankingsVendor struct {
	Rank         int      `json:"rank"`
	PreviousRank *int     `json:"previous_rank"`
	Vendor       string   `json:"vendor"`
	VendorID     string   `json:"vendor_id"`
	TotalTokens  int64    `json:"total_tokens"`
	Share        float64  `json:"share"`
	GrowthPct    *float64 `json:"growth_pct"`
	ModelsCount  int      `json:"models_count"`
	TopModel     string   `json:"top_model"`
	Requests     int64    `json:"requests"`
}

type RankingsHistoryPoint struct {
	TS       time.Time `json:"ts"`
	Label    string    `json:"label"`
	Model    string    `json:"model,omitempty"`
	Vendor   string    `json:"vendor"`
	VendorID string    `json:"vendor_id"`
	Tokens   int64     `json:"tokens"`
}

type RankingsHistory struct {
	Points []RankingsHistoryPoint `json:"points"`
}

type RankingsSnapshot struct {
	Period             string           `json:"period"`
	GeneratedAt        time.Time        `json:"generated_at"`
	StartAt            time.Time        `json:"start_at"`
	EndAt              time.Time        `json:"end_at"`
	Timezone           string           `json:"timezone"`
	ComparisonStartAt  time.Time        `json:"comparison_start_at"`
	ComparisonEndAt    time.Time        `json:"comparison_end_at"`
	TotalTokens        int64            `json:"total_tokens"`
	TotalRequests      int64            `json:"total_requests"`
	ModelsCount        int              `json:"models_count"`
	Models             []RankingsModel  `json:"models"`
	Vendors            []RankingsVendor `json:"vendors"`
	TopMovers          []RankingsModel  `json:"top_movers"`
	TopDroppers        []RankingsModel  `json:"top_droppers"`
	ModelsHistory      RankingsHistory  `json:"models_history"`
	VendorShareHistory RankingsHistory  `json:"vendor_share_history"`
}

type rankingsCacheEntry struct {
	snapshot *RankingsSnapshot
	expires  time.Time
	err      error
}

// RankingsService caches immutable snapshots and shares in-flight work.
// A single database slot also prevents four different public period requests
// from concurrently scanning usage history. Nothing is added to usage writes.
type RankingsService struct {
	repo         RankingsRepository
	clock        func() time.Time
	location     func() *time.Location
	mu           sync.RWMutex
	cache        map[string]rankingsCacheEntry
	flights      singleflight.Group
	querySlot    chan struct{}
	queryTimeout time.Duration
}

func NewRankingsService(repo RankingsRepository) *RankingsService {
	return &RankingsService{repo: repo, clock: time.Now, location: timezone.Location,
		cache: make(map[string]rankingsCacheEntry), querySlot: make(chan struct{}, 1), queryTimeout: rankingsQueryTimeout}
}

func (s *RankingsService) Get(ctx context.Context, period string) (*RankingsSnapshot, error) {
	if period == "" {
		period = "week"
	}
	now := s.clock()
	window, err := RankingsPeriodRange(period, now, s.location())
	if err != nil {
		return nil, err
	}
	if period == "year" {
		coverage, ok := s.repo.(RankingsArchiveCoverage)
		if !ok {
			return nil, ErrRankingsYearUnavailable
		}
		startedAt, lastCompletedDate, coverageErr := coverage.RankingsArchiveCoverage(ctx)
		if coverageErr != nil {
			return nil, coverageErr
		}
		localNow := now.In(s.location())
		availableAt := time.Date(startedAt.In(s.location()).Year(), startedAt.In(s.location()).Month(), startedAt.In(s.location()).Day(), 0, 0, 0, 0, s.location()).AddDate(1, 0, 0)
		if startedAt.IsZero() || localNow.Before(availableAt) || lastCompletedDate.Before(window.Start.In(s.location()).AddDate(0, 0, -1)) {
			return nil, ErrRankingsYearUnavailable
		}
		window.UseArchive = true
		today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, s.location())
		window.ArchiveCutoff = today.AddDate(0, 0, -89)
	}
	key := period + ":" + window.Timezone + ":" + window.End.Format("2006-01-02")
	if entry, ok := s.cached(key, now); ok {
		return entry.snapshot, entry.err
	}
	result := s.flights.DoChan(key, func() (any, error) {
		if entry, ok := s.cached(key, s.clock()); ok {
			return entry.snapshot, entry.err
		}
		// The initiating HTTP disconnect must not cancel work needed by other
		// waiters. The independent timeout bounds both queueing and database work.
		queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.queryTimeout)
		defer cancel()
		var rows []RankingsBucket
		var queryErr error
		select {
		case s.querySlot <- struct{}{}:
			defer func() { <-s.querySlot }()
			rows, queryErr = s.repo.Aggregate(queryCtx, window)
		case <-queryCtx.Done():
			queryErr = queryCtx.Err()
		}
		entry := rankingsCacheEntry{err: queryErr, expires: s.clock().Add(5 * time.Second)}
		if queryErr == nil {
			entry.snapshot = BuildRankingsSnapshot(period, now, window, rows)
			entry.expires = s.clock().Add(rankingsCacheTTL(period))
		}
		s.mu.Lock()
		// Only four current-date entries are retained, including errors.
		for oldKey, old := range s.cache {
			if !old.expires.After(s.clock()) {
				delete(s.cache, oldKey)
			}
		}
		s.cache[key] = entry
		s.mu.Unlock()
		return entry.snapshot, queryErr
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-result:
		if res.Err != nil {
			return nil, res.Err
		}
		return res.Val.(*RankingsSnapshot), nil
	}
}

func (s *RankingsService) cached(key string, now time.Time) (rankingsCacheEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.cache[key]
	return entry, ok && entry.expires.After(now)
}

func rankingsCacheTTL(period string) time.Duration {
	switch period {
	case "year":
		return 15 * time.Minute
	case "month":
		return 5 * time.Minute
	default:
		return time.Minute
	}
}

// RankingsPeriodRange resolves a supported period in the server's timezone.
func RankingsPeriodRange(period string, now time.Time, loc *time.Location) (RankingsRange, error) {
	now = now.In(loc)
	days, granularity := 0, "day"
	switch period {
	case "today":
		days, granularity = 1, "hour"
	case "week":
		days = 7
	case "month":
		days = 30
	case "quarter":
		days = 90
	case "year":
		days, granularity = 365, "month"
	default:
		return RankingsRange{}, ErrInvalidRankingsPeriod
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := today.AddDate(0, 0, 1-days)
	return RankingsRange{Start: start, End: now,
		ComparisonStart: start.AddDate(0, 0, -days), ComparisonEnd: now.AddDate(0, 0, -days),
		Timezone: loc.String(), Granularity: granularity}, nil
}

// RankingsModelVendor attributes usage to the model creator, regardless of
// whether Claude was served by an Anthropic, Kiro, Bedrock or other account.
func RankingsModelVendor(model string) (id, name string) {
	m := strings.ToLower(strings.TrimSpace(model))
	// Accept common namespace forms (anthropic/claude, us.anthropic.claude).
	for _, prefix := range []string{"claude", "gpt", "chatgpt", "o1", "o3", "o4", "codex", "gemini", "grok", "kimi", "moonshot", "glm", "deepseek", "minimax", "qwen", "llama", "mistral"} {
		if strings.HasPrefix(m, prefix) || strings.Contains(m, "/"+prefix) || strings.Contains(m, "."+prefix) {
			switch prefix {
			case "claude":
				return "anthropic", "Anthropic"
			case "gpt", "chatgpt", "o1", "o3", "o4", "codex":
				return "openai", "OpenAI"
			case "gemini":
				return "google", "Google"
			case "grok":
				return "xai", "xAI"
			case "kimi", "moonshot":
				return "moonshot", "Moonshot AI"
			case "glm":
				return "zhipu", "Zhipu AI"
			case "deepseek":
				return "deepseek", "DeepSeek"
			case "minimax":
				return "minimax", "MiniMax"
			case "qwen":
				return "alibaba", "Alibaba"
			case "llama":
				return "meta", "Meta"
			case "mistral":
				return "mistral", "Mistral AI"
			}
		}
	}
	return "unknown", "Other"
}

// BuildRankingsSnapshot converts anonymous aggregate buckets into a snapshot.
// It is also usable by exports without HTTP or gateway dependencies.
func BuildRankingsSnapshot(period string, now time.Time, window RankingsRange, buckets []RankingsBucket) *RankingsSnapshot {
	// Normalize before all aggregations, including the previous period and
	// histories. Work on a copy so exports and shared repository data keep
	// their original request model names.
	normalized := make([]RankingsBucket, len(buckets))
	for i, bucket := range buckets {
		bucket.Model = rankingsModelName(bucket.Model)
		normalized[i] = bucket
	}
	buckets = normalized
	snapshot := &RankingsSnapshot{Period: period, GeneratedAt: now, StartAt: window.Start, EndAt: window.End,
		Timezone: window.Timezone, ComparisonStartAt: window.ComparisonStart, ComparisonEndAt: window.ComparisonEnd,
		Models: []RankingsModel{}, Vendors: []RankingsVendor{}, TopMovers: []RankingsModel{}, TopDroppers: []RankingsModel{},
		ModelsHistory: RankingsHistory{Points: []RankingsHistoryPoint{}}, VendorShareHistory: RankingsHistory{Points: []RankingsHistoryPoint{}}}
	current, previous := make(map[string]*RankingsModel), make(map[string]*RankingsModel)
	for _, b := range buckets {
		target := current
		if b.Previous {
			target = previous
		}
		row, ok := target[b.Model]
		vendorID, vendor := rankingsBucketVendor(b)
		if !ok {
			row = &RankingsModel{ModelName: b.Model, Vendor: vendor, VendorID: vendorID}
			target[b.Model] = row
		} else if row.VendorID != vendorID {
			// A final model can be served by multiple upstream creators. Keep the
			// model row grouped by its final name while vendor statistics retain
			// each bucket's own creator rather than attributing everything to the
			// first one.
			row.VendorID, row.Vendor = "mixed", "Mixed"
		}
		row.InputTokens += b.InputTokens
		row.OutputTokens += b.OutputTokens
		row.CacheReadTokens += b.CacheReadTokens
		row.CacheCreationTokens += b.CacheCreationTokens
		row.TotalTokens += bucketTokens(b)
		row.Requests += b.Requests
	}
	models := sortedRankingsModels(current)
	_ = sortedRankingsModels(previous)
	snapshot.ModelsCount = len(models)
	for _, row := range models {
		snapshot.TotalTokens += row.TotalTokens
		snapshot.TotalRequests += row.Requests
	}
	for i := range models {
		row := &models[i]
		row.Share = rankingsShare(row.TotalTokens, snapshot.TotalTokens)
		if prev, ok := previous[row.ModelName]; ok && prev.TotalTokens > 0 {
			row.PreviousRank = rankingsIntPtr(prev.Rank)
			row.RankDelta = rankingsIntPtr(prev.Rank - row.Rank)
			growth := (float64(row.TotalTokens)/float64(prev.TotalTokens) - 1) * 100
			row.GrowthPct = &growth
		}
	}
	vendors, prevVendors := rankingsVendors(buckets, false, snapshot.TotalTokens), rankingsVendors(buckets, true, 0)
	prevVendorMap := make(map[string]RankingsVendor)
	for _, v := range prevVendors {
		prevVendorMap[v.VendorID] = v
	}
	for i := range vendors {
		if prev, ok := prevVendorMap[vendors[i].VendorID]; ok && prev.TotalTokens > 0 {
			vendors[i].PreviousRank = rankingsIntPtr(prev.Rank)
			growth := (float64(vendors[i].TotalTokens)/float64(prev.TotalTokens) - 1) * 100
			vendors[i].GrowthPct = &growth
		}
	}
	snapshot.Models = models
	if len(snapshot.Models) > rankingsMaxModels {
		snapshot.Models = snapshot.Models[:rankingsMaxModels]
	}
	snapshot.Vendors = vendors
	for _, row := range models {
		if row.RankDelta == nil {
			continue
		}
		if *row.RankDelta > 0 {
			snapshot.TopMovers = append(snapshot.TopMovers, row)
		}
		if *row.RankDelta < 0 {
			snapshot.TopDroppers = append(snapshot.TopDroppers, row)
		}
	}
	sort.Slice(snapshot.TopMovers, func(i, j int) bool {
		a, b := snapshot.TopMovers[i], snapshot.TopMovers[j]
		if *a.RankDelta != *b.RankDelta {
			return *a.RankDelta > *b.RankDelta
		}
		return a.Rank < b.Rank
	})
	sort.Slice(snapshot.TopDroppers, func(i, j int) bool {
		a, b := snapshot.TopDroppers[i], snapshot.TopDroppers[j]
		if *a.RankDelta != *b.RankDelta {
			return *a.RankDelta < *b.RankDelta
		}
		return a.Rank < b.Rank
	})
	if len(snapshot.TopMovers) > 5 {
		snapshot.TopMovers = snapshot.TopMovers[:5]
	}
	if len(snapshot.TopDroppers) > 5 {
		snapshot.TopDroppers = snapshot.TopDroppers[:5]
	}
	snapshot.ModelsHistory, snapshot.VendorShareHistory = rankingsHistories(window, models, vendors, buckets)
	return snapshot
}

// Claude thinking variants select a reasoning mode of the same base model.
// Keep this display grouping local to rankings; other providers and custom
// aliases retain their names rather than being replaced with upstream routes.
func rankingsModelName(model string) string {
	model = strings.TrimSpace(model)
	const thinkingSuffix = "-thinking"
	if strings.HasSuffix(strings.ToLower(model), thinkingSuffix) {
		if vendorID, _ := RankingsModelVendor(model); vendorID == "anthropic" {
			return model[:len(model)-len(thinkingSuffix)]
		}
	}
	return model
}

func bucketTokens(b RankingsBucket) int64 {
	return b.InputTokens + b.OutputTokens + b.CacheReadTokens + b.CacheCreationTokens
}
func rankingsIntPtr(n int) *int { return &n }
func rankingsShare(n, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(n) / float64(total)
}

func sortedRankingsModels(rows map[string]*RankingsModel) []RankingsModel {
	result := make([]RankingsModel, 0, len(rows))
	for _, r := range rows {
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalTokens != result[j].TotalTokens {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].ModelName < result[j].ModelName
	})
	for i := range result {
		result[i].Rank = i + 1
		rows[result[i].ModelName].Rank = i + 1
	}
	return result
}

func rankingsBucketVendor(bucket RankingsBucket) (id, name string) {
	id, name = RankingsModelVendor(bucket.Model)
	if id == "unknown" {
		id, name = RankingsModelVendor(bucket.CreatorModel)
	}
	return id, name
}

func rankingsVendors(buckets []RankingsBucket, previous bool, total int64) []RankingsVendor {
	byID := make(map[string]*RankingsVendor)
	modelTokens := make(map[string]map[string]int64)
	for _, b := range buckets {
		if b.Previous != previous {
			continue
		}
		id, name := rankingsBucketVendor(b)
		v, ok := byID[id]
		if !ok {
			v = &RankingsVendor{Vendor: name, VendorID: id}
			byID[id] = v
			modelTokens[id] = make(map[string]int64)
		}
		v.TotalTokens += bucketTokens(b)
		v.Requests += b.Requests
		modelTokens[id][b.Model] += bucketTokens(b)
	}
	result := make([]RankingsVendor, 0, len(byID))
	for _, v := range byID {
		v.ModelsCount = len(modelTokens[v.VendorID])
		var topTokens int64 = -1
		for model, tokens := range modelTokens[v.VendorID] {
			if tokens > topTokens || (tokens == topTokens && model < v.TopModel) {
				v.TopModel, topTokens = model, tokens
			}
		}
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalTokens != result[j].TotalTokens {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].VendorID < result[j].VendorID
	})
	for i := range result {
		result[i].Rank = i + 1
		result[i].Share = rankingsShare(result[i].TotalTokens, total)
	}
	return result
}

func rankingsHistories(window RankingsRange, models []RankingsModel, vendors []RankingsVendor, buckets []RankingsBucket) (RankingsHistory, RankingsHistory) {
	modelHistory, vendorHistory := RankingsHistory{Points: []RankingsHistoryPoint{}}, RankingsHistory{Points: []RankingsHistoryPoint{}}
	if len(models) == 0 {
		return modelHistory, vendorHistory
	}
	modelIDs, vendorIDs := make(map[string]RankingsModel), make(map[string]RankingsVendor)
	for i, m := range models {
		if i < rankingsHistoryModels {
			modelIDs[m.ModelName] = m
		}
	}
	for i, v := range vendors {
		if i < rankingsHistoryVendors {
			vendorIDs[v.VendorID] = v
		}
	}
	type seriesKey struct{ ts, id string }
	modelValues, vendorValues := make(map[seriesKey]int64), make(map[seriesKey]int64)
	for _, b := range buckets {
		if b.Previous {
			continue
		}
		modelID := b.Model
		if _, ok := modelIDs[modelID]; !ok {
			modelID = "__others__"
		}
		vendorID, _ := rankingsBucketVendor(b)
		if _, ok := vendorIDs[vendorID]; !ok {
			vendorID = "__others__"
		}
		ts := b.Bucket.In(window.Start.Location()).Format(time.RFC3339)
		modelValues[seriesKey{ts, modelID}] += bucketTokens(b)
		vendorValues[seriesKey{ts, vendorID}] += bucketTokens(b)
	}
	for ts := rankingsBucketStart(window.Start, window.Granularity); ts.Before(window.End); ts = rankingsNextBucket(ts, window.Granularity) {
		label := ts.Format("01-02")
		if window.Granularity == "hour" {
			label = ts.Format("15:04")
		}
		if window.Granularity == "month" {
			label = ts.Format("2006-01")
		}
		keyTS := ts.Format(time.RFC3339)
		for i, m := range models {
			if i >= rankingsHistoryModels {
				break
			}
			modelHistory.Points = append(modelHistory.Points, RankingsHistoryPoint{TS: ts, Label: label, Model: m.ModelName, Vendor: m.Vendor, VendorID: m.VendorID, Tokens: modelValues[seriesKey{keyTS, m.ModelName}]})
		}
		if len(models) > rankingsHistoryModels {
			modelHistory.Points = append(modelHistory.Points, RankingsHistoryPoint{TS: ts, Label: label, Model: "Others", Vendor: "Other", VendorID: "others", Tokens: modelValues[seriesKey{keyTS, "__others__"}]})
		}
		for i, v := range vendors {
			if i >= rankingsHistoryVendors {
				break
			}
			vendorHistory.Points = append(vendorHistory.Points, RankingsHistoryPoint{TS: ts, Label: label, Vendor: v.Vendor, VendorID: v.VendorID, Tokens: vendorValues[seriesKey{keyTS, v.VendorID}]})
		}
		if len(vendors) > rankingsHistoryVendors {
			vendorHistory.Points = append(vendorHistory.Points, RankingsHistoryPoint{TS: ts, Label: label, Vendor: "Others", VendorID: "others", Tokens: vendorValues[seriesKey{keyTS, "__others__"}]})
		}
	}
	return modelHistory, vendorHistory
}

func rankingsBucketStart(t time.Time, granularity string) time.Time {
	switch granularity {
	case "hour":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}
func rankingsNextBucket(t time.Time, granularity string) time.Time {
	switch granularity {
	case "hour":
		return t.Add(time.Hour)
	case "month":
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}
