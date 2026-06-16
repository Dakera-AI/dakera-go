package dakera

import "encoding/json"

// recalledMemoryWire is the intermediate decode target for RecalledMemory.
//
// The Dakera server returns recall/search results with memory fields nested
// under a "memory" key and score at the top level:
//
//	{"memory": {"id": "...", "content": "...", ...}, "score": 0.95}
//
// The flat fields (ID, Content, …) are retained so that the flat format used
// in unit test mocks and by the /v1/agents/{id}/memories endpoint continues to
// decode correctly.
type recalledMemoryWire struct {
	// Nested server format
	Memory *Memory `json:"memory"`

	// Top-level fields present in all formats
	Score float32 `json:"score"`
	Depth *int    `json:"depth,omitempty"`

	// Flat fields — populated when the "memory" wrapper is absent
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	MemoryType string                 `json:"memory_type"`
	Importance float32                `json:"importance"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  int64                  `json:"created_at,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for RecalledMemory.
//
// Handles two wire formats:
//   - Nested (server): {"memory": {"id": "...", "content": "..."}, "score": N}
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
	} else {
		// Flat format — fields are at the top level.
		r.ID = wire.ID
		r.Content = wire.Content
		r.MemoryType = wire.MemoryType
		r.Importance = wire.Importance
		r.Tags = wire.Tags
		r.Metadata = wire.Metadata
		r.CreatedAt = wire.CreatedAt
	}

	r.Score = wire.Score
	r.Depth = wire.Depth
	return nil
}
