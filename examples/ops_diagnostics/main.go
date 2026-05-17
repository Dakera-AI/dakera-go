// Example: Dakera Go SDK — Ops & Diagnostics (diagnostics, jobs, compaction, cache)
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	dakera "github.com/dakera-ai/dakera-go"
)

func dakeraURL() string {
	if u := os.Getenv("DAKERA_API_URL"); u != "" {
		return u
	}
	return "http://localhost:3300"
}

func dakeraAPIKey() string {
	if k := os.Getenv("DAKERA_API_KEY"); k != "" {
		return k
	}
	return "dk-mykey"
}

func main() {
	client := dakera.NewClientWithOptions(dakera.ClientOptions{
		BaseURL: dakeraURL(),
		APIKey:  dakeraAPIKey(),
	})
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// Server Health (prerequisite check)
	// -------------------------------------------------------------------------
	fmt.Println("--- Health Check ---")

	health, err := client.Health(ctx)
	if err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Printf("Server: %s (version: %s)\n", health.Status, health.Version)

	// -------------------------------------------------------------------------
	// Ops Diagnostics
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Ops Diagnostics ---")

	diag, err := client.OpsDiagnostics(ctx)
	if err != nil {
		log.Fatalf("OpsDiagnostics failed: %v", err)
	}
	if diag == nil {
		log.Fatalf("unexpected: diagnostics response is nil")
	}
	fmt.Printf("Diagnostics keys: ")
	for k := range diag {
		fmt.Printf("%s ", k)
	}
	fmt.Println()

	// -------------------------------------------------------------------------
	// Ops Stats
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Ops Stats ---")

	opsStats, err := client.OpsStats(ctx)
	if err != nil {
		log.Fatalf("OpsStats failed: %v", err)
	}
	if opsStats == nil {
		log.Fatalf("unexpected: ops stats is nil")
	}
	fmt.Printf("Ops stats: uptime=%ds, namespaces=%d, total_vectors=%d\n",
		opsStats.UptimeSeconds, opsStats.NamespaceCount, opsStats.TotalVectors)

	// -------------------------------------------------------------------------
	// Ops Metrics (Prometheus format)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Ops Metrics (Prometheus) ---")

	metrics, err := client.OpsMetrics(ctx)
	if err != nil {
		log.Fatalf("OpsMetrics failed: %v", err)
	}
	if metrics == "" {
		log.Fatalf("unexpected: metrics response is empty")
	}
	// Print first 200 chars to show format
	preview := metrics
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	fmt.Printf("%s\n", preview)

	// -------------------------------------------------------------------------
	// List Jobs
	// -------------------------------------------------------------------------
	fmt.Println("\n--- List Jobs ---")

	jobs, err := client.OpsListJobs(ctx)
	if err != nil {
		log.Fatalf("OpsListJobs failed: %v", err)
	}
	fmt.Printf("Active jobs: %d\n", len(jobs))
	for _, j := range jobs {
		fmt.Printf("  [%s] %s — status=%s, progress=%d%%\n",
			j.ID, j.JobType, j.Status, j.Progress)
	}

	// -------------------------------------------------------------------------
	// Compaction
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Compaction ---")

	compactResp, err := client.OpsCompact(ctx, dakera.CompactionRequest{
		Force: false,
	})
	if err != nil {
		log.Fatalf("OpsCompact failed: %v", err)
	}
	if compactResp.JobID == "" {
		log.Fatalf("unexpected: compaction job ID is empty")
	}
	fmt.Printf("Compaction job started: %s (%s)\n", compactResp.JobID, compactResp.Message)

	// Check job status
	jobInfo, err := client.OpsGetJob(ctx, compactResp.JobID)
	if err != nil {
		log.Fatalf("OpsGetJob failed: %v", err)
	}
	fmt.Printf("Job %s: status=%s, progress=%d%%\n", jobInfo.ID, jobInfo.Status, jobInfo.Progress)

	// -------------------------------------------------------------------------
	// Cache Stats
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Cache Stats ---")

	cacheStats, err := client.CacheStats(ctx)
	if err != nil {
		log.Fatalf("CacheStats failed: %v", err)
	}
	if cacheStats == nil {
		log.Fatalf("unexpected: cache stats is nil")
	}
	fmt.Printf("Cache entries: %d, Hit rate: %.2f%%, Memory: %d bytes\n",
		cacheStats.TotalEntries, cacheStats.HitRate*100, cacheStats.MemoryBytes)

	// -------------------------------------------------------------------------
	// Slow Queries
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Slow Queries ---")

	slowQueries, err := client.SlowQueries(ctx, &dakera.SlowQueryOptions{
		Limit: 5,
	})
	if err != nil {
		log.Fatalf("SlowQueries failed: %v", err)
	}
	fmt.Printf("Slow queries: %d\n", len(slowQueries))
	for _, sq := range slowQueries {
		fmt.Printf("  [%.2fms] %s (ns: %s)\n", sq.DurationMs, sq.Query, sq.Namespace)
	}

	fmt.Println("\nDone!")
}
