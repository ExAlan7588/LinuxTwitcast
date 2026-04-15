# LinuxTwitcast

<div align="center"><a href="https://github.com/ExAlan7588/LinuxTwitcast/blob/main/README_ZH.md">繁體中文 README</a></div>

<p align="center">
  <img src="admin/assets/icon.svg" alt="LinuxTwitcast icon" width="112">
</p>

<p align="center"><strong>Headless Linux Control Room</strong></p>

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

By default, this fork talks to the official Telegram Bot API at `https://api.telegram.org`.
If you manually change the Telegram API endpoint to a local Bot API server, see:

- Telegram Bot API server: https://github.com/tdlib/telegram-bot-api
- [Why would I need this?](https://github.com/ExAlan7588/LinuxTwitcast/blob/main/README.md#telegram-notes)

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

## Before You Start

If you are a beginner, install these first:

- `git` so you can clone and pull updates
- `go` so you can build `twitcast_bot`
- `ffmpeg` if you want Telegram audio extraction

On Ubuntu:

```bash
sudo apt update
sudo apt install -y git golang-go ffmpeg
```

If you want the process to stay alive in the background and auto-restart on a VPS, also install:

- `nodejs`
- `npm`
- `pm2`

On Ubuntu:

```bash
sudo apt install -y nodejs npm
sudo npm install pm2@latest -g
```

## Install

```bash
git clone https://github.com/ExAlan7588/LinuxTwitcast.git
cd LinuxTwitcast
go build -o twitcast_bot .
```

More deployment notes: [docs/ubuntu-vps.md](docs/ubuntu-vps.md)

## Beginner Quick Start

If you are new to Linux or self-hosting, use this order and do not skip steps:

1. Install Ubuntu packages.
2. Clone this repository.
3. Build `twitcast_bot`.
4. Choose your own web console username and password.
5. Start the web console with that username and password.
6. Open the web console in your browser or through your reverse proxy.
7. Fill in the frontend sections:
   `General & Streamer Settings`, `Discord Notifications`, and `Telegram & Conversion`.
8. Click `Save Settings`.
9. Start the recorder.

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

`admin` and `change-this-now` are only examples.
You must replace them with your own username and password.

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

Again, those two values are examples only.
Use your own username and your own password.

Then open:

```text
http://127.0.0.1:8080
```

Important Windows caveats:

- This path is not the primary maintained target of the fork.
- VPS deployment instructions in this repository are Linux-specific.
- If something behaves differently on Windows, prefer Linux before reporting it as a Linux deployment issue.

## Start Modes

If this section feels confusing, use this rule:

- most beginners should start with the web console mode

The most common command is:

```bash
TWITCAST_WEB_USERNAME=your-login-name \
TWITCAST_WEB_PASSWORD='your-own-password' \
./twitcast_bot web --addr 127.0.0.1:8080 --auto-start
```

Meaning:

- `TWITCAST_WEB_USERNAME`: the username you choose for the frontend login
- `TWITCAST_WEB_PASSWORD`: the password you choose for the frontend login
- `web`: start the frontend
- `--addr 127.0.0.1:8080`: listen on local port `8080`
- `--auto-start`: start the recorder automatically when the frontend boots

### 1. Most common: web console mode

```bash
TWITCAST_WEB_USERNAME=your-login-name \
TWITCAST_WEB_PASSWORD='your-own-password' \
./twitcast_bot web --addr 127.0.0.1:8080 --auto-start
```

Use this when you want to manage everything from the frontend.

### 2. Frontend only, recorder not auto-started

```bash
./twitcast_bot web --addr 127.0.0.1:8080
```

Use this when you want to open the frontend first, save settings, and then click `Start Recorder` manually.

### 3. Scheduled recorder only, no frontend

```bash
./twitcast_bot
```

or:

```bash
./twitcast_bot croned
```

For normal use, those are the same kind of mode:

- no frontend
- only scheduled recording

### 4. One-stream test mode

```bash
./twitcast_bot direct --streamer=azusa_shirokyan --retries=10 --retry-backoff=1m
```

Use this only when you want to test a single streamer directly.

`--addr` is the correct flag for web mode.

## Configuration Files

The recorder reads these files from the working directory:

- `config.json`
- `discord.json`
- `telegram.json`

The web console edits the same files, so browser changes and CLI runs stay in sync.

Beginner note:

- If you use the frontend, you usually do **not** need to edit these files by hand.
- Fill in the sections in the web page and click `Save Settings`.
- The program will write those values into the files for you.

## Discord Bot Setup

LinuxTwitcast uses Discord in two separate ways:

1. Send or edit live notification messages in channels.
2. Optionally register message context-menu commands and assign per-streamer roles for subscribe / unsubscribe flows.

### Discord Invite Checkboxes (Scopes)

If the word `Scopes` is confusing, treat it as:

- the checkboxes you tick on the Discord bot invite page

For this project, tick:

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

Default behavior:

- you do **not** need a local Telegram Bot API server
- the default API endpoint is `https://api.telegram.org`
- only switch to a local Bot API server if you intentionally change the API endpoint

### Why some users self-host Telegram Bot API

Telegram's official local Bot API server documentation currently states that local mode allows:

- unlimited file downloads
- uploads up to `2000 MB`
- HTTP webhook usage
- arbitrary local IPs and ports for webhooks

The same official README also says the local server listens on port `8081` by default and requires `--api-id` and `--api-hash`.

That is why some users self-host it:

- they need much larger file uploads
- they want the Bot API server inside the same VPS or local network
- they want local HTTP instead of public HTTPS restrictions

Sources:

- https://github.com/tdlib/telegram-bot-api
- https://tdlib.github.io/telegram-bot-api/build.html

### Why this project does not require self-hosting by default

For most beginners:

- `https://api.telegram.org` is enough to get started
- self-hosting adds more installation and maintenance work
- you usually only need self-hosting when you hit file-size or infrastructure limits

If Telegram upload is enabled:

- `telegram.json` needs a valid `bot_token`
- `telegram.json` needs a valid `chat_id`
- `ffmpeg` must exist in `PATH` when `convert_to_m4a` is enabled

If `api_endpoint` points to `http://127.0.0.1:8081`, you must also run a local Telegram Bot API service on the VPS.
One supported local server implementation is `tdlib/telegram-bot-api`.

### Quick self-host overview

The official `tdlib/telegram-bot-api` README shows a build flow like:

```bash
git clone --recursive https://github.com/tdlib/telegram-bot-api.git
cd telegram-bot-api
mkdir build
cd build
cmake -DCMAKE_BUILD_TYPE=Release ..
cmake --build . --target install
```

The same official README says the minimum required startup parameters are:

```bash
telegram-bot-api --api-id <your_api_id> --api-hash <your_api_hash> --local
```

Important beginner notes:

1. `api_id` and `api_hash` are not the same as a Bot Token.
2. The local Bot API server uses HTTP and normally listens on `8081`.
3. Telegram's official README says you should call `logOut` before moving a bot from `https://api.telegram.org` to a local server.
4. If you expose the local server remotely, you must handle reverse proxy and TLS yourself.

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
