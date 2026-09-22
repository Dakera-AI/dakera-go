package dakera

// R9 / DAK-10004 — forward-compat: lenient enums + GET /v1/capabilities.
//
// Contract under test (server routes/capabilities.rs): every field additive;
// unknown fields and unknown strings inside lists MUST be ignored; the SDK
// must never fail decoding on a model / index kind / search mode / metric /
// representation kind / dtype string it does not know.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A capabilities document as the v0.12 server emits it, PLUS a model string and
// an index kind this SDK does not know, PLUS an extra top-level field, an extra
// model field and an extra records field — all of which must be tolerated.
const capabilitiesFixture = `{
  "capabilities_version": 1,
  "server_version": "0.12.0",
  "api_versions": ["v1"],
  "default_model": "bge-large",
  "models": [
    {"name": "bge-large", "aliases": ["bge-large-en", "bge-large-en-v1.5"], "dimension": 1024,
     "max_seq_length": 512, "effective_max_seq_length": 512, "active": true, "modality": "text"},
    {"name": "modernbert-embed-base", "aliases": ["modernbert", "modern-bert"], "dimension": 768,
     "max_seq_length": 8192, "effective_max_seq_length": 2048, "active": false,
     "mrl_dimensions": [256, 768], "modality": "text"},
    {"name": "bge-m3", "aliases": ["bge-m3-dense", "baai/bge-m3"], "dimension": 1024,
     "max_seq_length": 8192, "effective_max_seq_length": 2048, "active": false, "modality": "text"},
    {"name": "colmodernvbert-v9", "aliases": [], "dimension": 128, "max_seq_length": 4096,
     "effective_max_seq_length": 4096, "active": false, "modality": "image", "quantised": true}
  ],
  "index_kinds": ["hnsw", "pq", "ivf", "ivfpq", "spfresh", "fulltext", "muvera_fde"],
  "vector_index_kinds": ["hnsw", "ivf", "ivfpq", "spfresh", "muvera_fde"],
  "live_vector_index_kinds": ["hnsw", "ivf", "spfresh"],
  "distance_metrics": ["cosine", "euclidean", "dot_product", "hamming"],
  "search_mode": "rabitq",
  "search_modes_accepted": "hybrid, binary, float, scalar (alias sq), rabitq, warp9",
  "fulltext_language": "de",
  "on_disk_format_version": 1,
  "records": {
    "enabled": true,
    "representation_kinds": ["dense", "token_multivector", "patch_multivector", "holo"],
    "dtypes": ["f32", "f16", "i8", "e4m3"],
    "max_representations": 8, "max_vectors": 4096, "max_bytes": 8388608,
    "compression": "zstd"
  },
  "query_languages": ["en", "de", "fr", "es", "it", "pt", "nl"],
  "reembed_pending": true,
  "future_top_level_field": {"anything": [1, 2, 3]}
}`

// capabilitiesServer serves the fixture (or a 404) on GET /v1/capabilities and
// an echoing text response elsewhere, recording every request method+path.
func capabilitiesServer(t *testing.T, capsStatus int) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/capabilities" {
			w.WriteHeader(capsStatus)
			if capsStatus == http.StatusOK {
				_, _ = w.Write([]byte(capabilitiesFixture))
			} else {
				_, _ = w.Write([]byte(`{"error":"not found"}`))
			}
			return
		}
		_, _ = w.Write([]byte(`{"upserted_count":1,"tokens_processed":1,"model":"colmodernvbert-v9","embedding_time_ms":1,"new_field_from_future":true}`))
	}))
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), calls...)
	}
}

// ---------------------------------------------------------------------------
// 1. Lenient enums
// ---------------------------------------------------------------------------

