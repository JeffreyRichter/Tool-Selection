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
	fmt.Printf("Tool count=%d, Execution time=%v\n\n", len(listToolsResult.Tools), time.Since(start))

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
	for toolName, prompts := range toolNameWithPrompts {
		for _, p := range prompts {
			promptCount++
			fmt.Printf("\nPrompt: %s\nExpected tool: %s", p, toolName)
			vector := createEmbeddings(p)
			queryResults := db.Query(vector, QueryOptions{TopK: 10})
			for _, qr := range queryResults {
				note := ""
				if string(qr.Entry.ID) == toolName {
					note = "*** EXPECTED ***"
				}
				fmt.Printf("\n   %f   %-50s     %s", qr.Score, qr.Entry.ID, note)
			}
		}
	}
	fmt.Printf("\n\nPrompt count=%d, Execution time=%v\n", promptCount, time.Since(start))
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
