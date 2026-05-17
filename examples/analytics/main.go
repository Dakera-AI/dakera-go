// Example: Dakera Go SDK — Analytics (agent stats, KPIs, overview, latency, throughput, storage, sessions)
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

	agentID := "analytics-demo-agent"

	// -------------------------------------------------------------------------
	// Store a memory & create a session for analytics data
	// -------------------------------------------------------------------------
	fmt.Println("--- Setup: Store Memory & Session ---")

	importance := float32(0.8)
	_, err := client.StoreMemory(ctx, agentID, dakera.StoreMemoryRequest{
		Content:    "The quarterly report shows 30% growth in API usage.",
		MemoryType: "semantic",
		Importance: &importance,
	})
	if err != nil {
		log.Fatalf("StoreMemory failed: %v", err)
	}

	session, err := client.StartSession(ctx, dakera.StartSessionRequest{
		AgentID: agentID,
		Metadata: map[string]interface{}{
			"purpose": "analytics-demo",
		},
	})
	if err != nil {
		log.Fatalf("StartSession failed: %v", err)
	}
	fmt.Printf("Session started: %s\n", session.ID)

	_, err = client.EndSession(ctx, session.ID)
	if err != nil {
		log.Fatalf("EndSession failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// Agent Stats
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Agent Stats ---")

	stats, err := client.AgentStats(ctx, agentID)
	if err != nil {
		log.Fatalf("AgentStats failed: %v", err)
	}
	if stats == nil {
		log.Fatalf("unexpected: agent stats is nil")
	}
	fmt.Printf("Agent: %s\n", stats.AgentID)
	fmt.Printf("  Total memories: %d\n", stats.TotalMemories)
	fmt.Printf("  Total sessions: %d\n", stats.TotalSessions)

	if stats.TotalMemories == 0 {
		log.Fatalf("unexpected: agent should have at least 1 memory")
	}

	// -------------------------------------------------------------------------
	// KPIs
	// -------------------------------------------------------------------------
	fmt.Println("\n--- KPIs ---")

	kpis, err := client.GetKpis(ctx)
	if err != nil {
		log.Fatalf("GetKpis failed: %v", err)
	}
	if kpis == nil {
		log.Fatalf("unexpected: KPI snapshot is nil")
	}
	fmt.Printf("Recall P50: %.2fms, P99: %.2fms\n", kpis.RecallLatencyP50Ms, kpis.RecallLatencyP99Ms)
	fmt.Printf("Store P50: %.2fms\n", kpis.StoreLatencyP50Ms)
	fmt.Printf("5xx error rate: %.4f%%\n", kpis.ApiErrorRate5xxPct)
	fmt.Printf("Active agents (24h): %d\n", kpis.ActiveAgentsCount)

	// -------------------------------------------------------------------------
	// Analytics Overview
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Analytics Overview ---")

	overview, err := client.AnalyticsOverview(ctx, &dakera.AnalyticsOptions{
		Period: "1h",
	})
	if err != nil {
		log.Fatalf("AnalyticsOverview failed: %v", err)
	}
	if overview == nil {
		log.Fatalf("unexpected: analytics overview is nil")
	}
	fmt.Printf("Total queries: %d\n", overview.TotalQueries)
	fmt.Printf("Avg latency: %.2fms (P95: %.2fms, P99: %.2fms)\n",
		overview.AvgLatencyMs, overview.P95LatencyMs, overview.P99LatencyMs)
	fmt.Printf("QPS: %.2f, Error rate: %.4f%%\n", overview.QueriesPerSecond, overview.ErrorRate*100)
	fmt.Printf("Cache hit rate: %.1f%%\n", overview.CacheHitRate*100)
	fmt.Printf("Storage: %d bytes, Vectors: %d, Namespaces: %d\n",
		overview.StorageUsedBytes, overview.TotalVectors, overview.TotalNamespaces)

	// -------------------------------------------------------------------------
	// Analytics Latency
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Analytics Latency ---")

	latency, err := client.AnalyticsLatency(ctx, &dakera.AnalyticsOptions{
		Period: "1h",
	})
	if err != nil {
		log.Fatalf("AnalyticsLatency failed: %v", err)
	}
	if latency == nil {
		log.Fatalf("unexpected: latency analytics is nil")
	}
	fmt.Printf("Period: %s\n", latency.Period)
	fmt.Printf("Avg: %.2fms, P50: %.2fms, P95: %.2fms, P99: %.2fms, Max: %.2fms\n",
		latency.AvgMs, latency.P50Ms, latency.P95Ms, latency.P99Ms, latency.MaxMs)
	if latency.ByOperation != nil {
		for op, opLat := range latency.ByOperation {
			fmt.Printf("  %s: avg=%.2fms, p95=%.2fms, count=%d\n",
				op, opLat.AvgMs, opLat.P95Ms, opLat.Count)
		}
	}

	// -------------------------------------------------------------------------
	// Analytics Throughput
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Analytics Throughput ---")

	throughput, err := client.AnalyticsThroughput(ctx, &dakera.AnalyticsOptions{
		Period: "1h",
	})
	if err != nil {
		log.Fatalf("AnalyticsThroughput failed: %v", err)
	}
	if throughput == nil {
		log.Fatalf("unexpected: throughput analytics is nil")
	}
	fmt.Printf("Period: %s\n", throughput.Period)
	fmt.Printf("Total ops: %d, OPS: %.2f\n", throughput.TotalOperations, throughput.OperationsPerSecond)
	if throughput.ByOperation != nil {
		for op, count := range throughput.ByOperation {
			fmt.Printf("  %s: %d\n", op, count)
		}
	}

	// -------------------------------------------------------------------------
	// Analytics Storage
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Analytics Storage ---")

	storage, err := client.AnalyticsStorage(ctx, "")
	if err != nil {
		log.Fatalf("AnalyticsStorage failed: %v", err)
	}
	if storage == nil {
		log.Fatalf("unexpected: storage analytics is nil")
	}
	fmt.Printf("Total: %d bytes (index: %d, data: %d)\n",
		storage.TotalBytes, storage.IndexBytes, storage.DataBytes)
	if storage.ByNamespace != nil {
		for ns, nsStorage := range storage.ByNamespace {
			fmt.Printf("  %s: %d bytes, %d vectors\n", ns, nsStorage.Bytes, nsStorage.VectorCount)
		}
	}

	// -------------------------------------------------------------------------
	// Sessions List
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Sessions ---")

	limit := 5
	sessions, err := client.ListSessions(ctx, &dakera.ListSessionsOptions{
		AgentID: agentID,
		Limit:   &limit,
	})
	if err != nil {
		log.Fatalf("ListSessions failed: %v", err)
	}
	fmt.Printf("Sessions for %s: %d\n", agentID, len(sessions))
	for _, s := range sessions {
		ended := "active"
		if s.EndedAt != nil {
			ended = "ended"
		}
		fmt.Printf("  %s (agent: %s, %s, memories: %d)\n", s.ID, s.AgentID, ended, s.MemoryCount)
	}

	fmt.Println("\nDone!")
}
