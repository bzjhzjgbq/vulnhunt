@echo off
chcp 65001 >nul 2>&1
rem Leave D:\VulnHunt first: the launcher must not hold D:\ARTEX open itself.
cd /d "%SystemRoot%"
powershell -NoProfile -ExecutionPolicy Bypass -File "D:\VulnHunt\finalize-migration.ps1"
echo.
echo Log: %TEMP%\vulnhunt-finalize.log
pause
