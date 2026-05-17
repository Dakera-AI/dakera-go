// Example: Dakera Go SDK — Memory & Session Operations
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

	agentID := "agent-001"

	// -------------------------------------------------------------------------
	// Store memories
	// -------------------------------------------------------------------------
	fmt.Println("--- Storing Memories ---")

	importance := float32(0.9)
	mem1, err := client.StoreMemory(ctx, agentID, dakera.StoreMemoryRequest{
		Content:    "The user prefers concise responses with code examples.",
		MemoryType: "semantic",
		Importance: &importance,
		Metadata: map[string]interface{}{
			"source": "user-feedback",
		},
	})
	if err != nil {
		log.Fatalf("StoreMemory failed: %v", err)
	} else {
		fmt.Printf("Stored memory: %s\n", mem1.Memory.ID)
	}

	imp2 := float32(0.7)
	mem2, err := client.StoreMemory(ctx, agentID, dakera.StoreMemoryRequest{
		Content:    "User is working on a Go web service using the chi router.",
		MemoryType: "episodic",
		Importance: &imp2,
	})
	if err != nil {
		log.Fatalf("StoreMemory failed: %v", err)
	} else {
		fmt.Printf("Stored memory: %s\n", mem2.Memory.ID)
	}

	// -------------------------------------------------------------------------
	// Recall memories
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Recalling Memories ---")

	recallResp, err := client.Recall(ctx, agentID, dakera.RecallRequest{
		Query: "What does the user prefer?",
		TopK:  5,
	})
	if err != nil {
		log.Fatalf("Recall failed: %v", err)
	} else {
		for _, m := range recallResp.Memories {
			fmt.Printf("  [%.2f] %s — %s\n", m.Score, m.MemoryType, m.Content)
		}
	}

	// -------------------------------------------------------------------------
	// Search memories by type
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Search Memories (type=semantic) ---")

	searched, err := client.SearchMemories(ctx, agentID, dakera.SearchMemoriesRequest{
		Query:      "user preferences",
		MemoryType: "semantic",
		TopK:       3,
	})
	if err != nil {
		log.Printf("SearchMemories (may not be supported): %v", err)
	} else {
		for _, m := range searched {
			fmt.Printf("  [%.2f] %s\n", m.Score, m.Content)
		}
	}

	// -------------------------------------------------------------------------
	// Sessions
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Session Management ---")

	session, err := client.StartSession(ctx, dakera.StartSessionRequest{
		AgentID: agentID,
		Metadata: map[string]interface{}{
			"task": "code-review",
		},
	})
	if err != nil {
		log.Printf("StartSession (may not be supported): %v", err)
	} else {
		fmt.Printf("Started session: %s\n", session.ID)

		sessionMem, err := client.StoreMemory(ctx, agentID, dakera.StoreMemoryRequest{
			Content:   "Reviewing PR #42: refactor authentication middleware.",
			SessionID: session.ID,
		})
		if err != nil {
			log.Printf("StoreMemory with session (may not be supported): %v", err)
		} else {
			fmt.Printf("Stored session memory: %s\n", sessionMem.Memory.ID)
		}

		endResp, err := client.EndSession(ctx, session.ID)
		if err != nil {
			log.Printf("EndSession (may not be supported): %v", err)
		} else {
			fmt.Printf("Ended session: %s (memories: %d)\n", endResp.Session.ID, endResp.MemoryCount)
		}
	}

	// -------------------------------------------------------------------------
	// Agent stats
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Agent Stats ---")

	stats, err := client.AgentStats(ctx, agentID)
	if err != nil {
		log.Printf("AgentStats (may not be supported): %v", err)
	} else {
		fmt.Printf("Agent: %s\n", stats.AgentID)
		fmt.Printf("  Total memories: %d\n", stats.TotalMemories)
		fmt.Printf("  Total sessions: %d\n", stats.TotalSessions)
	}

	// -------------------------------------------------------------------------
	// Summarize memories
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Summarize ---")

	summary, err := client.Summarize(ctx, dakera.SummarizeRequest{
		AgentID:    agentID,
		TargetType: "semantic",
	})
	if err != nil {
		log.Printf("Summarize (may not be supported): %v", err)
	} else {
		fmt.Printf("Summary (%d sources): %s\n", summary.SourceCount, summary.Summary)
	}

	fmt.Println("\nDone!")
}
