package dakera

// API keys and identity (server v0.12.2): key grants with prefix patterns,
// PATCH of a key's name and namespaces, rotation with a grace period, and
// GET /v1/auth/whoami.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Scope values of an API key.
const (
	// KeyScopeRead reads memories, sessions and vectors.
	KeyScopeRead = "read"
	// KeyScopeWrite also writes them.
	KeyScopeWrite = "write"
	// KeyScopeAdmin also manages namespaces and namespace keys.
	KeyScopeAdmin = "admin"
	// KeyScopeSuperAdmin manages every key and the server.
	KeyScopeSuperAdmin = "super_admin"
)

// MaxRotationGraceSecs is the longest grace period a rotation may give the old
// key (7 days); ServerCapabilities.Auth.RotationGraceMaxSecs reports the
// server's value.
const MaxRotationGraceSecs = 604800

// NamespaceGrants is the namespaces field of a key create or update request.
// It distinguishes the three values the server gives a meaning:
//
//   - a nil *NamespaceGrants omits the field (on update: leave the list
//     unchanged; on create: every namespace);
//   - GrantAllNamespaces() sends null (every namespace);
//   - GrantNamespaces(names...) sends the list; with no names it sends []
//     (no namespace).
//
// An entry is an exact namespace name or, from server v0.12.2, a prefix
// pattern with one trailing star ("team-*" reaches "team-a" and "team-a.b",
// not "team-" or "team"); "*" alone means every namespace. At most 100
// entries; the server refuses server-internal names and junk entries with
// 400. ["_dakera_agent_mlx-*"] lets a developer key create and use its own
// agents whose ids start with "mlx-".
type NamespaceGrants struct {
	// All grants every namespace (sent as null). Names is ignored when set.
	All bool
	// Names are the grants (sent as a list, [] when empty).
	Names []string
}

// GrantAllNamespaces returns grants that reach every namespace (JSON null).
func GrantAllNamespaces() *NamespaceGrants {
	return &NamespaceGrants{All: true}
}

// GrantNamespaces returns grants limited to names; with no names the key
// reaches no namespace (JSON []).
func GrantNamespaces(names ...string) *NamespaceGrants {
	return &NamespaceGrants{Names: append([]string{}, names...)}
}

// MarshalJSON renders null for All and the list (never null) otherwise.
func (g NamespaceGrants) MarshalJSON() ([]byte, error) {
	if g.All {
		return []byte("null"), nil
	}
	if g.Names == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(g.Names)
}

// UnmarshalJSON reads null as All and a list as Names.
func (g *NamespaceGrants) UnmarshalJSON(data []byte) error {
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}
	if names == nil {
		*g = NamespaceGrants{All: true}
		return nil
	}
	*g = NamespaceGrants{Names: names}
	return nil
}

// KeyInfo is an API key as the server describes it (no secret): GET/PATCH
// /admin/keys/{id}, PATCH /v1/namespaces/{ns}/keys/{id}.
type KeyInfo struct {
	KeyID string `json:"key_id"`
	Name  string `json:"name"`
	// Scope is read, write, admin or super_admin.
	Scope string `json:"scope"`
	// Namespaces are the key's grants: nil = every namespace (JSON null),
	// empty = no namespace.
	Namespaces []string `json:"namespaces"`
	// CreatedAt is the creation time (Unix seconds).
	CreatedAt int64 `json:"created_at"`
	// ExpiresAt is the expiry (Unix seconds); nil = never.
	ExpiresAt *int64 `json:"expires_at,omitempty"`
	Active    bool   `json:"active"`
	// GrantsVersion is the grant syntax the server reads Namespaces with
	// (server v0.12.2+): 1 = exact names and "p*" patterns; 0 = a key
	// created before v0.12.2, whose "foo*" entries are literal names that
	// grant nothing until its namespaces are saved again (UpdateKey with
	// Namespaces). nil on older servers.
	GrantsVersion *int `json:"grants_version,omitempty"`
	// InertNamespaces are the entries of Namespaces that grant nothing: a
	// legacy "foo*", or a server-internal name such as _dakera_sessions
	// (server v0.12.2+).
	InertNamespaces []string `json:"inert_namespaces,omitempty"`
}

// UpdateKeyRequest is the body of PATCH /admin/keys/{id} and PATCH
// /v1/namespaces/{ns}/keys/{id} (server v0.12.2+). At least one field must be
// set. A key's scope, secret, expiry and state cannot be changed this way.
type UpdateKeyRequest struct {
	// Name renames the key; nil leaves it unchanged.
	Name *string `json:"name,omitempty"`
	// Namespaces replaces the key's grants; nil leaves them unchanged,
	// GrantAllNamespaces() sends null (every namespace — an unrestricted
	// caller only), GrantNamespaces() sends [] (no namespace). Saving the
	// namespaces moves the key to grant syntax 1 (patterns active).
	Namespaces *NamespaceGrants `json:"namespaces,omitempty"`
}

// RotateKeyOptions are the optional parameters of RotateKeyWithOptions.
type RotateKeyOptions struct {
	// GraceSecs keeps the old key working for this many seconds after the
	// rotation (0..MaxRotationGraceSecs; server v0.12.2+). nil or 0
	// deactivates the old key at once, as before. Older servers ignore it.
	GraceSecs *int `json:"grace_secs,omitempty"`
}

