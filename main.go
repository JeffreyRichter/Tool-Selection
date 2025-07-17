package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"JeffreyRichter.com/ToolSelection/mcp"
	"github.com/joho/godotenv"
)

// isMarkdownOutput checks if the output should be in markdown format
// Only checks for output=md environment variable
func isMarkdownOutput() bool {
	return strings.ToLower(os.Getenv("output")) == "md"
}

// getAllTools returns the total number of tools in the database
func getAllTools(db *VectorDB) int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.entries)
}

// calculateSuccessRate calculates how many tests passed (expected tool was ranked #1)
func calculateSuccessRate(db *VectorDB, toolNameWithPrompts map[string][]string) int {
	successfulTests := 0
	for toolName, prompts := range toolNameWithPrompts {
		for _, p := range prompts {
			vector := createEmbeddings(p)
			queryResults := db.Query(vector, QueryOptions{TopK: 1})
			if len(queryResults) > 0 && string(queryResults[0].Entry.ID) == toolName {
				successfulTests++
			}
		}
	}
	return successfulTests
}

func main() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		// .env file is optional, so we just log if it's not found
		log.Printf("No .env file found or error loading it: %v", err)
	}

	listToolsResult := mcp.ListToolsResult{}
	{
		toolsListResultJson := string(must(os.ReadFile("list-tools.json")))
		toolsListResultJson = toolsListResultJson[1 : len(toolsListResultJson)-1]    // Remove the first and last characters (quotes)
		toolsListResultJson = strings.ReplaceAll(toolsListResultJson, "\\'", "'")    // Convert \' --> '
		toolsListResultJson = strings.ReplaceAll(toolsListResultJson, "\\\\\"", "'") // Convert \\" --> '
		err := json.Unmarshal(([]byte)(toolsListResultJson), &listToolsResult)
		_ = err
		//fmt.Println(err)
	}

	db := NewVectorDB(CosineSimilarity{}, nil)
	start := time.Now()
	tools2DB(db, listToolsResult.Tools)
	toolCount := getAllTools(db)
	executionTime := time.Since(start)

	// Check if output should use markdown format
	useMarkdown := isMarkdownOutput()

	if useMarkdown {
		// Markdown header for file output
		fmt.Println("# Tool Selection Analysis Setup")
		fmt.Println()
		fmt.Printf("**Setup completed:** %s  \n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Printf("**Tool count:** %d  \n", toolCount)
		fmt.Printf("**Database setup time:** %v  \n", executionTime)
		fmt.Println()
		fmt.Println("---")
		fmt.Println()
	} else {
		// Original terminal format
		fmt.Printf("Tool count=%d, Execution time=%v\n\n", toolCount, executionTime)
	}

	// Load prompts from JSON file
	toolNameAndPrompts := loadPromptsFromJSON("prompts.json")
	runPrompts(db, toolNameAndPrompts)
}

func tools2DB(db *VectorDB, tools []mcp.Tool) {
	const threshold = 2         // Each goroutine processes at most 'threshold' entries
	if len(tools) > threshold { // https://www.youtube.com/watch?v=P1tREHhINH4
		half := len(tools) / 2 // Split the entries in half
		wg := sync.WaitGroup{}
		// This goroutine processes half; 0 to (half-1) inclusive
		// wg.Do(func() { leftResult = db.querySlice(entries[:half], vector, o) })
		{ // Delete this {} block when wg.Do exists
			wg.Add(1)
			go func() { // This goroutine processes half
				defer wg.Done()
				tools2DB(db, tools[:half]) // 0 to (half-1) inclusive
			}()
		}
		// The current goroutine processes the other half
		tools2DB(db, tools[half:]) // half to (len-1) inclusive
		wg.Wait()                  // Wait for the left goroutine to finish
		return                     // All tools processed
	}

	for _, t := range tools {
		_, _, input := t.Name, t.Title, *t.Description
		vector := createEmbeddings(input)
		db.Upsert(&Entry{ID: ID(t.Name), Metadata: &t, Vector: vector})
	}
}

