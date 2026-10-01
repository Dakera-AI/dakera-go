package dakera

// Attachments, transcription / image-index jobs and records (server v0.12+).
//
// Everything here is opt-in on the server: a server started without
// DAKERA_ATTACHMENTS / DAKERA_VISION / DAKERA_RECORDS answers these routes with
// 501 FEATURE_DISABLED, which the client returns as a *FeatureDisabledError
// (Details names the variable). ServerCapabilities.Attachments, .Vision and
// .Records say what the connected server has on.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// ===========================================================================
// Attachments
// ===========================================================================

// AgentMemoryNamespace returns the namespace an agent's memories live in
// ("_dakera_agent_{agentID}"). A memory can only reference an attachment
// uploaded to its own namespace.
func AgentMemoryNamespace(agentID string) string {
	return "_dakera_agent_" + agentID
}

// AttachmentUploadResponse is returned by UploadAttachment.
type AttachmentUploadResponse struct {
	// AttachmentRef is "sha256:<hex of the bytes>" — what a memory's
	// attachment_ref carries and what the other attachment routes take.
	AttachmentRef string `json:"attachment_ref"`
	ContentType   string `json:"content_type"`
	SizeBytes     uint64 `json:"size_bytes"`
	// Created is false when the namespace already held these exact bytes (the
	// upload was a no-op and the existing media type was kept).
	Created bool `json:"created"`
}

// AttachmentEntry is one attachment in a listing (no bytes).
type AttachmentEntry struct {
	AttachmentRef string `json:"attachment_ref"`
	ContentType   string `json:"content_type"`
	SizeBytes     uint64 `json:"size_bytes"`
}

// AttachmentList is returned by ListAttachments.
type AttachmentList struct {
	Attachments []AttachmentEntry `json:"attachments"`
}

// AttachmentContent is a downloaded attachment.
type AttachmentContent struct {
	Data []byte
	// ContentType is the media type the attachment was uploaded with.
	ContentType string
	// ETag is the quoted content hash.
	ETag string
}

// TranscribeRequest is the body of TranscribeAttachment: the memory the
// transcript becomes, minus its content (that is the transcript). Same fields
// and defaults as StoreMemoryRequest.
type TranscribeRequest struct {
	// AgentID is the agent whose memory the transcript is stored as (required).
	AgentID    string                 `json:"agent_id"`
	MemoryType string                 `json:"memory_type,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Importance *float32               `json:"importance,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	TTLSeconds *int64                 `json:"ttl_seconds,omitempty"`
	ExpiresAt  *int64                 `json:"expires_at,omitempty"`
	// ID is an optional custom memory id.
	ID string `json:"id,omitempty"`
	// Lang is the language of the transcript for write-time derivations.
	Lang string `json:"lang,omitempty"`
}

// IndexImageRequest is the body of IndexAttachmentImage: the memory the image
// becomes. Same fields as TranscribeRequest plus Content.
type IndexImageRequest struct {
	// AgentID is the agent whose memory the page is stored as (required).
	AgentID string `json:"agent_id"`
	// Content is the caption stored as the memory's text (default "[image sha256:...]").
	// The vectors come from the pixels, not from this text.
	Content    string                 `json:"content,omitempty"`
	MemoryType string                 `json:"memory_type,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Importance *float32               `json:"importance,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	TTLSeconds *int64                 `json:"ttl_seconds,omitempty"`
	ExpiresAt  *int64                 `json:"expires_at,omitempty"`
	ID         string                 `json:"id,omitempty"`
	Lang       string                 `json:"lang,omitempty"`
}

// AttachmentJobAccepted is the 202 answer of TranscribeAttachment and
// IndexAttachmentImage.
type AttachmentJobAccepted struct {
	JobID         string `json:"job_id"`
	AttachmentRef string `json:"attachment_ref"`
	AgentID       string `json:"agent_id"`
	// MemoryID is the id of the memory the job stores. Jobs live in server
	// memory: after a restart the job id is unknown (404 JOB_NOT_FOUND) and
	// this is how to find what a completed job stored.
	MemoryID string `json:"memory_id"`
	// Model is the wire name of the model the job runs.
	Model string `json:"model"`
	// StatusURL is the route that reports the job (GET, same key scope as the namespace).
	StatusURL string `json:"status_url"`
}

