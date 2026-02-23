# Goggle CLI Design

A Go CLI app to authenticate with the GOG API, list owned games, and download them.

## Structure

```
goggle/
├── cmd/
│   ├── root.go        # Root cobra command, persistent flags
│   ├── login.go       # OAuth login flow
│   ├── list.go        # List owned games with promptui search/filter
│   └── download.go    # Select + download a game
├── pkg/gog/
│   ├── client.go      # HTTP client, token management, auth header injection
│   ├── auth.go        # OAuth flow (browser open, local HTTP server, token exchange/refresh)
│   ├── library.go     # List owned games, get game details
│   └── download.go    # Resolve download URLs, download files with progress
├── main.go
├── go.mod
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/manifoldco/promptui` — interactive prompts (select with search)

## Authentication

- `goggle login` opens browser to `https://auth.gog.com/auth?client_id=46899977096215655&redirect_uri=http://localhost:6969/callback&response_type=code&layout=client2`
- Local HTTP server on `localhost:6969` catches the redirect, extracts the `code` query param
- Exchanges code for tokens via `POST https://auth.gog.com/token` with `grant_type=authorization_code`, `client_id`, `client_secret`, `code`, `redirect_uri`
- Stores token response (`access_token`, `refresh_token`, `expires_in`, timestamp) in `~/.config/goggle/token.json` with `0600` permissions
- On every API call, the client checks expiry and auto-refreshes using `grant_type=refresh_token`

### OAuth Credentials

- Client ID: `46899977096215655`
- Client Secret: `9d85c43b1482497dbbce61f6e4aa173a433796eeae2ca8c5f6129f2dc4de46d9`

## List Games

- `goggle list` calls `GET https://embed.gog.com/user/data/games` (returns array of product IDs)
- Fetches details in batches of 50 via `GET https://api.gog.com/products?ids=...`
- Displays interactive promptui select list with search/filter by game title

## Download

- `goggle download` shows the same promptui game picker
- Once selected, calls `GET https://embed.gog.com/account/gameDetails/{id}.json` for download info
- Auto-detects OS (`darwin` -> mac, `linux` -> linux, default -> windows), allows `--os` flag override
- If multiple installers exist for that OS, shows a promptui picker
- Resolves actual download URL via the downlink endpoint
- Downloads to `~/GOG Games/{game_title}/` with a progress bar to stdout

## Token Storage

- Path: `~/.config/goggle/token.json`
- File permissions: `0600`
- Auto-refresh on expiry (tokens last ~3600s)
