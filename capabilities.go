package dakera

// GET /v1/capabilities — what the connected server can do (server v0.12+).
//
// R9 / DAK-10004. Contract (server routes/capabilities.rs): every field is
// additive; unknown fields and unknown strings inside lists MUST be ignored;
// capabilities_version bumps only on a breaking reshape of the document.
// encoding/json ignores unknown fields by default and every wire enum in this
// SDK is a string type, so a newer server never breaks decoding. The verbatim
// document is kept in ServerCapabilities.Raw.

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ===========================================================================
// Forward-compatible wire enums (string types: unknown values never fail)
// ===========================================================================

// IndexKind is an index kind the server may build or advertise (index_type
// values; the strings are the server's stable storage keys).
type IndexKind string

const (
	IndexKindHnsw     IndexKind = "hnsw"
	IndexKindPq       IndexKind = "pq"
	IndexKindIvf      IndexKind = "ivf"
	IndexKindIvfPq    IndexKind = "ivfpq"
	IndexKindSpFresh  IndexKind = "spfresh"
	IndexKindFullText IndexKind = "fulltext"
)

// KnownIndexKinds lists the index kinds this SDK version declares.
var KnownIndexKinds = []IndexKind{
	IndexKindHnsw,
	IndexKindPq,
	IndexKindIvf,
	IndexKindIvfPq,
	IndexKindSpFresh,
	IndexKindFullText,
}

// IsKnown reports whether this SDK version declares the index kind.
func (k IndexKind) IsKnown() bool {
	for _, known := range KnownIndexKinds {
		if k == known {
			return true
		}
	}
	return false
}

// SearchMode is the vector search mode a server process runs
// (DAKERA_SEARCH_MODE; process-wide, not selectable per request).
type SearchMode string

const (
	SearchModeHybrid SearchMode = "hybrid"
	SearchModeBinary SearchMode = "binary"
	SearchModeFloat  SearchMode = "float"
	SearchModeScalar SearchMode = "scalar"
	SearchModeRaBitQ SearchMode = "rabitq"
)

// KnownSearchModes lists the search modes this SDK version declares.
var KnownSearchModes = []SearchMode{
	SearchModeHybrid,
	SearchModeBinary,
	SearchModeFloat,
	SearchModeScalar,
	SearchModeRaBitQ,
}

// IsKnown reports whether this SDK version declares the search mode.
func (m SearchMode) IsKnown() bool {
	for _, known := range KnownSearchModes {
		if m == known {
			return true
		}
	}
	return false
}

// RepresentationKind is a kind a record representation slot may have (R2).
type RepresentationKind string

const (
	RepresentationKindDense            RepresentationKind = "dense"
	RepresentationKindTokenMultivector RepresentationKind = "token_multivector"
	RepresentationKindPatchMultivector RepresentationKind = "patch_multivector"
)

// KnownRepresentationKinds lists the representation kinds this SDK version declares.
var KnownRepresentationKinds = []RepresentationKind{
	RepresentationKindDense,
	RepresentationKindTokenMultivector,
	RepresentationKindPatchMultivector,
}

// IsKnown reports whether this SDK version declares the representation kind.
func (k RepresentationKind) IsKnown() bool {
	for _, known := range KnownRepresentationKinds {
		if k == known {
			return true
		}
	}
	return false
}

// BlockDType is a payload encoding a record slot may be stored as (store_as).
type BlockDType string

const (
	BlockDTypeF32 BlockDType = "f32"
	BlockDTypeF16 BlockDType = "f16"
	BlockDTypeI8  BlockDType = "i8"
)

// KnownBlockDTypes lists the block dtypes this SDK version declares.
var KnownBlockDTypes = []BlockDType{BlockDTypeF32, BlockDTypeF16, BlockDTypeI8}

// IsKnown reports whether this SDK version declares the dtype.
func (d BlockDType) IsKnown() bool {
	for _, known := range KnownBlockDTypes {
		if d == known {
			return true
		}
	}
	return false
}

// ===========================================================================
// The capabilities document
// ===========================================================================

