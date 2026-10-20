package dakera

import "encoding/json"

// recalledMemoryWire is the intermediate decode target for RecalledMemory.
//
// The Dakera server returns recall/search results with memory fields nested
// under a "memory" key and score at the top level:
//
//	{"memory": {"id": "...", "content": "...", ...}, "score": 0.95, "smart_score": 0.87}
//
// The flat fields (ID, Content, …) are retained so that the flat format used
// in unit test mocks and by the /v1/agents/{id}/memories endpoint continues to
// decode correctly.
type recalledMemoryWire struct {
	// Nested server format
	Memory *Memory `json:"memory"`

	// Top-level fields present in all formats
	Score         float32  `json:"score"`
	RankScore     *float32 `json:"rank_score,omitempty"`
	WeightedScore *float32 `json:"weighted_score,omitempty"`
	SmartScore    *float32 `json:"smart_score,omitempty"`
	Depth         *int     `json:"depth,omitempty"`
	VectorScore   *float32 `json:"vector_score,omitempty"`
	TextScore     *float32 `json:"text_score,omitempty"`

	// #917 additions — absent from older servers.
	MatchedText string   `json:"matched_text,omitempty"`
	MatchedIDs  []string `json:"matched_ids,omitempty"`
	Retrieval   string   `json:"retrieval,omitempty"`
	ContextOf   string   `json:"context_of,omitempty"`

	// Content preview fields (listings with content_preview_chars, server v0.12.2+)
	ContentLen       *int  `json:"content_len,omitempty"`
	ContentTruncated *bool `json:"content_truncated,omitempty"`

	// Flat fields — populated when the "memory" wrapper is absent
	ID            string                 `json:"id"`
	Content       string                 `json:"content"`
	MemoryType    string                 `json:"memory_type"`
	Importance    float32                `json:"importance"`
	Tags          []string               `json:"tags,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     int64                  `json:"created_at,omitempty"`
	AttachmentRef string                 `json:"attachment_ref,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for RecalledMemory.
//
// Unknown fields are ignored, and fields an older server does not send stay
// zero, so it decodes answers of older and newer servers alike.
//
// Handles two wire formats:
//   - Nested (server): {"memory": {"id": "...", "content": "..."}, "score": N, "smart_score": N}
//   - Flat (unit tests, /v1/agents/{id}/memories): {"id": "...", "content": "...", "score": N}
func (r *RecalledMemory) UnmarshalJSON(data []byte) error {
	var wire recalledMemoryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	if wire.Memory != nil {
		// Nested server format — extract from the envelope.
		r.ID = wire.Memory.ID
		r.Content = wire.Memory.Content
		r.MemoryType = wire.Memory.MemoryType
		r.Importance = wire.Memory.Importance
		r.Tags = wire.Memory.Tags
		r.Metadata = wire.Memory.Metadata
		r.CreatedAt = wire.Memory.CreatedAt
		r.AttachmentRef = wire.Memory.AttachmentRef
		r.SessionID = wire.Memory.SessionID
	} else {
		// Flat format — fields are at the top level.
		r.ID = wire.ID
		r.Content = wire.Content
		r.MemoryType = wire.MemoryType
		r.Importance = wire.Importance
		r.Tags = wire.Tags
		r.Metadata = wire.Metadata
		r.CreatedAt = wire.CreatedAt
		r.AttachmentRef = wire.AttachmentRef
		r.SessionID = wire.SessionID
	}

	// rank_score (server v0.12.3+, #940) is the ranking field, non-increasing over a
	// recall's results; older servers: smart_score, then weighted_score, then score.
	r.RankScore = wire.RankScore
	r.SmartScore = wire.SmartScore
	r.WeightedScore = wire.WeightedScore
	switch {
	case wire.RankScore != nil:
		r.Score = *wire.RankScore
	case wire.SmartScore != nil:
		r.Score = *wire.SmartScore
	case wire.WeightedScore != nil:
		r.Score = *wire.WeightedScore
	default:
		r.Score = wire.Score
	}
	r.Depth = wire.Depth
	r.ContentLen = wire.ContentLen
	r.ContentTruncated = wire.ContentTruncated
	r.VectorScore = wire.VectorScore
	r.TextScore = wire.TextScore
	r.MatchedText = wire.MatchedText
	r.MatchedIDs = wire.MatchedIDs
	r.Retrieval = wire.Retrieval
	r.ContextOf = wire.ContextOf
	return nil
}
