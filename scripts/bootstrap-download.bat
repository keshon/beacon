@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"
set "ROOT_DIR=%SCRIPT_DIR%\.."

set "BS_VERSION=5.3.3"
set "VENDOR_DIR=%ROOT_DIR%\uikit\vendor\bootstrap"

if exist "%VENDOR_DIR%\scss" (
  echo Bootstrap %BS_VERSION% SCSS already present.
  exit /b 0
)

echo Downloading Bootstrap %BS_VERSION% SCSS source...

if not exist "%VENDOR_DIR%" mkdir "%VENDOR_DIR%"

powershell -NoProfile -Command ^
  "$tmp = [System.IO.Path]::GetTempFileName() + '.tar.gz'; " ^
  "Invoke-WebRequest -Uri 'https://github.com/twbs/bootstrap/archive/refs/tags/v%BS_VERSION%.tar.gz' -OutFile $tmp; " ^
  "tar xzf $tmp --strip-components=1 -C '%VENDOR_DIR%' 'bootstrap-%BS_VERSION%/scss'; " ^
  "Remove-Item $tmp"

if errorlevel 1 (
  echo Download failed!
  exit /b 1
)

echo Done: %VENDOR_DIR%\scss
