"use client";

import * as React from "react";

import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import type { BountySubmission, BountySummary, Finding, SRCProgram } from "@/lib/types";
import { PlusIcon, Trash2Icon, WalletIcon } from "lucide-react";
import { toast } from "sonner";

// 赏金台账：每条 = 一次「漏洞 → 平台」提交；金额以分为单位存储，展示为元。

const STATUS_LABEL: Record<string, string> = {
  submitted: "已提交",
  accepted: "已受理",
  rejected: "已拒绝",
  duplicate: "重复",
  paid: "已到账",
  withdrawn: "已撤回",
};

const STATUS_CLS: Record<string, string> = {
  submitted: "bg-sky-500/10 text-sky-600 dark:text-sky-400",
  accepted: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  rejected: "bg-red-500/10 text-red-600 dark:text-red-400",
  duplicate: "bg-slate-500/10 text-slate-600 dark:text-slate-300",
  paid: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
  withdrawn: "bg-slate-500/10 text-slate-500",
};

function money(cents?: number): string {
  const n = (cents ?? 0) / 100;
  return `¥${n.toLocaleString(undefined, { maximumFractionDigits: 2 })}`;
}

type FormState = {
  finding_id: string;
  program_id: string;
  platform: string;
  external_id: string;
  status: string;
  severity: string;
  amount_yuan: string;
  note: string;
};

const EMPTY_FORM: FormState = {
  finding_id: "",
  program_id: "",
  platform: "",
  external_id: "",
  status: "submitted",
  severity: "",
  amount_yuan: "",
  note: "",
};

