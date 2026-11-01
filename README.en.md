<div align="center">

# VulnHunt · 猎洞

**A self-hosted SRC bug-bounty workbench** — pick a program, mine within its acceptance rules, triage before submission, track the payout.

[English](README.en.md) | [简体中文](README.md)

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16-black?logo=next.js)](https://nextjs.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/bzjhzjgbq/vulnhunt?style=social)](https://github.com/bzjhzjgbq/vulnhunt)

One job only: close the loop from **finding a bug → submitting it → getting paid**.

If VulnHunt saves you time filtering low-value findings, a ⭐ Star is appreciated.

</div>

## The problem

Bug-bounty work rarely fails at "finding a vulnerability". It fails at the boring parts:

- too many platforms, each with different acceptance rules, so valid work gets rejected;
- automated scanning produces piles of low-severity, duplicate or unproven findings, and triage costs more than hunting;
- submissions, statuses and payouts live in scattered spreadsheets, so you never know what you actually earned.

VulnHunt puts that whole chain in one workbench: **pick a program → launch with an authorized scope → produce to that platform's rules → double-check → submit → book the payout**.

## Features

- **Program catalog**: 100+ Chinese and international SRC programs with heat, tags and bounty data. Retired programs can be marked offline so nobody launches against a dead target.
- **Rules injected at launch**: the platform's acceptance standard, authorized scope and allow/deny constraints are written into the task prompts for both the planner and the workers.
- **Standards check**: flags findings the platform rejects (CORS/CSP/version disclosure/internal IPs…) or that are below the minimum severity, in bulk.
- **Evidence check before submission**: scores title, description, raw request/response, report structure and severity. Ready findings can be confirmed in bulk; weak ones can be ignored in bulk.
- **Payout ledger**: submission status and amount per platform, with pending/accepted/paid totals feeding the workbench.
- **Assets grouped by SRC program**: one task equals one program, so assets no longer blur together.
- **Idle auto-pause**: tasks with no new finding for N hours are paused automatically, so they stop burning model budget.
- **Workbench**: live targets, pending submissions, unresolved findings, platform opportunities and recent payouts on one screen.

## Demo

![VulnHunt demo](screenshots/vulnhunt/demo.gif)

Workbench → program catalog → acceptance rules → payout ledger → settings (finding titles are redacted in the demo).

## Screenshots

**Program catalog**

![Program catalog](screenshots/vulnhunt/src-programs.png)

**Payout ledger** (pending finding titles redacted in this demo image)

![Payout ledger](screenshots/vulnhunt/ledger.png)

**Settings**

![Settings](screenshots/vulnhunt/settings.png)

## Quick start (Windows, local)

Requirements: Go 1.26+, Node 20+, PostgreSQL 16 (Docker is fine).

```powershell
git clone https://github.com/bzjhzjgbq/vulnhunt.git
cd vulnhunt

# 1. build the frontend (static export)
cd web
npm ci
$env:NEXT_EXPORT="1"; npx next build
cd ..

# 2. embed the UI and build a single binary
Remove-Item -Recurse -Force server\webui\dist -ErrorAction SilentlyContinue
Copy-Item -Recurse web\out server\webui\dist
go build -tags embedui -trimpath -o vulnhunt.exe ./cmd/vulnhunt

# 3. configure PostgreSQL (copy and fill the DSN, or set VULNHUNT_PG_DSN)
Copy-Item config.example.json config.json

# 4. start (http://127.0.0.1:8787 by default)
.\start.bat
```

Or point it at a database with an environment variable:

```powershell
$env:VULNHUNT_PG_DSN="postgres://user:pass@127.0.0.1:5432/vulnhunt?sslmode=disable"
.\vulnhunt.exe -addr 127.0.0.1:8787
```

## Workflow

1. Pick a target in the **program catalog** and open **Standard** to see what the platform accepts and rejects.
2. Click **Launch**, enter the **authorized scope** and confirm authorization; the task is created with the standard and constraints injected.
3. Watch progress on the workbench or the task page. Every finding carries evidence and a report.
4. After a run, run the **standards check**, then the **evidence check**, and ignore what the platform rejects or what cannot be proven.
5. Export a submission-ready report for the confirmed findings, submit it on the platform, and book status + amount in the **payout ledger**.
6. The ledger feeds the workbench's pending/paid totals so you can see revenue by month.

## Architecture

- **Backend**: Go (HTTP API, task engine, planner/workers, LLM pool, intercept rules, report generation)
- **Frontend**: Next.js static export, embedded into the single binary via `go:embed` — no Node runtime in production
- **Storage**: PostgreSQL (tasks, findings, assets, LLM usage, payouts)
- **Realtime**: SSE for task activity and logs
- **Models**: OpenAI-compatible APIs (DeepSeek by default), multiple profiles with failover

## Security and compliance

- Only test targets you are **authorized** to test; launching requires an explicit scope and authorization confirmation.
- Listens on `127.0.0.1` by default. Enable access control and HTTPS before exposing it.
- Findings produced by the tool must be reviewed by a human before submission. Platform rules and legal boundaries are the operator's responsibility.

## License and upstream

VulnHunt is derived from [RuoJi6/ARTEX](https://github.com/RuoJi6/ARTEX) (original author [Autumn-27/ARTEX](https://github.com/Autumn-27/ARTEX)), licensed under **AGPL-3.0**, with the original commit history preserved.

Older deployment and development docs live in [README-UPSTREAM.md](README-UPSTREAM.md).
