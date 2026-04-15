# LinuxTwitcast

<div align="center">[English README](README.md)</div>

<p align="center">
  <img src="admin/assets/icon.svg" alt="LinuxTwitcast icon" width="112">
</p>

<p align="center"><strong>無頭 Linux 控制室</strong></p>

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

## 上游與相關專案

這個 repo 是基於原始開源專案修改而來：

- 原始上游： https://github.com/jzhang046/croned-twitcasting-recorder

這個 fork 預設連的是官方 Telegram Bot API：`https://api.telegram.org`。
只有在你自己手動把 Telegram API 位址改成本地服務時，才需要參考：

- Telegram Bot API server： https://github.com/tdlib/telegram-bot-api
- [為什麼我需要這個？](https://github.com/ExAlan7588/LinuxTwitcast/blob/main/README_ZH.md#telegram-%E8%A3%9C%E5%85%85)

## 平台支援狀態

這個 fork 目前採 **Linux 優先、Linux 正式支援** 路線。

- 有支援、也有文件：Linux，尤其是 Ubuntu VPS
- Windows：僅提供 best-effort 非正式指引
- 不支援、也不再寫使用說明：macOS

這裡要特別講清楚：

- Linux 是這個 fork 目前唯一有持續對準與驗證的主路線。
- 下面會補 Windows 操作方式，但那是非正式、best-effort 路線。
- 這不代表 Windows 享有和 Linux 一樣的維護、測試、部署保證。
- 如果你要最穩定、最符合 README 的使用方式，請直接用 Ubuntu `24.04 LTS`。

## 建議執行環境

此 fork 目前以 Linux 為主要目標，特別是 Ubuntu VPS。

建議環境：

- Ubuntu `24.04 LTS`
- `go`
- `ffmpeg`
- `pm2` 或 `systemd` 之類的常駐程序管理工具

## 開始前先裝這些

如果你是新手，先把這三樣裝好：

- `git`
  用來 `clone` 專案與之後 `pull` 更新
- `go`
  用來 build `twitcast_bot`
- `ffmpeg`
  如果你要用 Telegram 音訊轉檔，這個一定要有

Ubuntu 可以直接打：

```bash
sudo apt update
sudo apt install -y git golang-go ffmpeg
```

如果你想讓程式在 VPS 上背景常駐、自動重啟，建議再裝：

- `nodejs`
- `npm`
- `pm2`

Ubuntu 可以再打：

```bash
sudo apt install -y nodejs npm
sudo npm install pm2@latest -g
```

## 安裝

```bash
git clone https://github.com/ExAlan7588/LinuxTwitcast.git
cd LinuxTwitcast
go build -o twitcast_bot .
```

更多部署細節可看：[docs/ubuntu-vps.md](docs/ubuntu-vps.md)

## 新手快速開始

如果你是第一次碰 Linux 或 VPS，請照這個順序做，不要跳步：

1. 先安裝 Ubuntu 套件
2. clone 這個 repo
3. build `twitcast_bot`
4. 先自己決定一組前端登入帳號密碼
5. 用這組帳號密碼啟動 Web 管理頁
6. 在瀏覽器打開管理頁
7. 在前端頁面的這三個區塊填資料：
   `General & Streamer Settings`
   `Discord Notifications`
   `Telegram & Conversion`
8. 按 `Save Settings`
9. 啟動 recorder

最小可用流程：

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

這裡的 `admin` 與 `change-this-now` 只是示範值。
你要自己換成你想用的帳號與密碼。

接著打開：

```text
http://127.0.0.1:8080
```

如果你是遠端 VPS，請先透過反向代理或 SSH tunnel 暴露這個頁面。除非你已經有其他保護，否則不要直接把沒驗證的公開位址暴露出去。

## Windows 快速開始（非正式）

如果你真的想在 Windows 上使用，可以先照這段做。這段是本 repo 的非正式本機使用指引。

建議前提：

- Windows 10 或 Windows 11
- PowerShell
- 已安裝 Go
- 如果要用 Telegram 轉檔，`ffmpeg` 必須安裝並加入 `PATH`

在 PowerShell 內安裝與 build：

```powershell
git clone https://github.com/ExAlan7588/LinuxTwitcast.git
cd LinuxTwitcast
go build -o twitcast_bot.exe .
```

啟動預設排程錄影模式：

```powershell
.\twitcast_bot.exe
```

啟動 Web 管理頁：

```powershell
.\twitcast_bot.exe web --addr 127.0.0.1:8080
```

若要啟用內建驗證：

```powershell
$env:TWITCAST_WEB_USERNAME = "admin"
$env:TWITCAST_WEB_PASSWORD = "change-this-now"
.\twitcast_bot.exe web --addr 127.0.0.1:8080 --auto-start
```

這兩個值同樣只是示範，不是系統自動給你的固定帳密。

接著打開：

```text
http://127.0.0.1:8080
```

Windows 使用時要注意：

- 這不是此 fork 的主要維護目標。
- repo 內的 VPS / PM2 / systemd 說明仍然是 Linux 專用。
- 如果 Windows 與 Linux 行為不同，請先以 Linux 路線為準，不要把它直接當成 Linux 部署問題。

## 啟動方式

這段如果你看不懂，可以直接先記一個結論：

- **大多數新手只需要用 Web 管理頁模式**
- 也就是最常用的是這條：

```bash
TWITCAST_WEB_USERNAME=你自己要的帳號 \
TWITCAST_WEB_PASSWORD='你自己要的密碼' \
./twitcast_bot web --addr 127.0.0.1:8080 --auto-start
```

這條的意思拆開來看：

- `TWITCAST_WEB_USERNAME`
  你自己設定的前端登入帳號
- `TWITCAST_WEB_PASSWORD`
  你自己設定的前端登入密碼
- `web`
  啟動前端管理頁
- `--addr 127.0.0.1:8080`
  讓網站開在這台機器的 `127.0.0.1:8080`
- `--auto-start`
  管理頁啟動時，順便把錄影器一起啟動

你可以把幾種模式理解成：

### 1. 最常用：前端管理頁模式

```bash
TWITCAST_WEB_USERNAME=你自己要的帳號 \
TWITCAST_WEB_PASSWORD='你自己要的密碼' \
./twitcast_bot web --addr 127.0.0.1:8080 --auto-start
```

適合：

- 你想用前端管理
- 你想在網頁裡設定直播主、Discord、Telegram
- 你想一打開管理頁就順便開始排程錄影

### 2. 只有開前端，不自動啟動錄影器

```bash
./twitcast_bot web --addr 127.0.0.1:8080
```

適合：

- 你只想先打開前端看看
- 你想進前端先改設定，再手動按 `Start Recorder`

### 3. 純命令列排程錄影模式

```bash
./twitcast_bot
```

或：

```bash
./twitcast_bot croned
```

這兩條在一般使用上可以理解成同一類：

- 不開前端
- 只跑排程錄影

如果你是新手，通常不用先學這個，先用前端模式就好。

### 4. 只錄單一直播的測試模式

```bash
./twitcast_bot direct --streamer=azusa_shirokyan --retries=10 --retry-backoff=1m
```

適合：

- 你只是想臨時測一個直播主
- 你不是要跑長期排程

補充說明：

- `127.0.0.1:8080` 的意思是只在本機開這個頁面
- 如果你是 VPS，通常會再透過 Nginx、OpenResty、Caddy、Tailscale 或 SSH tunnel 轉出去
- `web` 模式正確使用的旗標是 `--addr`

## 設定檔

程式會讀取工作目錄中的：

- `config.json`
- `discord.json`
- `telegram.json`

Web 管理頁也是直接編輯這三個檔案，所以網頁與 CLI 會共用同一份設定。

新手可以這樣理解：

- 如果你是用前端管理頁，通常不需要自己手改這三個檔
- 你只要在前端區塊填資料，然後按 `Save Settings`
- 程式會幫你把資料寫進對應檔案

## Discord Bot 權限說明

LinuxTwitcast 在 Discord 內主要做兩件事：

1. 發送、更新、刪除直播通知訊息
2. 需要時註冊右鍵選單指令，並用每個直播主對應的身分組做訂閱 / 取消訂閱

### Discord 邀請頁上要勾選的項目（Scopes）

如果你看不懂 `Scopes`，你可以直接把它理解成：

- Discord Bot 邀請頁上要勾的選項

這個專案至少要勾：

- `bot`
- `applications.commands`

如果你是新手，先只用這兩個 scopes 就好，不要一開始亂加其他 scopes。

### 頻道權限

在通知頻道，以及你有使用的歸檔頻道，建議 Bot 至少有：

- `View Channels`
- `Send Messages`
- `Embed Links`
- `Read Message History`

如果你只是要最基本的開播通知，先把上面這四個權限給齊就夠了。

這些需求對應到目前的訊息建立、編輯、刪除流程：

- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:209>)
- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:228>)
- [discord/discord.go](</o:/Cursor AI/LinuxTwitcast/discord/discord.go:241>)

