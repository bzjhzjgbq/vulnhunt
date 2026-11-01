# Finalize the folder move

VulnHunt runs from `D:\VulnHunt`, which is currently a junction to the physical
folder `D:\ARTEX`. The physical folder can only be renamed when **no process**
holds `D:\ARTEX` open.

## Why the first attempts failed

1. The old launcher started inside `D:\VulnHunt` (= `D:\ARTEX`) and kept that as
   its working directory — it locked the very folder it wanted to rename.
   The launcher now does `cd /d %SystemRoot%` and the script does
   `Set-Location $env:TEMP` before touching anything.
2. `start.bat` is a watchdog: killing `vulnhunt.exe` made it restart the server
   immediately, and its own CWD kept the folder locked. The script now stops the
   `cmd.exe` running `start.bat` (whole process tree) first.
3. The DSH agent/terminal that started this session still has `D:\ARTEX` as its
   working directory. **It must be closed** before the rename can succeed.

## How to run

Close the DSH app and every terminal/editor opened in `D:\ARTEX`, then
double-click **`finalize-migration.cmd`**.

The window stays open, every step is printed and also written to
`%TEMP%\vulnhunt-finalize.log`.

## What it does

1. disables "VulnHunt Boot Stack", stops the watchdog and `vulnhunt.exe`;
2. removes the `D:\VulnHunt` junction (target kept);
3. renames `D:\ARTEX` to `D:\VulnHunt`;
4. creates `D:\ARTEX` as a junction to `D:\VulnHunt` for compatibility;
5. re-enables the task and starts the service.

## If it still fails

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
