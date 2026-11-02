"use client";

import * as React from "react";

import { Loader2Icon, PlusIcon } from "lucide-react";
import { toast } from "sonner";

import { CopyButton } from "@/components/copy-button";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import type { FindingEvidence } from "@/lib/types";

const KINDS = [
  { value: "http", label: "HTTP 原文" },
  { value: "command", label: "命令输出" },
  { value: "file", label: "文件 / 哈希" },
  { value: "screenshot", label: "截图说明" },
  { value: "text", label: "其他文本" },
] as const;

function kindLabel(kind: string) {
  return KINDS.find((k) => k.value === kind)?.label ?? kind;
}

// FindingEvidencePanel manages structured evidence that does not depend on
// captured HTTP traffic (command output, file hashes, screenshot notes).
export function FindingEvidencePanel({ findingId, readOnly }: { findingId: string; readOnly: boolean }) {
  const [items, setItems] = React.useState<FindingEvidence[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [formOpen, setFormOpen] = React.useState(false);
  const [kind, setKind] = React.useState<string>("command");
  const [label, setLabel] = React.useState("");
  const [content, setContent] = React.useState("");
  const [note, setNote] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      setItems(await api.findingEvidence(findingId));
    } catch (e) {
      toast.error("读取结构化证据失败：" + (e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [findingId]);

  React.useEffect(() => {
    void load();
  }, [load]);

  async function add() {
    if (!content.trim()) {
      setError("证据内容不能为空");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const res = await api.addFindingEvidence(findingId, { kind, label, content, note });
      setItems((cur) => [res.evidence, ...cur]);
      setLabel("");
      setContent("");
      setNote("");
      setFormOpen(false);
      toast.success("证据已添加");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  async function remove(id: number) {
    if (!window.confirm("删除这条证据？此操作不可撤销。")) return;
    try {
      await api.deleteFindingEvidence(findingId, id);
      setItems((cur) => cur.filter((e) => e.id !== id));
      toast.success("证据已删除");
    } catch (e) {
      toast.error("删除失败：" + (e as Error).message);
    }
  }

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2">
        <CardTitle className="text-sm">结构化证据</CardTitle>
        {!readOnly && (
          <Button size="sm" variant="outline" onClick={() => setFormOpen((v) => !v)}>
            <PlusIcon className="size-3.5" /> 添加证据
          </Button>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        {formOpen && (
          <div className="space-y-2 rounded-md border p-3">
            <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)]">
              <Select value={kind} onValueChange={setKind}>
                <SelectTrigger size="sm" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper">
                  <SelectGroup>
                    {KINDS.map((k) => (
                      <SelectItem key={k.value} value={k.value}>
                        {k.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="标签，例如：匿名上传回显" />
            </div>
            <Textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className="min-h-36 resize-y font-mono text-xs"
              placeholder="HTTP 原文 / 命令输出 / 文件哈希 / 截图说明"
            />
            <Input value={note} onChange={(e) => setNote(e.target.value)} placeholder="说明（可选）" />
            {error ? <p className="text-xs text-destructive">{error}</p> : null}
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="ghost" onClick={() => setFormOpen(false)} disabled={saving}>
                取消
              </Button>
              <Button size="sm" onClick={add} disabled={saving}>
                {saving ? <Loader2Icon className="size-3.5 animate-spin" /> : null}
                保存证据
              </Button>
            </div>
          </div>
        )}

        {loading ? (
          <p className="text-sm text-muted-foreground">
            <Loader2Icon className="mr-1 inline size-3 animate-spin" />
            加载中…
          </p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">暂无结构化证据。</p>
        ) : (
          items.map((ev) => (
            <div key={ev.id} className="rounded-md border p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <Badge variant="outline">{kindLabel(ev.kind)}</Badge>
                  <span className="truncate text-sm font-medium">{ev.label || "（未命名）"}</span>
                  <code className="text-[10px] text-muted-foreground" title={ev.content_hash}>
                    {ev.content_hash.slice(0, 12)}…
                  </code>
                </div>
                <div className="flex items-center gap-1">
                  <CopyButton text={ev.content} size="sm" variant="ghost" label="复制" />
                  {!readOnly && (
                    <Button size="sm" variant="ghost" className="text-destructive" onClick={() => remove(ev.id)}>
                      删除
                    </Button>
                  )}
                </div>
              </div>
              {ev.note ? <p className="mt-1 text-xs text-muted-foreground">{ev.note}</p> : null}
              <details className="mt-2">
                <summary className="cursor-pointer text-xs text-muted-foreground">查看内容</summary>
                <pre className="mt-1 max-h-60 overflow-auto rounded bg-muted p-2 font-mono text-xs whitespace-pre-wrap">
                  {ev.content}
                </pre>
              </details>
              <p className="mt-1 text-[10px] text-muted-foreground">
                {ev.author} · {new Date(ev.created_at).toLocaleString("zh-CN")}
              </p>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  );
}
