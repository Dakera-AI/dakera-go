package dakera

// Server v0.12.2 support: agents, key PATCH / rotation grace / whoami, key
// grant syntax, namespace kinds, capabilities v2, session idle lifecycle,
// derived-record listings, content previews, derivation status / drain and
// the additive response fields. Shapes follow the server's serializers.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int { return &v }

// captured is what a test server saw.
type captured struct {
	method string
	path   string
	query  map[string][]string
	body   []byte
}

func (c *captured) jsonBody(t *testing.T) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(c.body, &out), "body: %s", string(c.body))
	return out
}

// v0122Server answers every request with status and resp, recording it in got.
func v0122Server(t *testing.T, got *captured, status int, resp interface{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.path = r.URL.Path
		got.query = r.URL.Query()
		got.body = body
		v12JSON(w, status, resp)
	}))
	t.Cleanup(server.Close)
	return server
}

// ---------------------------------------------------------------------------
// POST /v1/agents
// ---------------------------------------------------------------------------

func TestV0122_CreateAgent_Created(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 201, map[string]interface{}{
		"agent_id": "mlx-dev", "namespace": "_dakera_agent_mlx-dev", "created": true,
		"dimension": 1024, "model": "bge-large",
	})
	resp, err := v12Client(server.URL).CreateAgent(context.Background(), "mlx-dev")
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/v1/agents", got.path)
	assert.Equal(t, map[string]interface{}{"agent_id": "mlx-dev"}, got.jsonBody(t))
	assert.True(t, resp.Created)
	assert.Equal(t, "_dakera_agent_mlx-dev", resp.Namespace)
	require.NotNil(t, resp.Dimension)
	assert.Equal(t, 1024, *resp.Dimension)
	assert.Equal(t, "bge-large", resp.Model)
}

func TestV0122_CreateAgent_Existing(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"agent_id": "mlx-dev", "namespace": "_dakera_agent_mlx-dev", "created": false,
		"dimension": nil, "model": "bge-large",
	})
	resp, err := v12Client(server.URL).CreateAgent(context.Background(), "mlx-dev")
	require.NoError(t, err)
	assert.False(t, resp.Created)
	assert.Nil(t, resp.Dimension)
}

// ---------------------------------------------------------------------------
// Keys: grants, PATCH, rotation, whoami
// ---------------------------------------------------------------------------

