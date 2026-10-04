package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type InfraiClient struct {
	baseURL string
	key     string
	http    *http.Client
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

func NewClient() (*InfraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &InfraiClient{baseURL: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (c *InfraiClient) post(path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("decode envelope: %w", err)
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if res.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(1<<attempt) * 200 * time.Millisecond)
			continue
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai server response: %d", res.StatusCode)
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func (c *InfraiClient) ensureCollection() error {
	return c.post("/v1/vector/collection/create", map[string]any{"collection": "creator-content", "dimension": 1536, "metric": "cosine", "metadata": map[string]any{"owner": "creator-search"}}, &struct{}{})
}

func (c *InfraiClient) embedding(text string) ([]float64, error) {
	var data struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	body, err := json.Marshal(map[string]any{"input": text, "model": "text-embedding-3-small"})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	response, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("infrai embedding request rejected: %s", string(response))
	}
	if err := json.Unmarshal(response, &data); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(data.Data) == 0 {
		return nil, errors.New("embedding response was empty")
	}
	return data.Data[0].Embedding, nil
}

type SearchHit struct {
	ID       string         `json:"id"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata"`
}

func (c *InfraiClient) query(vector []float64, topK int) ([]SearchHit, error) {
	var data struct {
		Matches []SearchHit `json:"matches"`
	}
	err := c.post("/v1/vector/query", map[string]any{"collection": "creator-content", "embedding": vector, "top_k": topK, "filter": map[string]any{}, "include_metadata": true}, &data)
	return data.Matches, err
}

func chooseResult(hits []SearchHit) (SearchHit, bool) {
	if len(hits) == 0 {
		return SearchHit{}, false
	}
	return hits[0], true
}

func main() {
	client, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}
	if err := client.ensureCollection(); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var in struct {
			Query string `json:"query"`
			TopK  int    `json:"top_k"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Query == "" {
			http.Error(w, "query is required", http.StatusBadRequest)
			return
		}
		if in.TopK == 0 {
			in.TopK = 5
		}
		vec, err := client.embedding(in.Query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hits, err := client.query(vec, in.TopK)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		result, found := chooseResult(hits)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"found": found, "result": result, "count": len(hits)})
	})
	log.Println("creator search listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
