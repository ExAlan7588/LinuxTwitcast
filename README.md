# LinuxTwitcast

[Traditional Chinese README](README_ZH.md)

LinuxTwitcast records TwitCasting streams on a schedule and adds a built-in web console for headless Ubuntu / VPS deployments.

The web console is now English-first by default and lets you manage:

- recorder start / stop / restart
- streamer schedules
- Discord and Telegram settings
- live logs
- file browsing, download, delete, and Telegram upload
- current build version display
- update checks against `origin/main`

## Project Lineage

This repository is a modified fork of the original open-source project:

- Original upstream: https://github.com/jzhang046/croned-twitcasting-recorder

For Telegram users who want to point `telegram.json` at a local Bot API server instead of `https://api.telegram.org`, see:

- Telegram Bot API server: https://github.com/tdlib/telegram-bot-api

## Platform Support

This fork is **Linux-first and Linux-supported**.

- Supported and documented: Linux, especially Ubuntu VPS deployments
- Best-effort only: Windows
- Not supported and not documented: macOS

Important clarification for beginners:

- Linux is the only platform this fork actively targets and validates.
- Windows instructions below are included as an unofficial convenience path.
- That does **not** mean Windows receives the same maintenance, testing, or deployment coverage as Linux.
- If you want the supported path, use Ubuntu `24.04 LTS`.

## Runtime Target

This fork is maintained for Linux, especially Ubuntu VPS deployments.

Recommended baseline:

- Ubuntu `24.04 LTS`
- `go`
- `ffmpeg`
- a process supervisor such as `pm2` or `systemd`

## Install

```bash
git clone https://github.com/ExAlan7588/LinuxTwitcast.git
cd LinuxTwitcast
go build -o twitcast_bot .
```

On Ubuntu:

```bash
sudo apt update
sudo apt install -y golang-go ffmpeg
```

More deployment notes: [docs/ubuntu-vps.md](docs/ubuntu-vps.md)

## Beginner Quick Start

If you are new to Linux or self-hosting, use this order and do not skip steps:

1. Install Ubuntu packages.
2. Clone this repository.
3. Build `twitcast_bot`.
4. Start the web console with a username and password.
5. Open the web console in your browser or through your reverse proxy.
6. Fill in `config.json`, `discord.json`, and `telegram.json` from the web console.
7. Save settings.
8. Start the recorder.

Minimal first-time setup:

```bash
sudo apt update
sudo apt install -y golang-go ffmpeg git
git clone https://github.com/ExAlan7588/LinuxTwitcast.git
cd LinuxTwitcast
go build -o twitcast_bot .
TWITCAST_WEB_USERNAME=admin \
TWITCAST_WEB_PASSWORD='change-this-now' \
./twitcast_bot web --addr 127.0.0.1:8080 --auto-start
```

Then open:

```text
http://127.0.0.1:8080
```

If you are on a remote VPS, expose it through your reverse proxy or SSH tunnel first. Do not bind a public address without authentication unless you already protect it elsewhere.

## Windows Quick Start (Best-Effort Only)

If you want to run this project on Windows, use this section as an unofficial local-use guide.

Recommended assumptions:

- Windows 10 or Windows 11
- PowerShell
- Go installed
- `ffmpeg` installed and added to `PATH` if you plan to use Telegram conversion

Install and build in PowerShell:

```powershell
git clone https://github.com/ExAlan7588/LinuxTwitcast.git
cd LinuxTwitcast
go build -o twitcast_bot.exe .
```

Start the default scheduled recorder:

```powershell
.\twitcast_bot.exe
```

Start the web console:

```powershell
.\twitcast_bot.exe web --addr 127.0.0.1:8080
```

Start the web console with built-in auth:

```powershell
$env:TWITCAST_WEB_USERNAME = "admin"
$env:TWITCAST_WEB_PASSWORD = "change-this-now"
.\twitcast_bot.exe web --addr 127.0.0.1:8080 --auto-start
```

Then open:

```text
http://127.0.0.1:8080
```

Important Windows caveats:

- This path is not the primary maintained target of the fork.
- VPS deployment instructions in this repository are Linux-specific.
- If something behaves differently on Windows, prefer Linux before reporting it as a Linux deployment issue.

## Start Modes

Default scheduled recorder mode:

```bash
./twitcast_bot
```

Explicit scheduled mode:

```bash
./twitcast_bot croned
```

