# Daily logical backup of the VulnHunt PostgreSQL database.
# Keeps the newest 7 dumps locally and mirrors them to E:\VulnHuntBackup when
# that volume exists. Run manually or via the scheduled tasks.
# NOTE: llm_records is a diagnostic recording table (tens of GB), so only its
# schema is backed up here. Export that table separately if ever needed.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$dir = Join-Path $root 'data\backup\db'
$mirror = 'E:\VulnHuntBackup\db'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$log = Join-Path $dir 'backup.log'
$stamp = Get-Date -Format 'yyyyMMdd-HHmm'
$container = 'vulnhunt-pg'
$tmp = "/tmp/vulnhunt-$stamp.dump"
$dest = Join-Path $dir "vulnhunt-$stamp.dump"

function Write-Log($msg) {
  $line = "{0} {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $msg
  Add-Content -LiteralPath $log -Value $line -Encoding UTF8
}

function Invoke-Native($exe, $argv) {
  $out = & $exe @argv 2>&1
  $code = $LASTEXITCODE
  if ($null -eq $code) { $code = 1 }
  if ($out -and $code -ne 0) { Add-Content -LiteralPath $log -Value ($out | Out-String).Trim() -Encoding UTF8 }
  return $code
}

function Prune-Dumps($path, $keep) {
  if (-not (Test-Path -LiteralPath $path)) { return }
  Get-ChildItem -LiteralPath $path -Filter 'vulnhunt-*.dump' |
    Sort-Object LastWriteTime -Descending |
    Select-Object -Skip $keep |
    ForEach-Object {
      Remove-Item -LiteralPath $_.FullName -Force
      Write-Log "pruned $($_.FullName)"
    }
}

try {
  Write-Log "backup start -> $dest"
  docker exec $container rm -f $tmp | Out-Null
  $code = Invoke-Native 'docker' @('exec', $container, 'pg_dump', '-U', 'vulnhunt', '-d', 'vulnhunt', '-Fc', '-Z', '6', '--exclude-table-data=llm_records', '-f', $tmp)
  if ($code -ne 0) { throw "pg_dump exited $code" }
  $code = Invoke-Native 'docker' @('exec', $container, 'pg_restore', '--list', $tmp)
  if ($code -ne 0) { throw "pg_restore --list exited $code" }
  $code = Invoke-Native 'docker' @('cp', "${container}:$tmp", $dest)
  if ($code -ne 0) { throw "docker cp exited $code" }
  docker exec $container rm -f $tmp | Out-Null
  $mb = [math]::Round((Get-Item -LiteralPath $dest).Length / 1MB, 1)
  Write-Log "backup ok ($mb MB, verified)"

  if (Test-Path -LiteralPath 'E:\') {
    New-Item -ItemType Directory -Force -Path $mirror | Out-Null
    Copy-Item -LiteralPath $dest -Destination $mirror -Force
    Prune-Dumps $mirror 7
    Write-Log "mirrored to $mirror"
  } else {
    Write-Log "mirror skipped: E: not available"
  }
  Prune-Dumps $dir 7
  exit 0
} catch {
  Write-Log "backup FAILED: $($_.Exception.Message)"
  exit 1
}
