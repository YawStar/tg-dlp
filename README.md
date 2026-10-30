<div id="top"></div>
<div align="center">

<img align="right" src="docs/assets/img/tg-dlp.png" width="250" alt="tg-dlp logo">

# tg-dlp

**A fast, secure, and cross-platform Telegram media downloader.**

<p>
  <code>tg-dlp</code> is a powerful Command-Line Interface (CLI) tool designed to download media, videos, photos, and messages from Telegram at lightning speed.
</p>
<p>
  Built with Go, it ensures high performance and reliability. Your Telegram sessions are protected locally using industry-standard AES-256-GCM encryption and Argon2id hashing. It works seamlessly across Windows, macOS, and Linux.
</p>

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey?style=flat-square)]()
[![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=flat-square&logo=docker&logoColor=white)]()

<br>
English | <a href="README_my.md">မြန်မာ</a>

</div>

---

## 📑 Table of Contents

- [✨ Features](#-features)
- [📸 Preview](#-preview)
- [📦 Installation](#-installation)
- [🚀 Quick Start](#-quick-start)
- [📖 Command Reference](#-command-reference)
- [⚙️ Global Options](#️-global-options)
- [🔍 Command-Specific Options](#-command-specific-options)
- [🔐 Security & Privacy](#-security--privacy)
- [🛠️ Development](#️-development)
- [📄 License](#-license)

---

## ✨ Features

- 🔐 **Military-grade Encryption**: Sessions are stored locally using AES-256-GCM and Argon2id.
- 📥 **Bulk Downloading**: Download entire channels, single posts, or specific forum threads.
- 📤 **Uploading**: Upload files or entire directories with include/exclude filters.
- 💾 **Backup & Restore**: Export channel messages to JSON, compress to ZIP, and restore them to any channel.
- 🔍 **Deep Info**: Inspect channel stats, post details, thread info, and your own account.
- 🗑️ **Purge & Delete**: Delete messages using links, or purge channels with text/media/extension filters.
- 🌐 **Proxy Support**: Native SOCKS5 and system environment proxy support.
- ⏸️ **Resume Support**: Automatically resumes incomplete downloads from `.part` files.
- 🚀 **Takeout Sessions**: Lower rate limits by utilizing Telegram's Takeout API.
- 🧪 **Dry-Run**: Preview what will be downloaded without actually downloading it.
- 🐛 **Debug Mode**: Detailed logging for troubleshooting.
- 🐳 **Docker Support**: Ready-to-use Dockerfile for containerized deployments.

---

## 📸 Preview

Here is how `tg-dlp` looks in action:

<div align="center">
  <img src="docs/assets/img/preview.gif" alt="tg-dlp Preview" width="800"/>
</div>

---

## 📦 Installation

### 1. Prebuilt Binaries (Recommended)

Download the latest compiled binary for your OS from the [Releases](https://github.com/YawStar/tg-dlp/releases) page.

### 2. Build from Source

Ensure you have **Go 1.22+** installed.

```bash
git clone https://github.com/YawStar/tg-dlp.git
cd tg-dlp
go build -o tg-dlp .
```

### 3. Docker

You can run `tg-dlp` inside a Docker container without installing Go.

```bash
# Build the image
docker build -t tg-dlp .

# Run the CLI (e.g., check version)
docker run --rm tg-dlp version

# Run a download (mount your local directory to save files)
docker run --rm -it \
  -v "$HOME/.tg-dlp:/home/tgdlp/.tg-dlp" \
  -v "$PWD/downloads:/home/tgdlp/downloads" \
  tg-dlp dl -u https://t.me/channel/123 -d /home/tgdlp/downloads
```

---

## 🚀 Quick Start

```bash
# 1. Show help menu
tg-dlp help

# 2. Login (First time only - prompts for API ID/Hash and phone)
tg-dlp login

# 3. Download a single post
tg-dlp dl -u https://t.me/channel/123

# 4. Download an entire channel (filtered and skip duplicates)
tg-dlp dl -u https://t.me/channel -i mp4,mkv --skip-same

# 5. Export a channel's messages to JSON (Limit 500 messages)
tg-dlp export -u https://t.me/channel/123 -l 500

# 6. Backup the exported files into a single ZIP
tg-dlp backup -d exports -o my_backup.zip

# 7. Restore the messages to Saved Messages (or a specific channel)
tg-dlp restore -f exports/export_123.json -c me

# 8. Upload a directory and delete local files after success
tg-dlp up -p ./videos -c @mychannel --rm

# 9. Purge text messages from a channel
tg-dlp purge -c @mychannel --text -y

# 10. Debugging an issue
tg-dlp dl -u https://t.me/channel/123 --debug
```

---

## 📖 Command Reference

| Command       | Aliases      | Description                                       |
| :------------ | :----------- | :------------------------------------------------ |
| `download`    | `dl`         | Download media from Telegram links.               |
| `upload`      | `up`         | Upload local files to Telegram.                   |
| `export`      | —            | Export channel messages to a JSON file.           |
| `backup`      | —            | Compress a folder (like exports) into a ZIP file. |
| `restore`     | —            | Restore messages from a JSON file to a chat.      |
| `info`        | —            | Show channel, post, or thread information.        |
| `whoami`      | —            | Show your account information.                    |
| `login`       | —            | Login or force re-login to Telegram.              |
| `logout`      | —            | Clear the cached session password.                |
| `clear-cache` | `clearcache` | Clear ONLY the cached password (keeps session).   |
| `delete`      | `del`, `rm`  | Delete a specific message using its URL.          |
| `purge`       | `clear`      | Delete multiple messages in a channel.            |
| `version`     | `-v`         | Show version information.                         |
| `help`        | `-h`         | Show the help menu.                               |

---

## ⚙️ Global Options

These flags work with **all** commands.

- `--proxy <url>`: Manual proxy URL (e.g., `socks5://127.0.0.1:1080` or `http://...`).
- `--no-proxy`: Ignore system proxy settings.
- `--skip-flood`: Skip waiting on Telegram's `FLOOD_WAIT` (Use with caution).
- `--debug`, `--verbose`: Show detailed debug logs (useful for troubleshooting).

---

## 🔍 Command-Specific Options

### `download` (`dl`)

- `-u, --url <link>`: Telegram link (repeatable).
- `-d, --dir <path>`: Output directory (Default: `downloads`).
- `-i, --filter <exts>`: File types to download (e.g., `mp4,mkv,mp3`).
- `-t, --template <str>`: Filename template (e.g., `{id}_{ext}` or `Video_{id}`).
- `--takeout`: Use Takeout Session to reduce rate limits.
- `--auto-join`: Automatically join private channels.
- `--skip-same`: Skip downloading if the file exists and matches size.
- `--overwrite`: Overwrite existing files.
- `--dry-run`: List what would be downloaded without downloading.

### `upload` (`up`)

- `-p, --path <path>`: File or directory to upload (repeatable).
- `-c, --chat <id/link>`: Target chat (Default: Saved Messages).
- `-i, --include <exts>`: Only upload specific extensions (e.g., `jpg,png`).
- `-e, --exclude <exts>`: Exclude specific extensions (e.g., `mp4,flv`).
- `--photo`: Force upload images as photos instead of documents.
- `--rm`: Delete local files after successful upload.

### `export`

- `-u, --url <link>`: Telegram channel link (Required).
- `-d, --dir <path>`: Output directory (Default: `exports`).
- `-l, --limit <num>`: Maximum messages to export (Default: `0` = all).

### `backup`

- `-d, --dir <path>`: Directory to backup (Default: `exports`).
- `-o, --out <name>`: Output ZIP file name (Default: `backup_YYYYMMDD.zip`).

### `restore`

- `-f, --file <path>`: JSON file to restore (Required).
- `-c, --chat <id/link/me>`: Target chat or `me` for Saved Messages.

### `delete` (`del`, `rm`)

- `-u, --url <link>`: Telegram message link.

### `purge` (`clear`)

- `-c, --chat <id/link>`: Target channel (Required).
- `-i, --include <exts>`: Only delete specific extensions.
- `-e, --exclude <exts>`: Exclude specific extensions.
- `--text`: Delete text messages only.
- `--media`: Delete media messages only.
- `-y, --yes`: Skip confirmation prompt.

---

## 🔐 Security & Privacy

Your Telegram session is highly sensitive. `tg-dlp` protects it using:

1. **AES-256-GCM**: Data is encrypted at rest.
2. **Argon2id**: The encryption key is derived from your password using a memory-hard hashing algorithm.
3. **In-Memory Cache**: You can choose how long to remember your password (Default: 30 minutes, up to "Until session expires").

> ⚠️ **WARNING:** Never share your `~/.tg-dlp/` directory. It contains your `session.enc` and `config.enc` files.
> The default directory is located at:
>
> - **Windows**: `C:\Users\<YourName>\.tg-dlp\`
> - **Linux/macOS**: `~/.tg-dlp/`

---

## 🛠️ Development

We use a standard Go toolchain and a Makefile for convenience.

```bash
make build       # Compile the binary into bin/
make test        # Run unit tests
make test-race   # Run tests with the race detector
make lint        # Run golangci-lint
make fmt         # Run gofmt
make help        # Show all available Make targets
```

### Project Architecture

```text
tg-dlp/
├── main.go               # Tiny entry point (calls cmd.Run)
├── cmd/                  # CLI flag parsing & command routing
├── app/
│   ├── info/             # Presentation layer for info commands
│   └── backup/           # Export, Backup, and Restore logic
├── core/
│   ├── client/           # Telegram client initialization, proxy, auth
│   ├── downloader/       # Download engine, bulk chunking, resume logic
│   ├── session/          # AES-GCM encryption & password cache
│   └── uploader/         # File walking & upload logic
└── pkg/
    ├── fsutil/           # Filename sanitization, unique paths
    ├── progress/         # Custom terminal progress bar
    ├── runtime/          # Global version, flags, debug state
    └── telegram/         # Link parsing, file extension mapping, API helpers
```

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgements

- [gotd/td](https://github.com/gotd/td) - The backbone Telegram MTProto library.
- [iyear/tdl](https://github.com/iyear/tdl) - For architectural inspiration.

---

<div align="center">

**[⬆ Back to Top](#top)**

Made with 💚 by YawHackka

</div>
