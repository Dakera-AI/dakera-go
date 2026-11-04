package dakera

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Memory import and export (POST /v1/import, GET /v1/export).
//
// The server reads an import as a multipart upload (one file field) with
// agent_id and format in the query string, and answers an export with the
// file itself (JSON Lines, CSV, or Mem0 / Zep JSON).

// Import job states (ImportJobStatus.Status).
const (
	// ImportStatusRunning: an asynchronous import is still writing.
	ImportStatusRunning = "running"
	// ImportStatusDone: the import finished.
	ImportStatusDone = "done"
	// ImportStatusFailed: every item was skipped, or the server stopped the
	// import part-way (overloaded; import the file again: items already
	// imported are rewritten in place).
	ImportStatusFailed = "failed"
)

// ImportMemoriesOptions configures ImportMemoryFile.
type ImportMemoriesOptions struct {
	// Format is "jsonl", "csv", "mem0" or "zep". Empty lets the server
	// detect it from FileName, then from the content.
	Format string
	// FileName is the uploaded file's name (default "memories.<Format>", or
	// "memories" without a Format).
	FileName string
	// Async makes the server answer at once (202) with the job in state
	// ImportStatusRunning and import in the background, so a large import
	// never outlasts a proxy's timeout; follow it with ImportJobStatus or
	// WaitForImportJob (server v0.12.4+; an older server ignores it and
	// imports within the request).
	Async bool
}

// ImportMemoryFile imports memories into agentID's namespace from an export
// file — POST /v1/import?agent_id=&format=&async= (Write scope). It returns
// the import job: its state, how many items were imported and skipped, and
// the first errors. With opts.Async (server v0.12.4+) the job is still
// ImportStatusRunning when it returns. opts may be nil.
func (c *Client) ImportMemoryFile(ctx context.Context, agentID string, data []byte, opts *ImportMemoriesOptions) (*ImportJobStatus, error) {
	if agentID == "" {
		return nil, NewValidationError("ImportMemoryFile: agentID is required", 0, nil, ErrorCodeInvalidRequest)
	}
	var o ImportMemoriesOptions
	if opts != nil {
		o = *opts
	}
	params := url.Values{}
	params.Set("agent_id", agentID)
	if o.Format != "" {
		params.Set("format", o.Format)
	}
	if o.Async {
		params.Set("async", "true")
	}
	fileName := o.FileName
	if fileName == "" {
		fileName = "memories"
		if o.Format != "" {
			fileName += "." + o.Format
		}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, fileName))
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("failed to build import upload: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("failed to build import upload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to build import upload: %w", err)
	}

	resp, err := c.requestRaw(ctx, "POST", "/v1/import?"+params.Encode(), writer.FormDataContentType(), body.Bytes())
	if err != nil {
		return nil, err
	}
	var result ImportJobStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal import response: %w", err)
	}
	return &result, nil
}

// ImportMemories imports memories from an external format (DX-1) —
// POST /v1/import. format is "mem0", "zep", "jsonl" or "csv"; agentID is
// required by the server.
//
// data is uploaded as the file: a []byte or string as it is; for "jsonl" a
// slice is written one JSON value per line; anything else is encoded as one
// JSON document. namespace is not a server parameter and is ignored (the
// memories go to agentID's namespace).
//
// Deprecated: use ImportMemoryFile, which returns the whole import job and
// can import asynchronously.
func (c *Client) ImportMemories(ctx context.Context, data interface{}, format string, agentID string, namespace string) (*MemoryImportResponse, error) {
	_ = namespace
	file, err := importFileBytes(data, format)
	if err != nil {
		return nil, err
	}
	job, err := c.ImportMemoryFile(ctx, agentID, file, &ImportMemoriesOptions{Format: format})
	if err != nil {
		return nil, err
	}
	return &MemoryImportResponse{
		ImportedCount: job.Imported,
		SkippedCount:  job.Skipped,
		Errors:        job.Errors,
		Job:           job,
	}, nil
}

