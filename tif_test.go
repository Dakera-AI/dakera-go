package dakera_test

import (
	"math"
	"testing"

	dakera "github.com/dakera-ai/dakera-go"
)

func makeHistory(signals ...string) *dakera.FeedbackHistoryResponse {
	entries := make([]dakera.FeedbackHistoryEntry, 0, len(signals))
	for _, s := range signals {
		entries = append(entries, dakera.FeedbackHistoryEntry{
			Signal:        dakera.FeedbackSignal(s),
			Timestamp:     0,
			OldImportance: 0.5,
			NewImportance: 0.5,
		})
	}
	return &dakera.FeedbackHistoryResponse{MemoryID: "test-mem", Entries: entries}
}

const epsilon = 1e-9

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestComputeTifScore_NoFeedback(t *testing.T) {
	score := dakera.ComputeTifScore(makeHistory())
	if score.Truth != 0 || score.Indeterminacy != 1 || score.Falsity != 0 || score.FeedbackCount != 0 {
		t.Errorf("no-feedback score wrong: %+v", score)
	}
	if score.Classification != dakera.TifAskClarification {
		t.Errorf("expected TifAskClarification, got %s", score.Classification)
	}
}

func TestComputeTifScore_AllUpvotes(t *testing.T) {
	score := dakera.ComputeTifScore(makeHistory("upvote", "upvote", "upvote"))
	if !approxEqual(score.Truth, 1.0) {
		t.Errorf("expected truth=1.0, got %f", score.Truth)
	}
	if score.FeedbackCount != 3 {
		t.Errorf("expected count=3, got %d", score.FeedbackCount)
	}
	if score.Classification != dakera.TifConfidentReuse {
		t.Errorf("expected TifConfidentReuse, got %s", score.Classification)
	}
}

func TestComputeTifScore_AllDownvotes(t *testing.T) {
	score := dakera.ComputeTifScore(makeHistory("downvote", "downvote"))
	if !approxEqual(score.Falsity, 0.8) {
		t.Errorf("expected falsity=0.8, got %f", score.Falsity)
	}
	if !approxEqual(score.Indeterminacy, 0.2) {
		t.Errorf("expected indeterminacy=0.2, got %f", score.Indeterminacy)
	}
	if score.Classification != dakera.TifSurfaceContradiction {
		t.Errorf("expected TifSurfaceContradiction, got %s", score.Classification)
	}
}

func TestComputeTifScore_AllFlags(t *testing.T) {
	score := dakera.ComputeTifScore(makeHistory("flag", "flag"))
	if !approxEqual(score.Indeterminacy, 1.0) {
		t.Errorf("expected indeterminacy=1.0, got %f", score.Indeterminacy)
	}
	if score.Classification != dakera.TifAskClarification {
		t.Errorf("expected TifAskClarification, got %s", score.Classification)
	}
}

func TestComputeTifScore_MixedSignals(t *testing.T) {
	// 4 upvotes, 2 downvotes, 4 flags → total 10
	score := dakera.ComputeTifScore(makeHistory(
		"upvote", "upvote", "upvote", "upvote",
		"downvote", "downvote",
		"flag", "flag", "flag", "flag",
	))
	if !approxEqual(score.Truth, 0.4) {
		t.Errorf("expected truth=0.4, got %f", score.Truth)
	}
	if !approxEqual(score.Falsity, 0.2) {
		t.Errorf("expected falsity=0.2, got %f", score.Falsity)
	}
	if !approxEqual(score.Indeterminacy, 0.4) {
		t.Errorf("expected indeterminacy=0.4, got %f", score.Indeterminacy)
	}
	if score.FeedbackCount != 10 {
		t.Errorf("expected count=10, got %d", score.FeedbackCount)
	}
}

func TestComputeTifScore_PositiveAlias(t *testing.T) {
	score := dakera.ComputeTifScore(makeHistory("positive", "positive", "downvote"))
	if !approxEqual(score.Truth, 2.0/3.0) {
		t.Errorf("expected truth=2/3, got %f", score.Truth)
	}
	if !approxEqual(score.Falsity, 1.0/3.0) {
		t.Errorf("expected falsity=1/3, got %f", score.Falsity)
	}
}

func TestComputeTifScore_NegativeAlias(t *testing.T) {
	score := dakera.ComputeTifScore(makeHistory("upvote", "negative", "negative"))
	if !approxEqual(score.Falsity, 2.0/3.0) {
		t.Errorf("expected falsity=2/3, got %f", score.Falsity)
	}
}

