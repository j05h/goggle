# Goggle CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI to authenticate with GOG, list owned games, and download them.

**Architecture:** Cobra CLI with three commands (login, list, download). A `pkg/gog` package handles all API interaction — auth, library, downloads. Token persisted to `~/.config/goggle/token.json`, auto-refreshed on expiry.

**Tech Stack:** Go, cobra, promptui, standard library net/http

---

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`

**Step 1: Initialize Go module and install dependencies**

Run:
```bash
cd /Users/josh/Work/goggle
go mod init github.com/josh/goggle
go get github.com/spf13/cobra@latest
go get github.com/manifoldco/promptui@latest
```

**Step 2: Create main.go**

```go
package main

import "github.com/josh/goggle/cmd"

func main() {
	cmd.Execute()
}
```

**Step 3: Create cmd/root.go**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "goggle",
	Short: "Download games from your GOG library",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 4: Verify it builds**

Run: `go build -o goggle .`
Expected: Binary created, no errors.

**Step 5: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go
git commit -m "feat: scaffold project with cobra root command"
```

---

### Task 2: Token storage and HTTP client

**Files:**
- Create: `pkg/gog/client.go`

**Step 1: Create pkg/gog/client.go**

This file defines `Token`, `Client`, token load/save, and an authenticated HTTP request helper that auto-refreshes expired tokens.

```go
package gog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ClientID     = "46899977096215655"
	ClientSecret = "9d85c43b1482497dbbce61f6e4aa173a433796eeae2ca8c5f6129f2dc4de46d9"
	RedirectURI  = "http://localhost:6969/callback"
	AuthURL      = "https://auth.gog.com/auth"
	TokenURL     = "https://auth.gog.com/token"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	SavedAt      time.Time `json:"saved_at"`
}

func (t *Token) Expired() bool {
	return time.Now().After(t.SavedAt.Add(time.Duration(t.ExpiresIn) * time.Second))
}

type Client struct {
	HTTPClient *http.Client
	Token      *Token
}

func tokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "goggle", "token.json"), nil
}

func LoadToken() (*Token, error) {
	p, err := tokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func SaveToken(t *Token) error {
	p, err := tokenPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

func NewClient() (*Client, error) {
	token, err := LoadToken()
	if err != nil {
		return nil, fmt.Errorf("not logged in — run 'goggle login' first: %w", err)
	}
	c := &Client{
		HTTPClient: &http.Client{},
		Token:      token,
	}
	if token.Expired() {
		if err := c.RefreshAuth(); err != nil {
			return nil, fmt.Errorf("token refresh failed — run 'goggle login': %w", err)
		}
	}
	return c, nil
}

func (c *Client) RefreshAuth() error {
	data := url.Values{
		"client_id":     {ClientID},
		"client_secret": {ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.Token.RefreshToken},
	}
	resp, err := c.HTTPClient.PostForm(TokenURL, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, body)
	}
	var t Token
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return err
	}
	t.SavedAt = time.Now()
	c.Token = &t
	return SaveToken(&t)
}

// AuthGet makes an authenticated GET request. Host should include scheme.
func (c *Client) AuthGet(rawURL string) (*http.Response, error) {
	if c.Token.Expired() {
		if err := c.RefreshAuth(); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token.AccessToken)
	return c.HTTPClient.Do(req)
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

**Step 3: Commit**

```bash
git add pkg/gog/client.go
git commit -m "feat: add GOG API client with token storage and auto-refresh"
```

---

### Task 3: OAuth login command

**Files:**
- Create: `pkg/gog/auth.go`
- Create: `cmd/login.go`

**Step 1: Create pkg/gog/auth.go**

Handles the browser-based OAuth flow: opens browser, runs local HTTP server to catch callback, exchanges code for token.

```go
package gog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	default:
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func Login() error {
	authURL := fmt.Sprintf(
		"%s?client_id=%s&redirect_uri=%s&response_type=code&layout=client2",
		AuthURL, ClientID, url.QueryEscape(RedirectURI),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback: %s", r.URL.String())
			fmt.Fprintln(w, "Error: no code received. Check the CLI.")
			return
		}
		codeCh <- code
		fmt.Fprintln(w, "Login successful! You can close this tab.")
	})

	listener, err := net.Listen("tcp", ":6969")
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	fmt.Println("Opening browser for GOG login...")
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Could not open browser. Visit this URL manually:\n%s\n", authURL)
	}

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		server.Shutdown(context.Background())
		return err
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return fmt.Errorf("login timed out after 5 minutes")
	}

	server.Shutdown(context.Background())

	return exchangeCode(code)
}

