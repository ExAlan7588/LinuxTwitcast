@echo off
setlocal enabledelayedexpansion

REM Check if FFmpeg is installed
where ffmpeg >nul 2>nul
if %errorlevel% neq 0 (
    echo Error: FFmpeg not found. Please make sure FFmpeg is installed and added to your PATH.
    echo You can download FFmpeg from https://ffmpeg.org/download.html
    echo After installation, add the FFmpeg bin directory to your system PATH.
    pause
    exit /b 1
)

echo Starting audio extraction...
echo Will extract audio from all MP4 and TS files with highest quality to M4A format

REM Create output folder
if not exist ".\m4a_output" mkdir ".\m4a_output"

REM Process all MP4 files
echo Processing MP4 files...
for /r %%i in (*.mp4) do (
    echo Processing: "%%i"
    ffmpeg -i "%%i" -vn -c:a copy ".\m4a_output\%%~ni.m4a" -y
)

REM Process all TS files
echo Processing TS files...
for /r %%i in (*.ts) do (
    echo Processing: "%%i"
    ffmpeg -i "%%i" -vn -c:a copy ".\m4a_output\%%~ni.m4a" -y
)

echo Done! All audio files have been extracted to the m4a_output folder
pause
