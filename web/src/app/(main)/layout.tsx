"use client";

import type { ReactNode } from "react";
import * as React from "react";

import { usePathname, useRouter } from "next/navigation";

import { AppSidebar } from "@/app/(main)/_components/sidebar/app-sidebar";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { auth } from "@/lib/auth";
import { api } from "@/lib/api";
import { getClientCookie } from "@/lib/cookie.client";
import {
  SIDEBAR_COLLAPSIBLE_VALUES,
  SIDEBAR_VARIANT_VALUES,
  type SidebarCollapsible,
  type SidebarVariant,
} from "@/lib/preferences/layout";
import { PREFERENCE_DEFAULTS } from "@/lib/preferences/preferences-config";
import { cn } from "@/lib/utils";

import { MainContent } from "./_components/main-content";

// Reads a layout-critical preference from the browser cookie (falls back to the
// default during static-export prerender where document is unavailable). Kept
// fully client-side so the app can be statically exported — no Server Actions /
// next/headers.
function readPref<T extends string>(key: string, allowed: readonly T[], fallback: T): T {
  if (typeof document === "undefined") return fallback;
  const value = getClientCookie(key);
  return value && (allowed as readonly string[]).includes(value) ? (value as T) : fallback;
}

export default function Layout({ children }: Readonly<{ children: ReactNode }>) {
  // Client-side auth gate — replaces the Next proxy/middleware that static export
  // disables. No token → bounce to /login; render nothing until confirmed so no
  // protected UI (or its API calls) flashes for a logged-out visitor.
  const [authed, setAuthed] = React.useState(false);
  // 已下线的产品面：直接访问旧地址会回到工作台，避免「功能还在但不该用」。
  // traffic / repeater / 任务详情 / 发现详情 不在其中——证据查看与重发仍被详情页引用。
  const pathname = usePathname();
  const router = useRouter();
  React.useEffect(() => {
    if (!pathname) return;
    const retired = [
      "/dashboard",
      "/chat",
      "/function/schedules",
      "/function/commands",
      "/function/llm-records",
      "/function/sync",
      "/function/workspace",
      "/system/agents",
      "/system/mcp",
      "/system/skills",
      "/system/tools",
      "/system/notify",
      "/system/intercept/assets",
      "/system/intercept/approvals",
    ];
    if (retired.some((p) => pathname === p || pathname.startsWith(p + "/"))) {
      router.replace("/workbench");
    }
  }, [pathname, router]);
  React.useEffect(() => {
    if (auth.getToken()) {
      setAuthed(true);
      return;
    }
    // 本机免登录部署（start.bat 里的 ARTEX_NO_LOGIN=1）：后端不再校验口令，这里
    // 静默换一个 token 再进主界面，省掉登录页那一跳。后端仍然要求登录时这个调用
    // 会拿到 401，此时不拦在这里——交给 api.ts 的 401 处理回落到 /login。
    let cancelled = false;
    void (async () => {
      try {
        const { token } = await api.login("ARTEX", "");
        if (!cancelled) {
          auth.setToken(token);
          setAuthed(true);
        }
      } catch {
        if (!cancelled) setAuthed(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const defaultOpen = typeof document === "undefined" ? true : getClientCookie("sidebar_state") !== "false";
  const variant = readPref<SidebarVariant>(
    "sidebar_variant",
    SIDEBAR_VARIANT_VALUES,
    PREFERENCE_DEFAULTS.sidebar_variant,
  );
  const collapsible = readPref<SidebarCollapsible>(
    "sidebar_collapsible",
    SIDEBAR_COLLAPSIBLE_VALUES,
    PREFERENCE_DEFAULTS.sidebar_collapsible,
  );

  if (!authed) return null;

  return (
    <SidebarProvider
      defaultOpen={defaultOpen}
      style={
        {
          "--sidebar-width": "calc(var(--spacing) * 68)",
        } as React.CSSProperties
      }
    >
      <AppSidebar variant={variant} collapsible={collapsible} />
      <SidebarInset
        className={cn(
          "[html[data-content-layout=centered]_&>*]:mx-auto",
          "[html[data-content-layout=centered]_&>*]:w-full",
          "[html[data-content-layout=centered]_&>*]:max-w-screen-2xl",
          "peer-data-[variant=inset]:border",
          "[--dashboard-header-height:--spacing(12)]",
          "min-w-0 overflow-x-hidden",
        )}
      >
        <MainContent>{children}</MainContent>
      </SidebarInset>
    </SidebarProvider>
  );
}