func TestComputeTifScore_ProportionsSumToOne(t *testing.T) {
	score := dakera.ComputeTifScore(makeHistory("upvote", "downvote", "flag"))
	sum := score.Truth + score.Indeterminacy + score.Falsity
	if !approxEqual(sum, 1.0) {
		t.Errorf("proportions don't sum to 1.0: %f", sum)
	}
}

func TestClassification_VerifyBeforeUse(t *testing.T) {
	// 4 upvotes, 3 downvotes, 3 flags → truth=0.4, falsity=0.3, indeterminacy=0.3
	score := dakera.ComputeTifScore(makeHistory(
		"upvote", "upvote", "upvote", "upvote",
		"downvote", "downvote", "downvote",
		"flag", "flag", "flag",
	))
	if score.Classification != dakera.TifVerifyBeforeUse {
		t.Errorf("expected TifVerifyBeforeUse, got %s", score.Classification)
	}
}

func TestClassification_FalsityOverIndeterminacy(t *testing.T) {
	// 3 downvotes + 3 flags: falsity=0.5, indeterminacy=0.5, falsity wins
	score := dakera.ComputeTifScore(makeHistory("downvote", "downvote", "downvote", "flag", "flag", "flag"))
	if score.Classification != dakera.TifSurfaceContradiction {
		t.Errorf("expected TifSurfaceContradiction, got %s", score.Classification)
	}
}

func TestTifScoreFromMetadata_RoundTrip(t *testing.T) {
	data := map[string]interface{}{
		"truth":          0.75,
		"indeterminacy":  0.15,
		"falsity":        0.10,
		"feedback_count": float64(20),
	}
	score, ok := dakera.TifScoreFromMetadata(data)
	if !ok {
		t.Fatal("TifScoreFromMetadata returned ok=false")
	}
	if !approxEqual(score.Truth, 0.75) {
		t.Errorf("expected truth=0.75, got %f", score.Truth)
	}
	if score.FeedbackCount != 20 {
		t.Errorf("expected feedback_count=20, got %d", score.FeedbackCount)
	}
	if score.Classification != dakera.TifConfidentReuse {
		t.Errorf("expected TifConfidentReuse, got %s", score.Classification)
	}
}

func TestTifScoreFromMetadata_MissingFeedbackCount(t *testing.T) {
	data := map[string]interface{}{
		"truth":         0.8,
		"indeterminacy": 0.1,
		"falsity":       0.1,
	}
	score, ok := dakera.TifScoreFromMetadata(data)
	if !ok {
		t.Fatal("TifScoreFromMetadata returned ok=false")
	}
	if score.FeedbackCount != 0 {
		t.Errorf("expected feedback_count=0, got %d", score.FeedbackCount)
	}
}

// ── Thin evidence ──────────────────────────────────────────────────────

func TestThinEvidence_SingleUpvoteNotConfident(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("upvote"))
	sum := s.Truth + s.Indeterminacy + s.Falsity
	if !approxEqual(sum, 1.0) {
		t.Errorf("proportions don't sum to 1.0: %f", sum)
	}
	if s.Indeterminacy <= 0 {
		t.Error("single upvote must inject indeterminacy")
	}
	if s.Truth >= 0.70 {
		t.Errorf("single upvote truth should be < 0.70, got %f", s.Truth)
	}
	if s.Classification != dakera.TifVerifyBeforeUse {
		t.Errorf("expected TifVerifyBeforeUse, got %s", s.Classification)
	}
}

func TestThinEvidence_TwoUpvotesConfident(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("upvote", "upvote"))
	if !approxEqual(s.Truth, 0.8) {
		t.Errorf("expected truth=0.8, got %f", s.Truth)
	}
	if !approxEqual(s.Indeterminacy, 0.2) {
		t.Errorf("expected indeterminacy=0.2, got %f", s.Indeterminacy)
	}
	if s.Classification != dakera.TifConfidentReuse {
		t.Errorf("expected TifConfidentReuse, got %s", s.Classification)
	}
}

