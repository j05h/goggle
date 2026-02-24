package gog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseInstallers(t *testing.T) {
	t.Run("multi-OS multi-language", func(t *testing.T) {
		downloads := json.RawMessage(`[
			["English", {"windows": [{"manualUrl": "/dl/win", "name": "setup.exe", "size": "1 GB"}], "mac": [{"manualUrl": "/dl/mac", "name": "setup.dmg", "size": "900 MB"}]}],
			["French", {"windows": [{"manualUrl": "/dl/win_fr", "name": "setup_fr.exe", "size": "1 GB"}]}]
		]`)
		details := &GameDetails{Title: "Test Game", Downloads: downloads}

		installers, err := ParseInstallers(details)
		if err != nil {
			t.Fatalf("ParseInstallers: %v", err)
		}
		if len(installers) != 3 {
			t.Fatalf("got %d installers, want 3", len(installers))
		}

		// Check that OS and Language are filled in
		found := map[string]bool{}
		for _, inst := range installers {
			found[inst.OS+"/"+inst.Language] = true
			if inst.ManualURL == "" {
				t.Error("ManualURL should not be empty")
			}
		}
		if !found["windows/English"] {
			t.Error("missing windows/English installer")
		}
		if !found["mac/English"] {
			t.Error("missing mac/English installer")
		}
		if !found["windows/French"] {
			t.Error("missing windows/French installer")
		}
	})

	t.Run("empty downloads", func(t *testing.T) {
		details := &GameDetails{Title: "Empty", Downloads: json.RawMessage(`[]`)}
		installers, err := ParseInstallers(details)
		if err != nil {
			t.Fatalf("ParseInstallers: %v", err)
		}
		if len(installers) != 0 {
			t.Errorf("got %d installers, want 0", len(installers))
		}
	})

	t.Run("single language", func(t *testing.T) {
		downloads := json.RawMessage(`[
			["English", {"linux": [{"manualUrl": "/dl/linux", "name": "game.sh", "size": "500 MB"}]}]
		]`)
		details := &GameDetails{Title: "Linux Game", Downloads: downloads}

		installers, err := ParseInstallers(details)
		if err != nil {
			t.Fatalf("ParseInstallers: %v", err)
		}
		if len(installers) != 1 {
			t.Fatalf("got %d installers, want 1", len(installers))
		}
		if installers[0].OS != "linux" {
			t.Errorf("OS = %q, want %q", installers[0].OS, "linux")
		}
		if installers[0].Language != "English" {
			t.Errorf("Language = %q, want %q", installers[0].Language, "English")
		}
	})
}

func TestFilterInstallersByOS(t *testing.T) {
	installers := []Installer{
		{ManualURL: "/a", OS: "windows"},
		{ManualURL: "/b", OS: "mac"},
		{ManualURL: "/c", OS: "linux"},
		{ManualURL: "/d", OS: "windows"},
	}

	t.Run("matching OS", func(t *testing.T) {
		got := FilterInstallersByOS(installers, "windows")
		if len(got) != 2 {
			t.Errorf("got %d, want 2", len(got))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		got := FilterInstallersByOS(installers, "freebsd")
		if len(got) != 0 {
			t.Errorf("got %d, want 0", len(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := FilterInstallersByOS(nil, "windows")
		if len(got) != 0 {
			t.Errorf("got %d, want 0", len(got))
		}
	})
}

func TestDetectOS(t *testing.T) {
	os := DetectOS()
	switch os {
	case "mac", "linux", "windows":
		// ok
	default:
		t.Errorf("DetectOS() = %q, want one of mac/linux/windows", os)
	}
}

func TestResolveDownloadURL(t *testing.T) {
	t.Run("302 redirect", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://cdn.example.com/file.bin", http.StatusFound)
		}))
		defer ts.Close()

		c := &Client{
			HTTPClient:   ts.Client(),
			EmbedBaseURL: ts.URL,
			Token: &Token{
				AccessToken:  "tok",
				RefreshToken: "ref",
				ExpiresIn:    3600,
				SavedAt:      time.Now(),
			},
		}

		got, err := c.ResolveDownloadURL("/dl/installer")
		if err != nil {
			t.Fatalf("ResolveDownloadURL: %v", err)
		}
		want := "https://cdn.example.com/file.bin"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("200 JSON response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(DownlinkResponse{
				Downlink: "https://cdn.example.com/downlink.bin",
				Checksum: "abc123",
			})
		}))
		defer ts.Close()

		c := &Client{
			HTTPClient:   ts.Client(),
			EmbedBaseURL: ts.URL,
			Token: &Token{
				AccessToken:  "tok",
				RefreshToken: "ref",
				ExpiresIn:    3600,
				SavedAt:      time.Now(),
			},
		}

		got, err := c.ResolveDownloadURL("/dl/installer")
		if err != nil {
			t.Fatalf("ResolveDownloadURL: %v", err)
		}
		want := "https://cdn.example.com/downlink.bin"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
