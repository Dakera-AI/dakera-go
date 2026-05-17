// Example: Dakera Go SDK — Vector Operations (bulk upsert/update/delete, count, aggregate, export)
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

	namespace := "vectors-example"

	// -------------------------------------------------------------------------
	// Create namespace
	// -------------------------------------------------------------------------
	fmt.Println("--- Setup ---")

	_, err := client.CreateNamespace(ctx, namespace, &dakera.CreateNamespaceOptions{
		Dimensions: 4,
	})
	if err != nil && !dakera.IsValidationError(err) {
		log.Fatalf("Failed to create namespace: %v", err)
	}
	fmt.Printf("Namespace %q ready\n", namespace)

	// -------------------------------------------------------------------------
	// Bulk Upsert
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Bulk Upsert ---")

	vectors := make([]dakera.VectorInput, 20)
	for i := range vectors {
		category := "group-a"
		if i >= 10 {
			category = "group-b"
		}
		vectors[i] = dakera.VectorInput{
			ID:     fmt.Sprintf("vec-%03d", i),
			Values: []float32{float32(i) * 0.1, float32(i) * 0.2, float32(i) * 0.3, float32(i) * 0.4},
			Metadata: map[string]interface{}{
				"category": category,
				"index":    i,
				"active":   true,
			},
		}
	}

	upsertResp, err := client.Upsert(ctx, namespace, vectors)
	if err != nil {
		log.Fatalf("Upsert failed: %v", err)
	}
	if upsertResp.UpsertedCount != 20 {
		log.Fatalf("unexpected: expected 20 upserted, got %d", upsertResp.UpsertedCount)
	}
	fmt.Printf("Upserted %d vectors\n", upsertResp.UpsertedCount)

	// -------------------------------------------------------------------------
	// Count Vectors
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Count Vectors ---")

	countAll, err := client.CountVectors(ctx, namespace, nil)
	if err != nil {
		log.Fatalf("CountVectors failed: %v", err)
	}
	if countAll.Count != 20 {
		log.Fatalf("unexpected: expected count=20, got %d", countAll.Count)
	}
	fmt.Printf("Total vectors: %d\n", countAll.Count)

	// Count with filter
	countFiltered, err := client.CountVectors(ctx, namespace, map[string]interface{}{
		"category": dakera.Eq("group-a"),
	})
	if err != nil {
		log.Fatalf("CountVectors (filtered) failed: %v", err)
	}
	fmt.Printf("Vectors in group-a: %d\n", countFiltered.Count)

	// -------------------------------------------------------------------------
	// Bulk Update
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Bulk Update ---")

	bulkUpdateResp, err := client.BulkUpdateVectors(ctx, namespace,
		map[string]interface{}{
			"category": dakera.Eq("group-b"),
		},
		map[string]interface{}{
			"active": false,
			"status": "archived",
		},
	)
	if err != nil {
		log.Fatalf("BulkUpdateVectors failed: %v", err)
	}
	if bulkUpdateResp == nil {
		log.Fatalf("unexpected: bulk update response is nil")
	}
	fmt.Printf("Updated: %d, Failed: %d\n", bulkUpdateResp.Updated, bulkUpdateResp.Failed)

	// -------------------------------------------------------------------------
	// Aggregate
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Aggregate ---")

	topGroups := 10
	aggResp, err := client.Aggregate(ctx, namespace, dakera.AggregationRequest{
		GroupBy:   "category",
		Metrics:   []string{"count", "avg"},
		TopGroups: &topGroups,
	})
	if err != nil {
		log.Fatalf("Aggregate failed: %v", err)
	}
	if aggResp == nil {
		log.Fatalf("unexpected: aggregation response is nil")
	}
	fmt.Printf("Aggregation groups: %d, total groups: %d\n",
		len(aggResp.Groups), aggResp.TotalGroups)
	for _, g := range aggResp.Groups {
		fmt.Printf("  %s: count=%d\n", g.Key, g.Count)
	}

	// -------------------------------------------------------------------------
	// Export Vectors
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Export Vectors ---")

	limit := 5
	exportResp, err := client.ExportVectors(ctx, namespace, dakera.ExportRequest{
		Limit:          &limit,
		IncludeVectors: true,
	})
	if err != nil {
		log.Fatalf("ExportVectors failed: %v", err)
	}
	if exportResp == nil {
		log.Fatalf("unexpected: export response is nil")
	}
	fmt.Printf("Exported %d vectors (has more: %v)\n", len(exportResp.Vectors), exportResp.NextCursor != "")
	for _, v := range exportResp.Vectors {
		fmt.Printf("  %s: dims=%d, metadata=%v\n", v.ID, len(v.Values), v.Metadata)
	}

	// -------------------------------------------------------------------------
	// Bulk Delete
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Bulk Delete ---")

	bulkDeleteResp, err := client.BulkDeleteVectors(ctx, namespace, map[string]interface{}{
		"category": dakera.Eq("group-b"),
	})
	if err != nil {
		log.Fatalf("BulkDeleteVectors failed: %v", err)
	}
	if bulkDeleteResp == nil {
		log.Fatalf("unexpected: bulk delete response is nil")
	}
	fmt.Printf("Deleted: %d, Failed: %d\n", bulkDeleteResp.Deleted, bulkDeleteResp.Failed)

	// Verify count after delete
	countAfter, err := client.CountVectors(ctx, namespace, nil)
	if err != nil {
		log.Fatalf("CountVectors failed: %v", err)
	}
	fmt.Printf("Remaining vectors: %d\n", countAfter.Count)

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