func TestV0122_NamespaceGrantsEncoding(t *testing.T) {
	cases := []struct {
		name string
		req  UpdateKeyRequest
		want string
	}{
		{"absent", UpdateKeyRequest{Name: strPtr("renamed")}, `{"name":"renamed"}`},
		{"null", UpdateKeyRequest{Namespaces: GrantAllNamespaces()}, `{"namespaces":null}`},
		{"empty", UpdateKeyRequest{Namespaces: GrantNamespaces()}, `{"namespaces":[]}`},
		{"zero value", UpdateKeyRequest{Namespaces: &NamespaceGrants{}}, `{"namespaces":[]}`},
		{"list", UpdateKeyRequest{Namespaces: GrantNamespaces("team-*", "docs")}, `{"namespaces":["team-*","docs"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.req)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(b))
		})
	}

	var g NamespaceGrants
	require.NoError(t, json.Unmarshal([]byte(`null`), &g))
	assert.True(t, g.All)
	require.NoError(t, json.Unmarshal([]byte(`["a*"]`), &g))
	assert.False(t, g.All)
	assert.Equal(t, []string{"a*"}, g.Names)
}

var keyInfoV0122 = map[string]interface{}{
	"key_id":           "dk_key_1a2b3c4d",
	"name":             "dev",
	"scope":            "write",
	"namespaces":       []string{"_dakera_agent_mlx-*", "foo*", "_dakera_sessions"},
	"created_at":       1791392203,
	"expires_at":       nil,
	"active":           true,
	"grants_version":   0,
	"inert_namespaces": []string{"foo*", "_dakera_sessions"},
}

func TestV0122_UpdateKey(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, keyInfoV0122)
	info, err := v12Client(server.URL).UpdateKey(context.Background(), "dk_key_1a2b3c4d", UpdateKeyRequest{
		Name:       strPtr("dev"),
		Namespaces: GrantNamespaces("_dakera_agent_mlx-*"),
	})
	require.NoError(t, err)
	assert.Equal(t, "PATCH", got.method)
	assert.Equal(t, "/admin/keys/dk_key_1a2b3c4d", got.path)
	assert.JSONEq(t, `{"name":"dev","namespaces":["_dakera_agent_mlx-*"]}`, string(got.body))

	assert.Equal(t, "dk_key_1a2b3c4d", info.KeyID)
	assert.Equal(t, KeyScopeWrite, info.Scope)
	assert.Equal(t, int64(1791392203), info.CreatedAt)
	assert.Nil(t, info.ExpiresAt)
	require.NotNil(t, info.GrantsVersion)
	assert.Equal(t, 0, *info.GrantsVersion)
	assert.Equal(t, []string{"foo*", "_dakera_sessions"}, info.InertNamespaces)
}

func TestV0122_UpdateKey_NullNamespacesAndError(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 409, map[string]interface{}{
		"error": "API key 'dk_key_x' is inactive", "code": "CONFLICT", "status": 409,
	})
	_, err := v12Client(server.URL).UpdateKey(context.Background(), "dk_key_x", UpdateKeyRequest{
		Namespaces: GrantAllNamespaces(),
	})
	require.Error(t, err)
	assert.True(t, IsConflictError(err), "got %T: %v", err, err)
	assert.JSONEq(t, `{"namespaces":null}`, string(got.body))
}

func TestV0122_KeyInfo_OlderServerDecodes(t *testing.T) {
	var info KeyInfo
	require.NoError(t, json.Unmarshal([]byte(`{"key_id":"k","name":"n","scope":"read","namespaces":null,"created_at":1,"expires_at":null,"active":true}`), &info))
	assert.Nil(t, info.GrantsVersion)
	assert.Nil(t, info.Namespaces)
	assert.Empty(t, info.InertNamespaces)
}

func TestV0122_UpdateNamespaceKey(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, keyInfoV0122)
	info, err := v12Client(server.URL).UpdateNamespaceKey(context.Background(), "team-a", "dk_key_1a2b3c4d", UpdateKeyRequest{
		Namespaces: GrantNamespaces("team-a", "team-a*"),
	})
	require.NoError(t, err)
	assert.Equal(t, "PATCH", got.method)
	assert.Equal(t, "/v1/namespaces/team-a/keys/dk_key_1a2b3c4d", got.path)
	assert.JSONEq(t, `{"namespaces":["team-a","team-a*"]}`, string(got.body))
	assert.Equal(t, "dev", info.Name)
}

func TestV0122_RotateKeyWithGrace(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"new_key":            "dk_secret_not_real",
		"key_id":             "dk_key_new",
		"old_key_id":         "dk_key_old",
		"old_key_expires_at": 1791395803,
		"warning":            "Save this new key now!",
	})
	resp, err := v12Client(server.URL).RotateKeyWithOptions(context.Background(), "dk_key_old", &RotateKeyOptions{GraceSecs: intPtr(3600)})
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/admin/keys/dk_key_old/rotate", got.path)
	assert.JSONEq(t, `{"grace_secs":3600}`, string(got.body))
	assert.Equal(t, "dk_key_new", resp.KeyID)
	assert.Equal(t, "dk_key_old", resp.OldKeyID)
	require.NotNil(t, resp.OldKeyExpiresAt)
	assert.Equal(t, int64(1791395803), *resp.OldKeyExpiresAt)
}

func TestV0122_RotateKeyWithoutGraceSendsNoBody(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"new_key": "dk_secret_not_real", "key_id": "dk_key_new", "old_key_id": "dk_key_old",
		"old_key_expires_at": nil, "warning": "w",
	})
	resp, err := v12Client(server.URL).RotateKeyWithOptions(context.Background(), "dk_key_old", nil)
	require.NoError(t, err)
	assert.Empty(t, got.body)
	assert.Nil(t, resp.OldKeyExpiresAt)

	// The legacy RotateKey reads the new secret and the new key's id.
	key, err := v12Client(server.URL).RotateKey(context.Background(), "dk_key_old")
	require.NoError(t, err)
	assert.Equal(t, "dk_secret_not_real", key.Key)
	assert.Equal(t, "dk_key_new", key.ID)
}

func TestV0122_Whoami(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"key_id": "dk_key_1a2b3c4d", "name": "dev", "scope": "write",
		"namespaces": []string{"foo*", "docs"}, "unrestricted": false, "expires_at": nil,
		"grants_version": 0, "inert_namespaces": []string{"foo*"}, "auth_enabled": true,
	})
	who, err := v12Client(server.URL).Whoami(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/v1/auth/whoami", got.path)
	assert.Equal(t, "dk_key_1a2b3c4d", who.KeyID)
	assert.False(t, who.Unrestricted)
	assert.True(t, who.AuthEnabled)
	assert.Equal(t, 0, who.GrantsVersion)
	assert.Equal(t, []string{"foo*"}, who.InertNamespaces)
}

func TestV0122_Whoami_AuthDisabled(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"key_id": "auth-disabled", "name": "Auth Disabled", "scope": "super_admin", "namespaces": nil,
		"unrestricted": true, "expires_at": nil, "grants_version": 1, "inert_namespaces": []string{},
		"auth_enabled": false,
	})
	who, err := v12Client(server.URL).Whoami(context.Background())
	require.NoError(t, err)
	assert.Nil(t, who.Namespaces)
	assert.True(t, who.Unrestricted)
	assert.False(t, who.AuthEnabled)
}

func TestV0122_ListKeysServerShape(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"keys":  []interface{}{keyInfoV0122},
		"total": 1,
	})
	keys, err := v12Client(server.URL).ListKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "dk_key_1a2b3c4d", keys[0].ID)
	assert.Equal(t, "1791392203", keys[0].CreatedAt)
	assert.Equal(t, "", keys[0].ExpiresAt)
	assert.Equal(t, "write", keys[0].Scope)
	require.NotNil(t, keys[0].GrantsVersion)
	assert.Equal(t, []string{"foo*", "_dakera_sessions"}, keys[0].InertNamespaces)
}

func TestV0122_CreateKeySendsScopeAndGrants(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 201, map[string]interface{}{
		"key_id": "dk_key_new", "key": "dk_secret_not_real", "name": "dev", "scope": "write",
		"namespaces": []string{"_dakera_agent_mlx-*"}, "created_at": 1791392203, "expires_at": nil,
		"warning": "Save this key now!",
	})
	key, err := v12Client(server.URL).CreateKey(context.Background(), CreateKeyRequest{
		Name:       "dev",
		Scope:      KeyScopeWrite,
		Namespaces: GrantNamespaces("_dakera_agent_mlx-*"),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"dev","scope":"write","namespaces":["_dakera_agent_mlx-*"]}`, string(got.body))
	assert.Equal(t, "dk_key_new", key.ID)
	assert.Equal(t, "dk_secret_not_real", key.Key)
	assert.Equal(t, "Save this key now!", key.Warning)
}

func TestV0122_NamespaceKeys(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"keys":  []interface{}{keyInfoV0122},
		"total": 1,
	})
	list, err := v12Client(server.URL).ListNamespaceKeys(context.Background(), "team-a")
	require.NoError(t, err)
	require.Len(t, list.Keys, 1)
	assert.Equal(t, "write", list.Keys[0].Scope)
	require.NotNil(t, list.Keys[0].GrantsVersion)
	assert.Equal(t, []string{"foo*", "_dakera_sessions"}, list.Keys[0].InertNamespaces)

	_, err = v12Client(server.URL).CreateNamespaceKey(context.Background(), "team-a", CreateNamespaceKeyRequest{
		Name: "ci", Scope: KeyScopeRead, ExtraNamespaces: []string{"team-b*"},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"ci","scope":"read","extra_namespaces":["team-b*"]}`, string(got.body))
}

// ---------------------------------------------------------------------------
// Namespace kinds, capabilities v2
// ---------------------------------------------------------------------------

func TestV0122_NamespaceKinds(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"namespaces": []string{"docs", "_dakera_agent_a"},
		"kinds":      map[string]string{"docs": "data", "_dakera_agent_a": "agent"},
	})
	list, err := v12Client(server.URL).ListNamespacesWithKinds(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"docs", "_dakera_agent_a"}, list.Namespaces)
	assert.Equal(t, NamespaceKindAgent, list.Kinds["_dakera_agent_a"])
	assert.Equal(t, NamespaceKindData, list.Kinds["docs"])

	names, err := v12Client(server.URL).ListNamespaces(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"docs", "_dakera_agent_a"}, names)

	var info NamespaceInfo
	require.NoError(t, json.Unmarshal([]byte(`{"namespace":"_dakera_agent_a","vector_count":3,"kind":"agent"}`), &info))
	assert.Equal(t, NamespaceKindAgent, info.Kind)
}

func TestV0122_CapabilitiesV2(t *testing.T) {
	doc := []byte(`{
		"capabilities_version": 2,
		"server_version": "0.12.2",
		"auth": {"prefix_patterns": true, "sessions_by_agent": true, "key_update": true,
		         "rotation_grace_max_secs": 604800, "max_grants": 100, "max_grant_len": 255},
		"naming": {"agent_id_pattern": "^[a-zA-Z0-9][a-zA-Z0-9_\\-.]*$", "agent_id_max_bytes": 241,
		           "agent_namespace_prefix": "_dakera_agent_", "agent_namespace_max_bytes": 255,
		           "namespace_pattern": "^[a-zA-Z0-9][a-zA-Z0-9_-]*$", "namespace_max_bytes": 128,
		           "reserved_prefixes": ["_", "system_", "internal_", "admin_"],
		           "internal_namespaces": ["_dakera_sessions", "_dakera_embedding_models"],
		           "internal_prefixes": ["_dakera_reembed_staging_"]},
		"sessions": {"idle_timeout_secs": 14400, "max_idle_timeout_secs": 2592000, "touch": true, "ended_reason": true}
	}`)
	caps, err := ParseCapabilities(doc)
	require.NoError(t, err)
	assert.Equal(t, 2, caps.CapabilitiesVersion)
	require.NotNil(t, caps.Auth)
	assert.True(t, caps.SupportsKeyUpdate())
	assert.True(t, caps.SupportsPrefixGrants())
	assert.Equal(t, int64(MaxRotationGraceSecs), caps.Auth.RotationGraceMaxSecs)
	assert.Equal(t, 100, caps.Auth.MaxGrants)
	require.NotNil(t, caps.Naming)
	assert.Equal(t, 241, caps.Naming.AgentIDMaxBytes)
	assert.Equal(t, "_dakera_agent_", caps.Naming.AgentNamespacePrefix)
	assert.Contains(t, caps.Naming.InternalNamespaces, "_dakera_embedding_models")
	require.NotNil(t, caps.Sessions)
	assert.True(t, caps.SupportsSessionTouch())
	assert.Equal(t, int64(14400), caps.Sessions.IdleTimeoutSecs)
	assert.Equal(t, int64(MaxSessionIdleTimeoutSecs), caps.Sessions.MaxIdleTimeoutSecs)

	old, err := ParseCapabilities([]byte(`{"capabilities_version": 1, "server_version": "0.12.1"}`))
	require.NoError(t, err)
	assert.Nil(t, old.Auth)
	assert.Nil(t, old.Naming)
	assert.Nil(t, old.Sessions)
	assert.False(t, old.SupportsKeyUpdate())
	assert.False(t, old.SupportsSessionTouch())
}

// ---------------------------------------------------------------------------
// Sessions: idle lifecycle
// ---------------------------------------------------------------------------

func TestV0122_StartSessionIdleTimeout(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"session": map[string]interface{}{
			"id": "sess_1", "agent_id": "a", "started_at": 1791392203, "memory_count": 0,
			"last_activity_at": 1791392203, "idle_timeout_secs": 0,
		},
	})
	client := v12Client(server.URL)

	sess, err := client.StartSession(context.Background(), StartSessionRequest{AgentID: "a", IdleTimeoutSecs: intPtr(0)})
	require.NoError(t, err)
	assert.Equal(t, "/v1/sessions/start", got.path)
	assert.JSONEq(t, `{"agent_id":"a","idle_timeout_secs":0}`, string(got.body))
	require.NotNil(t, sess.IdleTimeoutSecs)
	assert.Equal(t, int64(0), *sess.IdleTimeoutSecs)
	require.NotNil(t, sess.LastActivityAt)
	assert.Equal(t, int64(1791392203), *sess.LastActivityAt)
	assert.False(t, sess.IsEnded())

	_, err = client.StartSession(context.Background(), StartSessionRequest{AgentID: "a", ID: "custom"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"agent_id":"a","id":"custom"}`, string(got.body))
}

