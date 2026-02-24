package gog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(ts *httptest.Server) *Client {
	return &Client{
		HTTPClient:   ts.Client(),
		EmbedBaseURL: ts.URL,
		APIBaseURL:   ts.URL,
		Token: &Token{
			AccessToken:  "test_token",
			RefreshToken: "ref",
			ExpiresIn:    3600,
			SavedAt:      time.Now(),
		},
	}
}

func TestGetOwnedGameIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/data/games" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(OwnedGamesResponse{Owned: []int{1, 2, 3}})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	ids, err := c.GetOwnedGameIDs()
	if err != nil {
		t.Fatalf("GetOwnedGameIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("got %d IDs, want 3", len(ids))
	}
	if ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("ids = %v, want [1 2 3]", ids)
	}
}

func TestGetProducts(t *testing.T) {
	t.Run("single batch", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]Product{
				{ID: 1, Title: "Game 1"},
				{ID: 2, Title: "Game 2"},
			})
		}))
		defer ts.Close()

		c := newTestClient(ts)
		products, err := c.GetProducts([]int{1, 2})
		if err != nil {
			t.Fatalf("GetProducts: %v", err)
		}
		if len(products) != 2 {
			t.Fatalf("got %d products, want 2", len(products))
		}
	})

	t.Run("batching over 50 IDs", func(t *testing.T) {
		requestCount := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			idsParam := r.URL.Query().Get("ids")
			idStrs := strings.Split(idsParam, ",")
			products := make([]Product, len(idStrs))
			for i, s := range idStrs {
				products[i] = Product{Title: "Game " + s}
			}
			json.NewEncoder(w).Encode(products)
		}))
		defer ts.Close()

		// 75 IDs should require 2 batches (50 + 25)
		ids := make([]int, 75)
		for i := range ids {
			ids[i] = i + 1
		}

		c := newTestClient(ts)
		products, err := c.GetProducts(ids)
		if err != nil {
			t.Fatalf("GetProducts: %v", err)
		}
		if requestCount != 2 {
			t.Errorf("made %d requests, want 2", requestCount)
		}
		if len(products) != 75 {
			t.Errorf("got %d products, want 75", len(products))
		}
	})
}

func TestGetProductDetails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("expand") != "description" {
			t.Error("missing expand=description query param")
		}
		json.NewEncoder(w).Encode(ProductDetails{
			ID:    42,
			Title: "Cool Game",
			Slug:  "cool-game",
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	details, err := c.GetProductDetails(42)
	if err != nil {
		t.Fatalf("GetProductDetails: %v", err)
	}
	if details.Title != "Cool Game" {
		t.Errorf("Title = %q, want %q", details.Title, "Cool Game")
	}
	if details.ID != 42 {
		t.Errorf("ID = %d, want 42", details.ID)
	}
}