func TestLenientEnums_UnknownValuesDecodeAndAreNotKnown(t *testing.T) {
	assert.True(t, EmbeddingModelBgeM3.IsKnown())
	assert.True(t, EmbeddingModel("bge-large").IsKnown())
	assert.False(t, EmbeddingModel("colmodernvbert-v9").IsKnown())
	assert.False(t, IndexKind("muvera_fde").IsKnown())
	assert.True(t, IndexKindIvfPq.IsKnown())
	assert.False(t, SearchMode("warp9").IsKnown())
	assert.True(t, SearchModeRaBitQ.IsKnown())
	assert.False(t, DistanceMetric("hamming").IsKnown())
	assert.True(t, DistanceMetricDotProduct.IsKnown())
	assert.False(t, RepresentationKind("holo").IsKnown())
	assert.False(t, BlockDType("e4m3").IsKnown())
	assert.Equal(t, []RepresentationKind{"dense", "token_multivector", "patch_multivector"}, KnownRepresentationKinds)
	assert.Equal(t, []BlockDType{"f32", "f16", "i8"}, KnownBlockDTypes)
}

func TestLenientEnums_UnknownModelInResponseRoundTrips(t *testing.T) {
	srv, calls := capabilitiesServer(t, http.StatusOK)
	defer srv.Close()
	client := NewClient(srv.URL)

	resp, err := client.UpsertText(context.Background(), "ns", []TextDocument{{ID: "d", Text: "t"}}, nil)
	require.NoError(t, err)
	assert.Equal(t, EmbeddingModel("colmodernvbert-v9"), resp.Model)
	assert.False(t, resp.Model.IsKnown())
	assert.Equal(t, []string{"POST /v1/namespaces/ns/upsert-text"}, calls())

	// An unknown metric in a configure-namespace response decodes too.
	var cfg ConfigureNamespaceResponse
	require.NoError(t, json.Unmarshal([]byte(`{"namespace":"ns","dimension":8,"distance":"hamming","created":true}`), &cfg))
	assert.Equal(t, DistanceMetric("hamming"), cfg.Distance)
	assert.False(t, cfg.Distance.IsKnown())
}

// ---------------------------------------------------------------------------
// 2. ParseCapabilities
// ---------------------------------------------------------------------------

func TestParseCapabilities_FixtureWithUnknownStringsAndFields(t *testing.T) {
	caps, err := ParseCapabilities([]byte(capabilitiesFixture))
	require.NoError(t, err)

	assert.Equal(t, 1, caps.CapabilitiesVersion)
	assert.Equal(t, "0.12.0", caps.ServerVersion)
	assert.Equal(t, []string{"v1"}, caps.APIVersions)
	assert.Equal(t, EmbeddingModelBGELarge, caps.DefaultModel)
	assert.Equal(t, []string{"bge-large", "modernbert-embed-base", "bge-m3", "colmodernvbert-v9"}, caps.ModelNames())

	unknown := caps.FindModel("colmodernvbert-v9")
	require.NotNil(t, unknown)
	assert.False(t, unknown.Name.IsKnown())
	assert.Equal(t, "image", unknown.Modality)
	assert.Equal(t, []int{256, 768}, caps.FindModel("modernbert").MrlDimensions) // alias lookup
	assert.Nil(t, caps.FindModel("bge-large").MrlDimensions)
	require.NotNil(t, caps.ActiveModel())
	assert.Equal(t, EmbeddingModelBGELarge, caps.ActiveModel().Name)

	assert.Contains(t, caps.IndexKinds, IndexKindIvfPq)
	assert.Contains(t, caps.IndexKinds, IndexKind("muvera_fde"))
	assert.Equal(t, []IndexKind{"hnsw", "ivf", "spfresh"}, caps.LiveVectorIndexKinds)
	assert.Contains(t, caps.DistanceMetrics, DistanceMetric("hamming"))
	assert.Equal(t, SearchModeRaBitQ, caps.SearchMode)
	assert.Equal(t, AcceptedValues{"hybrid", "binary", "float", "scalar", "sq", "rabitq", "warp9"}, caps.SearchModesAccepted)
	assert.Equal(t, "de", caps.FulltextLanguage)
	assert.Equal(t, 1, caps.OnDiskFormatVersion)
	assert.True(t, caps.SupportsRecords())
	assert.Contains(t, caps.Records.RepresentationKinds, RepresentationKind("holo"))
	assert.Contains(t, caps.Records.DTypes, BlockDType("e4m3"))
	assert.Equal(t, 8*1024*1024, caps.Records.MaxBytes)
	assert.Equal(t, "nl", caps.QueryLanguages[len(caps.QueryLanguages)-1])
	assert.True(t, caps.ReembedPending)

	// Unknown fields are not fatal and are reachable through Raw.
	assert.Contains(t, caps.Raw, "future_top_level_field")
	models := caps.Raw["models"].([]interface{})
	assert.Equal(t, true, models[3].(map[string]interface{})["quantised"])
	assert.Equal(t, "zstd", caps.Raw["records"].(map[string]interface{})["compression"])
}

