"use client";

import * as React from "react";

import Link from "next/link";

import { BrainIcon, NetworkIcon, ServerCogIcon, ShieldCheckIcon, WrenchIcon } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { APP_CONFIG } from "@/config/app-config";
import { api } from "@/lib/api";
import type { Settings } from "@/lib/types";

import { BasicAuthCard } from "./_components/basic-auth-card";

// 系统设置：只保留赏金挖掘真正会用到的四组配置。
// 旧版的凭据提取/情报/流量捕获/实验压缩/聊天发送模式等入口已随对应产品面下线。

export default function SystemSettingsPage() {
  const [settings, setSettings] = React.useState<Settings | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [loaded, setLoaded] = React.useState(false);

  // 挖掘
  const [workers, setWorkers] = React.useState("3");
  const [concEnabled, setConcEnabled] = React.useState(false);
  const [concLimit, setConcLimit] = React.useState("5");
  const [autoPause, setAutoPause] = React.useState("24");
  const [injectPlanner, setInjectPlanner] = React.useState(true);
  const [injectWorker, setInjectWorker] = React.useState(true);

  // 网络
  const [globalProxy, setGlobalProxy] = React.useState("");
  const [webSearch, setWebSearch] = React.useState(false);
  const [searchBackend, setSearchBackend] = React.useState("ddgs");
  const [braveKey, setBraveKey] = React.useState("");
  const [tavilyKey, setTavilyKey] = React.useState("");
  const [searchProxy, setSearchProxy] = React.useState("");

  // 维护
  const [llmRecord, setLlmRecord] = React.useState(false);
  const [pyInterp, setPyInterp] = React.useState("");

  const apply = React.useCallback((s: Settings) => {
    setSettings(s);
    setWorkers(String(s.workers ?? 3));
    setConcEnabled(!!s.task_concurrency_enabled);
    setConcLimit(String(s.task_concurrency_limit ?? 5));
    setAutoPause(String(s.auto_pause_stale_hours ?? 24));
    setInjectPlanner(s.constraints_inject_planner !== false);
    setInjectWorker(s.constraints_inject_worker !== false);
    setGlobalProxy(s.global_proxy ?? "");
    setWebSearch(!!s.web_search_enabled);
    setSearchBackend(s.web_search_backend || "ddgs");
    setSearchProxy(s.web_search_proxy ?? "");
    setLlmRecord(!!s.llm_record);
    setPyInterp(s.python_interpreter ?? "");
  }, []);

  React.useEffect(() => {
    api
      .settings()
      .then((s) => {
        apply(s);
        setLoaded(true);
      })
      .catch((e) => toast.error(`加载设置失败：${(e as Error).message}`));
  }, [apply]);

  const patch = React.useCallback(
    async (p: Partial<Settings>, message = "已保存") => {
      setSaving(true);
      try {
        const s = await api.setSettings(p);
        apply(s);
        toast.success(message);
      } catch (e) {
        toast.error(`保存失败：${(e as Error).message}`);
      } finally {
        setSaving(false);
      }
    },
    [apply],
  );

  function saveWorkers() {
    const n = Number(workers);
    if (!Number.isInteger(n) || n <= 0 || n > 50) {
      toast.error("并发 Worker 必须是 1-50 的整数");
      return;
    }
    const p: Partial<Settings> = { workers: n };
    // 多模型池 UI 已下线；存在旧池配置时一并清掉，否则并发数不会生效。
    if ((settings?.worker_pools ?? []).length > 0) p.worker_pools = [];
    void patch(p, "已保存并发配置（对之后启动的任务生效）");
  }

  function saveConcurrency(enabled: boolean, limitText = concLimit) {
    const n = Number(limitText);
    if (enabled && (!Number.isInteger(n) || n <= 0)) {
      toast.error("并发任务上限必须是正整数");
      return;
    }
    void patch({ task_concurrency_enabled: enabled, task_concurrency_limit: enabled ? n : 0 });
  }

  async function saveWebSearchKey(kind: "brave" | "tavily") {
    const key = kind === "brave" ? braveKey.trim() : tavilyKey.trim();
    if (!key) {
      toast.error("请输入 API Key");
      return;
    }
    await patch(
      kind === "brave" ? { brave_search_api_key: key } : { tavily_search_api_key: key },
      "API Key 已保存",
    );
    if (kind === "brave") setBraveKey("");
    else setTavilyKey("");
  }

  return (
    <div className="flex flex-col gap-4 p-4">
      <div>
        <h1 className="text-xl font-semibold">系统设置</h1>
        <p className="text-muted-foreground text-xs">
          只保留赏金挖掘需要的配置：并发与约束、出口代理与搜索、访问控制、维护与版本。
        </p>
      </div>

      <Tabs defaultValue="mining" className="flex flex-col gap-4">
        <TabsList variant="default" className="w-fit">
          <TabsTrigger value="mining">
            <ServerCogIcon className="size-3.5" /> 挖掘
          </TabsTrigger>
          <TabsTrigger value="network">
            <NetworkIcon className="size-3.5" /> 网络与搜索
          </TabsTrigger>
          <TabsTrigger value="access">
            <ShieldCheckIcon className="size-3.5" /> 访问控制
          </TabsTrigger>
          <TabsTrigger value="maintenance">
            <WrenchIcon className="size-3.5" /> 维护与版本
          </TabsTrigger>
        </TabsList>

        <TabsContent value="mining" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">并发</CardTitle>
              <CardDescription>Worker 数对之后启动的任务生效；已有任务用任务页的「Worker 数」单独调。</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex items-end gap-3">
                <div className="flex w-40 flex-col gap-1.5">
                  <Label htmlFor="workers">每个任务的 Worker 数</Label>
                  <Input
                    id="workers"
                    type="number"
                    min={1}
                    max={50}
                    value={workers}
                    onChange={(e) => setWorkers(e.target.value)}
                    disabled={!loaded}
                  />
                </div>
                <Button onClick={saveWorkers} disabled={!loaded || saving}>
                  保存
                </Button>
                {(settings?.worker_pools ?? []).length > 0 ? (
                  <span className="text-muted-foreground text-xs">
                    检测到旧的多模型池配置，保存后会清空（当前只有一条模型链）
                  </span>
                ) : null}
              </div>

              <div className="flex flex-wrap items-center gap-3 border-t pt-4">
                <div className="flex items-center gap-2">
                  <Switch
                    id="concurrency"
                    checked={concEnabled}
                    onCheckedChange={(v) => {
                      setConcEnabled(v);
                      if (!v) void saveConcurrency(false);
                    }}
                    disabled={!loaded}
                  />
                  <Label htmlFor="concurrency">限制同时运行的任务数</Label>
                </div>
                <div className="flex w-28 items-center gap-2">
                  <Input
                    type="number"
                    min={1}
                    value={concLimit}
                    onChange={(e) => setConcLimit(e.target.value)}
                    disabled={!loaded || !concEnabled}
                  />
                </div>
                <Button
                  variant="outline"
                  onClick={() => saveConcurrency(true)}
                  disabled={!loaded || !concEnabled || saving}
                >
                  应用上限
                </Button>
              </div>

              <div className="flex flex-wrap items-center gap-3 border-t pt-4">
                <div className="flex w-32 flex-col gap-1.5">
                  <Label htmlFor="auto-pause">无新增自动暂停</Label>
                  <Input
                    id="auto-pause"
                    type="number"
                    min={0}
                    max={720}
                    value={autoPause}
                    onChange={(e) => setAutoPause(e.target.value)}
                    disabled={!loaded}
                  />
                </div>
                <Button
                  variant="outline"
                  onClick={() => {
                    const n = Number(autoPause);
                    if (!Number.isInteger(n) || n < 0 || n > 720) {
                      toast.error("自动暂停小时数必须是 0-720 的整数（0=关闭）");
                      return;
                    }
                    void patch({ auto_pause_stale_hours: n }, n === 0 ? "已关闭自动暂停" : `已设置 ${n} 小时`);
                  }}
                  disabled={!loaded || saving}
                >
                  保存
                </Button>
                <span className="text-muted-foreground text-xs">任务连续 N 小时没有新增发现时自动暂停，0=关闭</span>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">收录标准与约束注入</CardTitle>
              <CardDescription>
                任务的 allow/deny 约束（含每次开测自动写入的 SRC 收录标准）是否拼进对应 Agent 的系统提示。
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <ToggleRow
                id="inject-planner"
                title="注入规划者"
                checked={injectPlanner}
                disabled={!loaded || saving}
                onChange={(v) => {
                  setInjectPlanner(v);
                  void patch({ constraints_inject_planner: v });
                }}
              />
              <ToggleRow
                id="inject-worker"
                title="注入执行 Worker"
                checked={injectWorker}
                disabled={!loaded || saving}
                onChange={(v) => {
                  setInjectWorker(v);
                  void patch({ constraints_inject_worker: v });
                }}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <BrainIcon className="size-4" /> 模型
              </CardTitle>
              <CardDescription>当前模型、Key 与故障转移在独立页面管理。</CardDescription>
            </CardHeader>
            <CardContent>
              <Button variant="outline" asChild>
                <Link href="/system/llm">打开 LLM 配置</Link>
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="network" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">全局出口代理</CardTitle>
              <CardDescription>
                所有目标请求走这个代理（可带 user:pass）。部分 SRC 主机从本机不可达时，换代理往往能直接打开局面。
              </CardDescription>
            </CardHeader>
            <CardContent className="flex items-end gap-3">
              <div className="flex flex-1 flex-col gap-1.5">
                <Label htmlFor="global-proxy">代理地址</Label>
                <Input
                  id="global-proxy"
                  placeholder="http://user:pass@host:port 或 socks5://host:port"
                  value={globalProxy}
                  onChange={(e) => setGlobalProxy(e.target.value)}
                  disabled={!loaded}
                />
              </div>
              <Button onClick={() => void patch({ global_proxy: globalProxy.trim() })} disabled={!loaded || saving}>
                保存
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">联网搜索</CardTitle>
              <CardDescription>用于资产发现和资料检索；独立于上面的目标代理。</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex items-center gap-2">
                <Switch
                  id="web-search"
                  checked={webSearch}
                  onCheckedChange={(v) => {
                    setWebSearch(v);
                    void patch({ web_search_enabled: v });
                  }}
                  disabled={!loaded || saving}
                />
                <Label htmlFor="web-search">启用联网搜索</Label>
              </div>

              <div className="flex flex-wrap items-center gap-3">
                <span className="text-sm">搜索后端</span>
                <Select
                  value={searchBackend}
                  onValueChange={(v) => {
                    setSearchBackend(v);
                    void patch({ web_search_backend: v });
                  }}
                  disabled={!loaded || !webSearch}
                >
                  <SelectTrigger className="w-44">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ddgs">DuckDuckGo (免 Key)</SelectItem>
                    <SelectItem value="brave-free">Brave</SelectItem>
                    <SelectItem value="tavily">Tavily</SelectItem>
                    <SelectItem value="deepseek">DeepSeek</SelectItem>
                  </SelectContent>
                </Select>
                {settings?.brave_key_set ? <Badge variant="secondary">Brave Key 已配置</Badge> : null}
                {settings?.tavily_key_set ? <Badge variant="secondary">Tavily Key 已配置</Badge> : null}
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <div className="flex items-end gap-2">
                  <div className="flex flex-1 flex-col gap-1.5">
                    <Label htmlFor="brave-key">Brave API Key</Label>
                    <Input
                      id="brave-key"
                      type="password"
                      placeholder={settings?.brave_key_set ? "已配置，留空不修改" : "粘贴 Brave Key"}
                      value={braveKey}
                      onChange={(e) => setBraveKey(e.target.value)}
                      disabled={!loaded}
                    />
                  </div>
                  <Button variant="outline" onClick={() => void saveWebSearchKey("brave")} disabled={!loaded || saving}>
                    保存
                  </Button>
                </div>
                <div className="flex items-end gap-2">
                  <div className="flex flex-1 flex-col gap-1.5">
                    <Label htmlFor="tavily-key">Tavily API Key</Label>
                    <Input
                      id="tavily-key"
                      type="password"
                      placeholder={settings?.tavily_key_set ? "已配置，留空不修改" : "粘贴 Tavily Key"}
                      value={tavilyKey}
                      onChange={(e) => setTavilyKey(e.target.value)}
                      disabled={!loaded}
                    />
                  </div>
                  <Button variant="outline" onClick={() => void saveWebSearchKey("tavily")} disabled={!loaded || saving}>
                    保存
                  </Button>
                </div>
              </div>

              <div className="flex items-end gap-3">
                <div className="flex flex-1 flex-col gap-1.5">
                  <Label htmlFor="search-proxy">搜索代理（可选）</Label>
                  <Input
                    id="search-proxy"
                    placeholder="http://host:port"
                    value={searchProxy}
                    onChange={(e) => setSearchProxy(e.target.value)}
                    disabled={!loaded}
                  />
                </div>
                <Button
                  variant="outline"
                  onClick={() => void patch({ web_search_proxy: searchProxy.trim() })}
                  disabled={!loaded || saving}
                >
                  保存
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="access" className="flex flex-col gap-4">
          <BasicAuthCard />
          <Card>
            <CardHeader>
              <CardTitle className="text-base">登录密码</CardTitle>
              <CardDescription>
                账号固定为 VULNHUNT（后端既有约定）。改密码在左下角用户菜单 → 修改密码。
              </CardDescription>
            </CardHeader>
            <CardContent className="text-muted-foreground text-sm">
              本机部署默认开启免登录（start.bat 的 VULNHUNT_NO_LOGIN=1）并只监听 127.0.0.1；对外提供服务前请关闭免登录并启用上面的访问控制。
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="maintenance" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">记录与脚本</CardTitle>
              <CardDescription>排查问题时可临时开启 LLM 录制；Python 解释器留空则自动探测。</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <ToggleRow
                id="llm-record"
                title="LLM 调用录制"
                checked={llmRecord}
                disabled={!loaded || saving}
                onChange={(v) => {
                  setLlmRecord(v);
                  void patch({ llm_record: v });
                }}
              />
              <div className="flex items-end gap-3">
                <div className="flex flex-1 flex-col gap-1.5">
                  <Label htmlFor="python">Python 解释器</Label>
                  <Input
                    id="python"
                    placeholder="留空自动探测"
                    value={pyInterp}
                    onChange={(e) => setPyInterp(e.target.value)}
                    disabled={!loaded}
                  />
                </div>
                <Button
                  variant="outline"
                  onClick={() => void patch({ python_interpreter: pyInterp.trim() })}
                  disabled={!loaded || saving}
                >
                  保存
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">版本与更新</CardTitle>
              <CardDescription>
                当前为本地构建版本 v{APP_CONFIG.version}，不提供在线更新；升级时替换 vulnhunt.exe 后重启即可（旧版本会保留为备份）。
              </CardDescription>
            </CardHeader>
            <CardContent className="text-muted-foreground text-sm">
              二进制：<span className="font-mono">vulnhunt.exe</span> · 服务：vulnhunt · 数据库：vulnhunt
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">关于</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-2 text-sm">
              <div className="flex items-center gap-2">
                <span className="font-medium">{APP_CONFIG.name}</span>
                <Badge variant="secondary">{APP_CONFIG.latinName}</Badge>
                <span className="text-muted-foreground">v{APP_CONFIG.version}</span>
              </div>
              <p className="text-muted-foreground text-xs">
                SRC 赏金挖掘工作台：平台库选目标 → 按收录标准挖洞 → 发现 → 台账记账。运行日志见
                <Link className="mx-1 underline" href="/system/logs">
                  日志页
                </Link>
                。
              </p>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function ToggleRow({
  id,
  title,
  checked,
  disabled,
  onChange,
}: {
  id: string;
  title: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <Label htmlFor={id} className="text-sm font-normal">
        {title}
      </Label>
      <Switch id={id} checked={checked} onCheckedChange={onChange} disabled={disabled} />
    </div>
  );
}