export default function BountyPage() {
  const [items, setItems] = React.useState<BountySubmission[]>([]);
  const [summary, setSummary] = React.useState<BountySummary | null>(null);
  const [programs, setPrograms] = React.useState<SRCProgram[]>([]);
  const [findings, setFindings] = React.useState<Finding[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [open, setOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<BountySubmission | null>(null);
  const [form, setForm] = React.useState<FormState>(EMPTY_FORM);
  const [saving, setSaving] = React.useState(false);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const [list, sum, fs] = await Promise.all([
        api.bountySubmissions(),
        api.bountySummary(),
        api.findings(),
      ]);
      setItems(list.items);
      setSummary(sum);
      setFindings(fs);
    } catch (e) {
      toast.error(`加载赏金台账失败：${(e as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  React.useEffect(() => {
    api
      .srcPrograms()
      .then((r) => setPrograms(r.programs))
      .catch(() => setPrograms([]));
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(EMPTY_FORM);
    setOpen(true);
  }

  function openEdit(row: BountySubmission) {
    setEditing(row);
    setForm({
      finding_id: String(row.finding_id),
      program_id: row.program_id ? String(row.program_id) : "",
      platform: row.platform,
      external_id: row.external_id,
      status: row.status,
      severity: row.severity,
      amount_yuan: row.amount_cents ? String(row.amount_cents / 100) : "",
      note: row.note ?? "",
    });
    setOpen(true);
  }

  async function save() {
    const findingID = Number(form.finding_id);
    if (!Number.isInteger(findingID) || findingID <= 0) {
      toast.error("请填写有效的 finding id");
      return;
    }
    const amount = Number(form.amount_yuan || "0");
    if (!Number.isFinite(amount) || amount < 0) {
      toast.error("赏金金额无效");
      return;
    }
    setSaving(true);
    try {
      const body = {
        finding_id: findingID,
        program_id: form.program_id ? Number(form.program_id) : undefined,
        platform: form.platform.trim(),
        external_id: form.external_id.trim(),
        status: form.status as BountySubmission["status"],
        severity: form.severity.trim(),
        amount_cents: Math.round(amount * 100),
        currency: "CNY",
        note: form.note.trim(),
      };
      if (editing) {
        await api.updateBounty(editing.id, body);
        toast.success("已更新提交记录");
      } else {
        await api.createBounty(body);
        toast.success("已登记提交记录");
      }
      setOpen(false);
      await load();
    } catch (e) {
      toast.error(`保存失败：${(e as Error).message}`);
    } finally {
      setSaving(false);
    }
  }

  async function remove(row: BountySubmission) {
    if (!window.confirm(`删除 finding #${row.finding_id} 的这条提交记录？`)) return;
    try {
      await api.deleteBounty(row.id);
      toast.success("已删除");
      await load();
    } catch (e) {
      toast.error(`删除失败：${(e as Error).message}`);
    }
  }

  const submittedIDs = new Set(items.map((i) => i.finding_id));
  const pendingSubmit = findings.filter(
    (f) => f.status === "confirmed" && !submittedIDs.has(Number(f.finding_id ?? f.id)),
  );

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">赏金台账</h1>
          <p className="text-muted-foreground text-xs">
            记录每次「漏洞 → 平台」的提交结果与金额；金额按人民币元展示，后台按分存储。
          </p>
        </div>
        <Button size="sm" onClick={openCreate}>
          <PlusIcon className="size-4" />
          登记提交
        </Button>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard label="提交总数" value={String(summary?.total ?? 0)} sub="全部平台条目" />
        <StatCard label="待定金额" value={money(summary?.pending_amount_cents)} sub="已提交/已受理" />
        <StatCard label="已受理金额" value={money(summary?.accepted_amount_cents)} sub="含已到账" />
        <StatCard label="已到账金额" value={money(summary?.paid_amount_cents)} sub="实际收款" accent />
      </div>

      {pendingSubmit.length > 0 && (
        <Card>
          <CardContent className="flex flex-col gap-2 p-4">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">待登记的已确认漏洞（{pendingSubmit.length}）</span>
              <span className="text-muted-foreground text-xs">先去发现页确认，再回到这里登记提交结果</span>
            </div>
            {pendingSubmit.slice(0, 8).map((f) => (
              <div key={f.id} className="flex items-center gap-2 text-sm">
                <span className="text-muted-foreground w-12 shrink-0 text-xs uppercase">{f.severity}</span>
                <Link
                  className="min-w-0 flex-1 truncate hover:underline"
                  href={`/function/findings/detail?id=${encodeURIComponent(f.id)}`}
                >
                  {f.name || f.vulnclass}
                </Link>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setEditing(null);
                    setForm({ ...EMPTY_FORM, finding_id: String(f.finding_id ?? f.id) });
                    setOpen(true);
                  }}
                >
                  登记
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="p-0">
          <div className="divide-y">
            {loading ? (
              <div className="text-muted-foreground p-8 text-center text-sm">加载中…</div>
            ) : items.length === 0 ? (
              <div className="text-muted-foreground p-8 text-center text-sm">
                还没有记录。点右上角「登记提交」把已提交的漏洞登记进来。
              </div>
            ) : (
              items.map((row) => (
                <div key={row.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <Link
                        href={`/function/findings/detail?id=${encodeURIComponent(String(row.finding_id))}`}
                        className="truncate text-sm font-medium hover:underline"
                        title={row.finding_name}
                      >
                        {row.finding_name || `Finding #${row.finding_id}`}
                      </Link>
                      <Badge className={`text-[10px] ${STATUS_CLS[row.status] ?? ""}`}>
                        {STATUS_LABEL[row.status] ?? row.status}
                      </Badge>
                    </div>
                    <div className="text-muted-foreground mt-1 flex flex-wrap items-center gap-2 text-xs">
                      <span>#{row.finding_id}</span>
                      {row.program_name ? <span>· {row.program_name}</span> : null}
                      {row.platform ? <span>· {row.platform}</span> : null}
                      {row.external_id ? <span>· 单号 {row.external_id}</span> : null}
                      {row.task_id ? (
                        <Link
                          className="underline"
                          href={`/function/tasks/detail?id=${encodeURIComponent(String(row.task_id))}`}
                        >
                          · 任务 #{row.task_id}
                        </Link>
                      ) : null}
                      {row.note ? <span className="truncate">· {row.note}</span> : null}
                    </div>
                  </div>
                  <div className="w-24 text-right text-sm tabular-nums">{money(row.amount_cents)}</div>
                  <div className="w-40 text-right text-xs text-muted-foreground">
                    {row.submitted_at ? new Date(row.submitted_at).toLocaleDateString() : "—"}
                  </div>
                  <div className="flex items-center gap-1">
                    <Button variant="ghost" size="sm" onClick={() => openEdit(row)}>
                      编辑
                    </Button>
                    <Button variant="ghost" size="icon" className="size-8" onClick={() => void remove(row)}>
                      <Trash2Icon className="size-4" />
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>

      {summary && summary.by_program.length > 0 && (
        <Card>
          <CardContent className="flex flex-col gap-2 p-4">
            <div className="text-sm font-medium">按平台汇总</div>
            {summary.by_program.map((p) => (
              <div key={`${p.program_id ?? 0}-${p.program_name}`} className="flex items-center gap-3 text-xs">
                <span className="w-56 truncate">{p.program_name}</span>
                <span className="text-muted-foreground">提交 {p.total}</span>
                <span className="text-muted-foreground">受理 {money(p.accepted_cents)}</span>
                <span className="font-medium">到账 {money(p.paid_cents)}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editing ? "编辑提交记录" : "登记提交记录"}</DialogTitle>
            <DialogDescription>
              记录一条「漏洞提交到平台」的结果；金额按元填写，可后续修改状态与金额。
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-3 py-1">
            <label className="flex flex-col gap-1 text-xs">
              Finding ID
              <Input
                value={form.finding_id}
                onChange={(e) => setForm({ ...form, finding_id: e.target.value })}
                placeholder="如 370"
              />
            </label>
            <label className="flex flex-col gap-1 text-xs">
              关联平台
              <NativeSelect
                value={form.program_id}
                onChange={(e) => setForm({ ...form, program_id: e.target.value })}
              >
                <NativeSelectOption value="">未关联</NativeSelectOption>
                {programs.map((p) => (
                  <NativeSelectOption key={p.id} value={String(p.id)}>
                    {p.name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </label>
            <label className="flex flex-col gap-1 text-xs">
              提交状态
              <NativeSelect
                value={form.status}
                onChange={(e) => setForm({ ...form, status: e.target.value })}
              >
                {Object.entries(STATUS_LABEL).map(([k, v]) => (
                  <NativeSelectOption key={k} value={k}>
                    {v}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </label>
            <label className="flex flex-col gap-1 text-xs">
              赏金金额（元）
              <Input
                value={form.amount_yuan}
                onChange={(e) => setForm({ ...form, amount_yuan: e.target.value })}
                placeholder="如 1500"
              />
            </label>
            <label className="flex flex-col gap-1 text-xs">
              平台名称
              <Input
                value={form.platform}
                onChange={(e) => setForm({ ...form, platform: e.target.value })}
                placeholder="补天 / 漏洞盒子 / 厂商 SRC"
              />
            </label>
            <label className="flex flex-col gap-1 text-xs">
              平台单号
              <Input
                value={form.external_id}
                onChange={(e) => setForm({ ...form, external_id: e.target.value })}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs">
              平台评级
              <Input
                value={form.severity}
                onChange={(e) => setForm({ ...form, severity: e.target.value })}
                placeholder="高危 / 中危"
              />
            </label>
            <label className="col-span-2 flex flex-col gap-1 text-xs">
              备注
              <Textarea
                rows={3}
                value={form.note}
                onChange={(e) => setForm({ ...form, note: e.target.value })}
                placeholder="受理进度、沟通记录、驳回原因等"
              />
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={saving}>
              取消
            </Button>
            <Button onClick={() => void save()} disabled={saving || !form.finding_id}>
              {saving ? "保存中…" : "保存"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function StatCard({
  label,
  value,
  sub,
  accent,
}: {
  label: string;
  value: string;
  sub: string;
  accent?: boolean;
}) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-1 p-4">
        <span className="text-muted-foreground flex items-center gap-1 text-xs">
          <WalletIcon className="size-3" />
          {label}
        </span>
        <span className={`text-xl font-semibold tabular-nums ${accent ? "text-amber-600 dark:text-amber-400" : ""}`}>
          {value}
        </span>
        <span className="text-muted-foreground text-[11px]">{sub}</span>
      </CardContent>
    </Card>
  );
}
