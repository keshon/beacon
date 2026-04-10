@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"
set "ROOT_DIR=%SCRIPT_DIR%\.."

set "INPUT=%ROOT_DIR%\uikit\scss\uikit.scss"
set "OUTPUT=%ROOT_DIR%\static\uikit.css"

where sass >nul 2>nul
if errorlevel 1 (
  echo ERROR: "sass" not found in PATH. Install Dart Sass.
  exit /b 1
)

if not exist "%ROOT_DIR%\static" mkdir "%ROOT_DIR%\static"

sass "%INPUT%":"%OUTPUT%" --style=expanded --source-map --embed-sources --quiet-deps
if errorlevel 1 exit /b 1

