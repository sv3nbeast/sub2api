package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type rankingsRepoFunc func(context.Context, RankingsRange) ([]RankingsBucket, error)

func (f rankingsRepoFunc) Aggregate(ctx context.Context, r RankingsRange) ([]RankingsBucket, error) {
	return f(ctx, r)
}

type rankingsArchiveRepo struct {
	aggregate rankingsRepoFunc
	startedAt time.Time
	completed time.Time
}

func (r rankingsArchiveRepo) Aggregate(ctx context.Context, bounds RankingsRange) ([]RankingsBucket, error) {
	return r.aggregate(ctx, bounds)
}

func (r rankingsArchiveRepo) RankingsArchiveCoverage(context.Context) (time.Time, time.Time, error) {
	return r.startedAt, r.completed, nil
}

func TestRankingsService_YearUnavailableUntilFullRecordingWindow(t *testing.T) {
	svc := NewRankingsService(rankingsRepoFunc(func(context.Context, RankingsRange) ([]RankingsBucket, error) {
		t.Fatal("year query must not reach aggregate before archive coverage is ready")
		return nil, nil
	}))
	svc.clock = func() time.Time { return time.Date(2026, 12, 2, 12, 0, 0, 0, time.UTC) }
	svc.location = func() *time.Location { return time.UTC }
	_, err := svc.Get(context.Background(), "year")
	require.ErrorIs(t, err, ErrRankingsYearUnavailable)

	ready := NewRankingsService(rankingsArchiveRepo{
		aggregate: func(_ context.Context, bounds RankingsRange) ([]RankingsBucket, error) {
			require.True(t, bounds.UseArchive)
			require.Equal(t, time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -89), bounds.ArchiveCutoff)
			return nil, nil
		},
		startedAt: time.Date(2025, 12, 1, 12, 0, 0, 0, time.UTC),
		completed: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	})
	ready.clock = svc.clock
	ready.location = svc.location
	_, err = ready.Get(context.Background(), "year")
	require.NoError(t, err)
}

func TestRankingsPeriodRange_LocalCalendarAndDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	now := time.Date(2026, 3, 10, 12, 34, 0, 0, loc)
	for _, tc := range []struct {
		period      string
		days        int
		granularity string
	}{{"today", 1, "hour"}, {"week", 7, "day"}, {"month", 30, "day"}, {"quarter", 90, "day"}, {"year", 365, "month"}} {
		t.Run(tc.period, func(t *testing.T) {
			r, err := RankingsPeriodRange(tc.period, now, loc)
			require.NoError(t, err)
			require.Equal(t, time.Date(2026, 3, 10, 0, 0, 0, 0, loc).AddDate(0, 0, 1-tc.days), r.Start)
			require.Equal(t, r.Start.AddDate(0, 0, -tc.days), r.ComparisonStart)
			require.Equal(t, now.AddDate(0, 0, -tc.days), r.ComparisonEnd)
			require.Equal(t, "America/New_York", r.Timezone)
			require.Equal(t, tc.granularity, r.Granularity)
			// Local clock alignment is preserved even across DST transitions.
			require.Equal(t, 12, r.ComparisonEnd.Hour())
		})
	}
	_, err = RankingsPeriodRange("'month'", now, loc)
	require.ErrorIs(t, err, ErrInvalidRankingsPeriod)
	week, _ := RankingsPeriodRange("week", now, loc)
	require.Equal(t, now.UTC(), week.End.UTC())
	require.NotEqual(t, 7*24*time.Hour, week.End.Sub(week.ComparisonEnd))
}

