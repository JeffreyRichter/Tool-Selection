package main

// Performance: https://sourcegraph.com/blog/slow-to-simd

import (
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"
)

type ID string

type Entry struct {
	ID       ID
	Metadata any
	Vector   []float32
}

func (e *Entry) String() string {
	vectorHigh := min(len(e.Vector), 3)
	return fmt.Sprintf("ID=%s, Metadata=%#v, Vector=%v", e.ID, e.Metadata, e.Vector[:vectorHigh])
}

type VectorDB struct {
	mu             sync.RWMutex
	entries        []*Entry
	distanceMetric DistanceMetric
}

// NewVectorDB creates a new vector DB with the specified distance metric and entries.
// Note that the entries MUST be sorted by ID or all operations are unpredictable.
func NewVectorDB(distanceMetric DistanceMetric, entries []*Entry) *VectorDB {
	return &VectorDB{mu: sync.RWMutex{}, distanceMetric: distanceMetric, entries: entries}
}

func (db *VectorDB) search(id ID) (int, bool) {
	return slices.BinarySearchFunc(db.entries, id, func(e *Entry, searchID ID) int {
		if e.ID < searchID {
			return -1
		}
		if e.ID > searchID {
			return 1
		}
		return 0
	})
}

func (db *VectorDB) Upsert(entry *Entry) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if n, ok := db.search(entry.ID); !ok {
		db.entries = slices.Insert(db.entries, n, entry)
	} else {
		db.entries[n] = entry
	}
}

func (db *VectorDB) Get(id ID) (*Entry, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	n, ok := db.search(id)
	if !ok {
		return nil, false
	}
	return db.entries[n], true
}

func (db *VectorDB) Delete(id ID) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if n, ok := db.search(id); ok {
		db.entries = slices.Delete(db.entries, n, n+1)
	}
}

type QueryResult struct {
	Score float32
	Entry *Entry
}

type QueryOptions struct {
	TopK         int
	MinimumScore float32
	Predicate    func(e *Entry) bool // Optional predicate to filter results
}

func (db *VectorDB) Query(vector []float32, o QueryOptions) []QueryResult {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.querySlice(db.entries, vector, &o)
}

func (db *VectorDB) querySlice(entries []*Entry, vector []float32, o *QueryOptions) []QueryResult {
	const threshold = 100         // Each goroutine processes at most 'threshold' entries
	if len(entries) > threshold { // https://www.youtube.com/watch?v=P1tREHhINH4
		half := len(entries) / 2 // Split the entries in half
		wg := sync.WaitGroup{}
		// This goroutine processes half; 0 to (half-1) inclusive
		var leftResult []QueryResult
		// wg.Do(func() { leftResult = db.querySlice(entries[:half], vector, o) })
		{ // Delete this {} block when wg.Do exists
			wg.Add(1)
			go func() { // This goroutine processes half
				defer wg.Done()
				leftResult = db.querySlice(entries[:half], vector, o) // 0 to (half-1) inclusive
			}()
		}
		// The current goroutine processes the other half
		rightResult := db.querySlice(entries[half:], vector, o) // half to (len-1) inclusive
		wg.Wait()                                               // Wait for the left goroutine to finish

		// Return the top K scores from both left & right
		resultCount := len(leftResult) + len(rightResult)
		results := make([]QueryResult, 0, resultCount) // Slice sorted from best Score to worst score
		for len(results) < resultCount /* more available */ {
			switch {
			case len(leftResult) == 0: // Only right results left
				results = append(results, rightResult[0])
				rightResult = rightResult[1:]
			case len(rightResult) == 0: // Only left results left
				results = append(results, leftResult[0])
				leftResult = leftResult[1:]
			case leftResult[0].Score >= rightResult[0].Score: // Left result same or better than right
				results = append(results, leftResult[0])
				leftResult = leftResult[1:]
			default: // Right result less than left
				results = append(results, rightResult[0])
				rightResult = rightResult[1:]
			}
		}
		return results
	}

	results := make([]QueryResult, 0, o.TopK) // Slice of length 0, capacity topK; sorted from high Score to low score
	for _, e := range entries {
		if o.Predicate != nil && !o.Predicate(e) { // If predicate returns false, skip this entry
			continue
		}
		score := db.distanceMetric.Distance(vector, e.Vector)
		if score < o.MinimumScore {
			continue // If score is below the minimum, skip this entry
		}
		qr := QueryResult{Score: score, Entry: e} // Construct potential QueryResult
		// Find out where this score be inserted?
		n, _ := slices.BinarySearchFunc(results, qr, func(a, b QueryResult) int {
			n := 0
			switch {
			case a.Score < b.Score:
				n = 1
			case a.Score > b.Score:
				n = -1
			}
			if db.distanceMetric.BiggerIsCloser() {
				n = -n
			}
			return n
		})
		if n == cap(results) {
			// We're at capacity & Score is lower than anything we already have; do nothing (discard it)
		} else {
			if len(results) == o.TopK { // If there is no space, delete the worst (last) result
				results = slices.Delete(results, len(results)-1, len(results)) // Otherwise, delete the worst result and insert it
			}
			results = slices.Insert(results, n, qr) // Insert the new result
		}
	}
	return results
}