func createEmbeddings(input string) []float32 {
	// Docs: https://learn.microsoft.com/en-us/azure/ai-services/openai/reference#embeddings

	uri := os.Getenv("AOAI_ENDPOINT")
	if uri == "" {
		log.Fatalf("AOAI_ENDPOINT environment variable is required")
	}
	//const deploymentName = "text-embedding-3-large"

	// Check for environment variable first, then fall back to file
	apiKey := os.Getenv("TEXT_EMBEDDING_API_KEY")
	if apiKey == "" {
		// Try to read from file as fallback
		keyBytes, err := os.ReadFile("api-key.txt")
		if err != nil {
			log.Fatalf("API key not found. Please set TEXT_EMBEDDING_API_KEY environment variable or create api-key.txt file: %v", err)
		}
		apiKey = strings.TrimSpace(string(keyBytes))
	}

	// Create the request body using proper JSON marshaling to avoid escaping issues
	requestBody := struct {
		Input []string `json:"input"`
	}{
		Input: []string{input},
	}

	reqBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Fatalf("Failed to marshal request body: %v", err)
	}

	req := must(http.NewRequest(http.MethodPost, uri, strings.NewReader(string(reqBodyBytes))))
	req.Header.Add("api-key", apiKey)
	req.Header.Add("Content-Type", "application/json")
	response := must(http.DefaultClient.Do(req))

	embedResponse := struct {
		Data []struct {
			//Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}{}
	bytes := must(io.ReadAll(response.Body))
	response.Body.Close()

	must(0, json.Unmarshal(bytes, &embedResponse))

	// Check for API errors
	if embedResponse.Error != nil {
		log.Fatalf("API error: %s - %s", embedResponse.Error.Type, embedResponse.Error.Message)
	}

	// Check if we have data
	if len(embedResponse.Data) == 0 {
		log.Fatalf("No embedding data returned from API. Response: %s", string(bytes))
	}

	return embedResponse.Data[0].Embedding
}

func runPrompts(db *VectorDB, toolNameWithPrompts map[string][]string) {
	start := time.Now()
	promptCount := 0

	// Check if output should use markdown format
	useMarkdown := isMarkdownOutput()

	if useMarkdown {
		// Output markdown format
		fmt.Println("# Tool Selection Analysis Results")
		fmt.Println()
		fmt.Printf("**Analysis Date:** %s  \n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Printf("**Total Tools:** %d  \n", getAllTools(db))
		fmt.Println()
		fmt.Println("## Table of Contents")
		fmt.Println()

		// Generate TOC
		toolIndex := 1
		for toolName, prompts := range toolNameWithPrompts {
			for range prompts {
				fmt.Printf("- [Test %d: %s](#test-%d)\n", toolIndex, toolName, toolIndex)
				toolIndex++
			}
		}
		fmt.Println()
		fmt.Println("---")
		fmt.Println()
	}

	testNumber := 1
	for toolName, prompts := range toolNameWithPrompts {
		for _, p := range prompts {
			promptCount++

			if useMarkdown {
				// Markdown format
				fmt.Printf("## Test %d\n", testNumber)
				fmt.Println()
				fmt.Printf("**Expected Tool:** `%s`  \n", toolName)
				fmt.Printf("**Prompt:** %s  \n", p)
				fmt.Println()
				fmt.Println("### Results")
				fmt.Println()
				fmt.Println("| Rank | Score | Tool | Status |")
				fmt.Println("|------|-------|------|--------|")
			} else {
				// Original terminal format
				fmt.Printf("\nPrompt: %s\nExpected tool: %s", p, toolName)
			}

			vector := createEmbeddings(p)
			queryResults := db.Query(vector, QueryOptions{TopK: 10})

			for i, qr := range queryResults {
				if useMarkdown {
					status := ""
					if string(qr.Entry.ID) == toolName {
						status = "✅ **EXPECTED**"
					} else {
						status = "❌"
					}
					fmt.Printf("| %d | %.6f | `%s` | %s |\n", i+1, qr.Score, qr.Entry.ID, status)
				} else {
					note := ""
					if string(qr.Entry.ID) == toolName {
						note = "*** EXPECTED ***"
					}
					fmt.Printf("\n   %f   %-50s     %s", qr.Score, qr.Entry.ID, note)
				}
			}

			if useMarkdown {
				fmt.Println()
				fmt.Println("---")
				fmt.Println()
			}

			testNumber++
		}
	}

	if useMarkdown {
		fmt.Println("## Summary")
		fmt.Println()
		fmt.Printf("**Total Prompts Tested:** %d  \n", promptCount)
		fmt.Printf("**Execution Time:** %v  \n", time.Since(start))
		fmt.Println()

		// Calculate success rate
		successfulTests := calculateSuccessRate(db, toolNameWithPrompts)
		successRate := float64(successfulTests) / float64(promptCount) * 100
		fmt.Printf("**Success Rate:** %.1f%% (%d/%d tests passed)  \n", successRate, successfulTests, promptCount)
		fmt.Println()

		fmt.Println("### Success Rate Analysis")
		fmt.Println()
		if successRate >= 90 {
			fmt.Println("🟢 **Excellent** - The tool selection system is performing very well.")
		} else if successRate >= 75 {
			fmt.Println("🟡 **Good** - The tool selection system is performing adequately but has room for improvement.")
		} else if successRate >= 50 {
			fmt.Println("🟠 **Fair** - The tool selection system needs significant improvement.")
		} else {
			fmt.Println("🔴 **Poor** - The tool selection system requires major improvements.")
		}
		fmt.Println()
	} else {
		fmt.Printf("\n\nPrompt count=%d, Execution time=%v\n", promptCount, time.Since(start))
	}
}

func must[R any](r R, err error) R {
	if err != nil {
		panic(err)
	}
	return r
}

// loadPromptsFromJSON loads the tool prompts from a JSON file.
// The JSON structure should be: {"tool-name": ["prompt1", "prompt2", ...], ...}
// This allows for easy modification of test prompts without recompiling the application.
func loadPromptsFromJSON(filename string) map[string][]string {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read prompts file %s: %v", filename, err)
	}

	var prompts map[string][]string
	if err := json.Unmarshal(data, &prompts); err != nil {
		log.Fatalf("Failed to parse prompts JSON from %s: %v", filename, err)
	}

	return prompts
}