func exchangeCode(code string) error {
	data := url.Values{
		"client_id":     {ClientID},
		"client_secret": {ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {RedirectURI},
	}

	resp, err := http.PostForm(TokenURL, data)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var t Token
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}
	t.SavedAt = time.Now()

	if err := SaveToken(&t); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("Login successful! Token saved.")
	return nil
}
```

**Step 2: Create cmd/login.go**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/josh/goggle/pkg/gog"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with GOG",
	RunE: func(cmd *cobra.Command, args []string) error {
		return gog.Login()
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
```

**Step 3: Build and smoke test**

Run: `go build -o goggle . && ./goggle login --help`
Expected: Shows help for the login command.

**Step 4: Commit**

```bash
git add pkg/gog/auth.go cmd/login.go
git commit -m "feat: add OAuth login command with browser callback flow"
```

---

### Task 4: Library listing

**Files:**
- Create: `pkg/gog/library.go`
- Create: `cmd/list.go`

**Step 1: Create pkg/gog/library.go**

Fetches owned game IDs, then batch-fetches product details.

```go
package gog

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type OwnedGamesResponse struct {
	Owned []int `json:"owned"`
}

type Product struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func (c *Client) GetOwnedGameIDs() ([]int, error) {
	resp, err := c.AuthGet("https://embed.gog.com/user/data/games")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get owned games (%d): %s", resp.StatusCode, body)
	}

	var result OwnedGamesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Owned, nil
}

func (c *Client) GetProducts(ids []int) ([]Product, error) {
	var all []Product
	// Batch in groups of 50
	for i := 0; i < len(ids); i += 50 {
		end := i + 50
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		strs := make([]string, len(batch))
		for j, id := range batch {
			strs[j] = fmt.Sprintf("%d", id)
		}
		url := "https://api.gog.com/products?ids=" + strings.Join(strs, ",")

		resp, err := c.AuthGet(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to get products (%d): %s", resp.StatusCode, body)
		}

		var products []Product
		if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
			return nil, err
		}
		all = append(all, products...)
	}
	return all, nil
}
```

**Step 2: Create cmd/list.go**

```go
package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/josh/goggle/pkg/gog"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List your GOG library",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gog.NewClient()
		if err != nil {
			return err
		}

		fmt.Println("Fetching library...")
		ids, err := client.GetOwnedGameIDs()
		if err != nil {
			return err
		}
		fmt.Printf("Found %d games. Fetching details...\n", len(ids))

		products, err := client.GetProducts(ids)
		if err != nil {
			return err
		}

		sort.Slice(products, func(i, j int) bool {
			return products[i].Title < products[j].Title
		})

		templates := &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▸ {{ .Title | cyan }}",
			Inactive: "  {{ .Title }}",
			Selected: "✔ {{ .Title | green }}",
		}

		searcher := func(input string, index int) bool {
			product := products[index]
			return strings.Contains(
				strings.ToLower(product.Title),
				strings.ToLower(input),
			)
		}

		prompt := promptui.Select{
			Label:     "Your GOG Library",
			Items:     products,
			Templates: templates,
			Size:      20,
			Searcher:  searcher,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return err
		}

		selected := products[idx]
		fmt.Printf("Selected: %s (ID: %d)\n", selected.Title, selected.ID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
```

Note: add `"strings"` to the imports in cmd/list.go.

**Step 3: Build and verify**

Run: `go build -o goggle .`
Expected: Compiles successfully.

**Step 4: Commit**

```bash
git add pkg/gog/library.go cmd/list.go
git commit -m "feat: add list command with interactive game picker"
```