// ModelCapability is one embedding model the server can load (capabilities.models[]).
type ModelCapability struct {
	// Name is the wire name — the string accepted/returned in every model field.
	Name EmbeddingModel `json:"name"`
	// Aliases are other spellings accepted on input.
	Aliases   []string `json:"aliases"`
	Dimension int      `json:"dimension"`
	// MaxSeqLength is the model's own context window (tokens).
	MaxSeqLength int `json:"max_seq_length"`
	// EffectiveMaxSeqLength is what THIS server embeds before truncating.
	EffectiveMaxSeqLength int `json:"effective_max_seq_length"`
	// Active reports whether this is the model the server embeds with (one per store).
	Active bool `json:"active"`
	// MrlDimensions are Matryoshka truncation dimensions, when supported.
	MrlDimensions []int  `json:"mrl_dimensions,omitempty"`
	Modality      string `json:"modality"`
}

// Matches reports whether name is this model's wire name or one of its aliases.
func (m ModelCapability) Matches(name string) bool {
	if string(m.Name) == name {
		return true
	}
	for _, alias := range m.Aliases {
		if alias == name {
			return true
		}
	}
	return false
}

// RecordCapabilities is the R2 record / representation surface (capabilities.records).
type RecordCapabilities struct {
	// Enabled reports whether /v1/namespaces/{ns}/records answers (else 501 FEATURE_DISABLED).
	Enabled             bool                 `json:"enabled"`
	RepresentationKinds []RepresentationKind `json:"representation_kinds"`
	DTypes              []BlockDType         `json:"dtypes"`
	MaxRepresentations  int                  `json:"max_representations"`
	MaxVectors          int                  `json:"max_vectors"`
	MaxBytes            int                  `json:"max_bytes"`
}

// AcceptedValues is a list of accepted wire strings that the server may emit
// either as prose ("hybrid, binary, float, scalar (alias sq), rabitq") or as a
// JSON list. Aliases in the prose form are expanded ("scalar (alias sq)" yields
// both "scalar" and "sq").
type AcceptedValues []string

// UnmarshalJSON accepts a string or a list of strings; any other shape decodes
// to an empty list rather than failing (the contract says ignore, not error).
func (a *AcceptedValues) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*a = ParseAcceptedValues(s)
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*a = list
		return nil
	}
	*a = nil
	return nil
}

var (
	acceptedValueRe = regexp.MustCompile(`([^,()]+)(?:\(([^)]*)\))?`)
	aliasSplitRe    = regexp.MustCompile(`[\s,/]+`)
)

// ParseAcceptedValues parses the prose form of an accepted-values field.
func ParseAcceptedValues(value string) []string {
	out := []string{}
	for _, m := range acceptedValueRe.FindAllStringSubmatch(value, -1) {
		if head := strings.TrimSpace(m[1]); head != "" {
			out = append(out, head)
		}
		note := strings.TrimSpace(m[2])
		if strings.HasPrefix(strings.ToLower(note), "alias") {
			rest := strings.TrimPrefix(note[len("alias"):], "es")
			for _, alias := range aliasSplitRe.Split(rest, -1) {
				if alias != "" {
					out = append(out, alias)
				}
			}
		}
	}
	return out
}

// ServerCapabilities is the GET /v1/capabilities document.
type ServerCapabilities struct {
	CapabilitiesVersion int               `json:"capabilities_version"`
	ServerVersion       string            `json:"server_version"`
	APIVersions         []string          `json:"api_versions"`
	DefaultModel        EmbeddingModel    `json:"default_model"`
	Models              []ModelCapability `json:"models"`
	IndexKinds          []IndexKind       `json:"index_kinds"`
	VectorIndexKinds    []IndexKind       `json:"vector_index_kinds"`
	// LiveVectorIndexKinds is the subset the engine actually builds today.
	LiveVectorIndexKinds []IndexKind      `json:"live_vector_index_kinds"`
	DistanceMetrics      []DistanceMetric `json:"distance_metrics"`
	// SearchMode is the mode this server process runs (DAKERA_SEARCH_MODE).
	SearchMode SearchMode `json:"search_mode"`
	// SearchModesAccepted is every value the server accepts for DAKERA_SEARCH_MODE.
	SearchModesAccepted AcceptedValues     `json:"search_modes_accepted"`
	FulltextLanguage    string             `json:"fulltext_language"`
	OnDiskFormatVersion int                `json:"on_disk_format_version"`
	Records             RecordCapabilities `json:"records"`
	QueryLanguages      []string           `json:"query_languages"`
	// ReembedPending reports that a model change was acknowledged but the store
	// is not fully re-embedded yet (recall mixes two embedding spaces).
	ReembedPending bool `json:"reembed_pending"`
	// Raw is the verbatim document — carries fields this SDK does not model yet.
	Raw map[string]interface{} `json:"-"`
}

