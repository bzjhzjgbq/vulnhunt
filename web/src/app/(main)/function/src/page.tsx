"use client";

import * as React from "react";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import type { SRCProgram } from "@/lib/types";
import { ExternalLinkIcon, RefreshCwIcon, RocketIcon, SearchIcon } from "lucide-react";
import { toast } from "sonner";

// SRC 平台库：把 h0day 的 SRC 赏金目录接进任务流水线。
// 关键约束：只有用户勾选「已获得授权」并填写范围后才能开测，绝不自动扫。

const REGION_LABEL: Record<string, string> = { 国内: "国内", 国外: "国外" };

function heatLabel(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

export default function SRCProgramsPage() {
  const router = useRouter();
  const [programs, setPrograms] = React.useState<SRCProgram[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [syncing, setSyncing] = React.useState(false);
  const [q, setQ] = React.useState("");
  const [appliedQ, setAppliedQ] = React.useState("");
  const [region, setRegion] = React.useState("");

  const [target, setTarget] = React.useState<SRCProgram | null>(null);
  const [scopeText, setScopeText] = React.useState("");
  const [authorized, setAuthorized] = React.useState(false);
  const [launching, setLaunching] = React.useState(false);
  const [extracting, setExtracting] = React.useState(false);

  async function extractScope() {
    if (!target) return;
    setExtracting(true);
    try {
      const resp = await api.srcScopeCandidates(target.id);
      if (!resp.candidates.length) {
        toast.error("项目页没有提取到候选域名/IP，请手动填写");
        return;
      }
      setScopeText(resp.candidates.map((c) => c.value).join("\n"));
      toast.success(`从项目页提取到 ${resp.candidates.length} 条候选，请核对后再开测`);
    } catch (e) {
      toast.error(`提取失败：${(e as Error).message}`);
    } finally {
      setExtracting(false);
    }
  }

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const resp = await api.srcPrograms({ q: appliedQ, region });
      setPrograms(resp.programs);
    } catch (e) {
      toast.error(`加载 SRC 目录失败：${(e as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [appliedQ, region]);

  React.useEffect(() => {
    void load();
  }, [load]);

  async function sync() {
    setSyncing(true);
    try {
      const resp = await api.srcSync();
      const r = resp.result;
      toast.success(`同步完成：拉取 ${r.fetched}、新增 ${r.inserted}、更新 ${r.updated}`);
      await load();
    } catch (e) {
      toast.error(`同步失败：${(e as Error).message}`);
    } finally {
      setSyncing(false);
    }
  }

  function openLaunch(p: SRCProgram) {
    setTarget(p);
    setScopeText("");
    setAuthorized(false);
  }

  async function launch() {
    if (!target) return;
    const scope = scopeText
      .split(/[\n,，;；\s]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    if (scope.length === 0) {
      toast.error("请至少填写一条测试范围");
      return;
    }
    setLaunching(true);
    try {
      const resp = await api.srcLaunch(target.id, { scope, authorized });
      toast.success(`已创建任务 #${resp.task_id}，正在进入任务页`);
      setTarget(null);
      router.push(`/function/tasks/detail?id=${encodeURIComponent(resp.task_id)}`);
    } catch (e) {
      toast.error(`开测失败：${(e as Error).message}`);
    } finally {
      setLaunching(false);
    }
  }

  const launchedCount = programs.filter((p) => p.linked_task_id).length;
  const findingsTotal = programs.reduce((sum, p) => sum + (p.linked_task_findings ?? 0), 0);

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">SRC 平台库</h1>
          <p className="text-muted-foreground text-xs">
            h0day SRC 赏金目录 · 共 {programs.length} 个平台 · 已开测 {launchedCount} 个 · 已出洞{" "}
            {findingsTotal} 个 · 选中项目、填写授权范围后一键开测
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void sync()} disabled={syncing}>
          <RefreshCwIcon className={syncing ? "size-4 animate-spin" : "size-4"} />
          同步目录
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <div className="relative w-72">
          <SearchIcon className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4" />
          <Input
            className="pl-8"
            placeholder="搜索平台名 / 域名"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") setAppliedQ(q.trim());
            }}
          />
        </div>
        <Button variant="secondary" size="sm" onClick={() => setAppliedQ(q.trim())}>
          搜索
        </Button>
        <NativeSelect value={region} onChange={(e) => setRegion(e.target.value)} className="w-32">
          <NativeSelectOption value="">全部地区</NativeSelectOption>
          <NativeSelectOption value="国内">国内</NativeSelectOption>
          <NativeSelectOption value="国外">国外</NativeSelectOption>
        </NativeSelect>
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="divide-y">
            {loading ? (
              <div className="text-muted-foreground p-8 text-center text-sm">加载中…</div>
            ) : programs.length === 0 ? (
              <div className="text-muted-foreground p-8 text-center text-sm">
                暂无数据。点右上角「同步目录」从 h0day 拉取 SRC 平台列表。
              </div>
            ) : (
              programs.map((p) => (
                <div key={p.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium">{p.name}</span>
                      <Badge variant="outline" className="text-[10px]">
                        {REGION_LABEL[p.region] ?? p.region}
                      </Badge>
                      {p.status !== "normal" && (
                        <Badge variant="secondary" className="text-[10px]">
                          {p.status_label}
                        </Badge>
                      )}
                    </div>
                    <div className="text-muted-foreground mt-1 flex flex-wrap items-center gap-2 text-xs">
                      {p.target_url ? (
                        <a
                          className="inline-flex items-center gap-1 hover:underline"
                          href={p.target_url}
                          target="_blank"
                          rel="noreferrer noopener"
                        >
                          {p.host}
                          <ExternalLinkIcon className="size-3" />
                        </a>
                      ) : (
                        <span>{p.host}</span>
                      )}
                      {p.tags?.map((t) => (
                        <Badge key={t} variant="secondary" className="text-[10px]">
                          {t}
                        </Badge>
                      ))}
                      {p.analysis_has_data && (
                        <span className="text-emerald-600 dark:text-emerald-400">
                          赏金数据 · {p.analysis_latest_month}
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="text-muted-foreground w-16 text-right text-xs tabular-nums">
                    热度 {heatLabel(p.heat)}
                  </div>

                  <div className="w-36 text-right text-xs">
                    {p.linked_task_id ? (
                      <Link
                        href={`/function/tasks/detail?id=${encodeURIComponent(String(p.linked_task_id))}`}
                        className="hover:underline"
                      >
                        任务 #{p.linked_task_id}
                        {p.linked_task_status ? ` · ${p.linked_task_status}` : ""}
                        {p.linked_task_findings ? ` · ${p.linked_task_findings} 洞` : ""}
                      </Link>
                    ) : (
                      <span className="text-muted-foreground">未开测</span>
                    )}
                  </div>

                  <Button size="sm" onClick={() => openLaunch(p)}>
                    <RocketIcon className="size-4" />
                    开测
                  </Button>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>

      <Dialog open={!!target} onOpenChange={(open) => !open && setTarget(null)}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>开测 · {target?.name}</DialogTitle>
            <DialogDescription>
              将创建一条任务并写入下列测试范围。请确认你已加入该项目并被授权测试，范围外的资产不会被测试。
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3 py-1">
            <div className="text-muted-foreground text-xs">
              项目主页：
              {target?.target_url ? (
                <a className="ml-1 underline" href={target.target_url} target="_blank" rel="noreferrer noopener">
                  {target.target_url}
                </a>
              ) : (
                "—"
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium">测试范围（每行一个域名 / IP / CIDR）</span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={() => void extractScope()}
                  disabled={extracting}
                >
                  {extracting ? "提取中…" : "从项目页提取候选"}
                </Button>
              </div>
              <Textarea
                rows={5}
                placeholder={"example.com\n*.example.com\n1.2.3.4\n10.0.0.0/24"}
                value={scopeText}
                onChange={(e) => setScopeText(e.target.value)}
              />
              <span className="text-muted-foreground text-[11px]">
                支持通配域名（自动去掉 *.)、URL（自动取 host）、IP 与 CIDR；重复项会自动去重。提取结果只是线索，务必对照项目页收录范围人工核对。
              </span>
            </div>
            <label className="flex items-start gap-2 text-sm">
              <Checkbox
                checked={authorized}
                onCheckedChange={(v) => setAuthorized(v === true)}
                className="mt-0.5"
              />
              <span>
                我已加入该 SRC 项目，并确认上述范围在授权收录范围内；测试遵守该项目规则与漏洞盒子平台铁律。
              </span>
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTarget(null)} disabled={launching}>
              取消
            </Button>
            <Button onClick={() => void launch()} disabled={launching || !authorized || !scopeText.trim()}>
              {launching ? "创建中…" : "创建任务并开测"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