---

### Task 5: Download command

**Files:**
- Create: `pkg/gog/download.go`
- Create: `cmd/download.go`

**Step 1: Create pkg/gog/download.go**

Fetches game details, resolves download URLs, and downloads with progress.

```go
package gog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// gameDetails response has a unusual downloads structure:
// "downloads": [ ["English", {"windows": [...], "mac": [...]}], ... ]
// We need custom unmarshalling.

type GameDetails struct {
	Title     string          `json:"title"`
	Downloads json.RawMessage `json:"downloads"`
}

type Installer struct {
	ManualURL string `json:"manualUrl"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Size      string `json:"size"`
	OS        string // filled in during parsing
	Language  string // filled in during parsing
}

type DownlinkResponse struct {
	Downlink string `json:"downlink"`
	Checksum string `json:"checksum"`
}

func (c *Client) GetGameDetails(id int) (*GameDetails, error) {
	url := fmt.Sprintf("https://embed.gog.com/account/gameDetails/%d.json", id)
	resp, err := c.AuthGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get game details (%d): %s", resp.StatusCode, body)
	}

	var details GameDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}
	return &details, nil
}

func ParseInstallers(details *GameDetails) ([]Installer, error) {
	// downloads is an array of [language_string, {os: [installer, ...]}] pairs
	var raw []json.RawMessage
	if err := json.Unmarshal(details.Downloads, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse downloads: %w", err)
	}

	var installers []Installer
	for _, entry := range raw {
		// Each entry is a 2-element array: ["English", {"windows": [...], ...}]
		var pair []json.RawMessage
		if err := json.Unmarshal(entry, &pair); err != nil {
			return nil, err
		}
		if len(pair) != 2 {
			continue
		}

		var language string
		if err := json.Unmarshal(pair[0], &language); err != nil {
			return nil, err
		}

		var osSets map[string][]Installer
		if err := json.Unmarshal(pair[1], &osSets); err != nil {
			return nil, err
		}

		for osName, osInstallers := range osSets {
			for _, inst := range osInstallers {
				inst.OS = osName
				inst.Language = language
				installers = append(installers, inst)
			}
		}
	}
	return installers, nil
}

func DetectOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	default:
		return "windows"
	}
}

func FilterInstallersByOS(installers []Installer, targetOS string) []Installer {
	var filtered []Installer
	for _, inst := range installers {
		if inst.OS == targetOS {
			filtered = append(filtered, inst)
		}
	}
	return filtered
}

func (c *Client) ResolveDownloadURL(manualURL string) (string, error) {
	url := "https://embed.gog.com" + manualURL
	resp, err := c.AuthGet(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to resolve download URL (%d): %s", resp.StatusCode, body)
	}

	var dl DownlinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&dl); err != nil {
		return "", err
	}
	return dl.Downlink, nil
}

type ProgressWriter struct {
	Total      int64
	Downloaded int64
	Writer     io.Writer
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	pw.Downloaded += int64(n)
	if pw.Total > 0 {
		pct := float64(pw.Downloaded) / float64(pw.Total) * 100
		fmt.Fprintf(os.Stderr, "\r  %.1f%% (%d / %d bytes)", pct, pw.Downloaded, pw.Total)
	} else {
		fmt.Fprintf(os.Stderr, "\r  %d bytes downloaded", pw.Downloaded)
	}
	return n, err
}

