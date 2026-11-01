# Folder layout (final)

Copy-based migration completed on **2026-11-01 17:56**:

```
D:\VulnHunt   ← real directory, 36.4 GB, the service runs from here
D:\ARTEX      ← old snapshot taken at 17:50, kept only as a rollback copy
```

- All files were copied with `robocopy /E /COPY:DAT`; file counts matched
  (756,469) and `config.json` / `vulnhunt.exe` hashes and sizes were verified.
- `D:\VulnHunt` is **no longer a junction**. Scheduled tasks, the service,
  backups and the Git repository all use it directly.
- `D:\ARTEX` is now **stale**: it stopped receiving writes when the service was
  restarted from the new tree. Do not run the app or edit code there.

## Deleting D:\ARTEX

Close DSH/this agent session first — its working directory is still
`D:\ARTEX`, which is the only reason the folder is locked. After that you can
delete it normally:

```powershell
Remove-Item D:\ARTEX -Recurse -Force
```

Check `Get-Process | Where-Object { $_.Path -like 'D:\ARTEX\*' }` first; it
should be empty (the service now runs from `D:\VulnHunt`).

Before deleting, confirm the new tree is healthy:

```powershell
Invoke-WebRequest http://127.0.0.1:8787/api/health
cd D:\VulnHunt; git status
```

## Note for DSH / editors

Open the project as `D:\VulnHunt` from now on. The old `D:\ARTEX` path is a
snapshot and will be removed.
