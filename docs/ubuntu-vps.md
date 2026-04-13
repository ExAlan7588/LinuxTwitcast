# Ubuntu VPS Deployment

This project now includes a built-in web console so you can manage streamers, notifications, logs, and recording files without any desktop GUI.

## Target OS

Use **Ubuntu 24.04 LTS**.
Ubuntu `24.02` is not an official LTS release number.

## Packages

```bash
sudo apt update
sudo apt install -y golang-go ffmpeg
```

If you prefer a newer Go release than the Ubuntu package, install it from the official Go tarball instead.

## Build

```bash
cd /opt/LinuxTwitcast
go build -o twitcast_bot .
```

## Start The Web Console

Local-only access:

```bash
./twitcast_bot web --addr 127.0.0.1:8080
```

Public access with built-in auth:

```bash
TWITCAST_WEB_USERNAME=admin \
TWITCAST_WEB_PASSWORD='change-this-now' \
./twitcast_bot web --addr 0.0.0.0:8080 --auto-start
```

If you already protect the app behind Nginx, Caddy, Tailscale, or an SSH tunnel, you can keep the web console on `127.0.0.1`.

## What The Web Console Can Do

- Edit `config.json`, `discord.json`, and `telegram.json`
- Start, stop, and restart the recorder
- View recent logs captured from the Go process
- Browse recording folders and the project workspace
- Download files
- Delete files or empty directories

## Important Runtime Notes

- If Telegram audio extraction is enabled, `ffmpeg` must exist in `PATH`.
- If `telegram.json` uses `http://127.0.0.1:8081`, you must also run a local Telegram Bot API service on the VPS. Otherwise switch it back to `https://api.telegram.org`.
- `discord.json` and `telegram.json` contain secrets. On Linux, keep them readable only by the service account.

## Systemd

An example unit file is available at `deploy/twitcast-web.service.example`.

Recommended workflow:

1. Copy the example unit file to `/etc/systemd/system/twitcast-web.service`
2. Adjust `WorkingDirectory`, `ExecStart`, and credentials
3. Reload systemd and enable the service

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now twitcast-web.service
sudo systemctl status twitcast-web.service
```