func TestBuildRankingsSnapshot_CanonicalTokensCreatorsTiesAndComparison(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.FixedZone("test", 8*3600))
	range_, err := RankingsPeriodRange("week", now, now.Location())
	require.NoError(t, err)
	day := rankingsBucketStart(now, "day")
	rows := []RankingsBucket{
		{Bucket: day.UTC(), Model: "claude-opus-5", CreatorModel: "claude-opus-5", InputTokens: 100, OutputTokens: 20, CacheReadTokens: 200, CacheCreationTokens: 80, Requests: 2},
		{Bucket: day, Model: "gpt-5.6", CreatorModel: "gpt-5.6", InputTokens: 300, OutputTokens: 100, Requests: 3},
		{Bucket: day, Model: "my-alias", CreatorModel: "claude-fable-5-1", InputTokens: 25, Requests: 1},
		{Bucket: day, Model: "glm-5.3", CreatorModel: "glm-5.3", InputTokens: 10, Requests: 1},
		{Previous: true, Model: "gpt-5.6", InputTokens: 600, Requests: 4},
		{Previous: true, Model: "claude-opus-5", InputTokens: 200, Requests: 1},
	}
	snapshot := BuildRankingsSnapshot("week", now, range_, rows)
	require.Equal(t, int64(835), snapshot.TotalTokens)
	require.Equal(t, int64(7), snapshot.TotalRequests)
	// Equal usage is deterministically ordered by model name, across map iteration.
	require.Equal(t, "claude-opus-5", snapshot.Models[0].ModelName)
	require.Equal(t, "gpt-5.6", snapshot.Models[1].ModelName)
	require.Equal(t, int64(400), snapshot.Models[0].TotalTokens)
	require.Equal(t, int64(100), snapshot.Models[0].InputTokens)
	require.Equal(t, "anthropic", snapshot.Models[0].VendorID)
	require.InDelta(t, 400.0/835.0, snapshot.Models[0].Share, 1e-10)
	require.Equal(t, 2, *snapshot.Models[0].PreviousRank)
	require.Equal(t, 1, *snapshot.Models[0].RankDelta)
	require.InDelta(t, 100, *snapshot.Models[0].GrowthPct, 1e-10)
	require.Equal(t, -1, *snapshot.Models[1].RankDelta)
	require.InDelta(t, -100.0/3, *snapshot.Models[1].GrowthPct, 1e-10)
	require.Nil(t, snapshot.Models[2].PreviousRank)
	require.Nil(t, snapshot.Models[2].GrowthPct)
	require.Nil(t, snapshot.Models[2].RankDelta)
	require.Equal(t, "my-alias", snapshot.Models[2].ModelName)
	require.Equal(t, "anthropic", snapshot.Models[2].VendorID)
	require.Len(t, snapshot.TopMovers, 1)
	require.Len(t, snapshot.TopDroppers, 1)
	require.Equal(t, "Anthropic", snapshot.Vendors[0].Vendor)
	require.Equal(t, int64(425), snapshot.Vendors[0].TotalTokens)
	require.Equal(t, 2, snapshot.Vendors[0].ModelsCount)
	require.Equal(t, "claude-opus-5", snapshot.Vendors[0].TopModel)
	var modelSum, vendorSum int64
	for _, p := range snapshot.ModelsHistory.Points {
		modelSum += p.Tokens
		require.Equal(t, now.Location(), p.TS.Location())
	}
	for _, p := range snapshot.VendorShareHistory.Points {
		vendorSum += p.Tokens
	}
	require.Equal(t, snapshot.TotalTokens, modelSum)
	require.Equal(t, modelSum, vendorSum)
}

func TestBuildRankingsSnapshot_CapsKeepFullTotalsAndOthers(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	range_, _ := RankingsPeriodRange("month", now, time.UTC)
	rows := make([]RankingsBucket, 0, 120)
	for i := 0; i < 120; i++ {
		rows = append(rows, RankingsBucket{Bucket: rankingsBucketStart(now, "day"), Model: fmt.Sprintf("gpt-test-%03d", i), InputTokens: 1, Requests: 1})
	}
	snapshot := BuildRankingsSnapshot("month", now, range_, rows)
	require.Len(t, snapshot.Models, 100)
	require.Equal(t, int64(120), snapshot.TotalTokens)
	require.Equal(t, int64(120), snapshot.TotalRequests)
	require.Equal(t, 120, snapshot.ModelsCount)
	require.InDelta(t, 1.0/120.0, snapshot.Models[0].Share, 1e-10)
	require.Equal(t, 120, snapshot.Vendors[0].ModelsCount)
	require.Len(t, snapshot.ModelsHistory.Points, 30*9)
	var total, others int64
	for _, p := range snapshot.ModelsHistory.Points {
		total += p.Tokens
		if p.Model == "Others" {
			others += p.Tokens
		}
	}
	require.Equal(t, int64(120), total)
	require.Equal(t, int64(112), others)
}

