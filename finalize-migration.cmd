@echo off
chcp 65001 >nul 2>&1
rem This file lives inside D:\VulnHunt (= D:\ARTEX). Running it directly would keep
rem a handle inside the folder we are about to rename, so copy the script to TEMP
rem and launch it from there after this launcher has exited.
set "SRC=%~dp0finalize-migration.ps1"
set "DST=%TEMP%\vulnhunt-finalize.ps1"
copy /y "%SRC%" "%DST%" >nul
if errorlevel 1 (
  echo Failed to copy %SRC% to %DST%
  pause
  exit /b 1
)
start "" /d "%SystemRoot%" cmd /c "timeout /t 2 >nul & powershell -NoProfile -ExecutionPolicy Bypass -File \"%DST%\" & pause"
exit /b 0