func TestParseCapabilities_MinimalAndListShapedAcceptedValues(t *testing.T) {
	caps, err := ParseCapabilities([]byte(`{}`))
	require.NoError(t, err)
	assert.Empty(t, caps.Models)
	assert.False(t, caps.SupportsRecords())
	assert.False(t, caps.ReembedPending)
	assert.Empty(t, caps.SearchModesAccepted)

	// Reshaped into a real list one day: also fine. Unknown shape: ignored.
	caps, err = ParseCapabilities([]byte(`{"search_modes_accepted": ["hybrid", "float"]}`))
	require.NoError(t, err)
	assert.Equal(t, AcceptedValues{"hybrid", "float"}, caps.SearchModesAccepted)
	caps, err = ParseCapabilities([]byte(`{"search_modes_accepted": 42}`))
	require.NoError(t, err)
	assert.Empty(t, caps.SearchModesAccepted)
}

func TestParseAcceptedValues(t *testing.T) {
	assert.Equal(t, []string{"hybrid", "binary", "float", "scalar", "sq", "rabitq"},
		ParseAcceptedValues("hybrid, binary, float, scalar (alias sq), rabitq"))
	assert.Equal(t, []string{"a", "b", "c"}, ParseAcceptedValues("a (alias b, c)"))
	assert.Equal(t, []string{}, ParseAcceptedValues(""))
}

func TestServerCapabilities_SupportsAndRequire(t *testing.T) {
	caps, err := ParseCapabilities([]byte(capabilitiesFixture))
	require.NoError(t, err)

	assert.True(t, caps.Supports(CapabilityModel, "bge-m3"))
	assert.True(t, caps.Supports(CapabilityModel, "baai/bge-m3")) // alias
	assert.False(t, caps.Supports(CapabilityModel, "minilm"))
	assert.True(t, caps.Supports(CapabilityIndexKind, "ivfpq"))
	assert.False(t, caps.Supports(CapabilityIndexKind, "flat"))
	assert.True(t, caps.Supports(CapabilityDistanceMetric, "cosine"))
	assert.True(t, caps.Supports(CapabilitySearchMode, "sq"))
	assert.False(t, caps.Supports(CapabilitySearchMode, "exact"))
	assert.True(t, caps.Supports(CapabilityQueryLanguage, "de"))
	assert.False(t, caps.Supports(CapabilityQueryLanguage, "ja"))
	assert.Nil(t, caps.SupportedValues(CapabilityKind("nope")))

	assert.NoError(t, caps.Require(CapabilityModel, "bge-m3"))
	err = caps.Require(CapabilityModel, "minilm")
	require.Error(t, err)
	var unsupported *UnsupportedCapabilityError
	require.True(t, errors.As(err, &unsupported))
	assert.Equal(t, CapabilityModel, unsupported.Kind)
	assert.Equal(t, "minilm", unsupported.Requested)
	assert.Equal(t, []string{"bge-large", "modernbert-embed-base", "bge-m3", "colmodernvbert-v9"}, unsupported.Supported)
	assert.Equal(t, "0.12.0", unsupported.ServerVersion)
	assert.Equal(t, ErrorCodeInvalidRequest, unsupported.Code)
	assert.True(t, strings.Contains(err.Error(), "minilm"))
	assert.True(t, strings.Contains(err.Error(), "bge-m3"))
	assert.True(t, strings.Contains(err.Error(), "v0.12.0"))
}

