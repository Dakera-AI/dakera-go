package dakera

import (
	"encoding/json"
	"testing"
)

func ptr32(v float32) *float32 { return &v }

func TestRecalledMemory_SmartScorePriority(t *testing.T) {
	data := []byte(`{
		"memory": {"id": "m1", "content": "test", "memory_type": "episodic", "importance": 0.8},
		"score": 0.5,
		"weighted_score": 0.7,
		"smart_score": 0.9
	}`)
	var m RecalledMemory
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if m.Score != 0.9 {
		t.Errorf("Score = %v, want 0.9 (smart_score priority)", m.Score)
	}
	if m.SmartScore == nil || *m.SmartScore != 0.9 {
		t.Errorf("SmartScore = %v, want &0.9", m.SmartScore)
	}
	if m.WeightedScore == nil || *m.WeightedScore != 0.7 {
		t.Errorf("WeightedScore = %v, want &0.7", m.WeightedScore)
	}
}

func TestRecalledMemory_WeightedScoreFallback(t *testing.T) {
	data := []byte(`{
		"memory": {"id": "m2", "content": "test", "memory_type": "episodic", "importance": 0.8},
		"score": 0.5,
		"weighted_score": 0.7
	}`)
	var m RecalledMemory
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if m.Score != 0.7 {
		t.Errorf("Score = %v, want 0.7 (weighted_score fallback)", m.Score)
	}
	if m.SmartScore != nil {
		t.Errorf("SmartScore = %v, want nil", m.SmartScore)
	}
}

func TestRecalledMemory_RawScoreFallback(t *testing.T) {
	data := []byte(`{
		"memory": {"id": "m3", "content": "test", "memory_type": "episodic", "importance": 0.8},
		"score": 0.55
	}`)
	var m RecalledMemory
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if m.Score != 0.55 {
		t.Errorf("Score = %v, want 0.55 (raw score fallback)", m.Score)
	}
	if m.SmartScore != nil {
		t.Errorf("SmartScore = %v, want nil", m.SmartScore)
	}
	if m.WeightedScore != nil {
		t.Errorf("WeightedScore = %v, want nil", m.WeightedScore)
	}
}

func TestRecalledMemory_FlatFormatScore(t *testing.T) {
	data := []byte(`{"id": "m4", "content": "flat", "memory_type": "episodic", "importance": 0.5, "score": 0.6}`)
	var m RecalledMemory
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if m.Score != 0.6 {
		t.Errorf("Score = %v, want 0.6 (flat format)", m.Score)
	}
	if m.SmartScore != nil {
		t.Errorf("SmartScore = %v, want nil for flat format", m.SmartScore)
	}
}
