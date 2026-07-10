@echo off
setlocal enabledelayedexpansion

set OUTPUT_DIR=%~dp0
if "%OUTPUT_DIR%" neq "" set OUTPUT_DIR=%OUTPUT_DIR:~0,-1%
set ROOT_DIR=%~dp0..

if "%1" equ "" goto usage

goto %1

:windows
echo Building Windows x86_64...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
pushd "%ROOT_DIR%"
go build -trimpath -tags "with_gvisor,embed_inject" -ldflags="-s -w" -o "%OUTPUT_DIR%\wx_channel.exe" .
if errorlevel 1 (
    popd
    echo Build failed.
    exit /b 1
)
popd
echo Done: %OUTPUT_DIR%\wx_channel.exe
exit /b 0

:windows-sunnynet
echo Building Windows SunnyNet version...
echo This requires Docker on Windows:
echo   docker run --rm -v "%%cd%%:/workspace" -w /workspace golang:1.20 bash -c "...
echo Please run the Docker command manually from README.md
exit /b 1

:usage
echo Usage: build.bat [target]
echo   windows         - Windows x86_64
echo   windows-sunnynet - Windows SunnyNet ^(requires Docker^)
echo   all             - Build all targets
exit /b 1

:all
echo Building Windows...
call :windows
echo.
echo All done!
exit /b 0
