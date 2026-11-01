"use client";

import * as React from "react";

import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { BountySubmission, BountySummary, Finding, SRCProgram, Task } from "@/lib/types";
import { ArrowRightIcon, BugIcon, CrosshairIcon, TargetIcon, WalletIcon } from "lucide-react";
import { toast } from "sonner";

// 赏金工作台：整个产品的默认落地页，只回答四个问题——
// 在挖什么、挖到了什么、提交了什么、赚了多少钱。

function money(cents?: number): string {
  return `¥${((cents ?? 0) / 100).toLocaleString(undefined, { maximumFractionDigits: 0 })}`;
}

function severityCN(s: string): string {
  return { critical: "严重", high: "高危", medium: "中危", low: "低危" }[s] ?? s;
}

function sevClass(s: string): string {
  switch (s) {
    case "critical":
      return "bg-red-600/10 text-red-700 dark:text-red-400";
    case "high":
      return "bg-orange-500/10 text-orange-600 dark:text-orange-400";
    case "medium":
      return "bg-amber-500/10 text-amber-600 dark:text-amber-400";
    default:
      return "bg-slate-500/10 text-slate-600 dark:text-slate-300";
  }
}

export default function WorkbenchPage() {
  const [tasks, setTasks] = React.useState<Task[]>([]);
  const [findings, setFindings] = React.useState<Finding[]>([]);
  const [summary, setSummary] = React.useState<BountySummary | null>(null);
  const [subs, setSubs] = React.useState<BountySubmission[]>([]);
  const [programs, setPrograms] = React.useState<SRCProgram[]>([]);
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const [t, f, s, b, p] = await Promise.all([
        api.tasks(),
        api.findings(),
        api.bountySummary(),
        api.bountySubmissions(),
        api.srcPrograms(),
      ]);
      setTasks(t.tasks);
      setFindings(f);
      setSummary(s);
      setSubs(b.items);
      setPrograms(p.programs);
    } catch (e) {
      toast.error(`工作台加载失败：${(e as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(timer);
  }, [load]);

  const running = tasks.filter((t) => t.status === "running" || t.status === "paused");
  const submittedFindingIDs = new Set(subs.map((s) => s.finding_id));
  const confirmed = findings.filter((f) => f.status === "confirmed");
  const pendingSubmit = confirmed.filter((f) => !submittedFindingIDs.has(Number(f.finding_id ?? f.id)));
  const pendingTriage = findings.filter((f) => f.status === "pending");
  const paidRecent = subs.filter((s) => s.status === "paid").slice(0, 5);
  const opportunities = programs.filter((p) => !p.linked_task_id).slice(0, 5);

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">赏金工作台</h1>
          <p className="text-muted-foreground text-xs">
            在测 {running.length} 个目标 · 待提交 {pendingSubmit.length} 条 · 待处理 {pendingTriage.length} 条
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" asChild>
            <Link href="/function/src">
              <CrosshairIcon className="size-4" />
              开测新项目
            </Link>
          </Button>
          <Button size="sm" variant="outline" asChild>
            <Link href="/function/findings">
              <BugIcon className="size-4" />
              查看发现
            </Link>
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Kpi icon={<WalletIcon className="size-3" />} label="已到账" value={money(summary?.paid_amount_cents)} sub="累计赏金" accent />
        <Kpi icon={<WalletIcon className="size-3" />} label="待定金额" value={money(summary?.pending_amount_cents)} sub="已提交/已受理" />
        <Kpi icon={<TargetIcon className="size-3" />} label="在测目标" value={String(running.length)} sub="运行中/暂停" />
        <Kpi icon={<BugIcon className="size-3" />} label="待提交" value={String(pendingSubmit.length)} sub="已确认、未登记" />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader className="flex-row items-center justify-between space-y-0">
            <CardTitle className="flex items-center gap-2 text-base">
              <TargetIcon className="size-4 text-primary" /> 在测目标
            </CardTitle>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/function/tasks">全部任务 <ArrowRightIcon className="size-3.5" /></Link>
            </Button>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {loading ? (
              <div className="text-muted-foreground py-6 text-center text-sm">加载中…</div>
            ) : running.length === 0 ? (
              <div className="text-muted-foreground py-6 text-center text-sm">
                还没有在测任务，去 <Link className="underline" href="/function/src">SRC 平台库</Link> 选一个开测。
              </div>
            ) : (
              running.slice(0, 6).map((t) => (
                <Link
                  key={t.id}
                  href={`/function/tasks/detail?id=${encodeURIComponent(t.id)}`}
                  className="hover:bg-muted/50 flex items-center gap-2 rounded-md px-2 py-2"
                >
                  <span className="min-w-0 flex-1 truncate text-sm">{t.name || t.description}</span>
                  <Badge variant={t.status === "paused" ? "secondary" : "default"} className="text-[10px]">
                    {t.status === "paused" ? "已暂停" : "在测"}
                  </Badge>
                  <span className="text-muted-foreground text-xs">
                    H{t.findings?.high ?? 0} / M{t.findings?.medium ?? 0}
                  </span>
                </Link>
              ))
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex-row items-center justify-between space-y-0">
            <CardTitle className="flex items-center gap-2 text-base">
              <BugIcon className="size-4 text-primary" /> 待提交发现
            </CardTitle>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/function/bounty">去登记 <ArrowRightIcon className="size-3.5" /></Link>
            </Button>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {pendingSubmit.length === 0 ? (
              <div className="text-muted-foreground py-6 text-center text-sm">
                没有已确认但未登记的漏洞。
              </div>
            ) : (
              pendingSubmit.slice(0, 6).map((f) => (
                <Link
                  key={f.id}
                  href={`/function/findings/detail?id=${encodeURIComponent(f.id)}`}
                  className="hover:bg-muted/50 flex items-center gap-2 rounded-md px-2 py-2"
                >
                  <span className={`rounded px-1.5 py-0.5 text-[10px] ${sevClass(f.severity)}`}>
                    {severityCN(f.severity)}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-sm">{f.name || f.vulnclass}</span>
                  <span className="text-muted-foreground shrink-0 text-xs">{f.ts?.slice(0, 10)}</span>
                </Link>
              ))
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex-row items-center justify-between space-y-0">
            <CardTitle className="flex items-center gap-2 text-base">
              <CrosshairIcon className="size-4 text-primary" /> 平台机会
            </CardTitle>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/function/src">全部平台 <ArrowRightIcon className="size-3.5" /></Link>
            </Button>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {opportunities.map((p) => (
              <Link
                key={p.id}
                href="/function/src"
                className="hover:bg-muted/50 flex items-center gap-2 rounded-md px-2 py-2"
              >
                <span className="min-w-0 flex-1 truncate text-sm">{p.name}</span>
                {p.analysis_has_data ? (
                  <span className="text-emerald-600 dark:text-emerald-400 text-[10px]">有赏金数据</span>
                ) : null}
                <span className="text-muted-foreground text-xs">热度 {p.heat}</span>
              </Link>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex-row items-center justify-between space-y-0">
            <CardTitle className="flex items-center gap-2 text-base">
              <WalletIcon className="size-4 text-primary" /> 最近到账
            </CardTitle>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/function/bounty">台账 <ArrowRightIcon className="size-3.5" /></Link>
            </Button>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {paidRecent.length === 0 ? (
              <div className="text-muted-foreground py-6 text-center text-sm">还没有已到账记录。</div>
            ) : (
              paidRecent.map((s) => (
                <div key={s.id} className="flex items-center gap-2 rounded-md px-2 py-2 text-sm">
                  <span className="min-w-0 flex-1 truncate">{s.finding_name || `Finding #${s.finding_id}`}</span>
                  <span className="shrink-0 text-xs text-muted-foreground">{s.platform || s.program_name}</span>
                  <span className="shrink-0 font-medium tabular-nums">{money(s.amount_cents)}</span>
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Kpi({
  icon,
  label,
  value,
  sub,
  accent,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  sub: string;
  accent?: boolean;
}) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-1 p-4">
        <span className="text-muted-foreground flex items-center gap-1 text-xs">
          {icon}
          {label}
        </span>
        <span
          className={`text-2xl font-semibold tabular-nums ${accent ? "text-amber-600 dark:text-amber-400" : ""}`}
        >
          {value}
        </span>
        <span className="text-muted-foreground text-[11px]">{sub}</span>
      </CardContent>
    </Card>
  );
}
