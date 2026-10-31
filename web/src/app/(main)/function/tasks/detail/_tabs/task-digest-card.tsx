"use client";

import * as React from "react";

import { AlertTriangleIcon, RefreshCwIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { TaskDigest } from "@/lib/types";

// 运行摘要卡：一次请求拿到目标/意图状态/在跑意图/最近判负/最近发现，
// 让「这条任务为什么不动了」在总览页第一屏就能回答，而不用翻几百条意图。

const MODE_LABEL: Record<string, { text: string; cls: string }> = {
  exploring: { text: "探索中", cls: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" },
  stalled: { text: "疑似卡住", cls: "bg-amber-500/10 text-amber-600 dark:text-amber-400" },
  paused: { text: "已暂停", cls: "bg-slate-500/10 text-slate-600 dark:text-slate-300" },
  idle: { text: "空闲", cls: "bg-slate-500/10 text-slate-500 dark:text-slate-400" },
};

function fmtSilent(sec?: number): string {
  if (!sec || sec <= 0) return "";
  if (sec < 60) return `${sec}s 无活动`;
  if (sec < 3600) return `${Math.floor(sec / 60)}min 无活动`;
  return `${(sec / 3600).toFixed(1)}h 无活动`;
}

function truncate(s: string, n: number): string {
  const runes = Array.from(s);
  return runes.length > n ? runes.slice(0, n).join("") + "…" : s;
}

export function TaskDigestCard({ taskId }: { taskId: string }) {
  const [digest, setDigest] = React.useState<TaskDigest | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [failed, setFailed] = React.useState(false);

  const load = React.useCallback(async () => {
    setBusy(true);
    try {
      setDigest(await api.taskDigest(taskId));
      setFailed(false);
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }, [taskId]);

  React.useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 15_000);
    return () => window.clearInterval(timer);
  }, [load]);

  if (!digest) {
    return failed ? (
      <Card>
        <CardContent className="text-muted-foreground py-3 text-xs">运行摘要暂不可用（后端未升级或任务不存在）</CardContent>
      </Card>
    ) : null;
  }

  const mode = MODE_LABEL[digest.engine_mode] ?? MODE_LABEL.idle;
  const counts = digest.intent_counts ?? {};
  const running = digest.running ?? [];
  const blocked = digest.blocked_recent ?? [];
  const findings = digest.findings_recent ?? [];

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2 space-y-0 pb-0">
        <CardTitle className="flex items-center gap-2 text-base">
          运行摘要
          <span className={`rounded px-1.5 py-0.5 text-xs font-normal ${mode.cls}`}>{mode.text}</span>
          {digest.paused && <span className="text-muted-foreground text-xs font-normal">（任务已暂停）</span>}
        </CardTitle>
        <div className="flex items-center gap-2">
          <span className="text-muted-foreground text-xs tabular-nums">
            在飞 {digest.in_flight} · 生成于 {new Date(digest.generated_at).toLocaleTimeString()}
          </span>
          <Button variant="ghost" size="icon" className="size-7" disabled={busy} onClick={() => void load()}>
            <RefreshCwIcon className={busy ? "size-3.5 animate-spin" : "size-3.5"} />
          </Button>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-3 pt-3">
        <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
          <Stat label="目标" value={`${digest.goal_met}/${digest.goals.length}`} sub="已达成/总数" />
          <Stat label="意图" value={String(counts.running ?? 0) + "/" + String(digest.queued)} sub="在跑/排队" />
          <Stat label="判负" value={String(counts.blocked ?? 0)} sub={`耗尽 ${counts.exhausted ?? 0}`} />
          <Stat label="发现" value={String(findings.length)} sub="最近 10 条内" />
        </div>

        {running.length > 0 && (
          <div className="flex flex-col gap-1">
            <div className="text-muted-foreground text-xs font-medium">正在执行</div>
            {running.slice(0, 4).map((item) => (
              <div key={item.id} className="flex items-start gap-2 text-xs">
                <span
                  className={`mt-1 size-1.5 shrink-0 rounded-full ${
                    item.has_live_work ? "bg-emerald-500" : "bg-amber-500"
                  }`}
                  title={item.has_live_work ? "有活动句柄" : "无活动句柄，可能已被回收"}
                />
                <span className="min-w-0 flex-1 truncate text-muted-foreground" title={item.summary}>
                  <span className="text-foreground">#{item.id}</span> {truncate(item.summary, 80)}
                </span>
                {item.has_live_work && item.silent_secs ? (
                  <span className="shrink-0 text-amber-600">{fmtSilent(item.silent_secs)}</span>
                ) : null}
              </div>
            ))}
          </div>
        )}

        {blocked.length > 0 && (
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
              <AlertTriangleIcon className="size-3 text-amber-500" /> 最近判负
            </div>
            {blocked.slice(0, 3).map((item) => (
              <div key={item.id} className="truncate text-xs text-muted-foreground" title={item.summary}>
                <span className="text-foreground">#{item.id}</span> {truncate(item.summary, 90)}
              </div>
            ))}
          </div>
        )}

        {findings.length > 0 && (
          <div className="flex flex-col gap-1">
            <div className="text-muted-foreground text-xs font-medium">最近发现</div>
            {findings.slice(0, 3).map((f) => (
              <div key={f.id} className="flex items-center gap-2 text-xs">
                <span className="rounded bg-muted px-1 text-[10px] uppercase">{f.severity}</span>
                <span className="truncate" title={f.name}>
                  {truncate(f.name, 70)}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div className="flex flex-col">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className="text-lg font-semibold tabular-nums">{value}</span>
      <span className="text-muted-foreground text-[10px]">{sub}</span>
    </div>
  );
}
