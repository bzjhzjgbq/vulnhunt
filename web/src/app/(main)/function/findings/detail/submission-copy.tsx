"use client";

import * as React from "react";

import { toast } from "sonner";

import { CopyButton } from "@/components/copy-button";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { api } from "@/lib/api";

const TEMPLATES = [
  { value: "generic", label: "通用" },
  { value: "butian", label: "补天" },
  { value: "vulbox", label: "漏洞盒子" },
] as const;

type Template = (typeof TEMPLATES)[number]["value"];

// SubmissionCopy renders the platform-specific submission draft and copies it.
export function SubmissionCopy({ findingId }: { findingId: string }) {
  const [template, setTemplate] = React.useState<Template>("generic");
  const [markdown, setMarkdown] = React.useState("");
  const [loading, setLoading] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .findingSubmission(findingId, template)
      .then((res) => {
        if (!cancelled) setMarkdown(res.markdown);
      })
      .catch((e: Error) => {
        if (!cancelled) {
          setMarkdown("");
          toast.error("生成提交稿失败：" + e.message);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [findingId, template]);

  const label = TEMPLATES.find((t) => t.value === template)?.label ?? "通用";

  return (
    <div className="flex items-center gap-2">
      <Select value={template} onValueChange={(v) => setTemplate(v as Template)}>
        <SelectTrigger size="sm" className="h-8 w-24">
          <SelectValue />
        </SelectTrigger>
        <SelectContent position="popper" align="end">
          <SelectGroup>
            {TEMPLATES.map((t) => (
              <SelectItem key={t.value} value={t.value}>
                {t.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
      <CopyButton
        text={loading ? "" : markdown}
        label={loading ? "生成中…" : "复制提交稿"}
        successMessage={`已复制${label}提交稿`}
      />
    </div>
  );
}