type DistanceMetric interface {
	Distance(a, b []float32) float32
	BiggerIsCloser() bool
}

var _, _ DistanceMetric = CosineSimilarity{}, DotProduct{}

type CosineSimilarity struct{}

func (c CosineSimilarity) Distance(a, b []float32) float32 {
	// If the vector lengths do not match, this funtion panics
	// Algorithms: https://www.pinecone.io/learn/vector-similarity/
	// https://weaviate.io/blog/distance-metrics-in-vector-search
	dotProduct, magnitudeA, magnitudeB := 0.0, 0.0, 0.0
	for k := range a {
		dotProduct += float64(a[k] * b[k])
		magnitudeA += math.Pow(float64(a[k]), 2)
		magnitudeB += math.Pow(float64(b[k]), 2)
	}
	return float32(dotProduct / (math.Sqrt(magnitudeA) * math.Sqrt(magnitudeB)))
	// Potential perf improvements: https://sourcegraph.com/blog/slow-to-simd
}

func (c CosineSimilarity) BiggerIsCloser() bool { return false }

type DotProduct struct{}

func (d DotProduct) Distance(a, b []float32) float32 {
	// If the vector lengths do not match, this funtion panics
	// Algorithms: https://www.pinecone.io/learn/vector-similarity/
	// https://weaviate.io/blog/distance-metrics-in-vector-search
	dotProduct := float32(0.0)
	for k := range a {
		dotProduct += a[k] * b[k]
	}
	return dotProduct
}

func (d DotProduct) BiggerIsCloser() bool { return true }

func TestVectorDB(t *testing.T) {
	db := NewVectorDB(CosineSimilarity{}, nil)
	db.Upsert(&Entry{ID: "1", Metadata: &metadata{Name: "Jeff"}, Vector: []float32{1, 2, 3}})
	entry, ok := db.Get("2")
	fmt.Printf("Found=%v: %s\n", ok, entry)
	db.Upsert(&Entry{ID: "2", Metadata: &metadata{Name: "Marc"}, Vector: []float32{4, 5, 6}})
	db.Upsert(&Entry{ID: "3", Metadata: &metadata{Name: "Aidan"}, Vector: []float32{7, 8, 9}})
	db.Upsert(&Entry{ID: "4", Metadata: &metadata{Name: "Grant"}, Vector: []float32{10, 11, 12}})
	db.Upsert(&Entry{ID: "5", Vector: []float32{13, 14, 15}})
	entry, ok = db.Get("2")
	fmt.Printf("Found=%v: %s\n", ok, entry)
	//db.Delete("2")
	entry, ok = db.Get("2")
	fmt.Printf("Found=%v: %s\n", ok, entry)

	qo := QueryOptions{
		TopK: 30,
		Predicate: func(e *Entry) bool {
			if md, ok := e.Metadata.(*metadata); ok {
				return md.Name != "Grant"
			}
			return false
		},
	}

	qr := db.Query([]float32{1, 2, 3}, qo)
	for i := range qr {
		fmt.Printf("%f: %s\n", qr[i].Score, qr[i].Entry)
	}
}

type metadata struct {
	Name string
}

func (m *metadata) String() string {
	return "{ Name=" + m.Name + " }"
}
