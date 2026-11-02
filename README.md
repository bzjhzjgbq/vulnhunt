<div align="center">

# 猎洞 VulnHunt

**SRC 赏金挖掘工作台** — 选目标、按平台收录标准挖洞、提交自检、台账记账。

[English](README.en.md) | 简体中文

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16-black?logo=next.js)](https://nextjs.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/bzjhzjgbq/vulnhunt?style=social)](https://github.com/bzjhzjgbq/vulnhunt)

只做一件事：把「SRC 挖洞 → 提交 → 到账」跑成闭环。

**English**: VulnHunt is a self-hosted SRC bug-bounty workbench — pick a program, mine within the authorized scope with each platform's acceptance rules injected, triage findings with a standards check plus an evidence check, then track submissions and payouts in one ledger.

如果这个项目帮你省下了筛选低价值漏洞的时间，点个 ⭐ Star 支持一下。

</div>

## 它解决什么问题

SRC 挖洞的痛点通常不是「找不到漏洞」，而是：

- 平台太多，每家收录标准不一样，辛苦挖出来的洞平台不收；
- 自动扫描产出大量低危 / 重复 / 证据不足的条目，筛选比挖洞还累；
- 提交、状态、金额散落在表格里，不知道这个月到底赚了多少。

猎洞把这条链路收进一个工作台：**选平台 → 授权范围开测 → 按该平台标准产出 → 双重体检 → 提交 → 记账**。

## 功能

- **SRC 平台库**：100+ 国内/国外 SRC，含热度、标签、赏金数据；已停止运营的平台可标记下线，不再误开测。
- **开测即约束**：把平台收录标准、授权范围、allow/deny 约束写进任务提示，规划者和 Worker 必须遵守。
- **标准体检**：按平台「不收」清单（CORS/CSP/版本泄露/内网 IP…）和最低等级批量筛出不该提交的发现。
- **提交自检**：检查标题、描述、原始请求/响应、报告结构、等级；就绪的可批量确认，不完整的可批量忽略。
- **赏金台账**：按平台记录提交状态与金额，汇总待收/已收/到账，回填到工作台。
- **资产按 SRC 项目分组**：一个任务对应一个项目，资产不再混在一起。
- **长跑收敛**：任务连续 N 小时没有新增发现时自动暂停，避免空烧模型额度。
- **工作台**：在测目标、待提交、待处理、平台机会、最近到账一屏看完。

## 演示

![VulnHunt 演示](screenshots/vulnhunt/demo.gif)

工作台 → SRC 平台库 → 收录标准 → 赏金台账 → 系统设置（演示图中漏洞标题已脱敏）。

## 界面

**SRC 平台库**

![SRC 平台库](screenshots/vulnhunt/src-programs.png)

**赏金台账**

![赏金台账](screenshots/vulnhunt/ledger.png)

**系统设置**

![系统设置](screenshots/vulnhunt/settings.png)

## 快速开始（Windows 本地）

依赖：Go 1.26+、Node 20+、PostgreSQL 16（可用 Docker 起一个）。

```powershell
git clone https://github.com/bzjhzjgbq/vulnhunt.git
cd vulnhunt

# 1. 构建前端（静态导出）
cd web
npm ci
$env:NEXT_EXPORT="1"; npx next build
cd ..

# 2. 内嵌前端并编译单二进制
Remove-Item -Recurse -Force server\webui\dist -ErrorAction SilentlyContinue
Copy-Item -Recurse web\out server\webui\dist
go build -tags embedui -trimpath -o vulnhunt.exe ./cmd/vulnhunt

# 3. 配置数据库（复制后填 DSN，或用环境变量 VULNHUNT_PG_DSN）
Copy-Item config.example.json config.json

# 4. 启动（默认 http://127.0.0.1:8787）
.\start.bat
```

也可以直接用环境变量指定数据库：

```powershell
$env:VULNHUNT_PG_DSN="postgres://user:pass@127.0.0.1:5432/vulnhunt?sslmode=disable"
.\vulnhunt.exe -addr 127.0.0.1:8787
```

> **首次设置管理员密码**：`/api/auth/init` 默认只接受本机（回环地址）请求。
> 通过 Docker 或对外网卡部署时，请先设置 `ADMIN_PASSWORD`（至少 8 位），服务会在
> 首次启动时自动创建管理员密码；已有密码不会被覆盖。
>
> ```powershell
> $env:ADMIN_PASSWORD="change-me-please"
> ```

## 使用流程

1. **平台库**选目标，点「标准」先看该平台收什么、不收什么；
2. 点「开测」，填写**授权范围**并确认已获授权，系统创建任务并注入标准与约束；
3. 在工作台/任务页观察进度，Worker 产出的每条发现都带证据与报告；
4. 挖掘结束后先跑**标准体检**，再跑**提交自检**，把平台不收、证据不足的条目忽略；
5. 对确认的漏洞导出提交版报告，提交到平台后在**赏金台账**登记状态与金额；
6. 台账回填工作台的「待收 / 已收」，按月看收益。

## 技术架构

- **后端**：Go（HTTP API、任务引擎、规划者/Worker、LLM 池、拦截规则、报告生成）
- **前端**：Next.js 静态导出，`go:embed` 进单二进制，无独立 Node 运行时
- **存储**：PostgreSQL（任务、发现、资产、LLM 计量、台账）
- **实时**：SSE 推送任务活动与日志
- **模型**：OpenAI 兼容接口（默认 DeepSeek），支持多配置与故障转移

## 安全与合规

- 只对**已授权**的目标做测试；开测时必须显式确认授权范围。
- 默认只监听 `127.0.0.1`，本机部署；对外提供服务前请启用访问控制与 HTTPS。
- 工具产出的发现需要人工复核后再提交，平台规则与法律边界由使用者自行负责。
- 本机的真实漏洞报告、资产/发现/赏金截图和 PoC 默认已加入 `.gitignore`；不要用
  `git add -A`，只提交明确脱敏后的文件。
- 提交前可启用密钥扫描钩子（需安装 [gitleaks](https://github.com/gitleaks/gitleaks)）：

  ```bash
  git config core.hooksPath .githooks
  ```

  CI 的 `secret-scan` job 也会扫描完整提交历史。

## 许可证与上游

本项目基于 [RuoJi6/ARTEX](https://github.com/RuoJi6/ARTEX)（原始作者 [Autumn-27/ARTEX](https://github.com/Autumn-27/ARTEX)）改造，遵循 **AGPL-3.0**，保留原始提交历史。

早期部署与开发文档见 [README-UPSTREAM.md](README-UPSTREAM.md)。