func TestV0122_SessionFields(t *testing.T) {
	var s Session
	require.NoError(t, json.Unmarshal([]byte(`{"id":"s","agent_id":"a","started_at":1,"ended_at":9,"memory_count":7,
		"last_activity_at":5,"ended_reason":"idle","idle_since":5,"idle_timeout_secs":7200}`), &s))
	assert.Equal(t, SessionEndedIdle, s.EndedReason)
	require.NotNil(t, s.IdleSince)
	assert.Equal(t, int64(5), *s.IdleSince)
	require.NotNil(t, s.IdleTimeoutSecs)
	assert.Equal(t, int64(7200), *s.IdleTimeoutSecs)
	assert.True(t, s.IsEnded())

	// A v0.12.1 session decodes with the new fields unset.
	var old Session
	require.NoError(t, json.Unmarshal([]byte(`{"id":"s","agent_id":"a","started_at":1,"memory_count":0}`), &old))
	assert.Nil(t, old.LastActivityAt)
	assert.Empty(t, old.EndedReason)
	assert.Nil(t, old.IdleTimeoutSecs)
	assert.False(t, old.IsEnded())
}

func TestV0122_TouchSession(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"session":          map[string]interface{}{"id": "sess_1", "agent_id": "a", "started_at": 1, "memory_count": 0, "last_activity_at": 1791393000},
		"session_state":    "active",
		"idle_deadline_at": 1791407400,
	})
	resp, err := v12Client(server.URL).TouchSession(context.Background(), "sess_1")
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/v1/sessions/sess_1/touch", got.path)
	assert.Empty(t, got.body)
	assert.Equal(t, SessionStateActive, resp.SessionState)
	require.NotNil(t, resp.IdleDeadlineAt)
	assert.Equal(t, int64(1791407400), *resp.IdleDeadlineAt)
}

