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
cl /nologo /LD /O2 /MT /W4 /GS /utf-8 ..\src\client-host.c /Fe:ChineseLocalization.dll /link /NOIMPLIB /DEF:..\src\ChineseLocalization.def user32.lib
exit /b %ERRORLEVEL%