// ---------------------------------------------------------------------------
// 3. Client.Capabilities + pre-flight
// ---------------------------------------------------------------------------

func TestClientCapabilities_CachedAndRefreshed(t *testing.T) {
	srv, calls := capabilitiesServer(t, http.StatusOK)
	defer srv.Close()
	client := NewClient(srv.URL)
	ctx := context.Background()

	first, err := client.Capabilities(ctx)
	require.NoError(t, err)
	second, err := client.Capabilities(ctx)
	require.NoError(t, err)
	assert.Same(t, first, second)
	assert.Equal(t, []string{"GET /v1/capabilities"}, calls())
	assert.True(t, first.ReembedPending)
	assert.True(t, first.SupportsRecords())

	third, err := client.RefreshCapabilities(ctx)
	require.NoError(t, err)
	assert.NotSame(t, first, third)
	assert.Equal(t, []string{"GET /v1/capabilities", "GET /v1/capabilities"}, calls())
}

func TestClientCapabilities_Pre012ServerReturnsNotFound(t *testing.T) {
	srv, _ := capabilitiesServer(t, http.StatusNotFound)
	defer srv.Close()
	client := NewClient(srv.URL)

	_, err := client.Capabilities(context.Background())
	require.Error(t, err)
	var notFound *NotFoundError
	assert.True(t, errors.As(err, &notFound))
}

func TestPreflight_UnsupportedModelIsRejectedBeforeSending(t *testing.T) {
	srv, calls := capabilitiesServer(t, http.StatusOK)
	defer srv.Close()
	client := NewClient(srv.URL)
	ctx := context.Background()

	_, err := client.Capabilities(ctx) // populate the cache; no Preflight flag needed
	require.NoError(t, err)

	_, err = client.UpsertText(ctx, "ns", []TextDocument{{ID: "d", Text: "t"}}, &TextUpsertOptions{Model: EmbeddingModelMiniLM})
	require.Error(t, err)
	var unsupported *UnsupportedCapabilityError
	require.True(t, errors.As(err, &unsupported))
	assert.Equal(t, CapabilityModel, unsupported.Kind)
	assert.Contains(t, unsupported.Supported, "bge-m3")

	_, err = client.QueryText(ctx, "ns", "q", &TextQueryOptions{Model: "minilm"})
	assert.True(t, errors.As(err, &unsupported))
	_, err = client.BatchQueryText(ctx, "ns", []string{"q"}, &BatchTextQueryOptions{Model: "minilm"})
	assert.True(t, errors.As(err, &unsupported))
	_, err = client.CreateNamespace(ctx, "ns", &CreateNamespaceOptions{Dimensions: 8, IndexType: "flat"})
	require.True(t, errors.As(err, &unsupported))
	assert.Equal(t, CapabilityIndexKind, unsupported.Kind)
	_, err = client.ConfigureNamespace(ctx, "ns", ConfigureNamespaceRequest{Dimension: 8, Distance: "manhattan"})
	require.True(t, errors.As(err, &unsupported))
	assert.Equal(t, CapabilityDistanceMetric, unsupported.Kind)

	// Nothing but the capabilities GET went over the wire.
	assert.Equal(t, []string{"GET /v1/capabilities"}, calls())
}

