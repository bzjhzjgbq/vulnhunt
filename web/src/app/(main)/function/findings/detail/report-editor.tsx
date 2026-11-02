"use client";

import * as React from "react";

import { Loader2Icon } from "lucide-react";
import { toast } from "sonner";

import { Markdown } from "@/components/markdown";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import type { Finding, FindingRevision } from "@/lib/types";

const REPORT_TEMPLATE = `# 漏洞标题

- 漏洞等级：**中危**

## 一句话结论

## 成立前提

## 关键动作

## 影响范围
`;

type Props = {
  finding: Finding;
  onClose: () => void;
  onSaved: (finding: Finding) => void;
};

// FindingReportEditor edits the human-facing report/evidence/summary with a
// version history. The backend validates the fixed six-section report and uses
// content_version as an optimistic lock.
export function FindingReportEditor({ finding, onClose, onSaved }: Props) {
  const findingId = finding.finding_id ? finding.finding_id : finding.id;
  const [report, setReport] = React.useState(finding.report ?? "");
  const [evidence, setEvidence] = React.useState(finding.evidence ?? "");
  const [summary, setSummary] = React.useState(finding.summary ?? "");
  const [reason, setReason] = React.useState("");
  const [preview, setPreview] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");
  const [revisions, setRevisions] = React.useState<FindingRevision[]>([]);
  const [loadingHistory, setLoadingHistory] = React.useState(true);

  const loadHistory = React.useCallback(async () => {
    setLoadingHistory(true);
    try {
      setRevisions(await api.findingRevisions(findingId));
    } catch (e) {
      toast.error("读取版本历史失败：" + (e as Error).message);
    } finally {
      setLoadingHistory(false);
    }
  }, [findingId]);

  React.useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  async function save() {
    setSaving(true);
    setError("");
    try {
      const res = await api.saveFindingReport(findingId, {
        report,
        evidence,
        summary,
        content_version: finding.content_version ?? 0,
        reason: reason.trim() || undefined,
      });
      onSaved(res.finding);
      setReason("");
      toast.success(`已保存 v${res.revision.version}`);
      await loadHistory();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  function loadRevision(rev: FindingRevision) {
    setReport(rev.report);
    setEvidence(rev.evidence);
    setSummary(rev.summary);
    setReason(`从 v${rev.version} 载入`);
    setError("");
    toast.info(`已载入 v${rev.version}，保存后会生成新版本`);
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="flex max-h-[92vh] max-w-6xl flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>编辑报告</DialogTitle>
          <DialogDescription>保存时后端会校验固定六段结构，并以 content_version 防止覆盖他人编辑。</DialogDescription>
        </DialogHeader>

        <div className="grid min-h-0 flex-1 gap-3 overflow-auto lg:grid-cols-[minmax(0,1fr)_18rem]">
          <div className="flex min-w-0 flex-col gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" variant={preview ? "outline" : "secondary"} onClick={() => setPreview(false)}>
                编辑
              </Button>
              <Button size="sm" variant={preview ? "secondary" : "outline"} onClick={() => setPreview(true)}>
                预览
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setReport(REPORT_TEMPLATE)}>
                插入六段模板
              </Button>
            </div>

            {preview ? (
              <div className="min-h-[46vh] overflow-auto rounded-md border p-3">
                {report.trim() ? (
                  <Markdown text={report} />
                ) : (
                  <p className="text-sm text-muted-foreground">暂无报告。</p>
                )}
              </div>
            ) : (
              <Textarea
                value={report}
                onChange={(e) => setReport(e.target.value)}
                className="min-h-[46vh] resize-y font-mono text-xs leading-relaxed"
                placeholder={REPORT_TEMPLATE}
              />
            )}

            <div className="grid gap-3 md:grid-cols-2">
              <label htmlFor="report-evidence" className="flex flex-col gap-1 text-xs text-muted-foreground">
                证据 / PoC
                <Textarea
                  id="report-evidence"
                  value={evidence}
                  onChange={(e) => setEvidence(e.target.value)}
                  className="min-h-28 resize-y font-mono text-xs"
                />
              </label>
              <label htmlFor="report-summary" className="flex flex-col gap-1 text-xs text-muted-foreground">
                摘要
                <Textarea
                  id="report-summary"
                  value={summary}
                  onChange={(e) => setSummary(e.target.value)}
                  className="min-h-28 resize-y"
                />
              </label>
            </div>

            <label htmlFor="report-reason" className="flex flex-col gap-1 text-xs text-muted-foreground">
              修改说明（可选）
              <Input
                id="report-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="例如：补充 CDN 回读证据"
              />
            </label>
          </div>

          <aside className="flex min-h-0 flex-col gap-2 rounded-md border p-2">
            <div className="flex items-center justify-between px-1 text-xs font-medium">
              <span>版本历史</span>
              <span className="text-muted-foreground">当前 v{finding.content_version ?? 0}</span>
            </div>
            <div className="min-h-0 flex-1 space-y-2 overflow-auto">
              {loadingHistory ? (
                <p className="p-2 text-xs text-muted-foreground">
                  <Loader2Icon className="mr-1 inline size-3 animate-spin" />
                  加载中…
                </p>
              ) : revisions.length === 0 ? (
                <p className="p-2 text-xs text-muted-foreground">暂无历史版本，首次保存后生成。</p>
              ) : (
                revisions.map((rev) => (
                  <div key={rev.id} className="rounded border p-2 text-xs">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-medium">v{rev.version}</span>
                      <Button size="sm" variant="outline" className="h-6 px-2" onClick={() => loadRevision(rev)}>
                        载入
                      </Button>
                    </div>
                    <p className="mt-1 text-muted-foreground">
                      {rev.author} · {new Date(rev.created_at).toLocaleString("zh-CN")}
                    </p>
                    {rev.reason ? <p className="mt-0.5 text-muted-foreground">{rev.reason}</p> : null}
                  </div>
                ))
              )}
            </div>
          </aside>
        </div>

        {error ? <p className="text-xs text-destructive">{error}</p> : null}

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving ? <Loader2Icon className="size-4 animate-spin" /> : null}
            保存为新版本
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
