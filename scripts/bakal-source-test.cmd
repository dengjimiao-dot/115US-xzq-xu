@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0.."
title DFO Bakal - Source Candidate Test
if not exist "server\work\dfo-lan\bin\wireprobe-handoff-source.exe" (
    echo Source candidate is missing: wireprobe-handoff-source.exe
    exit /b 1
)
echo Candidate: server\work\dfo-lan\bin\wireprobe-handoff-source.exe
echo This entry selects --source-build for the current storage route.
set DFO_SHOP_OPEN_ALL=1
set DFO_CONTRACT_PURCHASE_CRASH_FIX=1
set DFO_MAX_ITEM_PERIOD=1
set DFO_QUEST_VISIBLE_NPC_RELAX=1
set DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\storage-route.ps1" game-current --source-build %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
    echo Source candidate launch failed. See the entry log.
    pause
)
exit /b %RC%
