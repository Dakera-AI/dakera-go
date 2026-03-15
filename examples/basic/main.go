// Example: Basic Dakera Go SDK usage
package main

import (
	"context"
	"fmt"
	"log"

	dakera "github.com/dakera-ai/dakera-go"
)

func main() {
	// Create client
	client := dakera.NewClient("http://localhost:3000")
	ctx := context.Background()

	// Check server health
	health, err := client.Health(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	fmt.Printf("Server status: %s (version: %s)\n", health.Status, health.Version)

	namespace := "example-namespace"

	// Create namespace
	_, err = client.CreateNamespace(ctx, namespace, &dakera.CreateNamespaceOptions{
		Dimensions: 3,
	})
	if err != nil && !dakera.IsValidationError(err) {
		log.Fatalf("Failed to create namespace: %v", err)
	}

	// Upsert vectors
	upsertResp, err := client.Upsert(ctx, namespace, []dakera.VectorInput{
		{
			ID:     "vec1",
			Values: []float32{0.1, 0.2, 0.3},
			Metadata: map[string]interface{}{
				"category": "electronics",
				"price":    299.99,
			},
		},
		{
			ID:     "vec2",
			Values: []float32{0.4, 0.5, 0.6},
			Metadata: map[string]interface{}{
				"category": "books",
				"price":    19.99,
			},
		},
		{
			ID:     "vec3",
			Values: []float32{0.15, 0.25, 0.35},
			Metadata: map[string]interface{}{
				"category": "electronics",
				"price":    599.99,
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to upsert: %v", err)
	}
	fmt.Printf("Upserted %d vectors\n", upsertResp.UpsertedCount)

	// Query similar vectors
	fmt.Println("\n--- Query Results ---")
	results, err := client.Query(ctx, namespace, []float32{0.1, 0.2, 0.3}, &dakera.QueryOptions{
		TopK:            10,
		IncludeMetadata: true,
	})
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}

	for _, result := range results.Results {
		fmt.Printf("ID: %s, Score: %.4f, Metadata: %v\n",
			result.ID, result.Score, result.Metadata)
	}

	// Query with filter
	fmt.Println("\n--- Filtered Query (electronics only) ---")
	filteredResults, err := client.Query(ctx, namespace, []float32{0.1, 0.2, 0.3}, &dakera.QueryOptions{
		TopK: 10,
		Filter: map[string]interface{}{
			"category": dakera.Eq("electronics"),
		},
		IncludeMetadata: true,
	})
	if err != nil {
		log.Fatalf("Failed to query with filter: %v", err)
	}

	for _, result := range filteredResults.Results {
		fmt.Printf("ID: %s, Score: %.4f, Category: %v\n",
			result.ID, result.Score, result.Metadata["category"])
	}

	// Fetch vectors by ID
	fmt.Println("\n--- Fetched Vectors ---")
	vectors, err := client.Fetch(ctx, namespace, []string{"vec1", "vec2"}, nil)
	if err != nil {
		log.Fatalf("Failed to fetch: %v", err)
	}

	for _, vec := range vectors {
		fmt.Printf("ID: %s, Values: %v\n", vec.ID, vec.Values)
	}

	// Batch query
	fmt.Println("\n--- Batch Query Results ---")
	batchResults, err := client.BatchQuery(ctx, namespace, []dakera.BatchQuerySpec{
		{Vector: []float32{0.1, 0.2, 0.3}, TopK: 2},
		{Vector: []float32{0.4, 0.5, 0.6}, TopK: 2},
	})
	if err != nil {
		log.Fatalf("Failed to batch query: %v", err)
	}

	for i, result := range batchResults {
		fmt.Printf("Query %d results:\n", i+1)
		for _, r := range result.Results {
			fmt.Printf("  ID: %s, Score: %.4f\n", r.ID, r.Score)
		}
	}

	// Get namespace info
	fmt.Println("\n--- Namespace Info ---")
	info, err := client.GetNamespace(ctx, namespace)
	if err != nil {
		log.Fatalf("Failed to get namespace: %v", err)
	}
	fmt.Printf("Name: %s, Vectors: %d, Dimensions: %d\n",
		info.Name, info.VectorCount, info.Dimensions)

	// Delete vectors
	deleteResp, err := client.Delete(ctx, namespace, dakera.DeleteOptions{
		IDs: []string{"vec1"},
	})
	if err != nil {
		log.Fatalf("Failed to delete: %v", err)
	}
	fmt.Printf("\nDeleted %d vectors\n", deleteResp.DeletedCount)

	// Cleanup - delete namespace
	err = client.DeleteNamespace(ctx, namespace)
	if err != nil {
		log.Fatalf("Failed to delete namespace: %v", err)
	}
	fmt.Println("Namespace deleted")
}
