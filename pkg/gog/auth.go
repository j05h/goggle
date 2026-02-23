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