func TestV0122_TouchEndedSession(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"session": map[string]interface{}{"id": "sess_1", "agent_id": "a", "started_at": 1, "ended_at": 9,
			"memory_count": 2, "last_activity_at": 5, "ended_reason": "idle", "idle_since": 5},
		"session_state": "ended",
	})
	sess, err := (&ChatMemorySession{client: v12Client(server.URL), agentID: "a", sessionID: "sess_1"}).Touch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, SessionStateEnded, sess.SessionState)
	assert.Nil(t, sess.IdleDeadlineAt)
	assert.True(t, sess.Session.IsEnded())
}

func TestV0122_TouchSessionNotFound(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 404, map[string]interface{}{"error": "Session not found: sess_x", "code": "NOT_FOUND", "status": 404})
	_, err := v12Client(server.URL).TouchSession(context.Background(), "sess_x")
	require.Error(t, err)
	assert.True(t, IsNotFoundError(err), "got %T: %v", err, err)
}

func TestV0122_StoreSessionState(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"memory":            map[string]interface{}{"id": "m1", "content": "x", "memory_type": "episodic", "importance": 0.5},
		"embedding_time_ms": 12,
		"session_state":     "ended",
	})
	resp, err := v12Client(server.URL).StoreMemory(context.Background(), "a", StoreMemoryRequest{Content: "x", SessionID: "sess_1"})
	require.NoError(t, err)
	assert.Equal(t, SessionStateEnded, resp.SessionState)

	var old StoreMemoryResponse
	require.NoError(t, json.Unmarshal([]byte(`{"memory":{"id":"m1","content":"x","memory_type":"episodic","importance":0.5},"embedding_time_ms":3}`), &old))
	assert.Empty(t, old.SessionState)
}

