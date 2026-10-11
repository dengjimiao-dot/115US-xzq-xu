@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0..\.."
if not defined VCVARS64 set "VCVARS64=C:\Program Files (x86)\Microsoft Visual Studio\2019\Community\VC\Auxiliary\Build\vcvars64.bat"
call "%VCVARS64%"
if errorlevel 1 exit /b 1
if not defined US115_SDK_VERSION set "US115_SDK_VERSION=10.0.26100.0"
if defined US115_SDK_ROOT set "INCLUDE=%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\ucrt;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\shared;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp\c\Include\%US115_SDK_VERSION%\um;%INCLUDE%"
if defined US115_SDK_ROOT set "LIB=%US115_SDK_ROOT%\microsoft.windows.sdk.cpp.x64\c\ucrt\x64;%US115_SDK_ROOT%\microsoft.windows.sdk.cpp.x64\c\um\x64;%LIB%"
if not exist "%~dp0dist" mkdir "%~dp0dist"
cd /d "%~dp0dist"
cl /std:c++17 /O2 /MT /EHsc /utf-8 /LD /I ..\src ..\src\shop115.cpp ..\src\categories.cpp /link user32.lib gdi32.lib /DEF:..\src\dinput8.def /OUT:dinput8.dll
if errorlevel 1 exit /b 1
cl /std:c++17 /O2 /MT /EHsc /utf-8 /I ..\src ..\src\test_avatar_marker.cpp /Fe:test_avatar_marker.exe
if errorlevel 1 exit /b 1
test_avatar_marker.exe
exit /b %ERRORLEVEL%
