# Finalize the folder move

VulnHunt runs from `D:\VulnHunt`, which is currently a junction to the physical
folder `D:\ARTEX`. The physical folder cannot be renamed while any process keeps
`D:\ARTEX` as its working directory — including the agent/terminal that started
this session.

## How to run it

1. Close the agent/DSH app and every terminal or editor whose working directory
   is `D:\ARTEX` (check with `Get-Location` in each terminal).
2. Double-click **`finalize-migration.cmd`** (or run it from a normal PowerShell).

The `.cmd` keeps the window open, prints every step and shows the log path.

## What it does

1. disables the "VulnHunt Boot Stack" task and stops `vulnhunt.exe`;
2. removes the `D:\VulnHunt` junction (the target is kept);
3. renames `D:\ARTEX` to `D:\VulnHunt`;
4. creates `D:\ARTEX` as a junction to `D:\VulnHunt` for backward compatibility;
5. re-enables the task, starts the service and verifies it.

## If it fails

Nothing is lost:

- the junction is recreated immediately;
- the service is started again;
- the reason is written to `%TEMP%\vulnhunt-finalize.log`.

The usual reason is `Cannot move item because the item at 'D:\ARTEX' is in use` —
close the remaining process and run it again. A successful run ends with
`SUCCESS: migration finalized` in the log.