func TestV0122_BatchStoreEndedSessions(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"stored": []interface{}{}, "stored_count": 3, "total_embedding_time_ms": 40,
		"ended_sessions": []string{"sess_a"},
	})
	resp, err := v12Client(server.URL).StoreMemoriesBatch(context.Background(), BatchStoreMemoryRequest{AgentID: "a"})
	require.NoError(t, err)
	assert.Equal(t, []string{"sess_a"}, resp.EndedSessions)
}

func TestV0122_SessionIdleTimeoutConfig(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"success": true,
		"config":  map[string]interface{}{"session_idle_timeout_secs": 7200, "query_timeout_ms": 0},
		"message": "ok",
	})
	applied, ok, err := v12Client(server.URL).SetSessionIdleTimeout(context.Background(), 7200)
	require.NoError(t, err)
	assert.Equal(t, "PUT", got.method)
	assert.Equal(t, "/v1/admin/config", got.path)
	assert.JSONEq(t, `{"session_idle_timeout_secs":7200}`, string(got.body))
	assert.True(t, ok)
	assert.Equal(t, int64(7200), applied)

	server2 := v0122Server(t, &got, 200, map[string]interface{}{"session_idle_timeout_secs": 0, "cache_enabled": true})
	secs, ok, err := v12Client(server2.URL).GetSessionIdleTimeout(context.Background())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(0), secs)

	server3 := v0122Server(t, &got, 200, map[string]interface{}{"cache_enabled": true})
	_, ok, err = v12Client(server3.URL).GetSessionIdleTimeout(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// Listings: include_derived, content previews
// ---------------------------------------------------------------------------

func TestV0122_AgentMemoriesOptions(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, []map[string]interface{}{
		{"id": "m1", "content": "first ten ", "memory_type": "episodic", "importance": 0.5,
			"content_len": 90000, "content_truncated": true},
	})
	mems, err := v12Client(server.URL).AgentMemories(context.Background(), "a", &AgentMemoriesOptions{
		Limit: intPtr(20), Offset: intPtr(40), IncludeDerived: true, ContentPreviewChars: intPtr(10),
	})
	require.NoError(t, err)
	assert.Equal(t, "/v1/agents/a/memories", got.path)
	assert.Equal(t, "20", got.query["limit"][0])
	assert.Equal(t, "40", got.query["offset"][0])
	assert.Equal(t, "true", got.query["include_derived"][0])
	assert.Equal(t, "10", got.query["content_preview_chars"][0])
	require.Len(t, mems, 1)
	require.NotNil(t, mems[0].ContentLen)
	assert.Equal(t, 90000, *mems[0].ContentLen)
	require.NotNil(t, mems[0].ContentTruncated)
	assert.True(t, *mems[0].ContentTruncated)

	// Defaults send neither include_derived nor content_preview_chars.
	_, err = v12Client(server.URL).AgentMemories(context.Background(), "a", &AgentMemoriesOptions{Limit: intPtr(5)})
	require.NoError(t, err)
	assert.NotContains(t, got.query, "include_derived")
	assert.NotContains(t, got.query, "content_preview_chars")
}

