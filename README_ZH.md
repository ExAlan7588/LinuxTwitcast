# LinuxTwitcast

[English README](README.md)

LinuxTwitcast 是一個用來定時檢查 TwitCasting 開播狀態並自動錄影的工具，這個 fork 額外提供了適合 Ubuntu / VPS 的內建 Web 管理頁。

目前網站預設語系為英文，但仍保留繁中說明文件供管理者查閱。

Web 管理頁可處理：

- 啟動 / 停止 / 重啟 recorder
- 管理 streamer 排程
- 設定 Discord 與 Telegram
- 查看即時日誌
- 瀏覽、下載、刪除檔案，或上傳到 Telegram
- 顯示目前版本號
- 檢查 `origin/main` 是否有更新

## 建議執行環境

此 fork 目前以 Linux 為主要目標，特別是 Ubuntu VPS。

建議環境：

- Ubuntu `24.04 LTS`
- `go`
- `ffmpeg`
- `pm2` 或 `systemd` 之類的常駐程序管理工具

## 安裝

```bash
git clone https://github.com/ExAlan7588/LinuxTwitcast.git
cd LinuxTwitcast
go build -o twitcast_bot .
```

Ubuntu 可先安裝：

```bash
sudo apt update
sudo apt install -y golang-go ffmpeg
```

更多部署細節可看：[docs/ubuntu-vps.md](docs/ubuntu-vps.md)

## 啟動方式

預設排程錄影模式：

```bash
./twitcast_bot
```

明確指定排程模式：

```bash
./twitcast_bot croned
```

直接錄單一直播：

```bash
./twitcast_bot direct --streamer=azusa_shirokyan --retries=10 --retry-backoff=1m
```

啟動 Web 管理頁：

```bash
./twitcast_bot web --addr 127.0.0.1:8080
```

若要啟用內建驗證：

```bash
TWITCAST_WEB_USERNAME=admin \
TWITCAST_WEB_PASSWORD='change-this-now' \
./twitcast_bot web --addr 127.0.0.1:8080 --auto-start
```

Web mode 正確使用的旗標是 `--addr`。

## 設定檔

程式會讀取工作目錄中的：

- `config.json`
- `discord.json`
- `telegram.json`

Web 管理頁也是直接編輯這三個檔案，所以網頁與 CLI 會共用同一份設定。

## Discord Bot 權限說明

LinuxTwitcast 在 Discord 內主要做兩件事：

1. 發送、更新、刪除直播通知訊息
2. 需要時註冊右鍵選單指令，並用每個直播主對應的身分組做訂閱 / 取消訂閱

### 邀請 Bot 時需要的 OAuth2 Scopes

請至少勾選：

- `bot`
- `applications.commands`

### 頻道權限

在通知頻道，以及你有使用的歸檔頻道，建議 Bot 至少有：

- `View Channels`
- `Send Messages`
- `Embed Links`
- `Read Message History`

這些需求對應到目前的訊息建立、編輯、刪除流程：

- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:209>)
- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:228>)
- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:241>)

### 身分組權限

如果你要開啟 `tag_role`，或要使用右鍵選單的訂閱 / 取消訂閱功能，Bot 還需要：

- `Manage Roles`
- Bot 自己的身分組排序必須高於它要建立或指派的目標身分組

相關程式邏輯在：

- [discord/commands.go](</o:/Cursor AI/LinuxTwitcast/discord/commands.go:106>)
- [discord/commands.go](</o:/Cursor AI/LinuxTwitcast/discord/commands.go:236>)
- [discord/commands.go](</o:/Cursor AI/LinuxTwitcast/discord/commands.go:259>)

### Guild ID 是否必填

如果你有開 `tag_role` 或要讓使用者透過右鍵選單管理訂閱，`Guild ID` 必須填。

### 是否需要 Privileged Intents

目前專案連 Discord Gateway 時使用的是 `intents: 0`，也就是目前這套互動流程不依賴 Message Content Intent 之類的 privileged intents：

- [discord/gateway.go](</o:/Cursor AI/LinuxTwitcast/discord/gateway.go:162>)

## Telegram 補充

如果開啟 Telegram 上傳：

- `telegram.json` 必須有有效的 `bot_token`
- `telegram.json` 必須有有效的 `chat_id`
- 若開啟 `convert_to_m4a`，系統 `PATH` 中必須找得到 `ffmpeg`

如果 `api_endpoint` 指向 `http://127.0.0.1:8081`，代表你還需要在 VPS 上另外啟動本地 Telegram Bot API 服務。

## VPS 更新指令

Web 管理頁是 embed 在 `twitcast_bot` binary 裡面的。  
所以你不能只 `git pull`，還要重新 `go build` 並重啟真正在線上的程序。

### PM2 範例

如果你的 VPS 是用 `pm2`，而程序名稱是 `twitcast-bot`：

```bash
cd /opt/LinuxTwitcast
git pull origin main
go build -o twitcast_bot .
pm2 restart twitcast-bot
pm2 logs twitcast-bot --lines 50
```

### systemd 範例

如果你是用 systemd：

```bash
cd /opt/LinuxTwitcast
git pull origin main
go build -o twitcast_bot .
sudo systemctl restart twitcast-web.service
sudo systemctl status twitcast-web.service --no-pager -l
```

如果你不確定到底是誰在管程序，先查：

```bash
pm2 ls
sudo systemctl list-units --type=service --all | grep -i twitcast
ps -ef | grep twitcast_bot | grep -v grep
```

## Web 管理頁補充

- 網站預設語系目前是 `English`
- System Info 內的版本號暫時固定顯示 `null`
- `Check for Updates` 會比對本地 commit 與 `origin/main`
- 若發現有新版本，且遠端倉庫 URL 可轉成瀏覽器網址，按鈕會直接跳轉到倉庫頁面

## 輸出檔案

錄影檔預設命名格式為：

```text
screen-id-yyyyMMdd-HHmm.ts
```

例如：

```text
azusa_shirokyan-20060102-1504.ts
```
