@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0.."
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "scripts\jp-voice.ps1" -Mode Check -ClientPath "%~1"
set "patch_exit=%errorlevel%"
pause
exit /b %patch_exit%
