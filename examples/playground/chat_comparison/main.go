// Example: Dakera Go SDK — LLM Chat Comparison
//
// Demonstrates the pattern used by the Dakera playground: run the same user
// query through two paths and compare responses.
//
//	Path A (memory-augmented) — recall relevant context, prepend to prompt
//	Path B (baseline)         — send the raw prompt with no memory context
//
// Run:
//
//	DAKERA_API_URL=https://5-75-177-31.sslip.io DAKERA_API_KEY=<key> \
//	  go run ./examples/playground/chat_comparison
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	dakera "github.com/dakera-ai/dakera-go"
)

const agentID = "playground-demo"

func playgroundURL() string {
	if u := os.Getenv("DAKERA_API_URL"); u != "" {
		return u
	}
	return "https://5-75-177-31.sslip.io"
}

func playgroundKey() string {
	if k := os.Getenv("DAKERA_API_KEY"); k != "" {
		return k
	}
	return "playground-demo"
}

func buildContextPrompt(memories []dakera.RecalledMemory, userMessage string) string {
	if len(memories) == 0 {
		return userMessage
	}
	lines := make([]string, len(memories))
	for i, m := range memories {
		lines[i] = "- " + m.Content
	}
	return "[Relevant context from memory]\n" +
		strings.Join(lines, "\n") +
		"\n\n[User message]\n" + userMessage
}

func callLLM(prompt string) string {
	// Placeholder — swap in any LLM provider:
	//
	//   client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	//   resp, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
	//       Model: openai.GPT4oMini,
	//       Messages: []openai.ChatCompletionMessage{
	//           {Role: openai.ChatMessageRoleUser, Content: prompt},
	//       },
	//   })
	//   return resp.Choices[0].Message.Content
	if strings.Contains(prompt, "[Relevant context from memory]") {
		return "I recall you mentioned this before. Here is a context-aware answer."
	}
	return "I have no prior context. Here is a generic answer."
}

func main() {
	client := dakera.NewClientWithOptions(dakera.ClientOptions{
		BaseURL: playgroundURL(),
		APIKey:  playgroundKey(),
	})
	ctx := context.Background()

	fmt.Println("=== Dakera Playground — LLM Chat Comparison Demo ===\n")

	// ------------------------------------------------------------------
	// Step 1: Seed some prior conversation turns
	// ------------------------------------------------------------------
	fmt.Println("Seeding prior conversation turns into Dakera memory...")
	seedSession, err := dakera.NewChatMemorySession(ctx, client, agentID, map[string]interface{}{
		"source": "playground-seed",
	})
	if err != nil {
		log.Fatalf("NewChatMemorySession failed: %v", err)
	}
	if _, err := seedSession.Store(ctx, "user", "I'm building a chatbot in Go using goroutines."); err != nil {
		log.Fatalf("Store failed: %v", err)
	}
	if _, err := seedSession.Store(ctx, "assistant", "Great choice — Go's concurrency model is excellent for chatbots."); err != nil {
		log.Fatalf("Store failed: %v", err)
	}
	if _, err := seedSession.Store(ctx, "user", "My team prefers simple HTTP APIs so we use Chi or Gin on the backend."); err != nil {
		log.Fatalf("Store failed: %v", err)
	}
	fmt.Printf("  Session %s: stored 3 turns\n\n", seedSession.SessionID())
	if _, err := seedSession.Close(ctx); err != nil {
		log.Printf("close seed session: %v", err)
	}

	// ------------------------------------------------------------------
	// Step 2: Start a new session and compare responses
	// ------------------------------------------------------------------
	followUp := "What framework should I use for the async background tasks?"

	compareSession, err := dakera.NewChatMemorySession(ctx, client, agentID, map[string]interface{}{
		"source": "playground-compare",
	})
	if err != nil {
		log.Fatalf("NewChatMemorySession failed: %v", err)
	}

	fmt.Printf("Comparison session: %s\n", compareSession.SessionID())
	fmt.Printf("User: %s\n\n", followUp)

	// Path A — memory-augmented
	memories, err := compareSession.Recall(ctx, followUp, 5)
	if err != nil {
		log.Fatalf("Recall failed: %v", err)
	}
	augmentedPrompt := buildContextPrompt(memories, followUp)
	responseWithMemory := callLLM(augmentedPrompt)

	// Path B — baseline (no memory)
	responseWithoutMemory := callLLM(followUp)

	// Store the actual exchange
	if _, err := compareSession.Store(ctx, "user", followUp); err != nil {
		log.Printf("store user turn: %v", err)
	}
	if _, err := compareSession.Store(ctx, "assistant", responseWithMemory); err != nil {
		log.Printf("store assistant turn: %v", err)
	}
	if _, err := compareSession.Close(ctx); err != nil {
		log.Printf("close compare session: %v", err)
	}

	// ------------------------------------------------------------------
	// Step 3: Print side-by-side comparison
	// ------------------------------------------------------------------
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│  WITHOUT Dakera memory                                      │")
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Printf("│  %s\n", responseWithoutMemory)
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Println("│  WITH Dakera memory                                         │")
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Printf("│  %s\n", responseWithMemory)
	fmt.Println("└─────────────────────────────────────────────────────────────┘")

	if len(memories) > 0 {
		fmt.Printf("\n  Memory used: %d relevant context item(s)\n", len(memories))
		for _, m := range memories {
			content := m.Content
			if len(content) > 80 {
				content = content[:80]
			}
			fmt.Printf("    • [%.2f] %s\n", m.Score, content)
		}
	}
}