// RotateKeyResponse is the response from POST /admin/keys/{id}/rotate.
type RotateKeyResponse struct {
	// NewKey is the new secret, shown only once.
	NewKey string `json:"new_key"`
	// KeyID is the NEW key's id.
	KeyID string `json:"key_id"`
	// OldKeyID is the id of the rotated key (server v0.12.2+).
	OldKeyID string `json:"old_key_id,omitempty"`
	// OldKeyExpiresAt is when the old key stops working with a grace period
	// (Unix seconds); nil when it was deactivated at once.
	OldKeyExpiresAt *int64 `json:"old_key_expires_at,omitempty"`
	Warning         string `json:"warning,omitempty"`
}

// WhoamiResponse is the response from GET /v1/auth/whoami (server v0.12.2+):
// the key the request authenticated with, as the server reads it.
type WhoamiResponse struct {
	// KeyID is the key's id ("auth-disabled" when authentication is off).
	KeyID string `json:"key_id"`
	Name  string `json:"name"`
	Scope string `json:"scope"`
	// Namespaces are the key's grants: nil = every namespace.
	Namespaces []string `json:"namespaces"`
	// Unrestricted reports whether the key reaches every namespace.
	Unrestricted bool   `json:"unrestricted"`
	ExpiresAt    *int64 `json:"expires_at,omitempty"`
	// GrantsVersion is the grant syntax (see KeyInfo.GrantsVersion).
	GrantsVersion int `json:"grants_version"`
	// InertNamespaces are the grants that grant nothing.
	InertNamespaces []string `json:"inert_namespaces"`
	// AuthEnabled is false when the server runs without authentication; every
	// caller is then an unrestricted super-admin.
	AuthEnabled bool `json:"auth_enabled"`
}

// UpdateKey renames a key or replaces its namespaces — PATCH
// /admin/keys/{keyID} (server v0.12.2+, super_admin with no namespace
// restriction). Returns the stored key.
//
// The env root key gets 400, an inactive key 409 (*ConflictError), an unknown
// key 404, and an invalid grant 400 naming the entry. A server older than
// v0.12.2 has no such route.
func (c *Client) UpdateKey(ctx context.Context, keyID string, req UpdateKeyRequest) (*KeyInfo, error) {
	data, err := c.request(ctx, "PATCH", fmt.Sprintf("/admin/keys/%s", url.PathEscape(keyID)), req)
	if err != nil {
		return nil, err
	}
	var result KeyInfo
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key info: %w", err)
	}
	return &result, nil
}

// UpdateNamespaceKey renames a key or replaces its namespaces as a namespace
// admin — PATCH /v1/namespaces/{namespace}/keys/{keyID} (server v0.12.2+,
// admin scope on namespace).
//
// The key must reach namespace and be manageable by the caller (its scope
// within the caller's, its grants contained in the caller's), otherwise the
// server answers 404 as if it did not exist. Every new grant must be contained
// in the caller's grants (403 otherwise); GrantAllNamespaces needs an
// unrestricted caller.
func (c *Client) UpdateNamespaceKey(ctx context.Context, namespace, keyID string, req UpdateKeyRequest) (*KeyInfo, error) {
	path := fmt.Sprintf("/v1/namespaces/%s/keys/%s", url.PathEscape(namespace), url.PathEscape(keyID))
	data, err := c.request(ctx, "PATCH", path, req)
	if err != nil {
		return nil, err
	}
	var result KeyInfo
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key info: %w", err)
	}
	return &result, nil
}

// RotateKeyWithOptions rotates a key — POST /admin/keys/{keyID}/rotate. The
// new key keeps the old one's name, scope, namespaces, expiry and grant
// syntax.
//
// With opts.GraceSecs set (server v0.12.2+) the old key keeps working until
// min(its own expiry, now + GraceSecs) and the response carries
// OldKeyExpiresAt; otherwise (nil opts, nil or 0 GraceSecs) the old key is
// deactivated at once and no body is sent, exactly as RotateKey does.
func (c *Client) RotateKeyWithOptions(ctx context.Context, keyID string, opts *RotateKeyOptions) (*RotateKeyResponse, error) {
	var body interface{}
	if opts != nil && opts.GraceSecs != nil {
		body = opts
	}
	data, err := c.request(ctx, "POST", fmt.Sprintf("/admin/keys/%s/rotate", url.PathEscape(keyID)), body)
	if err != nil {
		return nil, err
	}
	var result RotateKeyResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rotate key response: %w", err)
	}
	return &result, nil
}

// Whoami returns the key this client authenticates with, as the server reads
// it — GET /v1/auth/whoami (server v0.12.2+; any valid key, no scope needed).
// An invalid or missing key gets 401; an older server answers 404.
func (c *Client) Whoami(ctx context.Context) (*WhoamiResponse, error) {
	data, err := c.request(ctx, "GET", "/v1/auth/whoami", nil)
	if err != nil {
		return nil, err
	}
	var result WhoamiResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal whoami response: %w", err)
	}
	return &result, nil
}