func TestBuildRankingsSnapshot_EmptyAndZeroBaseline(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	range_, _ := RankingsPeriodRange("today", now, time.UTC)
	snapshot := BuildRankingsSnapshot("today", now, range_, nil)
	require.NotNil(t, snapshot.Models)
	require.Empty(t, snapshot.Models)
	require.NotNil(t, snapshot.Vendors)
	require.Empty(t, snapshot.ModelsHistory.Points)
	snapshot = BuildRankingsSnapshot("today", now, range_, []RankingsBucket{{Model: "new-model", InputTokens: 10}, {Model: "new-model", Previous: true}})
	require.Nil(t, snapshot.Models[0].GrowthPct)
	require.Nil(t, snapshot.Models[0].PreviousRank)
	loc, err := time.LoadLocation("Asia/Kolkata")
	require.NoError(t, err)
	require.Equal(t, 0, rankingsBucketStart(now.In(loc), "hour").Minute())
}

func TestBuildRankingsSnapshot_MultiCreatorAliasKeepsVendorBuckets(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	window, _ := RankingsPeriodRange("week", now, time.UTC)
	day := rankingsBucketStart(now, "day")
	buckets := []RankingsBucket{
		{Bucket: day, Model: "unified-model", CreatorModel: "claude-opus-5", InputTokens: 100, Requests: 2},
		{Bucket: day, Model: "unified-model", CreatorModel: "gpt-5.6", InputTokens: 60, Requests: 2},
		{Bucket: day, Model: "unified-model", CreatorModel: "claude-fable-5-1", InputTokens: 25, Requests: 1},
		{Previous: true, Model: "unified-model", CreatorModel: "claude-opus-5", InputTokens: 50},
		{Previous: true, Model: "unified-model", CreatorModel: "gpt-5.6", InputTokens: 10},
	}
	snapshot := BuildRankingsSnapshot("week", now, window, buckets)
	require.Equal(t, int64(185), snapshot.TotalTokens)
	require.Len(t, snapshot.Models, 1)
	require.Equal(t, "unified-model", snapshot.Models[0].ModelName)
	require.Equal(t, "mixed", snapshot.Models[0].VendorID)
	require.Equal(t, "Mixed", snapshot.Models[0].Vendor)
	require.Len(t, snapshot.Vendors, 2, "mixed is a display label, never a spurious creator")
	require.Equal(t, "anthropic", snapshot.Vendors[0].VendorID)
	require.Equal(t, int64(125), snapshot.Vendors[0].TotalTokens)
	require.Equal(t, int64(3), snapshot.Vendors[0].Requests)
	require.Equal(t, int64(60), snapshot.Vendors[1].TotalTokens)
	require.Equal(t, int64(2), snapshot.Vendors[1].Requests)
	require.Equal(t, 1, snapshot.Vendors[0].ModelsCount)
	require.Equal(t, 1, snapshot.Vendors[1].ModelsCount)
	require.InDelta(t, 150, *snapshot.Vendors[0].GrowthPct, 1e-10)
	require.InDelta(t, 500, *snapshot.Vendors[1].GrowthPct, 1e-10)
	var historyTotal int64
	byVendor := make(map[string]int64)
	for _, p := range snapshot.VendorShareHistory.Points {
		byVendor[p.VendorID] += p.Tokens
		historyTotal += p.Tokens
	}
	require.Equal(t, int64(125), byVendor["anthropic"])
	require.Equal(t, int64(60), byVendor["openai"])
	require.Equal(t, snapshot.TotalTokens, historyTotal)
	var modelTotal int64
	for _, p := range snapshot.ModelsHistory.Points {
		modelTotal += p.Tokens
		require.Equal(t, "mixed", p.VendorID)
	}
	require.Equal(t, snapshot.TotalTokens, modelTotal)
}

