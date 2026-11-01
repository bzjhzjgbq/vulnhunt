# Folder layout (final)

Decision: **the migration is complete without renaming the physical folder.**
`D:\VulnHunt` stays a junction to `D:\ARTEX`, and that is intentional.

```
D:\VulnHunt  ──junction──▶  D:\ARTEX   (physical files)
```

## Why

Everything user-visible was already migrated: the binary (`vulnhunt.exe`), the
service, the database (`vulnhunt` role and database), the Go module, environment
variables (`VULNHUNT_*`), scripts and scheduled tasks all use the VulnHunt name
and the `D:\VulnHunt` path.

Windows refuses to rename a directory while any process holds it open, and the
folder is held by the running agent session, the `start.bat` watchdog, the Boot
Stack task and Explorer at various times. Renaming adds no functional value and
repeated attempts only risk stopping the service, so the junction is the final
state.

## What still works

- `D:\VulnHunt\vulnhunt.exe` and `start.bat` — yes (junction is transparent);
- scheduled tasks and shortcuts pointing at `D:\VulnHunt` — yes;
- Git repository — yes;
- backups, logs, config — yes.

## If you ever really need a physical folder

The scripts are kept in the repo for reference:
`finalize-migration.ps1` / `finalize-migration.cmd`. They run the rename with
rollback, but must be started from outside `D:\ARTEX` and while the agent, DSH,
Explorer and the watchdog are all closed. Running them is optional and not
required for the product.
