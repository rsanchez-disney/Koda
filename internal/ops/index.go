package ops

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ContextIndex is a TF-IDF index over context file chunks.
type ContextIndex struct {
	Chunks []IndexChunk          `json:"chunks"`
	IDF    map[string]float64    `json:"idf"`
	DocCount int                 `json:"doc_count"`
}

// IndexChunk is one indexed segment of a context file.
type IndexChunk struct {
	File   string             `json:"file"`
	Offset int                `json:"offset"`
	Text   string             `json:"text"`
	TF     map[string]float64 `json:"tf"`
}

// QueryResult is a scored chunk from a query.
type QueryResult struct {
	File   string  `json:"file"`
	Offset int     `json:"offset"`
	Text   string  `json:"text"`
	Score  float64 `json:"score"`
}

// BuildContextIndex indexes all .md files in contextDir into _index.json.
func BuildContextIndex(contextDir string) error {
	entries, err := os.ReadDir(contextDir)
	if err != nil {
		return err
	}

	var chunks []IndexChunk
	df := map[string]int{} // document frequency

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "_index.json" {
			continue
		}
		if strings.HasPrefix(e.Name(), "_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(contextDir, e.Name()))
		if err != nil {
			continue
		}
		fileChunks := splitChunks(string(data), 500)
		for i, chunk := range fileChunks {
			tf := computeTF(chunk)
			chunks = append(chunks, IndexChunk{
				File:   e.Name(),
				Offset: i,
				Text:   chunk,
				TF:     tf,
			})
			seen := map[string]bool{}
			for term := range tf {
				if !seen[term] {
					df[term]++
					seen[term] = true
				}
			}
		}
	}

	// Compute IDF
	idf := map[string]float64{}
	n := float64(len(chunks))
	for term, count := range df {
		idf[term] = math.Log(1 + n/float64(count))
	}

	idx := ContextIndex{Chunks: chunks, IDF: idf, DocCount: len(chunks)}
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(contextDir, "_index.json"), data, 0644)
}

// QueryContextIndex queries the index and returns top-K results.
func QueryContextIndex(contextDir string, query string, topK int) ([]QueryResult, error) {
	data, err := os.ReadFile(filepath.Join(contextDir, "_index.json"))
	if err != nil {
		return nil, err
	}
	var idx ContextIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	queryTerms := tokenize(query)
	queryTF := map[string]float64{}
	for _, t := range queryTerms {
		queryTF[t]++
	}
	for t := range queryTF {
		queryTF[t] /= float64(len(queryTerms))
	}

	// Score each chunk
	type scored struct {
		idx   int
		score float64
	}
	var results []scored
	for i, chunk := range idx.Chunks {
		score := 0.0
		for term, qtf := range queryTF {
			idf := idx.IDF[term]
			ctf := chunk.TF[term]
			score += qtf * idf * ctf
		}
		if score > 0 {
			results = append(results, scored{i, score})
		}
	}

	// Sort by score descending
	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return top-K
	if topK > len(results) {
		topK = len(results)
	}
	var out []QueryResult
	for _, r := range results[:topK] {
		c := idx.Chunks[r.idx]
		out = append(out, QueryResult{File: c.File, Offset: c.Offset, Text: c.Text, Score: r.score})
	}
	return out, nil
}

func splitChunks(text string, maxTokens int) []string {
	words := strings.Fields(text)
	var chunks []string
	for i := 0; i < len(words); i += maxTokens {
		end := i + maxTokens
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}
	if len(chunks) == 0 {
		chunks = []string{text}
	}
	return chunks
}

func computeTF(text string) map[string]float64 {
	terms := tokenize(text)
	tf := map[string]float64{}
	for _, t := range terms {
		tf[t]++
	}
	n := float64(len(terms))
	if n == 0 {
		return tf
	}
	for t := range tf {
		tf[t] /= n
	}
	return tf
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 2 {
				tokens = append(tokens, current.String())
			}
			current.Reset()
		}
	}
	if current.Len() > 2 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