// ParseCapabilities decodes a capabilities document. Unknown fields and unknown
// strings inside lists are kept, never rejected.
func ParseCapabilities(data []byte) (*ServerCapabilities, error) {
	var caps ServerCapabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err == nil {
		caps.Raw = raw
	}
	return &caps, nil
}

// FindModel looks a model up by wire name or alias.
func (c *ServerCapabilities) FindModel(nameOrAlias string) *ModelCapability {
	for i := range c.Models {
		if c.Models[i].Matches(nameOrAlias) {
			return &c.Models[i]
		}
	}
	return nil
}

// ActiveModel is the model the server embeds with (a request naming another is rejected).
func (c *ServerCapabilities) ActiveModel() *ModelCapability {
	for i := range c.Models {
		if c.Models[i].Active {
			return &c.Models[i]
		}
	}
	return nil
}

// ModelNames returns the wire names of every model the server can load.
func (c *ServerCapabilities) ModelNames() []string {
	names := make([]string, 0, len(c.Models))
	for _, m := range c.Models {
		names = append(names, string(m.Name))
	}
	return names
}

// SupportsRecords reports whether the record routes are switched on (records.enabled).
func (c *ServerCapabilities) SupportsRecords() bool {
	return c.Records.Enabled
}

// CapabilityKind names a registry a pre-flight check validates against.
type CapabilityKind string

const (
	CapabilityModel          CapabilityKind = "model"
	CapabilityIndexKind      CapabilityKind = "index_kind"
	CapabilityDistanceMetric CapabilityKind = "distance_metric"
	CapabilitySearchMode     CapabilityKind = "search_mode"
	CapabilityQueryLanguage  CapabilityKind = "query_language"
)

// SupportedValues returns the wire strings the server advertises for kind
// (nil for an unknown kind).
func (c *ServerCapabilities) SupportedValues(kind CapabilityKind) []string {
	switch kind {
	case CapabilityModel:
		return c.ModelNames()
	case CapabilityIndexKind:
		out := make([]string, 0, len(c.IndexKinds))
		for _, k := range c.IndexKinds {
			out = append(out, string(k))
		}
		return out
	case CapabilityDistanceMetric:
		out := make([]string, 0, len(c.DistanceMetrics))
		for _, m := range c.DistanceMetrics {
			out = append(out, string(m))
		}
		return out
	case CapabilitySearchMode:
		return append([]string(nil), c.SearchModesAccepted...)
	case CapabilityQueryLanguage:
		return append([]string(nil), c.QueryLanguages...)
	}
	return nil
}

// Supports reports whether the server advertises value for kind (model aliases count).
func (c *ServerCapabilities) Supports(kind CapabilityKind, value string) bool {
	if kind == CapabilityModel {
		return c.FindModel(value) != nil
	}
	for _, v := range c.SupportedValues(kind) {
		if v == value {
			return true
		}
	}
	return false
}

// Require returns an *UnsupportedCapabilityError unless the server advertises
// value for kind; nil otherwise.
func (c *ServerCapabilities) Require(kind CapabilityKind, value string) error {
	if c.Supports(kind, value) {
		return nil
	}
	return NewUnsupportedCapabilityError(kind, value, c.SupportedValues(kind), c.ServerVersion)
}
