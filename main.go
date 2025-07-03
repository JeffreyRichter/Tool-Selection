package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"JeffreyRichter.com/ToolSelection/mcp"
)

func main() {
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

	const uri = "https://openai-shared.openai.azure.com/openai/deployments/text-embedding-3-large/embeddings?api-version=2023-05-15"
	//const deploymentName = "text-embedding-3-large"
	apiKey := string(must(os.ReadFile("api-key.txt")))

	//dimensions := 1024
	reqBody := fmt.Sprintf(`{ "input": [ "%s" ]	}`, input) //dimensions
	req := must(http.NewRequest(http.MethodPost, uri, strings.NewReader(reqBody)))
	req.Header.Add("api-key", apiKey)
	req.Header.Add("Content-Type", "application/json")
	response := must(http.DefaultClient.Do(req))

	embedResponse := struct {
		Data []struct {
			//Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}{}
	bytes := must(io.ReadAll(response.Body))
	response.Body.Close()
	must(0, json.Unmarshal(bytes, &embedResponse))
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