### 身分組權限

如果你要開啟 `tag_role`，或要使用右鍵選單的訂閱 / 取消訂閱功能，Bot 還需要：

- `Manage Roles`
- Bot 自己的身分組排序必須高於它要建立或指派的目標身分組

如果你沒有要用 `tag_role`，那就不用先開 `Manage Roles`。

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

預設行為先講清楚：

- 你 **不需要** 先架本地 Telegram Bot API 服務
- 專案預設會連 `https://api.telegram.org`
- 只有當你自己把 API 位址改成本地服務時，才需要另外架 `tdlib/telegram-bot-api`

### 為什麼有人會選擇自架 Telegram Bot API

Telegram 官方文件目前列出的差異是：

- 官方 Bot API：下載上限 `20MB`、上傳上限 `50MB`
- 本地 Bot API：下載不限大小、上傳可到 `2000MB`
- 官方 Bot API：Webhook 需要 HTTPS，而且只能用特定連接埠
- 本地 Bot API：可用 HTTP、任意連接埠、較大的 `max_webhook_connections`

所以會選擇自架，通常是因為：

- 你要傳很大的錄影檔
- 你想把 Bot API 放在內網或同一台 VPS 上
- 你想用本地 HTTP，不想另外處理外部 HTTPS 限制
- 你需要本地模式的一些額外能力