func TestPreflight_SupportedModelAndAliasPassThrough(t *testing.T) {
	srv, calls := capabilitiesServer(t, http.StatusOK)
	defer srv.Close()
	client := NewClient(srv.URL)
	ctx := context.Background()
	_, err := client.Capabilities(ctx)
	require.NoError(t, err)

	_, err = client.UpsertText(ctx, "ns", []TextDocument{{ID: "d", Text: "t"}}, &TextUpsertOptions{Model: EmbeddingModelBgeM3})
	require.NoError(t, err)
	_, err = client.UpsertText(ctx, "ns", []TextDocument{{ID: "d", Text: "t"}}, &TextUpsertOptions{Model: "baai/bge-m3"}) // alias
	require.NoError(t, err)
	assert.Equal(t, []string{"GET /v1/capabilities", "POST /v1/namespaces/ns/upsert-text", "POST /v1/namespaces/ns/upsert-text"}, calls())
}

func TestPreflight_RequireSupportedSearchModeAndLanguage(t *testing.T) {
	srv, _ := capabilitiesServer(t, http.StatusOK)
	defer srv.Close()
	client := NewClient(srv.URL)
	ctx := context.Background()

	assert.NoError(t, client.RequireSupported(ctx, CapabilitySearchMode, string(SearchModeRaBitQ)))
	assert.NoError(t, client.RequireSupported(ctx, CapabilitySearchMode, "sq")) // alias expanded from the prose field
	err := client.RequireSupported(ctx, CapabilitySearchMode, "exact")
	var unsupported *UnsupportedCapabilityError
	require.True(t, errors.As(err, &unsupported))
	assert.Equal(t, []string{"hybrid", "binary", "float", "scalar", "sq", "rabitq", "warp9"}, unsupported.Supported)
	assert.NoError(t, client.RequireSupported(ctx, CapabilityQueryLanguage, "fr"))
	assert.Error(t, client.RequireSupported(ctx, CapabilityQueryLanguage, "ja"))
}

func TestPreflight_OffByDefaultWithoutCache(t *testing.T) {
	srv, calls := capabilitiesServer(t, http.StatusOK)
	defer srv.Close()
	client := NewClient(srv.URL)

	_, err := client.UpsertText(context.Background(), "ns", []TextDocument{{ID: "d", Text: "t"}}, &TextUpsertOptions{Model: EmbeddingModelMiniLM})
	require.NoError(t, err)
	assert.Equal(t, []string{"POST /v1/namespaces/ns/upsert-text"}, calls())
}

func TestPreflight_FlagFetchesLazily(t *testing.T) {
	srv, calls := capabilitiesServer(t, http.StatusOK)
	defer srv.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: srv.URL, Preflight: true})

	_, err := client.UpsertText(context.Background(), "ns", []TextDocument{{ID: "d", Text: "t"}}, &TextUpsertOptions{Model: EmbeddingModelMiniLM})
	var unsupported *UnsupportedCapabilityError
	require.True(t, errors.As(err, &unsupported))
	assert.Equal(t, []string{"GET /v1/capabilities"}, calls())
}

func TestPreflight_FlagDegradesOnPre012Server(t *testing.T) {
	srv, calls := capabilitiesServer(t, http.StatusNotFound)
	defer srv.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: srv.URL, Preflight: true})
	ctx := context.Background()

	_, err := client.UpsertText(ctx, "ns", []TextDocument{{ID: "d", Text: "t"}}, &TextUpsertOptions{Model: EmbeddingModelMiniLM})
	require.NoError(t, err)
	_, err = client.UpsertText(ctx, "ns", []TextDocument{{ID: "d", Text: "t"}}, &TextUpsertOptions{Model: EmbeddingModelMiniLM})
	require.NoError(t, err)
	// 404 once, then never asked again for this client.
	assert.Equal(t, []string{"GET /v1/capabilities", "POST /v1/namespaces/ns/upsert-text", "POST /v1/namespaces/ns/upsert-text"}, calls())
}
