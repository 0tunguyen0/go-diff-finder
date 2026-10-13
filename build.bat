@echo off
REM Build Diff Finder for Windows (requires Go + gcc on PATH)
setlocal
set CGO_ENABLED=1
where gcc >nul 2>nul
if errorlevel 1 (
  echo ERROR: gcc not found on PATH. Install it with:
  echo   winget install -e --id BrechtSanders.WinLibs.POSIX.UCRT
  echo then restart your terminal and run build.bat again.
  exit /b 1
)
go build -trimpath -ldflags "-H windowsgui -s -w" -o "DiffFinder.exe" .
echo Built DiffFinder.exe
