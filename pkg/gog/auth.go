package gog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func Login() error {
	authURL := fmt.Sprintf(
		"%s?client_id=%s&redirect_uri=%s&response_type=code&layout=client2",
		AuthURL, ClientID, url.QueryEscape(RedirectURI),
	)

	fmt.Println("Launching browser for GOG login...")

	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).Headless(false).MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(authURL)

	fmt.Println("Waiting for login (this will close automatically)...")

	// Wait for the redirect to on_login_success with a code parameter
	var code string
	err := rod.Try(func() {
		page.Timeout(5 * time.Minute).MustWaitNavigation()

		// Poll the URL until we see the success redirect with a code
		for {
			currentURL := page.MustEval(`() => window.location.href`).String()
			if strings.Contains(currentURL, "on_login_success") {
				parsed, err := url.Parse(currentURL)
				if err == nil {
					code = parsed.Query().Get("code")
				}
				if code != "" {
					return
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	if code == "" {
		return fmt.Errorf("no authorization code received")
	}

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
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