這些差異來自 Telegram 官方與官方 open-source server 文件：

- https://core.telegram.org/bots/features
- https://github.com/tdlib/telegram-bot-api

### 為什麼這個專案預設不強制你自架

因為對大多數新手來說：

- 官方 `https://api.telegram.org` 就能先用
- 架本地 Bot API 服務會多一套安裝與維運成本
- 除非你真的遇到檔案大小或部署需求，不然先不要加複雜度

如果開啟 Telegram 上傳：

- `telegram.json` 必須有有效的 `bot_token`
- `telegram.json` 必須有有效的 `chat_id`
- 若開啟 `convert_to_m4a`，系統 `PATH` 中必須找得到 `ffmpeg`

如果 `api_endpoint` 指向 `http://127.0.0.1:8081`，代表你還需要在 VPS 上另外啟動本地 Telegram Bot API 服務。
可用的一個本地實作就是 `tdlib/telegram-bot-api`。

### 如果你要自架，最簡單怎麼做

Telegram 官方 `tdlib/telegram-bot-api` 文件目前要求至少準備：

- `api_id`
- `api_hash`
- Bot API server 本體

官方 build 入口：

- https://github.com/tdlib/telegram-bot-api
- https://tdlib.github.io/telegram-bot-api/build.html

官方 README 的基本 build 流程大致是：

```bash
git clone --recursive https://github.com/tdlib/telegram-bot-api.git
cd telegram-bot-api
mkdir build
cd build
cmake -DCMAKE_BUILD_TYPE=Release ..
cmake --build . --target install
```

啟動時，官方 README 指出至少要提供：

```bash
telegram-bot-api --api-id <你的api_id> --api-hash <你的api_hash> --local
```

幾個你一定要知道的點：

1. `api_id` / `api_hash` 不是 Bot Token
   這兩個要另外向 Telegram 申請。
2. 本地 Bot API 預設跑在 `8081`
3. 你把 Bot 從官方 API 切到本地 API 前，官方文件要求先呼叫 `logOut`
4. 這台本地 Bot API 只收 HTTP；如果你要對外暴露，還要自己做 TLS / 反向代理

如果你的目標只是先把 LinuxTwitcast 跑起來，我建議：

1. 先用官方 `https://api.telegram.org`
2. 真的遇到大檔案或架構需求，再改成自架

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