Direct one-stream mode:

```bash
./twitcast_bot direct --streamer=azusa_shirokyan --retries=10 --retry-backoff=1m
```

Web console mode:

```bash
./twitcast_bot web --addr 127.0.0.1:8080
```

Web console mode with built-in auth:

```bash
TWITCAST_WEB_USERNAME=admin \
TWITCAST_WEB_PASSWORD='change-this-now' \
./twitcast_bot web --addr 127.0.0.1:8080 --auto-start
```

`--addr` is the correct flag for web mode.

## Configuration Files

The recorder reads these files from the working directory:

- `config.json`
- `discord.json`
- `telegram.json`

The web console edits the same files, so browser changes and CLI runs stay in sync.

## Discord Bot Setup

LinuxTwitcast uses Discord in two separate ways:

1. Send or edit live notification messages in channels.
2. Optionally register message context-menu commands and assign per-streamer roles for subscribe / unsubscribe flows.

### Required OAuth2 Scopes

When inviting the bot, include:

- `bot`
- `applications.commands`

If you are a beginner, start with those two scopes only. Do not add extra scopes unless you know you need them.

### Required Channel Permissions

For the notify channel, and also the archive channel if you use it, the bot should be allowed to:

- `View Channels`
- `Send Messages`
- `Embed Links`
- `Read Message History`

If you only want basic live notifications, the four permissions above are the important starting point.

These are grounded in the current code path that creates, edits, archives, and removes notification messages:

- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:209>)
- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:228>)
- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:241>)

### Required Role Permissions

If you enable `tag_role` or want the context-menu subscribe / unsubscribe flow, the bot also needs:

- `Manage Roles`
- a bot role position above the per-streamer roles it creates or assigns

If you do **not** use `tag_role`, you can leave `Manage Roles` out.

Those operations are implemented here:

- [discord/commands.go](</o:/Cursor AI/LinuxTwitcast/discord/commands.go:106>)
- [discord/commands.go](</o:/Cursor AI/LinuxTwitcast/discord/commands.go:236>)
- [discord/commands.go](</o:/Cursor AI/LinuxTwitcast/discord/commands.go:259>)

### Guild ID Requirement

If you enable role tagging or context-menu role assignment, fill in `Guild ID` in the web console or `discord.json`.

### Privileged Intents

This project currently identifies to Discord Gateway with `intents: 0`, so the current interaction flow does not depend on privileged intents such as Message Content Intent:

- [discord/gateway.go](</o:/Cursor AI/LinuxTwitcast/discord/gateway.go:162>)

## Telegram Notes

If Telegram upload is enabled:

- `telegram.json` needs a valid `bot_token`
- `telegram.json` needs a valid `chat_id`
- `ffmpeg` must exist in `PATH` when `convert_to_m4a` is enabled

If `api_endpoint` points to `http://127.0.0.1:8081`, you must also run a local Telegram Bot API service on the VPS.
One supported local server implementation is `tdlib/telegram-bot-api`.

## Updating On A VPS

The web console is embedded into the `twitcast_bot` binary.
That means `git pull` alone is not enough. You must rebuild and restart the real running process.

### PM2 Example

If your VPS uses `pm2` and the process name is `twitcast-bot`:

```bash
cd /opt/LinuxTwitcast
git pull origin main
go build -o twitcast_bot .
pm2 restart twitcast-bot
pm2 logs twitcast-bot --lines 50
```

### Systemd Example

If your VPS uses a systemd service instead:

```bash
cd /opt/LinuxTwitcast
git pull origin main
go build -o twitcast_bot .
sudo systemctl restart twitcast-web.service
sudo systemctl status twitcast-web.service --no-pager -l
```

If you are unsure which supervisor is active, check first:

```bash
pm2 ls
sudo systemctl list-units --type=service --all | grep -i twitcast
ps -ef | grep twitcast_bot | grep -v grep
```

## Web Console Notes

- default UI language is `English`
- System Info currently shows `Version = null` during the testing stage
- the `Check for Updates` button compares the local build commit with `origin/main`
- if a newer commit exists and the repository remote can be mapped to a browser URL, the button redirects to the repository

## Output

Recordings are written to the working directory or the configured folder, using the pattern:

```text
screen-id-yyyyMMdd-HHmm.ts
```

Example:

```text
azusa_shirokyan-20060102-1504.ts
```
