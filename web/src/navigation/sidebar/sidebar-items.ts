import {
  Brain,
  Bug,
  Crosshair,
  LayoutDashboard,
  type LucideIcon,
  Network,
  ScrollText,
  Settings2,
  ShieldAlert,
  Target,
  Wallet,
} from "lucide-react";

// 产品范围：SRC 赏金挖掘。导航只保留赚钱闭环上的页面——
// 选目标（平台库）→ 跑任务 → 看发现 → 记账，其余能力从侧栏下线
// （后端接口仍在，避免破坏任务详情里的流量/证据查看）。
export type NavBadge = "new" | "soon";

export interface NavSubItem {
  id: string;
  title: string;
  url: string;
  icon?: LucideIcon;
  badge?: NavBadge;
  disabled?: boolean;
  newTab?: boolean;
}

interface NavItemBase {
  id: string;
  title: string;
  icon?: LucideIcon;
  badge?: NavBadge;
  disabled?: boolean;
  newTab?: boolean;
}

export interface NavMainLinkItem extends NavItemBase {
  url: string;
  subItems?: never;
}

export interface NavMainParentItem extends NavItemBase {
  subItems: NavSubItem[];
}

export type NavMainItem = NavMainLinkItem | NavMainParentItem;

export interface NavGroup {
  id: number;
  label?: string;
  items: NavMainItem[];
}

export const sidebarItems: NavGroup[] = [
  {
    id: 1,
    label: "赏金",
    items: [
      { id: "workbench", title: "工作台", url: "/workbench", icon: LayoutDashboard },
      { id: "src-programs", title: "SRC 平台库", url: "/function/src", icon: Crosshair },
      { id: "tasks", title: "任务", url: "/function/tasks", icon: Target },
      { id: "findings", title: "发现", url: "/function/findings", icon: Bug },
      { id: "bounty", title: "赏金台账", url: "/function/bounty", icon: Wallet },
      { id: "assets", title: "资产", url: "/function/assets", icon: Network },
    ],
  },
  {
    id: 2,
    label: "系统",
    items: [
      { id: "llm", title: "LLM", url: "/system/llm", icon: Brain },
      { id: "intercept", title: "拦截规则", url: "/system/intercept", icon: ShieldAlert },
      { id: "logs", title: "日志", url: "/system/logs", icon: ScrollText },
      { id: "settings", title: "系统配置", url: "/system/settings", icon: Settings2 },
    ],
  },
];
