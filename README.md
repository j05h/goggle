# goggle

A CLI tool to browse and download games from your GOG.com library. Built because GOG Galaxy doesn't always work.

## Install

Requires Go 1.24+.

```bash
go install github.com/josh/goggle@latest
```

Or build from source:

```bash
git clone https://github.com/j05h/goggle.git
cd goggle
go build -o goggle .
```

## Usage

### Login

Authenticate with your GOG account. This opens a Chromium window for you to sign in:

```bash
goggle login
```

Your token is saved to `~/.config/goggle/token.json` and auto-refreshes on expiry.

### List games

Browse your library with an interactive searchable list. Selecting a game shows its metadata:

```bash
goggle list
```

Type to search/filter by title.

### Download a game

Pick a game from your library and download it to `~/Downloads/`:

```bash
goggle download
```

By default it downloads installers for your current OS. Override with `--os`:

```bash
goggle download --os windows
goggle download --os mac
goggle download --os linux
```

On macOS, if the download is a `.pkg` file, you'll be prompted to install it.

## Development

### Project structure

```
goggle/
├── cmd/
│   ├── root.go          # Cobra root command
│   ├── login.go         # OAuth login command
│   ├── list.go          # Library browser with metadata display
│   └── download.go      # Game downloader with install prompt
├── pkg/gog/
│   ├── client.go        # HTTP client, token storage, auth header injection
│   ├── auth.go          # OAuth flow via go-rod (browser automation)
│   ├── library.go       # Library listing, product details
│   └── download.go      # Download URL resolution, file download with progress
├── main.go
└── go.mod
```

### Dependencies

- [cobra](https://github.com/spf13/cobra) - CLI framework
- [promptui](https://github.com/manifoldco/promptui) - Interactive terminal prompts
- [go-rod](https://github.com/go-rod/rod) - Browser automation for OAuth (uses Chromium)

### Building

```bash
go build -o goggle .
```

### GOG API

This tool uses the undocumented GOG API. Key endpoints:

- `auth.gog.com/auth` - OAuth authorization
- `auth.gog.com/token` - Token exchange/refresh
- `embed.gog.com/user/data/games` - List owned game IDs
- `api.gog.com/products?ids=...` - Batch product info
- `api.gog.com/products/{id}?expand=description` - Product details
- `embed.gog.com/account/gameDetails/{id}.json` - Download info
- `embed.gog.com/downlink/...` - Download URL resolution

API docs: https://gogapidocs.readthedocs.io/en/latest/