// Job statuses reported in JobInfo.Status.
const (
	JobStatusPending   = "Pending"
	JobStatusRunning   = "Running"
	JobStatusCompleted = "Completed"
	JobStatusFailed    = "Failed"
	JobStatusCancelled = "Cancelled"
)

// IsDone reports whether the job reached a terminal state.
func (j *JobInfo) IsDone() bool {
	return j.Status == JobStatusCompleted || j.Status == JobStatusFailed || j.Status == JobStatusCancelled
}

// JobFailedError is returned by the Wait* helpers when a job ends Failed or
// Cancelled. Job carries the final status, message and error code.
type JobFailedError struct {
	Job *JobInfo
}

func (e *JobFailedError) Error() string {
	msg := fmt.Sprintf("job %s %s", e.Job.ID, e.Job.Status)
	if e.Job.Message != "" {
		msg += ": " + e.Job.Message
	}
	if e.Job.Error != nil {
		msg += fmt.Sprintf(" (status %d, code %s)", e.Job.Error.Status, e.Job.Error.Code)
	}
	return msg
}

func attachmentPath(namespace, ref string) string {
	return fmt.Sprintf("/v1/namespaces/%s/attachments/%s", url.PathEscape(namespace), url.PathEscape(ref))
}

// UploadAttachment stores a file in a namespace — POST
// /v1/namespaces/{namespace}/attachments, Write scope — and returns its content
// hash reference. The bytes are sent as the raw request body with contentType
// as its media type ("application/octet-stream" when empty). Uploading the same
// bytes again returns the existing reference with Created=false. An upload over
// DAKERA_ATTACHMENT_MAX_BYTES (25 MiB by default) is a *PayloadTooLargeError.
// To attach it to a memory, upload to AgentMemoryNamespace(agentID) and set
// StoreMemoryRequest.AttachmentRef.
func (c *Client) UploadAttachment(ctx context.Context, namespace string, data []byte, contentType string) (*AttachmentUploadResponse, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if data == nil {
		data = []byte{}
	}
	resp, err := c.requestRaw(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/attachments", url.PathEscape(namespace)), contentType, data)
	if err != nil {
		return nil, err
	}
	var out AttachmentUploadResponse
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// ListAttachments lists a namespace's attachments (no bytes) — GET
// /v1/namespaces/{namespace}/attachments, Read scope.
func (c *Client) ListAttachments(ctx context.Context, namespace string) ([]AttachmentEntry, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/attachments", url.PathEscape(namespace)), nil)
	if err != nil {
		return nil, err
	}
	var out AttachmentList
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return out.Attachments, nil
}

// DownloadAttachment returns an attachment's bytes and media type — GET
// /v1/namespaces/{namespace}/attachments/{ref}, Read scope.
func (c *Client) DownloadAttachment(ctx context.Context, namespace, ref string) (*AttachmentContent, error) {
	resp, err := c.send(ctx, "GET", attachmentPath(namespace, ref), "application/json", nil, true)
	if err != nil {
		return nil, err
	}
	return &AttachmentContent{
		Data:        resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
		ETag:        resp.Header.Get("ETag"),
	}, nil
}

// DeleteAttachment removes an attachment — DELETE
// /v1/namespaces/{namespace}/attachments/{ref}, Write scope. While a memory
// still references it the server answers 409 (a *ConflictError): forget the
// memory instead, which removes the attachment with its last reference.
func (c *Client) DeleteAttachment(ctx context.Context, namespace, ref string) error {
	_, err := c.request(ctx, "DELETE", attachmentPath(namespace, ref), nil)
	return err
}

func decodeJobAccepted(body []byte) (*AttachmentJobAccepted, error) {
	var out AttachmentJobAccepted
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// TranscribeAttachment starts a speech-to-text job for an audio attachment —
// POST /v1/namespaces/{namespace}/attachments/{ref}/transcribe. It needs Read
// on namespace and Write on the agent's memory namespace, and a server with
// DAKERA_ATTACHMENTS. The audio must be WAV (anything else is a 400 before a
// job exists). The job stores the transcript as a memory for req.AgentID;
// follow it with GetTranscriptionJob or WaitForTranscription.
func (c *Client) TranscribeAttachment(ctx context.Context, namespace, ref string, req TranscribeRequest) (*AttachmentJobAccepted, error) {
	body, err := c.request(ctx, "POST", attachmentPath(namespace, ref)+"/transcribe", req)
	if err != nil {
		return nil, err
	}
	return decodeJobAccepted(body)
}

// GetTranscriptionJob reports a transcription job — GET
// /v1/namespaces/{namespace}/attachments/{ref}/transcribe/{jobID}. An unknown
// job (including one lost to a server restart) is a *NotFoundError with code
// JOB_NOT_FOUND.
func (c *Client) GetTranscriptionJob(ctx context.Context, namespace, ref, jobID string) (*JobInfo, error) {
	return c.getJob(ctx, attachmentPath(namespace, ref)+"/transcribe/"+url.PathEscape(jobID))
}

// IndexAttachmentImage starts an image-indexing job (a PNG attachment becomes a
// patch-multivector memory) — POST
// /v1/namespaces/{namespace}/attachments/{ref}/index. It needs a server with
// both DAKERA_ATTACHMENTS and DAKERA_VISION; follow it with GetImageIndexJob or
// WaitForImageIndex.
func (c *Client) IndexAttachmentImage(ctx context.Context, namespace, ref string, req IndexImageRequest) (*AttachmentJobAccepted, error) {
	body, err := c.request(ctx, "POST", attachmentPath(namespace, ref)+"/index", req)
	if err != nil {
		return nil, err
	}
	return decodeJobAccepted(body)
}

// GetImageIndexJob reports an image-indexing job — GET
// /v1/namespaces/{namespace}/attachments/{ref}/index/{jobID}.
func (c *Client) GetImageIndexJob(ctx context.Context, namespace, ref, jobID string) (*JobInfo, error) {
	return c.getJob(ctx, attachmentPath(namespace, ref)+"/index/"+url.PathEscape(jobID))
}

func (c *Client) getJob(ctx context.Context, path string) (*JobInfo, error) {
	body, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out JobInfo
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// JobWaitOptions configures the Wait* helpers.
type JobWaitOptions struct {
	// Timeout bounds the wait; zero waits until ctx is done.
	Timeout time.Duration
	// PollInterval is the delay between polls (default 1s).
	PollInterval time.Duration
}

func (c *Client) waitJob(ctx context.Context, opts JobWaitOptions, get func(context.Context) (*JobInfo, error)) (*JobInfo, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	interval := opts.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	for {
		job, err := get(ctx)
		if err != nil {
			return nil, err
		}
		if job.IsDone() {
			if job.Status != JobStatusCompleted {
				return job, &JobFailedError{Job: job}
			}
			return job, nil
		}
		if serr := sleepCtx(ctx, interval); serr != nil {
			return job, NewTimeoutError(fmt.Sprintf("job %s still %s (%d%%): %v", job.ID, job.Status, job.Progress, serr))
		}
	}
}

// WaitForTranscription polls a transcription job until it completes. A job that
// ends Failed or Cancelled returns its final JobInfo and a *JobFailedError.
func (c *Client) WaitForTranscription(ctx context.Context, namespace, ref, jobID string, opts JobWaitOptions) (*JobInfo, error) {
	return c.waitJob(ctx, opts, func(ctx context.Context) (*JobInfo, error) {
		return c.GetTranscriptionJob(ctx, namespace, ref, jobID)
	})
}

// WaitForImageIndex polls an image-indexing job until it completes (see
// WaitForTranscription).
func (c *Client) WaitForImageIndex(ctx context.Context, namespace, ref, jobID string, opts JobWaitOptions) (*JobInfo, error) {
	return c.waitJob(ctx, opts, func(ctx context.Context) (*JobInfo, error) {
		return c.GetImageIndexJob(ctx, namespace, ref, jobID)
	})
}

// IsJobFailedError reports whether err is (or wraps) a *JobFailedError.
func IsJobFailedError(err error) bool {
	var target *JobFailedError
	return errors.As(err, &target)
}

// ===========================================================================
// Records (one vector plus named representations)
// ===========================================================================

// RepresentationInput is one extra representation of a record on the write
// path: Vectors is a block of rows, all of the same length.
type RepresentationInput struct {
	// Name is the slot name, unique within the record and never "dense"
	// (reserved for the primary vector).
	Name string `json:"name"`
	// Kind is dense, token_multivector or patch_multivector.
	Kind RepresentationKind `json:"kind"`
	// Model is the wire name of the model that produced the vectors; empty
	// means the namespace's default model.
	Model string `json:"model,omitempty"`
	// Vectors are the rows (plain float arrays on the wire).
	Vectors [][]float32 `json:"vectors"`
	// StoreAs is how the server packs the block on disk: f32 (lossless,
	// default), f16 or i8. Empty uses the default.
	StoreAs BlockDType `json:"store_as,omitempty"`
}

// RecordInput is one record on the write path: the primary vector (the one
// that is indexed and searched) plus named extra representations.
type RecordInput struct {
	ID              string                 `json:"id"`
	Values          []float32              `json:"values"`
	Representations []RepresentationInput  `json:"representations,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	TTLSeconds      *int64                 `json:"ttl_seconds,omitempty"`
}

// RecordUpsertResponse is returned by UpsertRecords.
type RecordUpsertResponse struct {
	UpsertedCount int `json:"upserted_count"`
}

// RepresentationInfo describes one extra representation of a stored record.
type RepresentationInfo struct {
	Name  string             `json:"name"`
	Kind  RepresentationKind `json:"kind"`
	Model string             `json:"model,omitempty"`
	Dim   uint32             `json:"dim"`
	Count uint32             `json:"count"`
	DType BlockDType         `json:"dtype"`
	// Bytes is the packed size on disk.
	Bytes int `json:"bytes"`
	// Vectors holds the decoded rows; only present with includeVectors.
	Vectors [][]float32 `json:"vectors,omitempty"`
}

// RecordView is returned by GetRecord.
type RecordView struct {
	ID string `json:"id"`
	// Values is the primary vector; only present with includeVectors.
	Values []float32 `json:"values,omitempty"`
	// Dimension of the primary vector (always reported).
	Dimension       int                  `json:"dimension"`
	Representations []RepresentationInfo `json:"representations,omitempty"`
	// UnsupportedRepresentations counts slots this server could not read (a
	// kind or dtype written by a newer Dakera); they are skipped, not lost.
	UnsupportedRepresentations int                    `json:"unsupported_representations,omitempty"`
	Metadata                   map[string]interface{} `json:"metadata,omitempty"`
	TTLSeconds                 *int64                 `json:"ttl_seconds,omitempty"`
	ExpiresAt                  *int64                 `json:"expires_at,omitempty"`
}

// UpsertRecords writes records — POST /v1/namespaces/{namespace}/records, Write
// scope, server v0.12+ with DAKERA_RECORDS (otherwise *FeatureDisabledError). A
// record over the server's limits (representations, vectors, bytes) is a
// *PayloadTooLargeError; a malformed one a *ValidationError. A record without
// representations is stored exactly like a plain vector. There is no record
// delete route: delete the id with Delete.
func (c *Client) UpsertRecords(ctx context.Context, namespace string, records []RecordInput) (*RecordUpsertResponse, error) {
	body := map[string]interface{}{"records": records}
	resp, err := c.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/records", url.PathEscape(namespace)), body)
	if err != nil {
		return nil, err
	}
	var out RecordUpsertResponse
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// GetRecord reads one record — GET /v1/namespaces/{namespace}/records/{id},
// Read scope. It returns a manifest of the representations (name, kind, model,
// shape, dtype, bytes); the vectors themselves only come back with
// includeVectors, because a token block is 20-100x the dense vector.
func (c *Client) GetRecord(ctx context.Context, namespace, id string, includeVectors bool) (*RecordView, error) {
	path := fmt.Sprintf("/v1/namespaces/%s/records/%s", url.PathEscape(namespace), url.PathEscape(id))
	if includeVectors {
		path += "?include_vectors=true"
	}
	resp, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out RecordView
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}
