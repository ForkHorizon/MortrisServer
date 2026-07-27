package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/jackc/pgx/v5/pgxpool"
)

// topMoversLimit bounds how many events (beyond the flagged anomalies) get
// sent to Claude as "movers worth knowing about even if not alert-worthy".
const topMoversLimit = 5

// ErrAIUnavailable means no Anthropic API key was configured — an
// AI-backed endpoint (digest, NL query) exists but can't run.
var ErrAIUnavailable = errors.New("this feature requires an Anthropic API key")

type DigestResult struct {
	Narration string         `json:"narration"`
	Anomalies []EventAnomaly `json:"anomalies"`
	TopMovers []EventAnomaly `json:"top_movers"`
}

// GetDigest narrates today's anomalies (see GetAnomalies) plus the
// broader set of top-moving events in three sentences: what changed and
// where to look. This is only worth calling because the anomaly detection
// in GetAnomalies already did the real statistical work — Claude narrates
// signals that were computed in SQL/Go, it doesn't invent them (Phase 5 #3).
func GetDigest(ctx context.Context, pool *pgxpool.Pool, client *anthropic.Client, projectID string, loc *time.Location) (*DigestResult, error) {
	if client == nil {
		return nil, ErrAIUnavailable
	}

	stats, err := computeEventStats(ctx, pool, projectID, loc)
	if err != nil {
		return nil, err
	}
	anomalies, movers := selectAnomaliesAndMovers(stats)

	if len(anomalies) == 0 && len(movers) == 0 {
		return &DigestResult{Narration: "No notable event-volume changes today.", Anomalies: anomalies, TopMovers: movers}, nil
	}

	narration, err := narrateDigest(ctx, client, anomalies, movers)
	if err != nil {
		return nil, err
	}
	return &DigestResult{Narration: narration, Anomalies: anomalies, TopMovers: movers}, nil
}

// selectAnomaliesAndMovers splits the full per-event stats into the
// alert-worthy subset (anomalies) and the broader top-N by |z-score|
// (movers, which may overlap with anomalies) that give Claude context
// even for shifts that didn't cross the alert threshold.
func selectAnomaliesAndMovers(stats []EventAnomaly) (anomalies, movers []EventAnomaly) {
	sort.Slice(stats, func(i, j int) bool {
		return math.Abs(stats[i].ModifiedZScore) > math.Abs(stats[j].ModifiedZScore)
	})

	anomalies = make([]EventAnomaly, 0)
	for _, s := range stats {
		if math.Abs(s.ModifiedZScore) > anomalyZThreshold {
			anomalies = append(anomalies, s)
		}
	}
	movers = stats
	if len(movers) > topMoversLimit {
		movers = movers[:topMoversLimit]
	}
	return anomalies, movers
}

// narrateDigest sends the pre-computed signals to Claude and returns its
// three-sentence narration.
func narrateDigest(ctx context.Context, client *anthropic.Client, anomalies, movers []EventAnomaly) (string, error) {
	payload, err := json.Marshal(struct {
		Anomalies []EventAnomaly `json:"anomalies"`
		TopMovers []EventAnomaly `json:"top_movers"`
	}{anomalies, movers})
	if err != nil {
		return "", err
	}

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:        "claude-opus-5",
		MaxTokens:    1024,
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortLow},
		System: []anthropic.TextBlockParam{{
			Text: "You are an analytics assistant narrating a daily digest for a game " +
				"analytics dashboard. You are given today's event-count anomalies " +
				"(today vs. a trailing 14-day median, flagged by a modified z-score " +
				"outlier test) and the broader list of top-moving events. Write exactly " +
				"three sentences: what changed, how significant it is, and where to look " +
				"next. Name specific events and give magnitudes (percent or count). No " +
				"preamble, no bullet points, no disclaimers — just the three sentences.",
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(string(payload))),
		},
	})
	if err != nil {
		return "", err
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return "", fmt.Errorf("digest narration declined")
	}

	var narration string
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			narration += text.Text
		}
	}
	return narration, nil
}
