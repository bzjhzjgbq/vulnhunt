param([switch]$NoPause)
# Finalize the migration: make D:\VulnHunt the physical folder.
# Safe by design: the script runs from a neutral working directory (so it does
# not lock D:\ARTEX itself), stops the start.bat watchdog as well as the server,
# and rolls back the junction + service if the rename fails.
$ErrorActionPreference = 'Stop'
$link = 'D:\VulnHunt'
$real = 'D:\ARTEX'
$log = Join-Path $env:TEMP 'vulnhunt-finalize.log'

# Move our own CWD out of the folder we are about to rename. Double-clicking the
# launcher starts the process inside D:\VulnHunt (which resolves to D:\ARTEX).
Set-Location $env:TEMP

function Log($m) {
  $line = "{0} {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $m
  Add-Content -LiteralPath $log -Value $line -Encoding UTF8
  Write-Host $line
}

function Stop-VulnHunt {
  try { Disable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}
  # start.bat loops and would immediately restart the server, while its own CWD
  # (D:\VulnHunt) keeps D:\ARTEX locked. Kill the whole wrapper tree first.
  $wrappers = Get-CimInstance Win32_Process -Filter "Name='cmd.exe'" |
    Where-Object { $_.CommandLine -match 'start\.bat' }
  foreach ($w in $wrappers) {
    Log "stopping watcher cmd pid=$($w.ProcessId)"
    & taskkill /PID $w.ProcessId /T /F 2>&1 | Out-Null
  }
  Get-Process vulnhunt -ErrorAction SilentlyContinue | Stop-Process -Force
  for ($i = 0; $i -lt 10; $i++) {
    if (-not (Get-Process vulnhunt -ErrorAction SilentlyContinue)) { break }
    Start-Sleep -Seconds 1
  }
  Log 'service stopped'
}

function Start-VulnHunt {
  if (Get-Process vulnhunt -ErrorAction SilentlyContinue) { return }
  Start-Process -FilePath 'cmd.exe' -ArgumentList '/c "call D:\VulnHunt\start.bat >> D:\VulnHunt\data\vulnhunt.stderr.log 2>&1"' -WorkingDirectory 'D:\VulnHunt' -WindowStyle Hidden
  Log 'service launcher started'
}

try {
  Log "=== finalize-migration start (cwd=$((Get-Location).Path)) ==="
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

  $holders = Get-Process -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -and $_.Path.StartsWith($real, [System.StringComparison]::OrdinalIgnoreCase) } |
    Select-Object -Unique Id, ProcessName, Path
  foreach ($h in $holders) { Log "process using ${real}: pid=$($h.Id) $($h.ProcessName) $($h.Path)" }

  Stop-VulnHunt

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
    throw 'D:\ARTEX is still in use. Close the agent/DSH app and any terminal opened in D:\ARTEX, then run this again.'
  }

  cmd /c mklink /J $real $link | Out-Null
  Log 'compatibility junction created: D:\ARTEX -> D:\VulnHunt'
  try { Enable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}
  Start-VulnHunt
  Log 'SUCCESS: migration finalized'
} catch {
  Log ("FAILED: " + $_.Exception.Message)
  if (-not (Test-Path -LiteralPath $link) -and (Test-Path -LiteralPath $real)) {
    cmd /c mklink /J $link $real | Out-Null
    Log 'junction recreated for recovery'
  }
  try { Enable-ScheduledTask -TaskName 'VulnHunt Boot Stack' -ErrorAction SilentlyContinue | Out-Null } catch {}
  Start-VulnHunt
  Log 'service restored; nothing was changed'
} finally {
  if (-not $NoPause) { Read-Host 'Press Enter to close' | Out-Null }
}
