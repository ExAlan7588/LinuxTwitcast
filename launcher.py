import os
import sys
import json
import subprocess
import time
import msvcrt
import urllib.request
import urllib.error
import re
import threading

CONFIG_FILE = "config.json"
DISCORD_FILE = "discord.json"
TELEGRAM_FILE = "telegram.json"
BIN_FILE = "croned-twitcasting-recorder.exe"

# ANSI Colors for vibrant UI
C_CYAN = "\033[96m"
C_GREEN = "\033[92m"
C_YELLOW = "\033[93m"
C_RED = "\033[91m"
C_PURPLE = "\033[95m"
C_RESET = "\033[0m"
C_GRAY = "\033[90m"
C_WHITE = "\033[97m"
C_BLUE = "\033[94m"

STATUS_CACHE = {} # screen-id -> status string

I18N = {
    "ZH": {
        "title": "TwitCasting 自動控制與歸檔中心 (AUTO-RECORDER)",
        "status": "背景引擎狀態",
        "status_running": "[運作中] 正在錄影監控",
        "status_stopped": "[已停止] 待命/離線狀態",
        "no_streamer": "尚未設定任何實況主。請按 [1] 新增！",
        "col_header": "[ID]   [啟用] [間隔]  [儲存目錄]           [實況主 ID]      [狀態]      [顯示名稱]",
        "status_ok": "正常",
        "folder_default": "(同上層目錄)",
        "menu_1": "[1] 新增實況主       (Add New Streamer)",
        "menu_2": "[2] 變更存放資料夾   (Change Target Folder)",
        "menu_3": "[3] 切換錄影開關     (Toggle Monitor On/Off)",
        "menu_4": "[4] 移除實況主       (Remove Streamer)",
        "menu_5": "[5] 打開實況主資料夾 (Open Streamer Folder)",
        "menu_6": "[6] 切換語言         (Switch Language: ZH/EN)",
        "menu_7": "[7] 變更偵測間隔     (Change Detection Interval)",
        "menu_8": "[8] Discord 通知設定 (Discord Notification Settings)",
        "menu_9": "[9] Telegram 與轉檔設定 (Telegram & Auto-Convert)",
        "menu_l_on": "[L] 背景日誌記錄：已開啟（開關影響下次啟動）",
        "menu_l_off": "[L] 背景日誌記錄：已關閉",
        "menu_s": "[S] 開始背景錄影     (Start Background Monitor)",
        "menu_x": "[X] 停止錄影         (Stop Monitor)",
        "menu_q": "[Q] 退出控制台       (Quit)",
        "prompt_choice": " 請按對應按鍵選擇 > ",
        "err_no_enabled": "沒有啟用任何實況主，無法啟動！請先按 [3] 打勾啟用。",
        "start_failed": "啟動失敗:",
        "input_add_id": "\n\n 請輸入要新增的 TwitCasting 帳號 (screen-id): ",
        "verifying": " 正在與伺服器連線驗證帳號存在與否...",
        "not_found": "找不到該帳號！請確認拼字是否正確。",
        "fetch_success": "抓取成功！實況主名稱: ",
        "input_folder": " 請輸入該實況主存檔資料夾名稱 (可輸入絕對路徑如 D:\\Video，直接按 Enter 則套用預設): ",
        "add_success": "新增完畢！檔案將儲存於: ",
        "input_idx_change": "\n\n 請輸入要變更資料夾的項次 (數字): ",
        "curr_setting": " 目前設定: ",
        "new_folder": " 新資料夾名稱 (可輸入任意絕對/相對路徑，如 D:\\錄影): ",
        "input_idx_toggle": "\n\n 請輸入要「切換錄影開關」的項次 (數字): ",
        "input_idx_remove": "\n\n 請輸入要「刪除」的項次 (數字): ",
        "confirm_remove": " 確認要刪除嗎？若有錄影檔不會被刪除 (Y/N): ",
        "input_idx_open": "\n\n 請輸入要「打開資料夾」的項次 (數字): ",
        "folder_not_exist": "資料夾尚不存在，第一次開台錄影時就會自動建立！",
        "input_idx_interval": "\n\n 請輸入要「變更偵測間隔」的項次 (數字): ",
        "curr_interval": " 目前偵測間隔: ",
        "new_interval": " 新偵測間隔 (輸入範例: 10s, 1m, 5m): ",
        "loading": "讀取中...",
        "status_err": "失效/改名",
        "btn_y": "Y"
    },
    "EN": {
        "title": "TwitCasting Auto-Recorder Control Center",
        "status": "Engine Status",
        "status_running": "[Running] Background monitoring active",
        "status_stopped": "[Stopped] Standby mode",
        "no_streamer": "No streamers configured. Press [1] to add!",
        "col_header": "[ID]   [On]  [Intvl] [Target Folder]      [Streamer ID]    [Status]    [Display Name]",
        "status_ok": "OK",
        "folder_default": "(Current Dir)",
        "menu_1": "[1] Add New Streamer",
        "menu_2": "[2] Change Target Folder",
        "menu_3": "[3] Toggle Monitor On/Off",
        "menu_4": "[4] Remove Streamer",
        "menu_5": "[5] Open Streamer Folder",
        "menu_6": "[6] Switch Language (EN/ZH)",
        "menu_7": "[7] Change Detection Interval",
        "menu_8": "[8] Discord Notification Settings",
        "menu_9": "[9] Telegram & Auto-Convert Settings",
        "menu_l_on": "[L] Engine Log: ON (requires restart)",
        "menu_l_off": "[L] Engine Log: OFF",
        "menu_s": "[S] Start Background Monitor",
        "menu_x": "[X] Stop Monitor",
        "menu_q": "[Q] Quit",
        "prompt_choice": " Press a key to select > ",
        "err_no_enabled": "No streamers enabled! Press [3] to toggle them on first.",
        "start_failed": "Failed to start:",
        "input_add_id": "\n\n Enter the new TwitCasting ID (screen-id): ",
        "verifying": " Verifying account existence with server...",
        "not_found": "Account not found! Please check the spelling.",
        "fetch_success": "Successfully fetched! Streamer Name: ",
        "input_folder": " Enter target folder (Absolute paths like D:\\Video allowed, press Enter for default): ",
        "add_success": "Successfully added! Files will be saved to: ",
        "input_idx_change": "\n\n Enter index to change folder (Number): ",
        "curr_setting": " Current setting: ",
        "new_folder": " New folder name (absolute or relative path): ",
        "input_idx_toggle": "\n\n Enter index to toggle monitor (Number): ",
        "input_idx_remove": "\n\n Enter index to remove (Number): ",
        "confirm_remove": " Are you sure you want to remove? (Y/N): ",
        "input_idx_open": "\n\n Enter index to open folder (Number): ",
        "folder_not_exist": "Folder does not exist yet. It will be created on the first record!",
        "input_idx_interval": "\n\n Enter index to change detection interval (Number): ",
        "curr_interval": " Current interval: ",
        "new_interval": " New interval (e.g., 10s, 1m, 5m): ",
        "loading": "Loading...",
        "status_err": "Error",
        "btn_y": "Y"
    }
}