func TestBuildRankingsSnapshot_ClaudeThinkingMergesBothPeriodsWithoutMutatingUsage(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	window, err := RankingsPeriodRange("week", now, time.UTC)
	require.NoError(t, err)
	day := rankingsBucketStart(now, "day")
	buckets := []RankingsBucket{
		{Bucket: day, Model: "claude-opus-5-5", CreatorModel: "claude-opus-5-5", InputTokens: 100, OutputTokens: 20, CacheReadTokens: 200, CacheCreationTokens: 80, Requests: 2},
		{Bucket: day.AddDate(0, 0, -1), Model: "claude-opus-5-5-thinking", CreatorModel: "claude-opus-5-5-thinking", InputTokens: 150, OutputTokens: 30, CacheReadTokens: 300, CacheCreationTokens: 120, Requests: 3},
		{Bucket: day, Model: "gpt-5.6", CreatorModel: "gpt-5.6", InputTokens: 750, Requests: 2},
		{Bucket: window.ComparisonStart, Previous: true, Model: "claude-opus-5-5", CreatorModel: "claude-opus-5-5", InputTokens: 60, OutputTokens: 10, CacheReadTokens: 20, CacheCreationTokens: 10, Requests: 1},
		{Bucket: window.ComparisonStart, Previous: true, Model: "claude-opus-5-5-thinking", CreatorModel: "claude-opus-5-5-thinking", InputTokens: 240, OutputTokens: 40, CacheReadTokens: 80, CacheCreationTokens: 40, Requests: 4},
		{Bucket: window.ComparisonStart, Previous: true, Model: "gpt-5.6", CreatorModel: "gpt-5.6", InputTokens: 600, Requests: 2},
	}
	original := append([]RankingsBucket(nil), buckets...)
	snapshot := BuildRankingsSnapshot("week", now, window, buckets)
	require.Equal(t, original, buckets, "ranking normalization must not change source usage or billing fields")
	require.Equal(t, 2, snapshot.ModelsCount)
	require.Len(t, snapshot.Models, 2)
	require.Equal(t, int64(1750), snapshot.TotalTokens)
	require.Equal(t, int64(7), snapshot.TotalRequests)
	merged := snapshot.Models[0]
	require.Equal(t, "claude-opus-5-5", merged.ModelName)
	require.Equal(t, int64(250), merged.InputTokens)
	require.Equal(t, int64(50), merged.OutputTokens)
	require.Equal(t, int64(500), merged.CacheReadTokens)
	require.Equal(t, int64(200), merged.CacheCreationTokens)
	require.Equal(t, int64(1000), merged.TotalTokens)
	require.Equal(t, int64(5), merged.Requests)
	require.InDelta(t, 100, *merged.GrowthPct, 1e-10, "growth compares the merged current 1000 with merged previous 500")
	require.Equal(t, 2, *merged.PreviousRank)
	require.Equal(t, 1, *merged.RankDelta)
	require.Len(t, snapshot.TopMovers, 1)
	require.Equal(t, "claude-opus-5-5", snapshot.TopMovers[0].ModelName)
	require.Len(t, snapshot.Vendors, 2)
	claudeVendor := snapshot.Vendors[0]
	require.Equal(t, "anthropic", claudeVendor.VendorID)
	require.Equal(t, int64(1000), claudeVendor.TotalTokens)
	require.Equal(t, int64(5), claudeVendor.Requests)
	require.Equal(t, 1, claudeVendor.ModelsCount)
	require.Equal(t, "claude-opus-5-5", claudeVendor.TopModel)
	require.InDelta(t, 100, *claudeVendor.GrowthPct, 1e-10)
	var modelHistoryTotal, vendorHistoryTotal, mergedHistoryTotal int64
	for _, point := range snapshot.ModelsHistory.Points {
		require.NotContains(t, point.Model, "-thinking")
		modelHistoryTotal += point.Tokens
		if point.Model == "claude-opus-5-5" {
			mergedHistoryTotal += point.Tokens
		}
	}
	for _, point := range snapshot.VendorShareHistory.Points {
		vendorHistoryTotal += point.Tokens
	}
	require.Equal(t, int64(1000), mergedHistoryTotal)
	require.Equal(t, snapshot.TotalTokens, modelHistoryTotal)
	require.Equal(t, snapshot.TotalTokens, vendorHistoryTotal)
}

