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

	// The CDN URL from ResolveDownloadURL is a direct download, no auth needed
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Extract filename from URL path
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
