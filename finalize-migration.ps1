param([switch]$NoPause)
# Finalize the migration: make D:\VulnHunt the physical folder.
# Safe by design: if the rename fails (for example because a terminal or the
# agent still uses D:\ARTEX as its working directory), the junction is restored
# and the service is started again. Every step is written to a log.
$ErrorActionPreference = 'Stop'
$link = 'D:\VulnHunt'
$real = 'D:\ARTEX'
$log = Join-Path $env:TEMP 'vulnhunt-finalize.log'

function Log($m) {
  $line = "{0} {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $m
  Add-Content -LiteralPath $log -Value $line -Encoding UTF8
  Write-Host $line
}

function Start-Service-If-Needed {
  $running = Get-Process vulnhunt -ErrorAction SilentlyContinue
  if (-not $running) {
    Start-Process -FilePath 'cmd.exe' -ArgumentList '/c "call D:\VulnHunt\start.bat >> D:\VulnHunt\data\vulnhunt.stderr.log 2>&1"' -WorkingDirectory 'D:\VulnHunt' -WindowStyle Hidden
    Log 'service launcher started'
  }
}

try {
  Log '=== finalize-migration start ==='
  if (-not (Test-Path -LiteralPath $link)) {
    Log "$link is missing; recreating the junction and aborting"
    if (Test-Path -LiteralPath $real) { cmd /c mklink /J $link $real | Out-Null }
    throw "$link was missing (restored); run again after closing every process that uses D:\ARTEX"
  }
  $item = Get-Item -LiteralPath $link -Force
  if ($item.LinkType -ne 'Junction') {
    Log "$link is already a real directory - migration is already done"
    exit 0
  }
  if (-not (Test-Path -LiteralPath $real)) { throw "$real is missing" }

  try { Disable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}
  Get-Process vulnhunt -ErrorAction SilentlyContinue | Stop-Process -Force
  Start-Sleep -Seconds 3
  Log 'service stopped'

  cmd /c rmdir $link
  if ($LASTEXITCODE -ne 0) { throw "failed to remove junction $link" }
  Log 'junction removed'

  try {
    Move-Item -LiteralPath $real -Destination $link
    Log 'folder renamed: D:\ARTEX -> D:\VulnHunt'
  } catch {
    Log ("rename failed: " + $_.Exception.Message)
    cmd /c mklink /J $link $real | Out-Null
    Log 'junction restored'
    throw 'D:\ARTEX is still in use. Close the terminal/agent that runs inside it and try again.'
  }

  cmd /c mklink /J $real $link | Out-Null
  Log 'compatibility junction created: D:\ARTEX -> D:\VulnHunt'
  try { Enable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}
  Start-Service-If-Needed
  Log 'SUCCESS: migration finalized'
} catch {
  Log ("FAILED: " + $_.Exception.Message)
  if (-not (Test-Path -LiteralPath $link) -and (Test-Path -LiteralPath $real)) {
    cmd /c mklink /J $link $real | Out-Null
    Log 'junction recreated for recovery'
  }
  try { Enable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}
  Start-Service-If-Needed
  Log 'service restored; nothing was changed'
} finally {
  if (-not $NoPause) { Read-Host 'Press Enter to close' | Out-Null }
}