// importFileBytes renders ImportMemories' data as the uploaded file.
func importFileBytes(data interface{}, format string) ([]byte, error) {
	switch v := data.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	}
	if strings.EqualFold(format, "jsonl") {
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode import data: %w", err)
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil {
			var out bytes.Buffer
			for i, item := range items {
				if i > 0 {
					out.WriteByte('\n')
				}
				out.Write(item)
			}
			return out.Bytes(), nil
		}
		return raw, nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode import data: %w", err)
	}
	return raw, nil
}

// ImportJobStatus returns the status of an import job —
// GET /v1/import/{job_id}/status. Jobs are kept in the memory of the node that
// ran them.
func (c *Client) ImportJobStatus(ctx context.Context, jobID string) (*ImportJobStatus, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/v1/import/%s/status", url.PathEscape(jobID)), nil)
	if err != nil {
		return nil, err
	}
	var result ImportJobStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal import job status: %w", err)
	}
	return &result, nil
}

// WaitForImportJob polls an import job (ImportMemoryFile with Async) until it
// is no longer ImportStatusRunning and returns its final status: check
// Status for ImportStatusDone or ImportStatusFailed. opts.Timeout bounds the
// wait (a *TimeoutError with the last status read); the default poll
// interval is 1s.
func (c *Client) WaitForImportJob(ctx context.Context, jobID string, opts JobWaitOptions) (*ImportJobStatus, error) {
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
		job, err := c.ImportJobStatus(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if job.Status != ImportStatusRunning {
			return job, nil
		}
		if serr := sleepCtx(ctx, interval); serr != nil {
			return job, NewTimeoutError(fmt.Sprintf("import job %s still running (%d of %d imported): %v", jobID, job.Imported, job.Total, serr))
		}
	}
}

// ExportMemoryFile exports agentID's memories as a file — GET
// /v1/export?agent_id=&format= (Read scope). format is "jsonl" (the server
// default when empty), "csv", "mem0" or "zep". The result holds the file's
// bytes, its media type and how many memories the server skipped because their
// stored content cannot be opened.
func (c *Client) ExportMemoryFile(ctx context.Context, agentID, format string) (*MemoryExportFile, error) {
	params := url.Values{}
	params.Set("agent_id", agentID)
	if format != "" {
		params.Set("format", format)
	}
	resp, err := c.send(ctx, "GET", "/v1/export?"+params.Encode(), "application/json", nil, true)
	if err != nil {
		return nil, err
	}
	out := &MemoryExportFile{
		Data:        resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
		Format:      format,
	}
	if out.Format == "" {
		out.Format = "jsonl"
	}
	if skipped, err := strconv.Atoi(resp.Header.Get("X-Dakera-Export-Skipped")); err == nil {
		out.Skipped = skipped
	}
	return out, nil
}

// ExportMemories exports memories in a portable format (DX-1) — GET
// /v1/export. format is "mem0", "zep", "jsonl" or "csv"; agentID is required
// by the server. Data holds the records of a "jsonl" export (one per line),
// the "memories" of a "mem0" export and the "messages" of a "zep" export;
// Raw always holds the file as the server sent it (the only form of a "csv"
// export). namespace and limit are not server parameters and are ignored.
//
// Deprecated: use ExportMemoryFile.
func (c *Client) ExportMemories(ctx context.Context, format string, agentID string, namespace string, limit int) (*MemoryExportResponse, error) {
	_, _ = namespace, limit
	file, err := c.ExportMemoryFile(ctx, agentID, format)
	if err != nil {
		return nil, err
	}
	result := &MemoryExportResponse{Format: file.Format, Raw: file.Data, Skipped: file.Skipped}
	switch strings.ToLower(file.Format) {
	case "jsonl":
		scanner := bufio.NewScanner(bytes.NewReader(file.Data))
		scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var record map[string]interface{}
			if err := json.Unmarshal(line, &record); err != nil {
				return nil, fmt.Errorf("failed to parse exported memory: %w", err)
			}
			result.Data = append(result.Data, record)
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read export: %w", err)
		}
	case "mem0", "zep":
		var doc map[string][]map[string]interface{}
		if err := json.Unmarshal(file.Data, &doc); err != nil {
			return nil, fmt.Errorf("failed to parse export: %w", err)
		}
		if records, ok := doc["memories"]; ok {
			result.Data = records
		} else {
			result.Data = doc["messages"]
		}
	}
	result.Count = len(result.Data)
	return result, nil
}