func (c *Client) DownloadFile(downloadURL, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Follow redirects with auth — but the CDN URL is a direct download, no auth needed
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Extract filename from URL or Content-Disposition
	filename := filepath.Base(resp.Request.URL.Path)
	if filename == "" || filename == "." {
		filename = "download"
	}
	// Strip query params from filename
	if idx := strings.Index(filename, "?"); idx != -1 {
		filename = filename[:idx]
	}

	destPath := filepath.Join(destDir, filename)
	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	pw := &ProgressWriter{
		Total:  resp.ContentLength,
		Writer: f,
	}

	if _, err := io.Copy(pw, resp.Body); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	fmt.Fprintln(os.Stderr) // newline after progress

	return destPath, nil
}
```

**Step 2: Create cmd/download.go**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/josh/goggle/pkg/gog"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var downloadOS string

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a game from your GOG library",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gog.NewClient()
		if err != nil {
			return err
		}

		fmt.Println("Fetching library...")
		ids, err := client.GetOwnedGameIDs()
		if err != nil {
			return err
		}

		products, err := client.GetProducts(ids)
		if err != nil {
			return err
		}

		sort.Slice(products, func(i, j int) bool {
			return products[i].Title < products[j].Title
		})

		// Pick a game
		templates := &promptui.SelectTemplates{
			Active:   "▸ {{ .Title | cyan }}",
			Inactive: "  {{ .Title }}",
			Selected: "✔ {{ .Title | green }}",
		}
		searcher := func(input string, index int) bool {
			return strings.Contains(
				strings.ToLower(products[index].Title),
				strings.ToLower(input),
			)
		}
		gamePrompt := promptui.Select{
			Label:     "Select a game to download",
			Items:     products,
			Templates: templates,
			Size:      20,
			Searcher:  searcher,
		}
		idx, _, err := gamePrompt.Run()
		if err != nil {
			return err
		}
		selected := products[idx]

		fmt.Printf("Fetching details for %s...\n", selected.Title)
		details, err := client.GetGameDetails(selected.ID)
		if err != nil {
			return err
		}

		installers, err := gog.ParseInstallers(details)
		if err != nil {
			return err
		}

		targetOS := downloadOS
		if targetOS == "" {
			targetOS = gog.DetectOS()
		}

		filtered := gog.FilterInstallersByOS(installers, targetOS)
		if len(filtered) == 0 {
			return fmt.Errorf("no %s installers found for %s", targetOS, selected.Title)
		}

		// Pick installer if multiple
		var chosen gog.Installer
		if len(filtered) == 1 {
			chosen = filtered[0]
		} else {
			instTemplates := &promptui.SelectTemplates{
				Active:   "▸ {{ .Name | cyan }} ({{ .Size }}, {{ .Language }})",
				Inactive: "  {{ .Name }} ({{ .Size }}, {{ .Language }})",
				Selected: "✔ {{ .Name | green }}",
			}
			instPrompt := promptui.Select{
				Label:     "Select installer",
				Items:     filtered,
				Templates: instTemplates,
			}
			instIdx, _, err := instPrompt.Run()
			if err != nil {
				return err
			}
			chosen = filtered[instIdx]
		}

		fmt.Printf("Resolving download URL for %s...\n", chosen.Name)
		dlURL, err := client.ResolveDownloadURL(chosen.ManualURL)
		if err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		destDir := filepath.Join(home, "GOG Games", selected.Title)

		fmt.Printf("Downloading to %s...\n", destDir)
		path, err := client.DownloadFile(dlURL, destDir)
		if err != nil {
			return err
		}

		fmt.Printf("Done! Saved to %s\n", path)
		return nil
	},
}

func init() {
	downloadCmd.Flags().StringVar(&downloadOS, "os", "", "Target OS (windows, mac, linux). Defaults to current OS.")
	rootCmd.AddCommand(downloadCmd)
}
```

**Step 3: Build and verify**

Run: `go build -o goggle .`
Expected: Compiles successfully.

**Step 4: Commit**

```bash
git add pkg/gog/download.go cmd/download.go
git commit -m "feat: add download command with OS detection and progress bar"
```

---

### Task 6: Manual integration test

**Step 1: Build final binary**

Run: `go build -o goggle .`

**Step 2: Test login flow**

Run: `./goggle login`
Expected: Browser opens, GOG login page shown. After login, terminal shows "Login successful! Token saved."
Verify: `cat ~/.config/goggle/token.json` shows valid token JSON.

**Step 3: Test list command**

Run: `./goggle list`
Expected: Shows "Fetching library...", then interactive list of owned games with search.

**Step 4: Test download command**

Run: `./goggle download`
Expected: Shows game picker, then downloads selected game to `~/GOG Games/`.

**Step 5: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: integration test fixes"
```