func TestBuildRankingsSnapshot_ThinkingNormalizationSupportsFutureClaudeOnly(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	window, err := RankingsPeriodRange("today", now, time.UTC)
	require.NoError(t, err)
	for _, tc := range []struct {
		name  string
		merge bool
	}{
		{name: "claude-opus-8-2", merge: true},
		{name: "claude-sonnet-4-20250514", merge: true},
		{name: "claude-haiku-9", merge: true},
		{name: "gpt-5.6", merge: false},
		{name: "custom-thing", merge: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buckets := []RankingsBucket{
				{Bucket: window.Start, Model: tc.name, InputTokens: 10, Requests: 1},
				{Bucket: window.Start, Model: tc.name + "-thinking", InputTokens: 20, Requests: 2},
			}
			snapshot := BuildRankingsSnapshot("today", now, window, buckets)
			require.Equal(t, int64(30), snapshot.TotalTokens)
			require.Equal(t, int64(3), snapshot.TotalRequests)
			if tc.merge {
				require.Equal(t, 1, snapshot.ModelsCount)
				require.Len(t, snapshot.Models, 1)
				require.Equal(t, tc.name, snapshot.Models[0].ModelName)
				require.Equal(t, int64(30), snapshot.Models[0].InputTokens)
				require.Equal(t, int64(3), snapshot.Models[0].Requests)
				require.Equal(t, 1, snapshot.Vendors[0].ModelsCount)
			} else {
				require.Equal(t, 2, snapshot.ModelsCount)
				require.Len(t, snapshot.Models, 2)
				require.Equal(t, tc.name+"-thinking", snapshot.Models[0].ModelName)
				require.Equal(t, tc.name, snapshot.Models[1].ModelName)
				require.Equal(t, 2, snapshot.Vendors[0].ModelsCount)
			}
		})
	}
}

func TestBuildRankingsSnapshot_FallBackHoursRemainDistinct(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	now := time.Date(2026, 11, 1, 23, 59, 59, 0, loc)
	window, _ := RankingsPeriodRange("today", now, loc)
	// 01:00 occurs twice: EDT (-04:00), then EST (-05:00).
	first := time.Date(2026, 11, 1, 5, 0, 0, 0, time.UTC).In(loc)
	second := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC).In(loc)
	require.Equal(t, 1, first.Hour())
	require.Equal(t, 1, second.Hour())
	snapshot := BuildRankingsSnapshot("today", now, window, []RankingsBucket{
		{Bucket: first, Model: "gpt-5.6", InputTokens: 10, Requests: 1},
		{Bucket: second, Model: "gpt-5.6", InputTokens: 20, Requests: 1},
	})
	require.Len(t, snapshot.ModelsHistory.Points, 25)
	var repeated []RankingsHistoryPoint
	var total int64
	for _, p := range snapshot.ModelsHistory.Points {
		total += p.Tokens
		if p.Label == "01:00" {
			repeated = append(repeated, p)
		}
	}
	require.Len(t, repeated, 2)
	require.Equal(t, time.Hour, repeated[1].TS.Sub(repeated[0].TS))
	require.Equal(t, int64(10), repeated[0].Tokens)
	require.Equal(t, int64(20), repeated[1].Tokens)
	require.Equal(t, int64(30), total)
	var vendorsTotal int64
	for _, p := range snapshot.VendorShareHistory.Points {
		vendorsTotal += p.Tokens
	}
	require.Equal(t, total, vendorsTotal)
}

