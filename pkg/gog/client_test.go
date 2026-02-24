package gog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenExpired(t *testing.T) {
	tests := []struct {
		name    string
		savedAt time.Time
		expires int
		wantExp bool
	}{
		{
			name:    "expired token",
			savedAt: time.Now().Add(-2 * time.Hour),
			expires: 3600,
			wantExp: true,
		},
		{
			name:    "fresh token",
			savedAt: time.Now(),
			expires: 3600,
			wantExp: false,
		},
		{
			name:    "exactly at expiry",
			savedAt: time.Now().Add(-3600 * time.Second),
			expires: 3600,
			wantExp: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := &Token{
				AccessToken:  "test",
				RefreshToken: "test",
				ExpiresIn:    tt.expires,
				SavedAt:      tt.savedAt,
			}
			if got := tok.Expired(); got != tt.wantExp {
				t.Errorf("Expired() = %v, want %v", got, tt.wantExp)
			}
		})
	}
}

func TestSaveAndLoadToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	original := &Token{
		AccessToken:  "acc123",
		RefreshToken: "ref456",
		ExpiresIn:    3600,
		SavedAt:      time.Now().Truncate(time.Second),
	}

	if err := SaveTokenTo(original, path); err != nil {
		t.Fatalf("SaveTokenTo: %v", err)
	}

	loaded, err := LoadTokenFrom(path)
	if err != nil {
		t.Fatalf("LoadTokenFrom: %v", err)
	}

	if loaded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, original.AccessToken)
	}
	if loaded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, original.RefreshToken)
	}
	if loaded.ExpiresIn != original.ExpiresIn {
		t.Errorf("ExpiresIn = %d, want %d", loaded.ExpiresIn, original.ExpiresIn)
	}
	if !loaded.SavedAt.Equal(original.SavedAt) {
		t.Errorf("SavedAt = %v, want %v", loaded.SavedAt, original.SavedAt)
	}
}

func TestRefreshAuth(t *testing.T) {
	var gotParams map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		gotParams = map[string]string{
			"client_id":     r.FormValue("client_id"),
			"client_secret": r.FormValue("client_secret"),
			"grant_type":    r.FormValue("grant_type"),
			"refresh_token": r.FormValue("refresh_token"),
		}
		_ = json.NewEncoder(w).Encode(Token{
			AccessToken:  "new_access",
			RefreshToken: "new_refresh",
			ExpiresIn:    3600,
		})
	}))
	defer ts.Close()

	tokenPath := filepath.Join(t.TempDir(), "token.json")
	c := &Client{
		HTTPClient: ts.Client(),
		Token: &Token{
			AccessToken:  "old_access",
			RefreshToken: "old_refresh",
			ExpiresIn:    3600,
			SavedAt:      time.Now().Add(-2 * time.Hour),
		},
		TokenURL:  ts.URL,
		TokenPath: tokenPath,
	}

	if err := c.RefreshAuth(); err != nil {
		t.Fatalf("RefreshAuth: %v", err)
	}

	if gotParams["client_id"] != ClientID {
		t.Errorf("client_id = %q, want %q", gotParams["client_id"], ClientID)
	}
	if gotParams["grant_type"] != "refresh_token" {
		t.Errorf("grant_type = %q, want %q", gotParams["grant_type"], "refresh_token")
	}
	if gotParams["refresh_token"] != "old_refresh" {
		t.Errorf("refresh_token = %q, want %q", gotParams["refresh_token"], "old_refresh")
	}
	if c.Token.AccessToken != "new_access" {
		t.Errorf("Token.AccessToken = %q, want %q", c.Token.AccessToken, "new_access")
	}

	// Verify token was persisted
	saved, err := LoadTokenFrom(tokenPath)
	if err != nil {
		t.Fatalf("LoadTokenFrom: %v", err)
	}
	if saved.AccessToken != "new_access" {
		t.Errorf("saved AccessToken = %q, want %q", saved.AccessToken, "new_access")
	}
}

func TestAuthGet(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := &Client{
		HTTPClient: ts.Client(),
		Token: &Token{
			AccessToken:  "my_token",
			RefreshToken: "ref",
			ExpiresIn:    3600,
			SavedAt:      time.Now(),
		},
	}

	resp, err := c.AuthGet(ts.URL + "/test")
	if err != nil {
		t.Fatalf("AuthGet: %v", err)
	}
	resp.Body.Close()

	want := "Bearer my_token"
	if gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}
