package dakera

// Document import (server v0.12.3+, Dakera-AI/dakera#923): a PDF, DOCX,
// HTML, Markdown or text document becomes a set of chunk memories of an
// agent, each one carrying its breadcrumb and its provenance under the
// "_dakera_document" metadata key (DocumentChunkMeta). Shapes follow the
// server's crates/common/src/types/document.rs and documents/routes.rs.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

const (
	// DocumentMetaKey is the user-metadata key of a chunk memory.
	DocumentMetaKey = "_dakera_document"
	// DocumentTagPrefix prefixes the tag every chunk of a document carries:
	// "document:<document_id>".
	DocumentTagPrefix = "document:"
	// DocumentImportJobType is the job type of document imports in /ops/jobs.
	DocumentImportJobType = "document_import"

	// MediaTypeTextPlain, MediaTypeMarkdown and MediaTypeHTML are the media
	// types ImportDocument accepts (text/plain is the default).
	MediaTypeTextPlain = "text/plain"
	MediaTypeMarkdown  = "text/markdown"
	MediaTypeHTML      = "text/html"
	// MediaTypePDF and MediaTypeDOCX are attachment media types
	// ImportAttachmentDocument can read (besides HTML, Markdown and text).
	MediaTypePDF  = "application/pdf"
	MediaTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// ChunkStrategy says how the extracted text is split. Unknown values a newer
// server sends are kept as they are.
type ChunkStrategy string

const (
	// ChunkStrategyStructure splits at headings, then paragraphs, sentences
	// and words (the server default).
	ChunkStrategyStructure ChunkStrategy = "structure"
	// ChunkStrategyFixed uses plain token windows with overlap; headings are
	// text.
	ChunkStrategyFixed ChunkStrategy = "fixed"
)

// ChunkingOptions is the "chunking" field of an import. The server refuses
// unknown fields in it; unset fields are omitted (server defaults:
// DAKERA_DOCUMENT_CHUNK_TOKENS, DAKERA_DOCUMENT_CHUNK_OVERLAP_TOKENS,
// "structure").
type ChunkingOptions struct {
	// MaxTokens is the token budget of one chunk's text, clamped to the
	// embedding model's token cap.
	MaxTokens *int `json:"max_tokens,omitempty"`
	// OverlapTokens are repeated from the end of the previous chunk (at most
	// a quarter of the budget).
	OverlapTokens *int `json:"overlap_tokens,omitempty"`
	// Strategy is ChunkStrategyStructure or ChunkStrategyFixed.
	Strategy ChunkStrategy `json:"strategy,omitempty"`
}

// DocumentImportOptions are the fields both imports share: what every chunk
// memory carries (as StoreMemoryRequest), the chunking and Decompose. Zero
// values are omitted, so the server defaults apply.
type DocumentImportOptions struct {
	MemoryType string                 `json:"memory_type,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Importance *float32               `json:"importance,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	TTLSeconds *uint64                `json:"ttl_seconds,omitempty"`
	ExpiresAt  *uint64                `json:"expires_at,omitempty"`
	// Lang is the language of the document for the write-time derivations.
	Lang     string           `json:"lang,omitempty"`
	Chunking *ChunkingOptions `json:"chunking,omitempty"`
	// Decompose derives sentence sub-memories from every chunk (server
	// default false: a chunk is already a retrieval unit).
	Decompose bool `json:"decompose,omitempty"`
}

// ImportDocumentRequest is the body of POST /v1/memory/documents: a document
// given as text.
type ImportDocumentRequest struct {
	AgentID string `json:"agent_id"`
	// Content is the document itself (at most DAKERA_DOCUMENT_MAX_BYTES).
	Content string `json:"content"`
	// MediaType is MediaTypeTextPlain (default), MediaTypeMarkdown or
	// MediaTypeHTML.
	MediaType string `json:"media_type,omitempty"`
	// Title wins over the document's own (front matter, first heading,
	// <title>).
	Title string `json:"title,omitempty"`
	// DocumentID defaults to an id derived from the agent and the content
	// hash, so importing the same bytes again replaces the document.
	DocumentID string `json:"document_id,omitempty"`
	DocumentImportOptions
}

// ImportAttachmentDocumentRequest is the body of
// POST /v1/namespaces/{ns}/attachments/{ref}/document.
type ImportAttachmentDocumentRequest struct {
	AgentID    string `json:"agent_id"`
	Title      string `json:"title,omitempty"`
	DocumentID string `json:"document_id,omitempty"`
	DocumentImportOptions
}

// ImportDocumentAccepted is the 202 answer of both imports.
type ImportDocumentAccepted struct {
	JobID      string `json:"job_id"`
	DocumentID string `json:"document_id"`
	AgentID    string `json:"agent_id"`
	// StatusURL is the route to poll for the job.
	StatusURL string `json:"status_url"`
}

// DocumentChunkMeta is what "_dakera_document" holds on every chunk memory.
// The document-level fields are repeated on every chunk.
type DocumentChunkMeta struct {
	// ID is the document id.
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	MediaType string `json:"media_type"`
	// ChunkIndex is this chunk's position, from 0, of ChunkCount.
	ChunkIndex int `json:"chunk_index"`
	ChunkCount int `json:"chunk_count"`
	// PageStart / PageEnd are the pages the chunk's text comes from (PDF;
	// form-feed pages of plain text).
	PageStart *uint32 `json:"page_start,omitempty"`
	PageEnd   *uint32 `json:"page_end,omitempty"`
	// HeadingPath are the headings above the chunk, outermost first.
	HeadingPath []string `json:"heading_path"`
	// CharStart / CharEnd are the chunk's character range in the extracted text.
	CharStart int `json:"char_start"`
	CharEnd   int `json:"char_end"`
	// SHA256 (hex) and Bytes describe the source document.
	SHA256 string  `json:"sha256"`
	Bytes  int     `json:"bytes"`
	Pages  *uint32 `json:"pages,omitempty"`
	// CreatedAt / UpdatedAt: first and last import (Unix seconds).
	CreatedAt uint64 `json:"created_at"`
	UpdatedAt uint64 `json:"updated_at"`
	// Decompose: sentence sub-memories are derived from this chunk.
	Decompose   bool   `json:"decompose,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// DocumentChunkMetaOf reads "_dakera_document" from a memory's metadata. It
// returns nil when the memory is not a document chunk.
func DocumentChunkMetaOf(metadata map[string]interface{}) *DocumentChunkMeta {
	raw, ok := metadata[DocumentMetaKey]
	if !ok || raw == nil {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var meta DocumentChunkMeta
	if err := json.Unmarshal(encoded, &meta); err != nil || meta.ID == "" {
		return nil
	}
	return &meta
}

// DocumentChunk is the document provenance of a recalled memory (document,
// pages, heading path) when it is a document chunk, else nil.
func (r RecalledMemory) DocumentChunk() *DocumentChunkMeta {
	return DocumentChunkMetaOf(r.Metadata)
}

// DocumentChunk is the document provenance of a memory when it is a
// document chunk, else nil.
func (m Memory) DocumentChunk() *DocumentChunkMeta {
	return DocumentChunkMetaOf(m.Metadata)
}

// DocumentSummary is one document of ListDocuments.
type DocumentSummary struct {
	DocumentID    string  `json:"document_id"`
	Title         string  `json:"title,omitempty"`
	MediaType     string  `json:"media_type"`
	ChunkCount    int     `json:"chunk_count"`
	Bytes         int     `json:"bytes"`
	Pages         *uint32 `json:"pages,omitempty"`
	CreatedAt     uint64  `json:"created_at"`
	UpdatedAt     uint64  `json:"updated_at"`
	SHA256        string  `json:"sha256"`
	AttachmentRef string  `json:"attachment_ref,omitempty"`
}

// DocumentListResponse is the answer of GET /v1/memory/documents.
type DocumentListResponse struct {
	Documents []DocumentSummary `json:"documents"`
	// NextCursor is passed as ListDocumentsOptions.Cursor for the next page;
	// empty on the last one.
	NextCursor string `json:"next_cursor,omitempty"`
}

// ListDocumentsOptions pages ListDocuments.
type ListDocumentsOptions struct {
	// Cursor is the NextCursor of the previous page.
	Cursor string
	// Limit is the page size (1–500, server default 50; 0 = default).
	Limit int
}

// DocumentChunkInfo is one chunk of GetDocument.
type DocumentChunkInfo struct {
	// ID is the chunk's memory id.
	ID          string   `json:"id"`
	ChunkIndex  int      `json:"chunk_index"`
	PageStart   *uint32  `json:"page_start,omitempty"`
	PageEnd     *uint32  `json:"page_end,omitempty"`
	HeadingPath []string `json:"heading_path"`
	CharStart   int      `json:"char_start"`
	CharEnd     int      `json:"char_end"`
}

// DocumentDetail is the answer of GET /v1/memory/documents/{id}.
type DocumentDetail struct {
	Document DocumentSummary     `json:"document"`
	Chunks   []DocumentChunkInfo `json:"chunks"`
}

// DeleteDocumentResponse is the answer of DELETE /v1/memory/documents/{id}.
type DeleteDocumentResponse struct {
	DocumentID string `json:"document_id"`
	// DeletedChunks counts the chunk memories removed.
	DeletedChunks int `json:"deleted_chunks"`
}

// documentRouteError makes a 404 from a server without the document routes
// (older than v0.12.3) say so.
func documentRouteError(err error) error {
	var nf *NotFoundError
	if errors.As(err, &nf) && nf.Code == ErrorCodeRouteNotFound {
		nf.Message += " (document import needs Dakera server >= 0.12.3; see Capabilities().SupportsDocuments())"
	}
	return err
}

func decodeInto[T any](body []byte) (*T, error) {
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// ImportDocument imports a document given as text (text/plain, text/markdown
// or text/html) — POST /v1/memory/documents, Write scope on the agent. The
// server answers 202 with a job: follow it with GetDocumentJob or
// WaitForDocumentJob. A server with DAKERA_DOCUMENTS off answers 501
// (*FeatureDisabledError); one older than v0.12.3 a 404 (*NotFoundError).
func (c *Client) ImportDocument(ctx context.Context, req ImportDocumentRequest) (*ImportDocumentAccepted, error) {
	body, err := c.request(ctx, "POST", "/v1/memory/documents", req)
	if err != nil {
		return nil, documentRouteError(err)
	}
	return decodeInto[ImportDocumentAccepted](body)
}

// ImportAttachmentDocument imports an uploaded attachment (PDF, DOCX, HTML,
// Markdown or text, by its stored media type) as a document — POST
// /v1/namespaces/{namespace}/attachments/{ref}/document, Read on namespace and
// Write on the agent. Upload it first with UploadAttachment. Needs
// DAKERA_ATTACHMENTS too.
func (c *Client) ImportAttachmentDocument(ctx context.Context, namespace, ref string, req ImportAttachmentDocumentRequest) (*ImportDocumentAccepted, error) {
	body, err := c.request(ctx, "POST", attachmentPath(namespace, ref)+"/document", req)
	if err != nil {
		return nil, documentRouteError(err)
	}
	return decodeInto[ImportDocumentAccepted](body)
}

// GetDocumentJob reports a document import job — GET
// /v1/memory/documents/jobs/{jobID}?agent_id=. Status is Pending, Running
// (Progress, Message "chunks_stored n/total"), Completed or Failed (Error
// holds the status and code, e.g. 413, 422 UNPROCESSABLE_CONTENT, 504
// EXTRACTION_TIMEOUT). An unknown job is a *NotFoundError.
func (c *Client) GetDocumentJob(ctx context.Context, agentID, jobID string) (*JobInfo, error) {
	job, err := c.getJob(ctx, "/v1/memory/documents/jobs/"+url.PathEscape(jobID)+"?agent_id="+url.QueryEscape(agentID))
	if err != nil {
		return nil, documentRouteError(err)
	}
	return job, nil
}

// WaitForDocumentJob polls a document import job until it ends, bounded by
// ctx and opts.Timeout. A job that ends Failed or Cancelled returns its final
// JobInfo and a *JobFailedError.
func (c *Client) WaitForDocumentJob(ctx context.Context, agentID, jobID string, opts JobWaitOptions) (*JobInfo, error) {
	return c.waitJob(ctx, opts, func(ctx context.Context) (*JobInfo, error) {
		return c.GetDocumentJob(ctx, agentID, jobID)
	})
}

// ListDocuments lists an agent's documents, sorted by id — GET
// /v1/memory/documents?agent_id=&cursor=&limit=, Read scope. opts may be nil.
func (c *Client) ListDocuments(ctx context.Context, agentID string, opts *ListDocumentsOptions) (*DocumentListResponse, error) {
	params := url.Values{}
	params.Set("agent_id", agentID)
	if opts != nil {
		if opts.Cursor != "" {
			params.Set("cursor", opts.Cursor)
		}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	body, err := c.request(ctx, "GET", "/v1/memory/documents?"+params.Encode(), nil)
	if err != nil {
		return nil, documentRouteError(err)
	}
	return decodeInto[DocumentListResponse](body)
}

// GetDocument returns one document and its chunks — GET
// /v1/memory/documents/{documentID}?agent_id=, Read scope. An unknown
// document is a *NotFoundError (Resource "document").
func (c *Client) GetDocument(ctx context.Context, agentID, documentID string) (*DocumentDetail, error) {
	body, err := c.request(ctx, "GET", "/v1/memory/documents/"+url.PathEscape(documentID)+"?agent_id="+url.QueryEscape(agentID), nil)
	if err != nil {
		return nil, documentRouteError(err)
	}
	return decodeInto[DocumentDetail](body)
}

// DeleteDocument deletes every chunk of a document — DELETE
// /v1/memory/documents/{documentID}?agent_id=, Write scope. Its attachment
// goes when no memory references it any more.
func (c *Client) DeleteDocument(ctx context.Context, agentID, documentID string) (*DeleteDocumentResponse, error) {
	body, err := c.request(ctx, "DELETE", "/v1/memory/documents/"+url.PathEscape(documentID)+"?agent_id="+url.QueryEscape(agentID), nil)
	if err != nil {
		return nil, documentRouteError(err)
	}
	return decodeInto[DeleteDocumentResponse](body)
}