func TestV0122_SessionMemoriesServerShape(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"session": map[string]interface{}{"id": "sess_1", "agent_id": "a", "started_at": 1, "memory_count": 2, "last_activity_at": 3},
		"memories": []map[string]interface{}{
			{"id": "m1", "content": "abc", "memory_type": "episodic", "importance": 0.5, "content_len": 3, "content_truncated": false},
		},
		"total": 2,
	})
	client := v12Client(server.URL)
	resp, err := client.SessionMemoriesWithOptions(context.Background(), "sess_1", &SessionMemoriesOptions{
		Limit: intPtr(1), Offset: intPtr(1), ContentPreviewChars: intPtr(200),
	})
	require.NoError(t, err)
	assert.Equal(t, "/v1/sessions/sess_1/memories", got.path)
	assert.Equal(t, "1", got.query["limit"][0])
	assert.Equal(t, "1", got.query["offset"][0])
	assert.Equal(t, "200", got.query["content_preview_chars"][0])
	require.NotNil(t, resp.Session)
	assert.Equal(t, "sess_1", resp.Session.ID)
	require.NotNil(t, resp.Total)
	assert.Equal(t, 2, *resp.Total)
	require.Len(t, resp.Memories, 1)
	require.NotNil(t, resp.Memories[0].ContentTruncated)
	assert.False(t, *resp.Memories[0].ContentTruncated)

	// SessionMemories reads the same object answer.
	mems, err := client.SessionMemories(context.Background(), "sess_1")
	require.NoError(t, err)
	require.Len(t, mems, 1)
	assert.Equal(t, "m1", mems[0].ID)
	assert.Empty(t, got.query)
}

func TestV0122_WakeUpIncludeDerived(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{"agent_id": "a", "memories": []interface{}{}, "total_available": 0})
	_, err := v12Client(server.URL).GetWakeUpContext(context.Background(), "a", &WakeUpOptions{IncludeDerived: true})
	require.NoError(t, err)
	assert.Equal(t, "true", got.query["include_derived"][0])

	_, err = v12Client(server.URL).GetWakeUpContext(context.Background(), "a", &WakeUpOptions{TopN: intPtr(5)})
	require.NoError(t, err)
	assert.NotContains(t, got.query, "include_derived")
}

