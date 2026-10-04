package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddingDecodesOpenAIResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("got path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"embedding":[0.25,0.75],"index":0}]}`))
	}))
	defer server.Close()

	client := &InfraiClient{baseURL: server.URL, key: "test-key", http: server.Client()}
	got, err := client.embedding("creator update")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 0.25 || got[1] != 0.75 {
		t.Fatalf("got embedding %v", got)
	}
}

func TestChooseResult(t *testing.T) {
	tests := []struct {
		name  string
		hits  []SearchHit
		found bool
		id    string
	}{
		{"first hit is selected", []SearchHit{{ID: "post-7", Score: .92}}, true, "post-7"},
		{"empty search is explicit", nil, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := chooseResult(tt.hits)
			if found != tt.found || got.ID != tt.id {
				t.Fatalf("got (%v,%q), want (%v,%q)", found, got.ID, tt.found, tt.id)
			}
		})
	}
}
