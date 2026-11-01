# Finalize the folder move

VulnHunt runs from `D:\VulnHunt`, which is currently a junction to the physical
folder `D:\ARTEX`. Until the rename happens **every file inside D:\VulnHunt is
physically stored in D:\ARTEX** — that is expected.

The rename can only succeed when no process holds `D:\ARTEX` open, including:

- the DSH agent/terminal that started this session (its working directory);
- `start.bat` (a watchdog that restarts the server and keeps its CWD there);
- **the migration script itself**, if it is executed from inside `D:\ARTEX`.

## Two ways to finish

### A. Automatically at the next sign-in (recommended)

A one-shot `RunOnce` entry is already registered (current user):

```
powershell -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden
  -File "C:\VulnHuntFinalize\finalize-migration.ps1" -NoPause
```

Sign out and sign back in (or reboot). Do **not** open DSH until the migration
has finished — it takes a few seconds. Check afterwards:

```powershell
Get-Content "$env:TEMP\vulnhunt-finalize.log" -Tail 5
Invoke-WebRequest http://127.0.0.1:8787/api/health
```

To cancel the automatic run:

```powershell
Remove-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\RunOnce' VulnHuntFinalizeOnce
```

### B. Manually, after closing DSH

Close the DSH app and any terminal/editor opened in `D:\ARTEX`, then double-click
the desktop shortcut **`VulnHunt 迁移`** (it points at
`C:\VulnHuntFinalize\finalize-migration.cmd`, outside the tree).

`D:\VulnHunt\finalize-migration.cmd` also works: it copies the script to
`%TEMP%`, starts it detached and exits, so it does not hold a handle inside the
folder.

## What it does

1. disables "VulnHunt Boot Stack", stops the watchdog process tree and
   `vulnhunt.exe`;
2. removes the `D:\VulnHunt` junction (target kept);
3. renames `D:\ARTEX` to `D:\VulnHunt`;
4. creates `D:\ARTEX` as a junction to `D:\VulnHunt` for compatibility;
5. re-enables the task and starts the service.

## If it fails

Nothing is lost: the junction is recreated, the service is restarted, and the
log names the reason plus any process running from `D:\ARTEX`.

`Cannot move item ... in use` means a process still holds the folder. Find it
with Resource Monitor (CPU tab → Associated Handles → search `D:\ARTEX`) or:

```powershell
Get-Process | Where-Object { $_.Path -like 'D:\ARTEX\*' } | Select-Object Id,ProcessName,Path
```

A successful run ends with `SUCCESS: migration finalized`.

> Not finalizing is harmless: the `D:\VulnHunt` junction works exactly like a
> real folder for the app, scripts and scheduled tasks.