func TestRankingsService_CacheSingleflightAndDisconnect(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	svc := NewRankingsService(rankingsRepoFunc(func(ctx context.Context, r RankingsRange) ([]RankingsBucket, error) {
		calls.Add(1)
		close(started)
		<-release
		require.NoError(t, ctx.Err())
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), rankingsQueryTimeout)
		return []RankingsBucket{{Bucket: rankingsBucketStart(r.End, r.Granularity), Model: "gpt-5.6", InputTokens: 10}}, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	disconnected := make(chan error, 1)
	go func() { _, err := svc.Get(ctx, "week"); disconnected <- err }()
	<-started
	cancel()
	require.ErrorIs(t, <-disconnected, context.Canceled)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot, err := svc.Get(context.Background(), "week")
			require.NoError(t, err)
			require.Equal(t, int64(10), snapshot.TotalTokens)
		}()
	}
	close(release)
	wg.Wait()
	require.Equal(t, int32(1), calls.Load())
	_, err := svc.Get(context.Background(), "malicious")
	require.ErrorIs(t, err, ErrInvalidRankingsPeriod)
	require.Equal(t, int32(1), calls.Load())
}

func TestRankingsService_ExpiryMidnightAndNegativeCache(t *testing.T) {
	now := time.Date(2026, 10, 2, 23, 59, 40, 0, time.UTC)
	var calls int
	svc := NewRankingsService(rankingsRepoFunc(func(context.Context, RankingsRange) ([]RankingsBucket, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("unavailable")
		}
		return nil, nil
	}))
	svc.clock = func() time.Time { return now }
	svc.location = func() *time.Location { return time.UTC }
	_, err := svc.Get(context.Background(), "week")
	require.Error(t, err)
	_, err = svc.Get(context.Background(), "week")
	require.Error(t, err)
	require.Equal(t, 1, calls)
	now = now.Add(6 * time.Second)
	first, err := svc.Get(context.Background(), "week")
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	second, err := svc.Get(context.Background(), "week")
	require.NoError(t, err)
	require.Same(t, first, second)
	now = now.Add(20 * time.Second)
	_, err = svc.Get(context.Background(), "week")
	require.NoError(t, err)
	require.Equal(t, 3, calls, "cache must not cross local midnight")
	now = now.Add(time.Minute)
	_, err = svc.Get(context.Background(), "week")
	require.NoError(t, err)
	require.Equal(t, 4, calls)
}

func TestRankingsService_QueueTimeoutIsBoundedAndNegativelyCached(t *testing.T) {
	var calls atomic.Int32
	svc := NewRankingsService(rankingsRepoFunc(func(context.Context, RankingsRange) ([]RankingsBucket, error) { calls.Add(1); return nil, nil }))
	svc.queryTimeout = 20 * time.Millisecond
	svc.querySlot <- struct{}{}
	_, err := svc.Get(context.Background(), "quarter")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	<-svc.querySlot
	// Even after the slot opens, a burst of retries must not start another scan.
	_, err = svc.Get(context.Background(), "quarter")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int32(0), calls.Load())
}

func TestRankingsService_DifferentPeriodsShareOneDatabaseSlot(t *testing.T) {
	var active, maxActive atomic.Int32
	started, release := make(chan struct{}, 4), make(chan struct{})
	svc := NewRankingsService(rankingsRepoFunc(func(ctx context.Context, r RankingsRange) ([]RankingsBucket, error) {
		current := active.Add(1)
		for {
			old := maxActive.Load()
			if current <= old || maxActive.CompareAndSwap(old, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		return nil, nil
	}))
	var wg sync.WaitGroup
	for _, period := range []string{"today", "week", "month", "quarter"} {
		wg.Add(1)
		go func(p string) { defer wg.Done(); _, err := svc.Get(context.Background(), p); require.NoError(t, err) }(period)
	}
	<-started
	// Three other distinct periods are allowed to wait, never to scan in parallel.
	close(release)
	wg.Wait()
	require.Equal(t, int32(1), maxActive.Load())
}

func TestRankingsModelVendor_CreatorsAndNamespacedModels(t *testing.T) {
	for model, id := range map[string]string{"claude-opus-5": "anthropic", "us.anthropic.claude-opus-5": "anthropic", "openai/gpt-5.6": "openai", "o3": "openai", "gemini-3-pro": "google", "grok-4.3": "xai", "kimi-k3": "moonshot", "glm-5.3": "zhipu", "deepseek-v4": "deepseek", "qwen3": "alibaba", "unknown": "unknown"} {
		got, _ := RankingsModelVendor(model)
		require.Equal(t, id, got, model)
	}
}
