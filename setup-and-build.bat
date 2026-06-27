@echo off
REM ===========================================================================
REM  One Way Kafka Manager - one-click setup & build (run as Administrator)
REM
REM  Installs the BUILD prerequisites (Go + mingw-w64 gcc) and, optionally, a
REM  JDK (needed at RUNTIME by Kafka), then builds kafka-desktop.exe.
REM
REM  Just double-click this file. It will request Administrator rights itself.
REM ===========================================================================

setlocal EnableExtensions EnableDelayedExpansion

REM ---- Self-elevate to Administrator -----------------------------------------
net session >nul 2>&1
if %errorlevel% NEQ 0 (
    echo Requesting Administrator privileges...
    powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

REM ---- Work from the folder this script lives in ------------------------------
cd /d "%~dp0"
echo ===========================================================================
echo  Working directory: %CD%
echo ===========================================================================

REM ---- Check winget is available ---------------------------------------------
where winget >nul 2>&1
if %errorlevel% NEQ 0 (
    echo [ERROR] winget ^(App Installer^) was not found.
    echo         Install "App Installer" from the Microsoft Store, then re-run.
    goto :fail
)

set "WGFLAGS=-e --accept-package-agreements --accept-source-agreements --silent"

REM ===========================================================================
echo.
echo [1/6] Installing Go (if missing)...
REM ===========================================================================
where go >nul 2>&1
if %errorlevel% EQU 0 (
    echo       Go already present.
) else (
    call winget install %WGFLAGS% --id GoLang.Go
)

REM ===========================================================================
echo.
echo [2/6] Installing MSYS2 + mingw-w64 gcc (if missing)...
REM ===========================================================================
if exist "C:\msys64\ucrt64\bin\gcc.exe" (
    echo       gcc already present.
) else (
    if not exist "C:\msys64\usr\bin\pacman.exe" (
        call winget install %WGFLAGS% --id MSYS2.MSYS2
    )
    echo       Installing gcc via pacman ^(this can take a few minutes^)...
    "C:\msys64\usr\bin\pacman.exe" -Sy --noconfirm --needed mingw-w64-ucrt-x86_64-gcc
)

REM ===========================================================================
echo.
echo [3/6] Installing a JDK for running Kafka (optional but recommended)...
REM ===========================================================================
where java >nul 2>&1
if %errorlevel% EQU 0 (
    echo       Java already present.
) else (
    call winget install %WGFLAGS% --id EclipseAdoptium.Temurin.17.JDK
)

REM ---- Resolve the Go executable (PATH may not be refreshed in this session) --
set "GO="
where go >nul 2>&1 && set "GO=go"
if not defined GO if exist "%ProgramFiles%\Go\bin\go.exe" set "GO=%ProgramFiles%\Go\bin\go.exe"
if not defined GO if exist "C:\Program Files\Go\bin\go.exe" set "GO=C:\Program Files\Go\bin\go.exe"
if not defined GO (
    echo [ERROR] Go was installed but could not be located. Close this window,
    echo         open a NEW Administrator prompt and run this script again.
    goto :fail
)

REM ---- Put the C compiler on PATH and enable CGO for this build ---------------
set "PATH=C:\msys64\ucrt64\bin;%ProgramFiles%\Go\bin;%PATH%"
set "CGO_ENABLED=1"
set "CC=C:\msys64\ucrt64\bin\gcc.exe"

if not exist "C:\msys64\ucrt64\bin\gcc.exe" (
    echo [ERROR] gcc was not found at C:\msys64\ucrt64\bin\gcc.exe
    echo         MSYS2/gcc install may have failed. See REQUIREMENTS.txt.
    goto :fail
)

echo.
echo       Toolchain:
"%GO%" version
"C:\msys64\ucrt64\bin\gcc.exe" --version | findstr /i gcc

REM ===========================================================================
echo.
echo [4/6] Downloading Go dependencies (first time may take a while)...
REM ===========================================================================
"%GO%" mod download
if %errorlevel% NEQ 0 (
    echo       Download hiccup - retrying once...
    "%GO%" mod download
)

REM ===========================================================================
echo.
echo [5/6] Generating icon assets...
REM ===========================================================================
"%GO%" run ./tools/genicon
"%GO%" run github.com/akavel/rsrc@latest -ico icon.ico -arch amd64 -o icon_windows.syso
if not exist "icon_windows.syso" (
    echo       [warn] icon_windows.syso not created ^(network?^); building without
    echo              the embedded exe icon. The app/splash icon still works.
)

REM ===========================================================================
echo.
echo [6/6] Building kafka-desktop.exe (cold build can take 3-6 minutes)...
REM ===========================================================================
"%GO%" build -ldflags="-H windowsgui" -o kafka-desktop.exe .
if %errorlevel% NEQ 0 goto :buildfail
if not exist "kafka-desktop.exe" goto :buildfail

echo.
echo ===========================================================================
echo  SUCCESS! Built: %CD%\kafka-desktop.exe
echo  Run it by double-clicking kafka-desktop.exe
echo ===========================================================================
echo.
choice /C YN /M "Launch the app now"
if errorlevel 2 goto :done
start "" "%CD%\kafka-desktop.exe"
goto :done

:buildfail
echo.
echo [ERROR] Build failed. Common causes:
echo   - gcc not on PATH (close and re-run in a NEW admin prompt)
echo   - another go build/run of this project running at the same time
echo   See REQUIREMENTS.txt -> TROUBLESHOOTING.
goto :fail

:fail
echo.
echo Setup did not complete successfully.
pause
exit /b 1

:done
echo.
pause
exit /b 0