func TestV0122_FullKnowledgeGraphPreview(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "mem_a", "content": "first", "content_len": 90000, "content_truncated": true, "memory_type": "Semantic",
				"importance": 0.9, "tags": []string{"t"}, "created_at": "1700000000", "cluster_id": 0, "centrality": 1.0},
			{"id": "mem_b", "content": "short", "content_len": 5, "content_truncated": false, "memory_type": "Semantic",
				"importance": 0.5, "tags": []string{}, "created_at": nil, "cluster_id": 1, "centrality": 0.5},
		},
		"edges":    []map[string]interface{}{{"source": "mem_a", "target": "mem_b", "similarity": 0.81, "shared_tags": []string{"t"}}},
		"clusters": []map[string]interface{}{{"id": 0, "node_count": 1, "top_tags": []string{"t"}, "avg_importance": 0.9}, {"id": 1, "node_count": 1, "top_tags": []string{}, "avg_importance": 0.5}},
		"stats":    map[string]interface{}{"total_memories": 4, "included_memories": 2, "total_edges": 1, "cluster_count": 2, "density": 0.5, "hub_memory_id": "mem_a"},
	})
	graph, err := v12Client(server.URL).FullKnowledgeGraph(context.Background(), FullKnowledgeGraphRequest{
		AgentID: "a", ContentPreviewChars: intPtr(5),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"agent_id":"a","content_preview_chars":5}`, string(got.body))
	require.Len(t, graph.Nodes, 2)
	require.NotNil(t, graph.Nodes[0].ContentLen)
	assert.Equal(t, 90000, *graph.Nodes[0].ContentLen)
	assert.True(t, *graph.Nodes[0].ContentTruncated)
	assert.Nil(t, graph.Nodes[1].CreatedAt)
	assert.Equal(t, []string{"t"}, graph.Edges[0].SharedTags)
	require.Len(t, graph.ClusterInfo, 2)
	assert.Equal(t, [][]string{{"mem_a"}, {"mem_b"}}, graph.Clusters)
	require.NotNil(t, graph.Stats)
	assert.Equal(t, 4, graph.Stats.TotalMemories)
	require.NotNil(t, graph.Stats.HubMemoryID)
	assert.Equal(t, "mem_a", *graph.Stats.HubMemoryID)
}

func TestV0122_CrossAgentNetworkPreview(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"agents": []interface{}{},
		"nodes": []map[string]interface{}{
			{"id": "m1", "agent_id": "a", "content": "abc", "content_len": 1234, "content_truncated": true,
				"importance": 0.7, "tags": []string{}, "memory_type": "Semantic", "created_at": 1700000000},
		},
		"edges": []interface{}{},
		"stats": map[string]interface{}{"total_agents": 1, "total_nodes": 1, "total_cross_edges": 0, "density": 0},
	})
	resp, err := v12Client(server.URL).CrossAgentNetwork(context.Background(), CrossAgentNetworkRequest{ContentPreviewChars: intPtr(3)})
	require.NoError(t, err)
	assert.JSONEq(t, `{"content_preview_chars":3}`, string(got.body))
	require.Len(t, resp.Nodes, 1)
	require.NotNil(t, resp.Nodes[0].ContentLen)
	assert.Equal(t, 1234, *resp.Nodes[0].ContentLen)
	assert.True(t, *resp.Nodes[0].ContentTruncated)
}

// ---------------------------------------------------------------------------
// Derivations
// ---------------------------------------------------------------------------

var derivationStatusV0122 = map[string]interface{}{
	"settled": false, "pending_sentences": 3, "pending_parents": 1, "unmarked_parents": 0,
	"stale_children": 0, "orphan_children": 0, "remeta_children": 0, "duplicate_children": 0,
	"legacy_children": 0, "bm25_missing": 2, "graph_owed": 0, "in_flight": 1, "graph_queue_owed": 0,
	"dirty_namespaces": []string{"_dakera_agent_x"}, "namespaces": 3, "unreadable_namespaces": []string{},
	"heal": map[string]interface{}{"version": 1, "complete": true, "namespace": nil, "cursor": nil,
		"parents_healed": 12, "graph_adopted": 40, "started_at": 1760000000, "completed_at": 1760000010},
	"reconciler": map[string]interface{}{"state": "sleeping", "last_tick_at": 1760000100, "ticks": 7, "next_namespace": "_dakera_agent_x"},
	"counters":   map[string]interface{}{"derived": 5, "bm25_restored": 1},
}

func TestV0122_DerivationStatus(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, derivationStatusV0122)
	st, err := v12Client(server.URL).AdminDerivationStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/v1/admin/derivations/status", got.path)
	assert.False(t, st.Settled)
	assert.Equal(t, int64(3), st.PendingSentences)
	assert.Equal(t, int64(2), st.BM25Missing)
	assert.Equal(t, []string{"_dakera_agent_x"}, st.DirtyNamespaces)
	require.NotNil(t, st.Heal)
	assert.True(t, st.Heal.Complete)
	assert.Nil(t, st.Heal.Namespace)
	assert.Equal(t, "sleeping", st.Reconciler.State)
	assert.Equal(t, int64(5), st.Counters.Derived)
}

func TestV0122_DrainDerivations(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"settled": true, "timed_out": false, "rounds": 1, "elapsed_ms": 1234, "parents_run": 17,
		"pending_left": 0, "deleted": 3, "bm25_restored": 0, "graph_queued": 1,
		"status": map[string]interface{}{"settled": true, "reconciler": map[string]interface{}{"state": "idle", "ticks": 0}, "heal": nil},
	})
	client := v12Client(server.URL)
	resp, err := client.AdminDrainDerivations(context.Background(), &DrainDerivationsRequest{TimeoutSecs: intPtr(120)})
	require.NoError(t, err)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/v1/admin/derivations/drain", got.path)
	assert.JSONEq(t, `{"timeout_secs":120}`, string(got.body))
	assert.True(t, resp.Settled)
	assert.Equal(t, int64(17), resp.ParentsRun)
	assert.True(t, resp.Status.Settled)
	assert.Nil(t, resp.Status.Heal)

	_, err = client.AdminDrainDerivations(context.Background(), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(got.body))
}

func TestV0122_DrainDerivationsConflict(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 409, map[string]interface{}{
		"error": "derivation drain already in progress", "code": "CONFLICT", "status": 409,
	})
	_, err := v12Client(server.URL).AdminDrainDerivations(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, IsConflictError(err), "got %T: %v", err, err)
}

// ---------------------------------------------------------------------------
// Additive response fields
// ---------------------------------------------------------------------------

func TestV0122_DeduplicateServerShape(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"groups": []map[string]interface{}{
			{"canonical_id": "m1", "duplicate_ids": []string{"m2", "m3"}, "avg_similarity": 0.97},
		},
		"duplicates_found": 3, "duplicates_merged": 2, "duplicates_skipped_changed": 1,
	})
	resp, err := v12Client(server.URL).Deduplicate(context.Background(), DeduplicateRequest{AgentID: "a"})
	require.NoError(t, err)
	assert.Equal(t, 3, resp.DuplicatesFound)
	assert.Equal(t, 2, resp.DuplicatesMerged)
	assert.Equal(t, 2, resp.RemovedCount)
	assert.Equal(t, 1, resp.DuplicatesSkippedChanged)
	require.Len(t, resp.DuplicateGroups, 1)
	assert.Equal(t, "m1", resp.DuplicateGroups[0].CanonicalID)
	assert.Equal(t, [][]string{{"m1", "m2", "m3"}}, resp.Groups)
}

func TestV0122_CompressSummariesSkipped(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"agent_id": "a", "memories_scanned": 40, "clusters_found": 3, "summaries_created": 2,
		"originals_deprecated": 9, "summary_ids": []string{"mem_compress_1", "mem_compress_2"},
		"deprecated_ids": []string{"m1"},
		"summaries_skipped": []map[string]interface{}{
			{"summary_id": "mem_compress_3", "reason": "content: content exceeds maximum of 100000 bytes (100001 bytes)"},
		},
	})
	resp, err := v12Client(server.URL).CompressAgent(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, int64(40), resp.MemoriesScanned)
	assert.Equal(t, int64(2), resp.SummariesCreated)
	assert.Equal(t, int64(9), resp.OriginalsDeprecated)
	assert.Len(t, resp.SummaryIDs, 2)
	require.Len(t, resp.SummariesSkipped, 1)
	assert.Equal(t, "mem_compress_3", resp.SummariesSkipped[0].SummaryID)
	assert.Contains(t, resp.SummariesSkipped[0].Reason, "bytes")
}

func TestV0122_UnavailableNamespaces(t *testing.T) {
	var got captured
	server := v0122Server(t, &got, 200, map[string]interface{}{
		"version": "0.12.2", "total_vectors": 10, "namespace_count": 3, "uptime_seconds": 1, "timestamp": 1, "state": "ok",
		"unavailable": []map[string]string{{"namespace": "_dakera_agent_y", "reason": "did not answer within 2000 ms"}},
	})
	stats, err := v12Client(server.URL).OpsStats(context.Background())
	require.NoError(t, err)
	require.Len(t, stats.Unavailable, 1)
	assert.Equal(t, "_dakera_agent_y", stats.Unavailable[0].Namespace)

	server2 := v0122Server(t, &got, 200, []map[string]interface{}{
		{"agent_id": "x", "memory_count": 0, "vector_count": 0, "session_count": 0, "active_sessions": 0, "unavailable": "did not answer within 2000 ms"},
		{"agent_id": "y", "memory_count": 2, "vector_count": 5, "session_count": 1, "active_sessions": 1},
	})
	agents, err := v12Client(server2.URL).ListAgents(context.Background())
	require.NoError(t, err)
	require.Len(t, agents, 2)
	assert.Equal(t, "did not answer within 2000 ms", agents[0].Unavailable)
	assert.Empty(t, agents[1].Unavailable)
	assert.Equal(t, int64(5), agents[1].VectorCount)
}

func TestV0122_AgentStatsNumericTimestamps(t *testing.T) {
	var stats AgentStats
	require.NoError(t, json.Unmarshal([]byte(`{"agent_id":"a","total_memories":2,"sub_memories":5,"memories_by_type":{"Episodic":2},
		"total_sessions":1,"active_sessions":0,"avg_importance":0.5,"oldest_memory_at":1700000000,"newest_memory_at":null}`), &stats))
	assert.Equal(t, "1700000000", stats.OldestMemoryAt)
	assert.Equal(t, "", stats.NewestMemoryAt)
	assert.Equal(t, int64(5), stats.SubMemories)
	assert.Equal(t, int64(2), stats.TotalMemories)
}

func TestV0122_SessionEndedEventReason(t *testing.T) {
	var ev MemoryEvent
	require.NoError(t, json.Unmarshal([]byte(`{"event_type":"session_ended","agent_id":"a","session_id":"s","timestamp":1791406603000,"reason":"idle"}`), &ev))
	require.NotNil(t, ev.Reason)
	assert.Equal(t, SessionEndedIdle, *ev.Reason)
}
