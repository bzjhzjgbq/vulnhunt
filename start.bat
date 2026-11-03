@echo off
rem 控制台切 UTF-8，否则本文件里的中文在 GBK 终端下是乱码。
chcp 65001 >nul 2>&1
rem VULNHUNT 守护启动脚本（Windows）
rem
rem 用法：
rem   start.bat                  前台运行（Ctrl-C 停止）
rem   start.bat -addr :9000      额外参数原样透传给 vulnhunt
rem
rem 它只做一件事：把 vulnhunt.exe 跑起来，进程退出后按退出码决定要不要再拉起。
rem
rem   0      用户正常停止     -> 退出循环
rem   75     程序请求重启     -> 立刻重跑（页面点了"一键更新"或"回滚"）
rem   其他   崩溃             -> 退避后重跑（1->2->4…最多 60 秒）
rem
rem 下载、SHA256 校验、换装都不在这里，全部由 vulnhunt 自己在启动时完成
rem （selfupdate 包）。脚本保持傻瓜化，详见 start.sh 顶部的说明。

setlocal enabledelayedexpansion
cd /d "%~dp0"

rem ── 本机私人部署：免登录 + 只监听回环 ───────────────────────────────
rem VULNHUNT_NO_LOGIN=1 让后端不再校验 JWT 与登录口令（见 server/auth.go 里 noLogin 的
rem 注释）；配套把监听地址收回 127.0.0.1，局域网里的其他机器连不上，等于用"不可达"
rem 代替"要密码"。要恢复默认（需要登录 + 监听所有网卡），删掉这两行即可；只想换回
rem 监听所有网卡但保留免登录，把 -addr 改成 0.0.0.0:8787（或删掉 -addr 参数）。
set "VULNHUNT_NO_LOGIN=1"
set "BIND_ADDR=127.0.0.1:8787"

set "BIN=%~dp0vulnhunt.exe"
if not exist "%BIN%" (
	echo [vulnhunt] 找不到可执行文件 %BIN% 1>&2
	exit /b 1
)

set "RESTART_CODE=75"
set "MAX_DELAY=60"
set /a delay=1

:loop
"%BIN%" -addr "%BIND_ADDR%" %*
set "code=!ERRORLEVEL!"

if "!code!"=="0" (
	echo [vulnhunt] 正常退出
	exit /b 0
)

if "!code!"=="%RESTART_CODE%" (
	rem 更新/回滚已就绪：重跑后 vulnhunt 会在启动时完成换装。
	echo [vulnhunt] 请求重启（应用新版本）…
	set /a delay=1
	goto loop
)

echo [vulnhunt] 异常退出 ^(code=!code!^)，!delay!s 后重启 1>&2
rem timeout 在被重定向的控制台里会失败，用 ping 兜底（延时 N 秒需要 N+1 次）。
set /a pings=!delay!+1
ping -n !pings! 127.0.0.1 >nul 2>&1
set /a delay=!delay!*2
if !delay! gtr %MAX_DELAY% set /a delay=%MAX_DELAY%
goto loop
