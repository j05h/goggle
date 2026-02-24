package gog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	ClientID     = "46899977096215655"
	ClientSecret = "9d85c43b1482497dbbce61f6e4aa173a433796eeae2ca8c5f6129f2dc4de46d9"
	RedirectURI  = "https://embed.gog.com/on_login_success?origin=client"
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
	HTTPClient   *http.Client
	Token        *Token
	EmbedBaseURL string // default: "https://embed.gog.com"
	APIBaseURL   string // default: "https://api.gog.com"
	TokenURL     string // default: TokenURL constant
	TokenPath    string // default: ~/.config/goggle/token.json
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
	return SaveTokenTo(t, p)
}

func SaveTokenTo(t *Token, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func LoadTokenFrom(path string) (*Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (c *Client) embedBaseURL() string {
	if c.EmbedBaseURL != "" {
		return c.EmbedBaseURL
	}
	return "https://embed.gog.com"
}

func (c *Client) apiBaseURL() string {
	if c.APIBaseURL != "" {
		return c.APIBaseURL
	}
	return "https://api.gog.com"
}

func (c *Client) tokenURL() string {
	if c.TokenURL != "" {
		return c.TokenURL
	}
	return TokenURL
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
	resp, err := c.HTTPClient.PostForm(c.tokenURL(), data)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
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
	if c.TokenPath != "" {
		return SaveTokenTo(&t, c.TokenPath)
	}
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
