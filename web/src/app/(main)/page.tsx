"use client";

import * as React from "react";

import { useRouter } from "next/navigation";

// 默认落地页：赏金工作台。原来的仪表盘偏「平台运维」，与挖洞赚钱无关，故下线。
export default function HomePage() {
  const router = useRouter();
  React.useEffect(() => {
    router.replace("/workbench");
  }, [router]);
  return <div className="text-muted-foreground p-8 text-sm">正在进入赏金工作台…</div>;
}
