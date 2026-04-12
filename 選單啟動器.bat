@echo off
setlocal
pushd "%~dp0"
title TwitCasting 自動控制與歸檔中心
where uv >nul 2>nul
if %errorlevel% neq 0 (
    echo Error: uv not found. Install uv first:
    echo https://docs.astral.sh/uv/getting-started/installation/
    popd
    pause
    exit /b 1
)

uv run launcher.py
set "exit_code=%errorlevel%"
popd
pause
exit /b %exit_code%
