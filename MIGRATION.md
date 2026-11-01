# Finalize the folder move (run only after closing every process that uses D:\ARTEX)

VulnHunt currently runs from `D:\VulnHunt`, which is a junction to the physical
folder `D:\ARTEX`. The physical folder could not be renamed while this session
held its working directory open.

Run this script from an elevated PowerShell **after** stopping VulnHunt and
closing the terminal/IDE/agent that keeps `D:\ARTEX` as its working directory:

    powershell -NoProfile -ExecutionPolicy Bypass -File .\finalize-migration.ps1

It will:
  1. stop the "VulnHunt Boot Stack" task (if present),
  2. stop a running vulnhunt.exe,
  3. remove the D:\VulnHunt junction (the target is kept),
  4. rename D:\ARTEX to D:\VulnHunt,
  5. recreate D:\ARTEX as a junction to D:\VulnHunt for backward compatibility,
  6. print the command that starts the service again.

Nothing is deleted: step 3 removes only the junction entry, and step 4 is a rename.
