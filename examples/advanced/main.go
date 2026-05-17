// Example: Dakera Go SDK — Text Search, Hybrid Search & Admin Operations
package main

import (
	"context"
	"fmt"
	"log"

	dakera "github.com/dakera-ai/dakera-go"
)

func main() {
	client := dakera.NewClient("http://localhost:3000")
	ctx := context.Background()

	namespace := "docs-namespace"

	// -------------------------------------------------------------------------
	// Create namespace
	// -------------------------------------------------------------------------
	fmt.Println("--- Setup ---")
	_, err := client.CreateNamespace(ctx, namespace, &dakera.CreateNamespaceOptions{
		Dimensions: 1024,
	})
	if err != nil && !dakera.IsValidationError(err) {
		log.Fatalf("Failed to create namespace: %v", err)
	}

	// -------------------------------------------------------------------------
	// Text upsert (auto-embedding)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Text Upsert (auto-embed) ---")

	textResp, err := client.UpsertText(ctx, namespace, []dakera.TextDocument{
		{
			ID:   "doc-1",
			Text: "Go is a statically typed, compiled programming language designed for simplicity.",
			Metadata: map[string]interface{}{
				"category": "programming",
				"lang":     "go",
			},
		},
		{
			ID:   "doc-2",
			Text: "Python is great for data science and machine learning workloads.",
			Metadata: map[string]interface{}{
				"category": "programming",
				"lang":     "python",
			},
		},
		{
			ID:   "doc-3",
			Text: "Rust provides memory safety without garbage collection via ownership.",
			Metadata: map[string]interface{}{
				"category": "programming",
				"lang":     "rust",
			},
		},
	}, &dakera.TextUpsertOptions{
		Model: dakera.EmbeddingModelMiniLM,
	})
	if err != nil {
		log.Fatalf("Failed to upsert text: %v", err)
	}
	fmt.Printf("Upserted %d documents (model: %s, embed time: %dms)\n",
		textResp.UpsertedCount, textResp.Model, textResp.EmbeddingTimeMs)

	// -------------------------------------------------------------------------
	// Text query (semantic search)
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Text Query ---")

	textResults, err := client.QueryText(ctx, namespace, "which language is good for systems programming?", &dakera.TextQueryOptions{
		TopK:        3,
		IncludeText: true,
	})
	if err != nil {
		log.Fatalf("Failed to query text: %v", err)
	}

	for _, r := range textResults.Results {
		fmt.Printf("  [%.4f] %s: %s\n", r.Score, r.ID, r.Text)
	}

	// -------------------------------------------------------------------------
	// Index documents for full-text search
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Index Documents for Full-Text Search ---")

	_, err = client.IndexDocuments(ctx, namespace, []dakera.DocumentInput{
		{ID: "doc-1", Content: "Go is a statically typed compiled language"},
		{ID: "doc-2", Content: "Python is great for data science and ML"},
		{ID: "doc-3", Content: "Rust provides memory safety without GC"},
	})
	if err != nil {
		log.Fatalf("Failed to index documents: %v", err)
	}
	fmt.Println("Documents indexed")

	// -------------------------------------------------------------------------
	// Full-text search
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Full-Text Search ---")

	ftResults, err := client.FulltextSearch(ctx, namespace, "memory safety", &dakera.FullTextSearchOptions{
		TopK: 3,
	})
	if err != nil {
		log.Fatalf("Failed to full-text search: %v", err)
	}

	for _, r := range ftResults {
		fmt.Printf("  [%.4f] %s: %s\n", r.Score, r.ID, r.Content)
	}

	// -------------------------------------------------------------------------
	// Hybrid search
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Hybrid Search ---")

	queryVec := make([]float32, 1024)
	hybridResults, err := client.HybridSearch(ctx, namespace, queryVec, "compiled language", &dakera.HybridSearchOptions{
		TopK:         3,
		VectorWeight: 0.5,
	})
	if err != nil {
		log.Fatalf("Failed to hybrid search: %v", err)
	}

	for _, r := range hybridResults {
		fmt.Printf("  [%.4f] %s (vec=%.4f, text=%.4f)\n",
			r.Score, r.ID, r.VectorScore, r.TextScore)
	}

	// -------------------------------------------------------------------------
	// Cache warming
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Cache Warming ---")

	warmResp, err := client.WarmCache(ctx, namespace, dakera.WarmCacheRequest{
		VectorIDs:  []string{"doc-1", "doc-2", "doc-3"},
		Priority:   "high",
		TargetTier: "l2",
		Background: false,
	})
	if err != nil {
		log.Printf("Cache warm (may not be supported): %v", err)
	} else {
		fmt.Printf("Warmed %d entries in cache (status: %s)\n", warmResp.EntriesWarmed, warmResp.Status)
	}

	// -------------------------------------------------------------------------
	// Admin: index stats & rebuild
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Admin: Index Stats ---")

	indexStats, err := client.AdminIndexStats(ctx, namespace)
	if err != nil {
		log.Printf("AdminIndexStats (may require admin key): %v", err)
	} else {
		fmt.Printf("Index stats: %v\n", indexStats)
	}

	fmt.Println("\n--- Admin: Rebuild Indexes ---")

	rebuildResp, err := client.RebuildIndexes(ctx, namespace)
	if err != nil {
		log.Printf("RebuildIndexes (may require admin key): %v", err)
	} else {
		fmt.Printf("Rebuild status: %s\n", rebuildResp.Status)
	}

	// -------------------------------------------------------------------------
	// Analytics
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Analytics Overview ---")

	analytics, err := client.AnalyticsOverview(ctx, nil)
	if err != nil {
		log.Printf("Analytics (may require admin key): %v", err)
	} else {
		fmt.Printf("Total queries: %d, Avg latency: %.2fms, Cache hit rate: %.1f%%\n",
			analytics.TotalQueries, analytics.AvgLatencyMs, analytics.CacheHitRate*100)
	}

	// -------------------------------------------------------------------------
	// Cleanup
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Cleanup ---")

	if err := client.DeleteNamespace(ctx, namespace); err != nil {
		log.Fatalf("Failed to delete namespace: %v", err)
	}
	fmt.Println("Namespace deleted")
	fmt.Println("\nDone!")
}
