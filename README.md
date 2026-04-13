## **LinuxTwitcast**
Checks the live status of streamers on twitcasting.tv automatically at scheduled time, and records the live stream if it's available.

This fork uses `config.json` / `discord.json` / `telegram.json` and adds a built-in `web` mode so the project can be managed from a browser on a headless Linux or VPS host.

---

### **Disclaimer** 
This application constantly calls unofficial, non-documented twitcasting API to fetch live stream status. Please note that: 
* This application might not work in the future, subjecting to any change of twitcasting APIs 
* Checking live stream status at high frequency might result in being banned from using twitcasting service, subjecting to twitcasting's terms and condition

<span style="color:red">Please note the above and use this application at your own risk. </span>

---

### **Installation**
* **Target runtime**
  This fork is maintained for Ubuntu / Linux deployments.
* **Build from source**
  Ensure that [Go](https://go.dev/doc/install) and `ffmpeg` are installed on your system.
  ```bash
  git clone https://github.com/ExAlan7588/LinuxTwitcast.git
  cd LinuxTwitcast
  go build -o twitcast_bot .
  ```
* **Ubuntu VPS guide**
  See [docs/ubuntu-vps.md](docs/ubuntu-vps.md) for package install, startup, and service examples.

--- 

### **Usage** 
* **Croned recording mode _(default)_**  
  Please refer to [configuration](#configuration) section below to create configuration file. 
  ```Bash
  # Execute below command to start the recorder
  ./twitcast_bot

  # Or specify croned recording mode explicitly 
  ./twitcast_bot croned
  ```

* **Direct recording mode**  
  Direct recording mode supports recording to start immediately, with configurable number of retries and retry backoff period. 
  ```Bash
  # Start in direct recording mode  
  ./twitcast_bot direct --streamer=${STREAMER_SCREEN_ID}
  """
  Usage of direct:
  -retries int
    	[optional] number of retries (default 0)
  -retry-backoff duration
    	[optional] retry backoff period (default 15s)
  -streamer string
    	[required] streamer URL
  """
  # Streamer URL must be supplied as argument 

  # Example: 
  ./twitcast_bot direct --streamer=azusa_shirokyan --retries=10 --retry-backoff=1m
  ```

* **Web management mode**
  The built-in web console is useful on Ubuntu / VPS deployments where no GUI is available.
  ```Bash
  ./twitcast_bot web --addr=127.0.0.1:8080
  ```

---

### **Configuration**
  Configuration file `config.json` should be placed in the current directory under croned recording mode.  
  Streamers are stored under the `streamers` array in `config.json`.  
  Multiple streamers could be specified with individual schedules. Status check and recording for different streamers would _not_ affect each other.  

  #### Field explanations: 
  + `screen-id`:  
    Presented on the URL of the screamer's top page.  
    Example: Top page URL of streamer [小野寺梓@真っ白なキャンバス](https://twitcasting.tv/azusa_shirokyan) is `https://twitcasting.tv/azusa_shirokyan`, the corresponding screen-id is `azusa_shirokyan`
  + `schedule`:   
    Please refer to the below docs for supported schedule definitions: 
    - https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format
    - https://pkg.go.dev/github.com/robfig/cron/v3#hdr-Predefined_schedules   

---

### **Output**  
  Output recording file would be put under the current directory, named after `screen-id-yyyyMMdd-HHmm.ts`  
  For example, a recording starts at 15:04 on 2nd Jan 2006 of streamer [小野寺梓@真っ白なキャンバス](https://twitcasting.tv/azusa_shirokyan) would create recording file `azusa_shirokyan-20060102-1504.ts`