func TestThinEvidence_ThreeUpvotesNoBase(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("upvote", "upvote", "upvote"))
	if !approxEqual(s.Truth, 1.0) {
		t.Errorf("expected truth=1.0, got %f", s.Truth)
	}
	if !approxEqual(s.Indeterminacy, 0.0) {
		t.Errorf("expected indeterminacy=0.0, got %f", s.Indeterminacy)
	}
	if s.Classification != dakera.TifConfidentReuse {
		t.Errorf("expected TifConfidentReuse, got %s", s.Classification)
	}
}

// ── Golden vectors (canonical T-I-F v1 contract) ──────────────────────

func TestGolden_NoFeedback(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory())
	if !approxEqual(s.Truth, 0.0) || !approxEqual(s.Indeterminacy, 1.0) || !approxEqual(s.Falsity, 0.0) {
		t.Errorf("golden no-feedback: %+v", s)
	}
	if s.Classification != dakera.TifAskClarification {
		t.Errorf("expected TifAskClarification, got %s", s.Classification)
	}
}

func TestGolden_OneUpvote(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("upvote"))
	if math.Abs(s.Truth-2.0/3.0) > 1e-4 {
		t.Errorf("expected truth≈0.6667, got %f", s.Truth)
	}
	if math.Abs(s.Indeterminacy-1.0/3.0) > 1e-4 {
		t.Errorf("expected indeterminacy≈0.3333, got %f", s.Indeterminacy)
	}
	if s.Classification != dakera.TifVerifyBeforeUse {
		t.Errorf("expected TifVerifyBeforeUse, got %s", s.Classification)
	}
}

func TestGolden_TwoUpvotes(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("upvote", "upvote"))
	if !approxEqual(s.Truth, 0.8) || !approxEqual(s.Indeterminacy, 0.2) || !approxEqual(s.Falsity, 0.0) {
		t.Errorf("golden two-upvotes: %+v", s)
	}
	if s.Classification != dakera.TifConfidentReuse {
		t.Errorf("expected TifConfidentReuse, got %s", s.Classification)
	}
}

func TestGolden_ThreeUpvotes(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("upvote", "upvote", "upvote"))
	if !approxEqual(s.Truth, 1.0) || !approxEqual(s.Indeterminacy, 0.0) || !approxEqual(s.Falsity, 0.0) {
		t.Errorf("golden three-upvotes: %+v", s)
	}
	if s.Classification != dakera.TifConfidentReuse {
		t.Errorf("expected TifConfidentReuse, got %s", s.Classification)
	}
}

func TestGolden_TwoDownvotes(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("downvote", "downvote"))
	if !approxEqual(s.Truth, 0.0) || !approxEqual(s.Indeterminacy, 0.2) || !approxEqual(s.Falsity, 0.8) {
		t.Errorf("golden two-downvotes: %+v", s)
	}
	if s.Classification != dakera.TifSurfaceContradiction {
		t.Errorf("expected TifSurfaceContradiction, got %s", s.Classification)
	}
}

func TestGolden_TwoFlags(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("flag", "flag"))
	if !approxEqual(s.Truth, 0.0) || !approxEqual(s.Indeterminacy, 1.0) || !approxEqual(s.Falsity, 0.0) {
		t.Errorf("golden two-flags: %+v", s)
	}
	if s.Classification != dakera.TifAskClarification {
		t.Errorf("expected TifAskClarification, got %s", s.Classification)
	}
}

func TestGolden_8Up1Down1Flag(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory(
		"upvote", "upvote", "upvote", "upvote",
		"upvote", "upvote", "upvote", "upvote",
		"downvote", "flag",
	))
	if !approxEqual(s.Truth, 0.8) || !approxEqual(s.Indeterminacy, 0.1) || !approxEqual(s.Falsity, 0.1) {
		t.Errorf("golden 8up-1down-1flag: %+v", s)
	}
	if s.Classification != dakera.TifConfidentReuse {
		t.Errorf("expected TifConfidentReuse, got %s", s.Classification)
	}
}

func TestGolden_3Down3Flag(t *testing.T) {
	s := dakera.ComputeTifScore(makeHistory("downvote", "downvote", "downvote", "flag", "flag", "flag"))
	if !approxEqual(s.Truth, 0.0) || !approxEqual(s.Indeterminacy, 0.5) || !approxEqual(s.Falsity, 0.5) {
		t.Errorf("golden 3down-3flag: %+v", s)
	}
	if s.Classification != dakera.TifSurfaceContradiction {
		t.Errorf("expected TifSurfaceContradiction, got %s", s.Classification)
	}
}
