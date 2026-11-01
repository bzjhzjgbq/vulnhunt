# Finalize the migration: make D:\VulnHunt the physical folder.
# Preconditions: VulnHunt stopped, and no process is using D:\ARTEX as its CWD
# (close terminals, editors and the agent that started this session first).
$ErrorActionPreference = 'Stop'

$link = 'D:\VulnHunt'
$real = 'D:\ARTEX'

$item = Get-Item -LiteralPath $link -Force -ErrorAction SilentlyContinue
if (-not $item) { throw "$link does not exist" }
if ($item.LinkType -ne 'Junction') { throw "$link is not a junction; nothing to finalize" }
if (-not (Test-Path -LiteralPath $real)) { throw "$real is missing" }

try { Disable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}
Get-Process vulnhunt -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 3

# Remove only the junction entry; the physical target stays.
cmd /c rmdir $link
if ($LASTEXITCODE -ne 0) { throw "failed to remove junction $link" }

Move-Item -LiteralPath $real -Destination $link
if (-not (Test-Path -LiteralPath $link)) { throw "rename failed" }

# Keep the old path working for scripts that still reference D:\ARTEX.
cmd /c mklink /J $real $link | Out-Null

try { Enable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}

Write-Host "Migration finalized."
Write-Host "Physical folder: $link"
Write-Host "Compatibility:   $real -> $link"
Write-Host "Start with:      cmd /c call D:\VulnHunt\start.bat"