def load_config():
    if not os.path.exists(CONFIG_FILE):
        return {"lang": "ZH", "streamers": []}
    try:
        with open(CONFIG_FILE, 'r', encoding='utf-8') as f:
            c = json.load(f)
            if "lang" not in c: c["lang"] = "ZH"
            if "streamers" not in c: c["streamers"] = []
            return c
    except Exception as e:
        print(f"Error loading config: {e}")
        return {"lang": "ZH", "streamers": []}

def setup_console():
    os.system("") # Enable ANSI escape codes in Windows
    os.system('mode con: cols=105 lines=30')  # Set console size slightly larger

def save_config(config):
    with open(CONFIG_FILE, 'w', encoding='utf-8') as f:
        json.dump(config, f, indent=4, ensure_ascii=False)

def load_discord_config():
    """Load discord.json. Returns a default empty dict if file doesn't exist."""
    if not os.path.exists(DISCORD_FILE):
        return {"enabled": False, "bot_token": "", "guild_id": "",
                "notify_channel_id": "", "archive_channel_id": "", "tag_role": False}
    try:
        with open(DISCORD_FILE, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        print(f"Error loading discord.json: {e}")
        return {"enabled": False, "bot_token": "", "guild_id": "",
                "notify_channel_id": "", "archive_channel_id": "", "tag_role": False}

def save_discord_config(dc):
    """Write discord.json."""
    with open(DISCORD_FILE, 'w', encoding='utf-8') as f:
        json.dump(dc, f, indent=4, ensure_ascii=False)

def load_telegram_config():
    if not os.path.exists(TELEGRAM_FILE):
        return {"enabled": False, "bot_token": "", "chat_id": "",
                "api_endpoint": "https://api.telegram.org", "convert_to_m4a": True, "keep_original": False}
    try:
        with open(TELEGRAM_FILE, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        print(f"Error loading telegram.json: {e}")
        return {"enabled": False, "bot_token": "", "chat_id": "",
                "api_endpoint": "https://api.telegram.org", "convert_to_m4a": True, "keep_original": False}

def save_telegram_config(tc):
    with open(TELEGRAM_FILE, 'w', encoding='utf-8') as f:
        json.dump(tc, f, indent=4, ensure_ascii=False)

def load_telegram_config():
    if not os.path.exists(TELEGRAM_FILE):
        return {"enabled": False, "bot_token": "", "chat_id": "",
                "api_endpoint": "https://api.telegram.org", "convert_to_m4a": True, "keep_original": False}
    try:
        with open(TELEGRAM_FILE, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        print(f"Error loading telegram.json: {e}")
        return {"enabled": False, "bot_token": "", "chat_id": "",
                "api_endpoint": "https://api.telegram.org", "convert_to_m4a": True, "keep_original": False}

def save_telegram_config(tc):
    with open(TELEGRAM_FILE, 'w', encoding='utf-8') as f:
        json.dump(tc, f, indent=4, ensure_ascii=False)

def check_twitcast_id(streamer_id):
    url = f"https://twitcasting.tv/{streamer_id}"
    req = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            # Check for generic home page redirect
            real_url = resp.geturl().lower().rstrip('/')
            if real_url == "https://twitcasting.tv" or real_url == "http://twitcasting.tv":
                return False, ""
                
            html = resp.read().decode('utf-8')
            
            # 1. Try to find NAME (@ID) pattern in <title> - This is the most stable source
            # We use re.IGNORECASE for the ID match just in case user input case differs
            title_pat = re.escape(streamer_id) + r'\s*\)'
            title_match = re.search(r'<title>(.*?)\s+\(@' + title_pat, html, re.IGNORECASE)
            if title_match:
                name = title_match.group(1).strip()
                if name.lower() not in ["twitcasting", "ツイキャス", "twitcast", "ツイキャス - ライブ配信"]:
                    return True, name

            # 2. Try author meta tag - TwitCast usually sets this correctly
            author_match = re.search(r'<meta name="author" content="([^"]+)">', html)
            if author_match:
                name = author_match.group(1).strip()
                if name.lower() not in ["twitcasting", "ツイキャス", "twitcast"]:
                    return True, name
            
            # 3. Look for structured data (Twitter Card) - Often overridden by stream title if live
            name_match = re.search(r'<meta name="twitter:title" content="([^"]+)">', html)
            if name_match:
                name = name_match.group(1).strip()
                if name.lower() not in ["twitcasting", "ツイキャス", "twitcast"]:
                    return True, name

            # 4. Final title fallback
            title_match = re.search(r'<title>(.*?)</title>', html)
            if title_match:
                title = title_match.group(1).strip()
                if "Watch Live Streaming" not in title and title.lower() not in ["twitcasting", "twitcast"]:
                    return True, title
                
        return False, ""
    except Exception:
        return False, ""

def update_status(streamer_id):
    is_valid, name = check_twitcast_id(streamer_id)
    STATUS_CACHE[streamer_id] = name if is_valid else "ERROR"

def refresh_all_statuses(config):
    for s in config.get("streamers", []):
        sid = s["screen-id"]
        if sid not in STATUS_CACHE or STATUS_CACHE[sid] == "ERROR":
            threading.Thread(target=update_status, args=(sid,), daemon=True).start()

def draw_menu(config, proc, force_clear=False):
    if force_clear:
        os.system('cls' if os.name == 'nt' else 'clear')
    else:
        # Move cursor to 0,0 without clearing buffer to completely prevent flickering
        sys.stdout.write("\033[H")
        sys.stdout.flush()
        
    lang = config.get("lang", "ZH")
    texts = I18N.get(lang, I18N["ZH"])
    
    # Calculate screen center length
    title_text = f" {texts['title']} "
    pad_len = max(0, (97 - len(title_text)) // 2)
    
    print(f"\n{C_CYAN}{'=' * 97}{C_RESET}\033[K")
    print(f"{' ' * pad_len}{C_PURPLE}{title_text}{C_RESET}\033[K")
    print(f"{C_CYAN}{'=' * 97}{C_RESET}\033[K")
    
    status_str = f"{C_GREEN}{texts['status_running']}{C_RESET}" if proc and proc.poll() is None else f"{C_GRAY}{texts['status_stopped']}{C_RESET}"
    print(f"\n   {C_WHITE}{texts['status']}:{C_RESET} {status_str}\033[K\n\033[K", end="")
    print(f"{C_BLUE}{'-' * 97}{C_RESET}\033[K")
    
    streamers = config.get("streamers", [])
    if not streamers:
        print(f"   {C_YELLOW}{texts['no_streamer']}{C_RESET}\033[K")
    else:
        print(f"   {C_CYAN}{texts['col_header']}{C_RESET}\033[K")
        for i, s in enumerate(streamers):
            sid = s['screen-id']
            if s.get("enabled", True):
                enabled_icon = f"{C_GREEN}[v]{C_RESET}"
            else:
                enabled_icon = f"{C_RED}[x]{C_RESET}"
            
            # Format plain text strings first
            folder_plain = s.get("folder", texts["folder_default"])
            if not folder_plain.strip(): folder_plain = texts["folder_default"]
            folder_plain = folder_plain[:18]
            
            schedule_plain = s.get("schedule", "@every 5s").replace("@every ", "")[:5]
            sid_plain = sid[:15]
            
            # Retrieve status dynamically
            curr_status = STATUS_CACHE.get(sid, texts["loading"])
            if curr_status == "ERROR":
                status_icon = f"{C_RED}[ {texts['status_err']:^5} ]{C_RESET}"
                s_name = f"{C_GRAY}---{C_RESET}"
            elif curr_status == texts["loading"]:
                status_icon = f"{C_GRAY}[ {texts['loading']:^5} ]{C_RESET}"
                s_name = f"{C_GRAY}---{C_RESET}"
            else:
                status_icon = f"{C_GREEN}[ {texts['status_ok']:^5} ]{C_RESET}"
                s_name = f"{C_WHITE}{curr_status[:24]}{C_RESET}"
            
            # Colored format string layout (End with \033[K to erase lingering artifact characters)
            # Standardized widths: i(2), enabled(3), interval(5), folder(20), id(15), status(10), name(24)
            print(f"   {C_CYAN}{i+1:2d}.{C_RESET}    {enabled_icon}   {C_YELLOW}{schedule_plain:^5}{C_RESET}  {C_WHITE}{folder_plain:<20}{C_RESET} {C_CYAN}{sid_plain:<15}{C_RESET}  {status_icon}   {s_name}\033[K")
            
    print(f"{C_BLUE}{'-' * 97}{C_RESET}\033[K")
    print(f"   {C_WHITE}{texts['menu_1']}{C_RESET}\033[K")
    print(f"   {C_WHITE}{texts['menu_2']}{C_RESET}\033[K")
    print(f"   {C_WHITE}{texts['menu_3']}{C_RESET}\033[K")
    print(f"   {C_WHITE}{texts['menu_4']}{C_RESET}\033[K")
    print(f"   {C_WHITE}{texts['menu_5']}{C_RESET}\033[K")
    print(f"   {C_GRAY}{texts['menu_6']}{C_RESET}\033[K")
    print(f"   {C_GRAY}{texts['menu_7']}{C_RESET}\033[K")
    # Discord status indicator — read directly from discord.json
    dc_enabled = load_discord_config().get("enabled", False)
    dc_status = f"{C_GREEN}[已啟用]{C_RESET}" if dc_enabled else f"{C_GRAY}[已停用]{C_RESET}"
    print(f"   {C_BLUE}{texts['menu_8']}  {dc_status}{C_RESET}\033[K")

    tg_enabled = load_telegram_config().get("enabled", False)
    tg_status = f"{C_GREEN}[已啟用]{C_RESET}" if tg_enabled else f"{C_GRAY}[已停用]{C_RESET}"
    print(f"   {C_YELLOW}{texts['menu_9']}  {tg_status}{C_RESET}\033[K")
    print(f"\033[K")
    
    # Logging toggle
    enable_log = config.get("enable_log", False)
    log_status_text = texts['menu_l_on'] if enable_log else texts['menu_l_off']
    log_color = C_GREEN if enable_log else C_GRAY
    print(f"   {log_color}{log_status_text}{C_RESET}\033[K")
    print(f"\033[K")
    
    print(f"   {C_GREEN}{texts['menu_s']}{C_RESET}\033[K")
    print(f"   {C_RED}{texts['menu_x']}{C_RESET}\033[K")
    print(f"   {C_GRAY}{texts['menu_q']}{C_RESET}\033[K")
    # Overwrite the end of the screen space in case previous elements were longer
    print(f"\033[J{C_YELLOW}{texts['prompt_choice']}{C_RESET}", end="", flush=True)

def wait_keypress(timeout=1.0):
    start = time.time()
    while timeout is None or time.time() - start < timeout:
        if msvcrt.kbhit():
            try:
                key = msvcrt.getch()
                return key.decode('utf-8').upper()
            except:
                pass
        time.sleep(0.05)
    return None

def _mask_token(token):
    """Display first 10 chars + .... + last 5 chars for token preview."""
    if not token:
        return "(未設定)" 
    if len(token) <= 15:
        return "*" * len(token)
    return token[:10] + "...." + token[-5:]

def _read_masked_input():
    """Read input character by character, echoing '*' for each char. Returns the string."""
    import msvcrt as _msvcrt
    result = []
    while True:
        ch = _msvcrt.getch()
        if ch in (b'\r', b'\n'):  # Enter
            sys.stdout.write('\n')
            sys.stdout.flush()
            break
        elif ch == b'\x08':  # Backspace
            if result:
                result.pop()
                sys.stdout.write('\b \b')
                sys.stdout.flush()
        elif ch == b'\x03':  # Ctrl+C
            raise KeyboardInterrupt
        elif ch and ch[0] >= 32:  # Printable char
            result.append(ch.decode('utf-8', errors='replace'))
            sys.stdout.write('*')
            sys.stdout.flush()
    return ''.join(result)

def handle_discord_settings(config, texts):
    """Interactive Discord notification settings handler with pending-save workflow."""
    lang = config.get("lang", "ZH")
    is_zh = (lang == "ZH")

    # Work on a deep copy of discord.json content; only write on [S]ave
    import copy
    dc_saved   = load_discord_config()
    dc_pending = copy.deepcopy(dc_saved)

    while True:
        os.system('cls')
        dc_enabled  = dc_pending.get("enabled", False)
        token       = dc_pending.get("bot_token", "")
        guild_id    = dc_pending.get("guild_id", "")
        notify_ch   = dc_pending.get("notify_channel_id", "")
        archive_ch  = dc_pending.get("archive_channel_id", "")
        tag_role    = dc_pending.get("tag_role", False)

        # Detect unsaved changes
        has_changes  = (dc_pending != dc_saved)
        changed_hint = f" {C_YELLOW}[未儲存變更]{C_RESET}" if (has_changes and is_zh) else \
                       (f" {C_YELLOW}[Unsaved changes]{C_RESET}" if has_changes else "")

        status_str   = f"{C_GREEN}{'已啟用' if is_zh else 'Enabled'}{C_RESET}" \
                       if dc_enabled else f"{C_GRAY}{'已停用' if is_zh else 'Disabled'}{C_RESET}"
        archive_disp = f"{C_GRAY}{'(不歸檔)' if is_zh else '(No archiving)'}{C_RESET}" \
                       if not archive_ch else f"{C_CYAN}{archive_ch}{C_RESET}"
        tag_str      = f"{C_GREEN}{'已啟用' if is_zh else 'On'}{C_RESET}" \
                       if tag_role else f"{C_GRAY}{'已停用' if is_zh else 'Off'}{C_RESET}"

        # Header
        print(f"\n{C_CYAN}{'=' * 97}{C_RESET}")
        hdr = " Discord 通知設定 " if is_zh else " Discord Notification Settings "
        pad = max(0, (97 - len(hdr)) // 2)
        print(f"{' ' * pad}{C_PURPLE}{hdr}{C_RESET}{changed_hint}")
        print(f"{C_CYAN}{'=' * 97}{C_RESET}\n")

        lbl_w = 22
        print(f"   {C_WHITE}{'狀態：' if is_zh else 'Status:':<{lbl_w}}{C_RESET}{status_str}")
        print(f"   {C_WHITE}{'Bot Token：' if is_zh else 'Bot Token:':<{lbl_w}}{C_RESET}{C_YELLOW}{_mask_token(token)}{C_RESET}")
        print(f"   {C_WHITE}{'伺服器 ID：' if is_zh else 'Guild ID:':<{lbl_w}}{C_RESET}{C_CYAN}{guild_id or ('(未設定)' if is_zh else '(Not set)')}{C_RESET}")
        print(f"   {C_WHITE}{'通知頻道 ID：' if is_zh else 'Notify Channel ID:':<{lbl_w}}{C_RESET}{C_CYAN}{notify_ch or ('(未設定)' if is_zh else '(Not set)')}{C_RESET}")
        print(f"   {C_WHITE}{'歸檔頻道 ID：' if is_zh else 'Archive Channel ID:':<{lbl_w}}{C_RESET}{archive_disp}")
        print(f"   {C_WHITE}{'標記身分組：' if is_zh else 'Tag Role:':<{lbl_w}}{C_RESET}{tag_str}")
        print(f"{C_BLUE}{'-' * 97}{C_RESET}")

        if is_zh:
            print(f"   {C_GREEN}[1]{C_RESET} 啟用／停用通知")
            print(f"   {C_WHITE}[2]{C_RESET} 設定 Bot Token")
            print(f"   {C_WHITE}[3]{C_RESET} 設定伺服器 ID（Guild ID）")
            print(f"   {C_WHITE}[4]{C_RESET} 設定通知頻道 ID")
            print(f"   {C_WHITE}[5]{C_RESET} 設定歸檔頻道 ID（輸入 none 表示不歸檔）")
            print(f"   {C_WHITE}[6]{C_RESET} 切換身分組標記（開播時以 @screen-id 通知，無身分組則自動建立）")
            print(f"   {C_GREEN}[S]{C_RESET} 儲存設定（儲存後才會生效）")
            print(f"   {C_GRAY}[Q]{C_RESET} 返回主選單（未儲存的變更將被捨棄）")
            prompt = f"\n{C_YELLOW} 請按對應按鍵 > {C_RESET}"
        else:
            print(f"   {C_GREEN}[1]{C_RESET} Toggle Enable/Disable")
            print(f"   {C_WHITE}[2]{C_RESET} Set Bot Token")
            print(f"   {C_WHITE}[3]{C_RESET} Set Guild ID (Server ID)")
            print(f"   {C_WHITE}[4]{C_RESET} Set Notify Channel ID")
            print(f"   {C_WHITE}[5]{C_RESET} Set Archive Channel ID (type 'none' to disable archiving)")
            print(f"   {C_WHITE}[6]{C_RESET} Toggle Role Tag (@screen-id mention; auto-creates role if missing)")
            print(f"   {C_GREEN}[S]{C_RESET} Save Settings (changes only apply after saving)")
            print(f"   {C_GRAY}[Q]{C_RESET} Back to Main Menu (unsaved changes will be discarded)")
            prompt = f"\n{C_YELLOW} Press a key > {C_RESET}"

        print(prompt, end="", flush=True)

        import msvcrt as _msvcrt
        while not _msvcrt.kbhit():
            pass
        try:
            sub_key = _msvcrt.getch().decode('utf-8').upper()
        except Exception:
            continue

        if sub_key == 'Q':
            break

        elif sub_key == 'S':
            save_discord_config(dc_pending)
            dc_saved = copy.deepcopy(dc_pending)
            os.system('cls')
            ok_msg = "✔ 設定已儲存至 discord.json！" if is_zh else "✔ Settings saved to discord.json!"
            print(f"\n   {C_GREEN}{ok_msg}{C_RESET}")
            time.sleep(1.2)

        elif sub_key == '1':
            dc_pending["enabled"] = not dc_pending.get("enabled", False)

        elif sub_key == '2':
            if is_zh:
                print(f"\n\n   Bot Token 說明：請至 Discord Developer Portal 取得 Bot Token。\n"
                      f"   請輸入 Bot Token（輸入時以 * 遮蔽）：", end="", flush=True)
            else:
                print(f"\n\n   Bot Token: Obtain from the Discord Developer Portal.\n"
                      f"   Enter Bot Token (input is masked with *): ", end="", flush=True)
            val = _read_masked_input().strip()
            if val:
                dc_pending["bot_token"] = val

        elif sub_key == '3':
            if is_zh:
                print(f"\n\n   伺服器 ID（Guild ID）：右鍵點擊 Discord 伺服器名稱 > 複製伺服器 ID。\n"
                      f"   需啟用開發者模式才能複製。\n"
                      f"   請輸入伺服器 ID：", end="", flush=True)
            else:
                print(f"\n\n   Guild ID: Right-click the server name in Discord > Copy Server ID.\n"
                      f"   Developer Mode must be enabled in Discord settings.\n"
                      f"   Enter Guild ID: ", end="", flush=True)
            val = sys.stdin.readline().strip()
            if val:
                dc_pending["guild_id"] = val

        elif sub_key == '4':
            if is_zh:
                print(f"\n\n   通知頻道 ID：直播開始/結束的通知訊息會發到此頻道。\n"
                      f"   請輸入通知頻道 ID：", end="", flush=True)
            else:
                print(f"\n\n   Notify Channel ID: Live start/end notifications will be sent here.\n"
                      f"   Enter Notify Channel ID: ", end="", flush=True)
            val = sys.stdin.readline().strip()
            if val:
                dc_pending["notify_channel_id"] = val

        elif sub_key == '5':
            if is_zh:
                print(f"\n\n   歸檔頻道 ID：直播結束後，通知訊息會複製到此頻道保存。\n"
                      f"   輸入 none 可停用歸檔功能（通知訊息不會被複製）。\n"
                      f"   請輸入歸檔頻道 ID（none = 不歸檔）：", end="", flush=True)
            else:
                print(f"\n\n   Archive Channel ID: Ended-stream notifications are copied here for archiving.\n"
                      f"   Type 'none' to disable archiving (no message will be copied).\n"
                      f"   Enter Archive Channel ID (none = no archiving): ", end="", flush=True)
            val = sys.stdin.readline().strip()
            if val:
                dc_pending["archive_channel_id"] = "" if val.lower() == "none" else val

        elif sub_key == '6':
            dc_pending["tag_role"] = not dc_pending.get("tag_role", False)


def handle_telegram_settings(config, texts):
    """Handle Telegram notifications and automated FFmpeg conversion settings."""
    tc = load_telegram_config()
    pending_tc = tc.copy()

    while True:
        os.system('cls' if os.name == 'nt' else 'clear')
        print(f"\n{C_CYAN}{'=' * 97}{C_RESET}")
        print(f"   {C_PURPLE}Telegram 與自動轉檔設定{C_RESET}")
        print(f"{C_CYAN}{'=' * 97}{C_RESET}")
        
        status_on_off = f"{C_GREEN}開啟{C_RESET}" if pending_tc.get("enabled", False) else f"{C_GRAY}關閉{C_RESET}"
        api_endpoint = pending_tc.get("api_endpoint", "https://api.telegram.org")
        convert_to_m4a = f"{C_GREEN}開啟{C_RESET}" if pending_tc.get("convert_to_m4a", True) else f"{C_GRAY}關閉{C_RESET}"
        keep_original = f"{C_GREEN}保留{C_RESET}" if pending_tc.get("keep_original", False) else f"{C_GRAY}刪除{C_RESET}"

        tk = pending_tc.get("bot_token", "")
        if len(tk) > 15:
            tk_masked = tk[:10] + "...." + tk[-5:]
        elif len(tk) > 0:
            tk_masked = "********"
        else:
            tk_masked = "未設定"
            
        chat_id = pending_tc.get("chat_id", "未設定")
        if not chat_id: chat_id = "未設定"

        print(f"   [1] 啟用 Telegram 推播與轉檔     : {status_on_off}")
        print(f"   [2] 設定 Telegram Bot Token      : {tk_masked}")
        print(f"   [3] 設定 推播目標 Channel/Chat ID: {chat_id}")
        print(f"   [4] 設定 Telegram API Endpoint   : {api_endpoint}")
        print(f"   [5] 自動提取 M4A 音訊檔          : {convert_to_m4a}")
        print(f"   [6] 提取後是否保留原 TS 檔       : {keep_original}")
        print(f"{C_BLUE}{'-' * 97}{C_RESET}")
        print(f"   {C_GREEN}[S] 保存設定並返回主畫面{C_RESET}")
        print(f"   {C_RED}[Q] 放棄更改並返回主畫面{C_RESET}\n")

        print(" 請選擇設定項 > ", end="", flush=True)
        key = wait_keypress(timeout=None)

        if key == 'Q':
            return
        elif key == 'S':
            save_telegram_config(pending_tc)
            print(f"\n   {C_GREEN}✔ 設定已儲存至 telegram.json！{C_RESET}")
            time.sleep(1.5)
            return
        elif key == '1':
            pending_tc["enabled"] = not pending_tc.get("enabled", False)
        elif key == '2':
            print("\n\n   請輸入 Telegram Bot Token (輸入 none 則清空): ", end="", flush=True)
            val = sys.stdin.readline().strip()
            if val.lower() == 'none':
                pending_tc["bot_token"] = ""
            elif val:
                pending_tc["bot_token"] = val
        elif key == '3':
            print("\n\n   請輸入推播目標 Channel ID / Chat ID (輸入 none 則不推播): ", end="", flush=True)
            val = sys.stdin.readline().strip()
            if val.lower() == 'none':
                pending_tc["chat_id"] = ""
            elif val:
                pending_tc["chat_id"] = val
        elif key == '4':
            print(f"\n\n   請輸入 Telegram API 網址 (目前: {api_endpoint}): ", end="", flush=True)
            val = sys.stdin.readline().strip()
            if val:
                pending_tc["api_endpoint"] = val
        elif key == '5':
            pending_tc["convert_to_m4a"] = not pending_tc.get("convert_to_m4a", True)
        elif key == '6':
            pending_tc["keep_original"] = not pending_tc.get("keep_original", False)


def main():
    setup_console()
    config = load_config()
    proc = None

    # Load initial statuses
    refresh_all_statuses(config)
    
    force_redraw = True
    last_status_cache = {}
    last_proc_state = False

    while True:
        curr_proc_state = (proc and proc.poll() is None)
        
        # Only redraw if something changed, or if a user interaction requires a complete screen wipe
        if force_redraw or STATUS_CACHE != last_status_cache or curr_proc_state != last_proc_state:
            draw_menu(config, proc, force_clear=force_redraw)
            last_status_cache = STATUS_CACHE.copy()
            last_proc_state = curr_proc_state
            force_redraw = False

        key = wait_keypress(0.2)
        
        # If timeout, it returns None. We just loop and let it dynamically update if states changed.
        if key is None:
            continue
            
        force_redraw = True # Any keypress triggers a full redraw next loop
            
        lang = config.get("lang", "ZH")
        texts = I18N.get(lang, I18N["ZH"])

        if key == 'Q':
            if proc and proc.poll() is None:
                proc.terminate()
            break
            
        elif key == 'S':
            if proc and proc.poll() is None:
                continue
            if not any(s.get("enabled", True) for s in config.get("streamers", [])):
                print(f"\n\n {C_RED}{texts['err_no_enabled']}{C_RESET}")
                time.sleep(2)
                continue
                
            SW_HIDE = 0
            info = subprocess.STARTUPINFO()
            info.dwFlags = subprocess.STARTF_USESHOWWINDOW
            info.wShowWindow = SW_HIDE
            try:
                env = os.environ.copy()
                if config.get("enable_log", False):
                    env["TWITCAST_LOG_CONSOLE"] = "1"
                else:
                    env.pop("TWITCAST_LOG_CONSOLE", None)

                proc = subprocess.Popen([BIN_FILE, "croned"], startupinfo=info, creationflags=subprocess.CREATE_NO_WINDOW, env=env)
            except Exception as e:
                print(f"\n\n {C_RED}{texts['start_failed']} {e}{C_RESET}")
                time.sleep(2)
                
        elif key == 'X':
            if proc and proc.poll() is None:
                proc.terminate()
                proc = None
                
        elif key == '1':
            print(texts["input_add_id"], end="", flush=True)
            streamer_id = sys.stdin.readline().strip()
            if not streamer_id: continue
            
            print(texts["verifying"])
            is_valid, name = check_twitcast_id(streamer_id)
            if not is_valid:
                print(f"{C_RED}{texts['not_found']}{C_RESET}")
                time.sleep(2)
                continue
                
            print(f"{C_GREEN}{texts['fetch_success']}{name}{C_RESET}")
            print(texts["input_folder"], end="", flush=True)
            folder = sys.stdin.readline().strip()
            if not folder:
                safe_name = re.sub(r'[\/:*?"<>|]', '_', name)
                folder = f"Recordings/{safe_name}"
            
            # Create folder immediately
            try:
                os.makedirs(folder, exist_ok=True)
            except Exception:
                pass
                
            config.setdefault("streamers", []).append({
                "screen-id": streamer_id,
                "schedule": "@every 5s",
                "folder": folder,
                "enabled": True
            })
            STATUS_CACHE[streamer_id] = name
            save_config(config)
            print(f"{C_CYAN}{texts['add_success']}{folder}{C_RESET}")
            time.sleep(1.5)
            
        elif key in ['2', '3', '4', '5', '7']:
            streamers = config.get("streamers", [])
            if not streamers: continue
            
            prompt_map = {
                '2': texts["input_idx_change"],
                '3': texts["input_idx_toggle"],
                '4': texts["input_idx_remove"],
                '5': texts["input_idx_open"],
                '7': texts["input_idx_interval"],
            }
            
            print(prompt_map[key], end="", flush=True)
            try:
                val = sys.stdin.readline().strip()
                if not val: continue
                idx = int(val) - 1
                
                if 0 <= idx < len(streamers):
                    sid = streamers[idx]['screen-id']
                    if key == '2':
                        print(f"{texts['curr_setting']}{streamers[idx].get('folder', '...')}")
                        print(texts["new_folder"], end="", flush=True)
                        new_folder = sys.stdin.readline().strip()
                        if new_folder:
                            streamers[idx]["folder"] = new_folder
                            save_config(config)
                    elif key == '3':
                        curr = streamers[idx].get("enabled", True)
                        streamers[idx]["enabled"] = not curr
                        save_config(config)
                    elif key == '4':
                        print(f"{C_RED}{sid} - {texts['confirm_remove']}{C_RESET}", end="", flush=True)
                        confirm = sys.stdin.readline().strip()
                        if confirm.upper() == texts['btn_y'].upper():
                            del streamers[idx]
                            STATUS_CACHE.pop(sid, None)
                            save_config(config)
                    elif key == '5':
                        folder_path = streamers[idx].get('folder', '')
                        if not folder_path: folder_path = os.getcwd()
                        absolute_path = os.path.abspath(folder_path)
                        if os.path.exists(absolute_path):
                            os.startfile(absolute_path)
                        else:
                            print(f"{C_YELLOW}{texts['folder_not_exist']}{C_RESET}")
                            time.sleep(2)
                    elif key == '7':
                        curr = streamers[idx].get("schedule", "@every 5s").replace("@every ", "")
                        print(f"{texts['curr_interval']}{curr}")
                        print(texts["new_interval"], end="", flush=True)
                        new_val = sys.stdin.readline().strip().lower()
                        if new_val:
                            if not new_val.startswith("@every"):
                                new_val = f"@every {new_val.replace(' ', '')}"
                            streamers[idx]["schedule"] = new_val
                            save_config(config)
                else: pass
            except ValueError:
                pass
                
        elif key == '6':
            curr_lang = config.get("lang", "ZH")
            config["lang"] = "EN" if curr_lang == "ZH" else "ZH"
            save_config(config)

        elif key == '8':
            handle_discord_settings(config, texts)

        elif key == '9':
            handle_telegram_settings(config, texts)

        elif key == 'L':
            config["enable_log"] = not config.get("enable_log", False)
            save_config(config)

if __name__ == "__main__":
    main()
