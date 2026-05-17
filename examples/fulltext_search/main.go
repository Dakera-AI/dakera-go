// Example: Dakera Go SDK — Full-Text Search (index, search, stats, delete)
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

	namespace := "fulltext-example"

	// -------------------------------------------------------------------------
	// Create namespace
	// -------------------------------------------------------------------------
	fmt.Println("--- Setup ---")

	_, err := client.CreateNamespace(ctx, namespace, &dakera.CreateNamespaceOptions{
		Dimensions: 3,
	})
	if err != nil && !dakera.IsValidationError(err) {
		log.Fatalf("Failed to create namespace: %v", err)
	}
	fmt.Printf("Namespace %q ready\n", namespace)

	// -------------------------------------------------------------------------
	// Index Documents
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Index Documents ---")

	indexResp, err := client.IndexDocuments(ctx, namespace, []dakera.DocumentInput{
		{ID: "article-1", Text: "Kubernetes orchestrates containerized applications across clusters of machines."},
		{ID: "article-2", Text: "Docker provides OS-level virtualization using containers for application isolation."},
		{ID: "article-3", Text: "Terraform enables infrastructure as code for provisioning cloud resources declaratively."},
		{ID: "article-4", Text: "Prometheus is an open-source monitoring system with a dimensional data model."},
		{ID: "article-5", Text: "Grafana provides dashboards and visualization for metrics, logs, and traces."},
	})
	if err != nil {
		log.Fatalf("IndexDocuments failed: %v", err)
	}
	if indexResp == nil {
		log.Fatalf("unexpected: index response is nil")
	}
	fmt.Printf("Indexed %d documents\n", indexResp.IndexedCount)

	// -------------------------------------------------------------------------
	// Full-Text Search
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Full-Text Search: 'container' ---")

	results, err := client.FulltextSearch(ctx, namespace, "container", &dakera.FullTextSearchOptions{
		TopK: 5,
	})
	if err != nil {
		log.Fatalf("FulltextSearch failed: %v", err)
	}
	if len(results) == 0 {
		log.Fatalf("unexpected: no fulltext search results for 'container'")
	}
	for _, r := range results {
		fmt.Printf("  [%.4f] %s: %s\n", r.Score, r.ID, r.Text)
	}

	fmt.Println("\n--- Full-Text Search: 'monitoring metrics' ---")

	results2, err := client.FulltextSearch(ctx, namespace, "monitoring metrics", &dakera.FullTextSearchOptions{
		TopK: 3,
	})
	if err != nil {
		log.Fatalf("FulltextSearch failed: %v", err)
	}
	for _, r := range results2 {
		fmt.Printf("  [%.4f] %s: %s\n", r.Score, r.ID, r.Text)
	}

	// -------------------------------------------------------------------------
	// Fulltext Stats
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Fulltext Index Stats ---")

	stats, err := client.FulltextStats(ctx, namespace)
	if err != nil {
		log.Fatalf("FulltextStats failed: %v", err)
	}
	if stats == nil {
		log.Fatalf("unexpected: fulltext stats is nil")
	}
	fmt.Printf("Documents: %d, Unique terms: %d, Avg doc length: %.1f\n",
		stats.DocumentCount, stats.UniqueTerms, stats.AvgDocLength)

	if stats.DocumentCount == 0 {
		log.Fatalf("unexpected: document count should be > 0 after indexing")
	}

	// -------------------------------------------------------------------------
	// Fulltext Delete
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Fulltext Delete ---")

	deleteResp, err := client.FulltextDelete(ctx, namespace, []string{"article-4", "article-5"})
	if err != nil {
		log.Fatalf("FulltextDelete failed: %v", err)
	}
	if deleteResp == nil {
		log.Fatalf("unexpected: fulltext delete response is nil")
	}
	fmt.Printf("Deleted %d documents from fulltext index\n", deleteResp.DeletedCount)

	if deleteResp.DeletedCount != 2 {
		log.Fatalf("unexpected: expected 2 deletions, got %d", deleteResp.DeletedCount)
	}

	// Verify stats after deletion
	fmt.Println("\n--- Stats After Delete ---")

	statsAfter, err := client.FulltextStats(ctx, namespace)
	if err != nil {
		log.Fatalf("FulltextStats failed: %v", err)
	}
	fmt.Printf("Documents remaining: %d\n", statsAfter.DocumentCount)

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
