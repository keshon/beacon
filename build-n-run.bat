@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

set "MAIN_PKG=%SCRIPT_DIR%\cmd\beacon"
set "OUTPUT=%SCRIPT_DIR%\beacon.exe"

if not exist "%MAIN_PKG%\main.go" (
    echo ERROR: main.go not found in %MAIN_PKG%
    exit /b 1
)

echo [1/3] Gathering build info...

for /f "tokens=*" %%a in ('powershell -NoProfile -Command "Get-Date -Format yyyy-MM-ddTHH-mm-ssZ"') do set "BUILD_DATE=%%a"

for /f "tokens=*" %%c in ('git -C "%SCRIPT_DIR%" rev-parse --short HEAD 2^>nul') do set "GIT_COMMIT=%%c"
if "%GIT_COMMIT%"=="" set "GIT_COMMIT=none"

set "LD=-X github.com/keshon/buildinfo.Version=dev"
set "LD=%LD% -X github.com/keshon/buildinfo.Commit=%GIT_COMMIT%"
set "LD=%LD% -X github.com/keshon/buildinfo.BuildTime=%BUILD_DATE%"
set "LD=%LD% -X github.com/keshon/buildinfo.Project=Beacon"
set "LD=%LD% -X 'github.com/keshon/buildinfo.Description=Beacon - uptime monitoring service in Go'"

echo [2/3] Building...

go build -o "%OUTPUT%" -ldflags "%LD%" "%MAIN_PKG%"
if errorlevel 1 (
    echo Build failed!
    exit /b 1
)

echo [3/3] Running %OUTPUT%...

"%OUTPUT%"
set EXIT_CODE=%errorlevel%
echo.
echo Process exited with code %EXIT_CODE%. Press any key to close...
pause >nul
exit /b %EXIT_CODE%