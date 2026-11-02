"use client";

import * as React from "react";

import { Loader2Icon } from "lucide-react";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { LLMUsageBucket, LLMUsageReport } from "@/lib/types";

const RANGES = [7, 30, 90] as const;
const fmtInt = new Intl.NumberFormat("zh-CN");
const fmtCompact = new Intl.NumberFormat("zh-CN", { notation: "compact", maximumFractionDigits: 1 });

function tokens(b: LLMUsageBucket) {
  return b.input_tokens + b.output_tokens;
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className={`mt-1 text-2xl font-semibold tabular-nums ${tone ?? ""}`}>{value}</p>
      </CardContent>
    </Card>
  );
}

function UsageTable({ rows, keyLabel, showBar }: { rows: LLMUsageBucket[]; keyLabel: string; showBar?: boolean }) {
  const max = Math.max(1, ...rows.map(tokens));
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-xs text-muted-foreground">
            <th className="py-2 pr-3 font-medium">{keyLabel}</th>
            <th className="py-2 pr-3 text-right font-medium">调用</th>
            <th className="py-2 pr-3 text-right font-medium">输入</th>
            <th className="py-2 pr-3 text-right font-medium">输出</th>
            <th className="py-2 pr-3 text-right font-medium">错误</th>
            <th className="py-2 text-right font-medium">平均延迟</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.key} className="border-b last:border-0">
              <td className="max-w-[20rem] py-2 pr-3">
                <div className="truncate" title={r.key}>
                  {r.key}
                </div>
                {showBar && (
                  <div className="mt-1 h-1.5 w-full rounded-full bg-muted">
                    <div
                      className="h-full rounded-full bg-primary"
                      style={{ width: `${Math.max(2, (tokens(r) / max) * 100)}%` }}
                    />
                  </div>
                )}
              </td>
              <td className="py-2 pr-3 text-right tabular-nums">{fmtInt.format(r.calls)}</td>
              <td className="py-2 pr-3 text-right tabular-nums">{fmtInt.format(r.input_tokens)}</td>
              <td className="py-2 pr-3 text-right tabular-nums">{fmtInt.format(r.output_tokens)}</td>
              <td className={`py-2 pr-3 text-right tabular-nums ${r.errors > 0 ? "text-destructive" : ""}`}>
                {fmtInt.format(r.errors)}
              </td>
              <td className="py-2 text-right tabular-nums">{fmtInt.format(r.avg_latency_ms)} ms</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function LLMUsagePage() {
  const [days, setDays] = React.useState<number>(30);
  const [data, setData] = React.useState<LLMUsageReport | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    api
      .llmUsage(days)
      .then((report) => {
        if (!cancelled) setData(report);
      })
      .catch((e: Error) => {
        if (!cancelled) setError(e.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [days]);

  const daily = (data?.daily ?? []).map((b) => ({
    date: b.key,
    tokens: tokens(b),
    calls: b.calls,
    input: b.input_tokens,
    output: b.output_tokens,
    errors: b.errors,
  }));
  const summary = data?.summary;

  return (
    <div className="flex flex-1 flex-col gap-4 p-4 lg:p-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold">LLM 用量</h1>
          <p className="text-sm text-muted-foreground">
            来自始终开启的 <code className="font-mono text-xs">llm_usage</code> 计量表，按 UTC 日聚合。
            成本未计算：不同 provider 单价不同，这里只展示实测 token、调用、错误与延迟。
          </p>
        </div>
        <div className="flex items-center gap-1 rounded-md border p-1">
          {RANGES.map((d) => (
            <Button key={d} size="sm" variant={days === d ? "secondary" : "ghost"} onClick={() => setDays(d)}>
              {d} 天
            </Button>
          ))}
        </div>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertDescription>读取用量失败：{error}</AlertDescription>
        </Alert>
      ) : null}

      {loading && !data ? (
        <p className="text-sm text-muted-foreground">
          <Loader2Icon className="mr-1 inline size-4 animate-spin" />
          加载中…
        </p>
      ) : null}

      {summary ? (
        <>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
            <Stat label="调用次数" value={fmtInt.format(summary.calls)} />
            <Stat label="输入 tokens" value={fmtCompact.format(summary.input_tokens)} />
            <Stat label="输出 tokens" value={fmtCompact.format(summary.output_tokens)} />
            <Stat
              label="错误调用"
              value={fmtInt.format(summary.errors)}
              tone={summary.errors > 0 ? "text-destructive" : undefined}
            />
            <Stat label="平均延迟" value={`${fmtInt.format(summary.avg_latency_ms)} ms`} />
            <Stat
              label="缓存读 / 写"
              value={`${fmtCompact.format(summary.cache_read)} / ${fmtCompact.format(summary.cache_write)}`}
            />
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm">每日 token 用量</CardTitle>
            </CardHeader>
            <CardContent>
              {daily.length === 0 ? (
                <p className="text-sm text-muted-foreground">该时间范围内没有用量记录。</p>
              ) : (
                <>
                  <div role="img" aria-label="每日 token 用量折线图" className="h-[280px] w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={daily} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                        <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" vertical={false} />
                        <XAxis
                          dataKey="date"
                          tickLine={false}
                          axisLine={false}
                          tick={{ fill: "var(--muted-foreground)", fontSize: 11 }}
                          tickFormatter={(value: string) => value.slice(5)}
                        />
                        <YAxis
                          tickLine={false}
                          axisLine={false}
                          width={56}
                          tick={{ fill: "var(--muted-foreground)", fontSize: 11 }}
                          tickFormatter={(value: number) => fmtCompact.format(value)}
                        />
                        <Tooltip
                          contentStyle={{
                            background: "var(--card)",
                            border: "1px solid var(--border)",
                            borderRadius: 8,
                            fontSize: 12,
                          }}
                          formatter={(value) => [fmtInt.format(Number(value)), "tokens"]}
                          labelFormatter={(label) => String(label)}
                        />
                        <Area
                          type="monotone"
                          dataKey="tokens"
                          name="tokens"
                          stroke="var(--primary)"
                          fill="var(--primary)"
                          fillOpacity={0.18}
                          strokeWidth={2}
                        />
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                  <details className="mt-2">
                    <summary className="cursor-pointer text-xs text-muted-foreground">表格视图</summary>
                    <div className="mt-2 overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b text-left text-xs text-muted-foreground">
                            <th className="py-2 pr-3 font-medium">日期</th>
                            <th className="py-2 pr-3 text-right font-medium">调用</th>
                            <th className="py-2 pr-3 text-right font-medium">输入</th>
                            <th className="py-2 pr-3 text-right font-medium">输出</th>
                            <th className="py-2 text-right font-medium">错误</th>
                          </tr>
                        </thead>
                        <tbody>
                          {daily.map((d) => (
                            <tr key={d.date} className="border-b last:border-0">
                              <td className="py-1.5 pr-3 tabular-nums">{d.date}</td>
                              <td className="py-1.5 pr-3 text-right tabular-nums">{fmtInt.format(d.calls)}</td>
                              <td className="py-1.5 pr-3 text-right tabular-nums">{fmtInt.format(d.input)}</td>
                              <td className="py-1.5 pr-3 text-right tabular-nums">{fmtInt.format(d.output)}</td>
                              <td className="py-1.5 text-right tabular-nums">{fmtInt.format(d.errors)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </details>
                </>
              )}
            </CardContent>
          </Card>

          <div className="grid gap-4 xl:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">按模型</CardTitle>
              </CardHeader>
              <CardContent>
                <UsageTable rows={data?.by_model ?? []} keyLabel="模型" showBar />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">按 Profile</CardTitle>
              </CardHeader>
              <CardContent>
                <UsageTable rows={data?.by_profile ?? []} keyLabel="Profile" showBar />
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm">按任务</CardTitle>
            </CardHeader>
            <CardContent>
              <UsageTable rows={data?.by_task ?? []} keyLabel="任务" />
            </CardContent>
          </Card>
        </>
      ) : null}
    </div>
  );
}
