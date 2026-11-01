# ARTEX 开机自举：按依赖顺序拉起 Docker Desktop -> artex-pg -> artex.exe 守护。
#
# 为什么需要它：手工启动链有三层依赖（Docker 守护 -> Postgres 容器 -> 后端），
# 顺序错了就会出现「artex 起来了但连不上库」；而且从别的会话里 Start-Process
# 拉起的进程会被连带杀掉，所以本脚本只负责把栈按序拉齐，由计划任务在登录时调用。
#
# 幂等：artex 已在 8787 上跑就直接退出，不会起第二个实例。
# 日志：追加到 data\boot-stack.log，不覆盖。

$ErrorActionPreference = 'SilentlyContinue'
$root = 'D:\ARTEX'
$log  = Join-Path $root 'data\boot-stack.log'

# LLM 调用记录保留天数。每条记录含完整提示词与原始响应（实测均 223KB），
# 实测增速约 1359 条/小时 ≈ 7.3GB/天，而引擎自身没有任何自动清理 ——
# 无限增长最终会填满磁盘。这里由看门狗顺带按天裁剪。
$llmRecordKeepDays = 7

function Log([string]$m) {
    "$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') [boot] $m" | Add-Content -Path $log -Encoding UTF8
}

# 按天裁剪 LLM 调用记录（llm_records）。
# 为什么放在看门狗里：引擎没有自带的保留策略，而本脚本每 5 分钟被触发一次，
# 是最省事的现成定时器。必须放在下面那道「已在运行则 return」守卫【之前】，
# 否则 artex 健康时脚本会提前返回，清理永远不执行。
function Invoke-LLMRecordRetention {
    if ($llmRecordKeepDays -le 0) { return }
    # 水位安全阀：完整 raw_request/raw_response 是取证的唯一来源，正常保留 N 天；
    # 但若保留策略本身失效（看门狗任务损坏等），这两级水位保证磁盘永不被填满。
    $sz = docker exec artex-pg psql -U artex -d artex -t -A -c "SELECT pg_total_relation_size('llm_records');" 2>$null
    if ("$sz".Trim() -match '^\d+$') {
        $gb = [double]$sz / 1GB
        if ($gb -gt 150) {
            docker exec artex-pg psql -U artex -d artex -c "TRUNCATE llm_records;" 2>$null
            Log "WARN: 记录表 $([math]::Round($gb,1))GB 超应急阈值，已清空"
            return
        }
        if ($gb -gt 80) {
            Log "记录表 $([math]::Round($gb,1))GB 超高水位，临时收紧保留到 1 天"
            $llmRecordKeepDays = 1
        }
    }

    $before = docker exec artex-pg psql -U artex -d artex -t -A -c "SELECT count(*) FROM llm_records;" 2>$null
    if ("$before".Trim() -notmatch '^\d+$') { return }   # 库不可达，静默跳过
    $del = docker exec artex-pg psql -U artex -d artex -t -A -c "WITH d AS (DELETE FROM llm_records WHERE ts < now() - interval '$llmRecordKeepDays days' RETURNING 1) SELECT count(*) FROM d;" 2>$null
    $n = 0
    if ([int]::TryParse("$del".Trim(), [ref]$n) -and $n -gt 0) {
        Log "LLM 记录清理：删除 $n 条（早于 $llmRecordKeepDays 天前），原 $("$before".Trim()) 条"
    }
}
# 复探已排除（pool_exclude）的渠道，额度恢复后自动解除排除。
# 为什么需要它：pool_exclude 是静态黑名单，而被排除的渠道几乎都是「额度型」故障
# （402 积分耗尽 / 503 no_healthy_account / 上游 112、4008 超限）。这类故障大多会随
# 上游的额度冷却、或渠道自身的每日签到自行恢复，但黑名单不会自己解开 ——
# 结果就是池只能越用越小（42 → 26 成员）。这里每轮看门狗只探测 1 个排除项（轮转，
# 17 个约 85 分钟一轮），避免为探测额外消耗本就紧张的额度。
# 判定保守：只有 HTTP 200 才解除排除，其余一律维持现状。
function Invoke-ExcludedChannelProbe {
    $ids = @(docker exec artex-pg psql -U artex -d artex -t -A -c "SELECT id FROM llm_profiles WHERE pool_exclude ORDER BY id;" 2>$null |
        Where-Object { "$_".Trim() -match '^\d+$' } | ForEach-Object { [int]"$_".Trim() })
    if ($ids.Count -eq 0) { return }

    # 轮转指针：从上次探测的下一个 id 开始，走到末尾回头，保证每个都被轮到。
    # 注意指针只在「探针真正得出结论」后才前移（见下），否则一次 API 抖动就会让某个
    # 已恢复的渠道被跳过整整一圈（18 项 ≈ 90 分钟）才轮到——2026-10-15 13:53 就出现过：
    # 探针判定 qodercn 已恢复，但解除排除的 POST 正好撞上 artex 重启而失败，指针照旧前移。
    $stateFile = Join-Path $root 'data\excluded-probe.state'
    $last = 0
    if (Test-Path $stateFile) { [void][int]::TryParse((Get-Content $stateFile -Raw).Trim(), [ref]$last) }
    $target = ($ids | Where-Object { $_ -gt $last } | Select-Object -First 1)
    if (-not $target) { $target = $ids[0] }

    # 不用 coalesce：psql 的 -A -t 本就把 NULL 渲染成空字段，而 coalesce(x,'') 在
    # PowerShell 双引号里会写成 coalesce(x,''')（SQL 语法错误），让整条查询静默返回空。
    $row = (docker exec artex-pg psql -U artex -d artex -t -A -F'|' -c "SELECT id,name,format,base_url,model,priority,streaming,context_window_k,reasoning_effort,thinking_type,rate_per_second,rate_per_minute,api_key FROM llm_profiles WHERE id=$target;" 2>$null | Out-String).Trim()
    $g = $row -split '\|'
    if ($row -eq '' -or $g.Count -lt 13) { Log "WARN: id=$target 读取失败，跳过本轮探测"; return }
    $model = $g[4]; $baseUrl = $g[3]; $apiKey = $g[12]
    if (-not $baseUrl -or -not $model) { return }

    # 探测载荷必须接近真实工况：引擎发的是大上下文（实测单次输入 68–87K token），
    # 而很多渠道小 ping 秒回、大 prompt 直接卡死或 502。2026-10-15 实测：ping 全绿的
    # qodercn 在 40KB 输入上 HTTP=000（挂满 90s）、traework 变 502、monkeycode 变 402，
    # 真正扛得住 40KB 的只有 glm 与 loomy。用 ping 探测会把这类「假恢复」放回链里，
    # 结果就是引擎每次都要先撞一次墙再故障转移（当时 7 分钟内 6 次 stream stalled）。
    # 这里用 ~40KB 填充文本（≈11K token）作为判据，仍只发 16 token 输出，成本可忽略。
    $probePad = 'x' * 40000
    $body = @{ model = $model; messages = @(@{ role = 'user'; content = $probePad }, @{ role = 'user'; content = 'reply OK' }); max_tokens = 16; stream = $false } | ConvertTo-Json -Depth 5 -Compress
    $code = 0
    try {
        $resp = Invoke-WebRequest -Uri "$($baseUrl.TrimEnd('/'))/chat/completions" -Method POST -TimeoutSec 75 -UseBasicParsing `
            -Headers @{ Authorization = "Bearer $apiKey" } -ContentType 'application/json' `
            -Body ([System.Text.Encoding]::UTF8.GetBytes($body))
        $code = [int]$resp.StatusCode
    } catch {
        if ($_.Exception.Response) { $code = [int]$_.Exception.Response.StatusCode } else { $code = -1 }
    }
    if ($code -ne 200) {
        # 未恢复：本轮结论已得出，指针前移，下轮换下一个。
        Set-Content -Path $stateFile -Value "$target" -Encoding ASCII
        return
    }

    Log "排除渠道 id=$target $model 已恢复（HTTP 200），解除排除"
    # 必须提交完整字段：服务端的更新分支在 api_key 为空时会保留密钥，但其余字段会被写入值，
    # 少传就等于把该字段清零（rate/priority/窗口等）。所以整行读出来再原样回写。
    $payload = [ordered]@{
        id = [int]$g[0]; name = $g[1]; format = $g[2]; base_url = $baseUrl; model = $model;
        priority = [int]$g[5]; streaming = ($g[6] -eq 't'); context_window_k = [int]$g[7];
        reasoning_effort = $g[8]; thinking_type = $g[9];
        rate_per_second = [double]$g[10]; rate_per_minute = [double]$g[11];
        pool_exclude = $false; max_tokens = 0; api_key = $apiKey;
        retry = @{ connect_attempts = 0; connect_interval_ms = 0; empty_attempts = 0; empty_interval_ms = 0; stream_attempts = 0; stream_interval_ms = 0 }
    }
    try {
        $req = @{
            Uri = 'http://127.0.0.1:8787/api/llm/profiles'; Method = 'POST'; TimeoutSec = 25; UseBasicParsing = $true;
            Headers = (Get-ArtexAuthHeader); ContentType = 'application/json'
            Body = ([System.Text.Encoding]::UTF8.GetBytes(($payload | ConvertTo-Json -Depth 6 -Compress)))
        }
        # 系统设置里开了 HTTP Basic Auth 后，/api 也会被 gate 挡住：先拿 gate cookie 再带 JWT 调。
        $gate = Get-ArtexGateSession
        if ($gate) { $req.WebSession = $gate }
        $null = Invoke-WebRequest @req
        # 清掉可能残留的熔断记录，让它以干净状态回到链里
        docker exec artex-pg psql -U artex -d artex -c "DELETE FROM llm_profile_health WHERE profile_id=$target;" 2>$null | Out-Null
        Set-Content -Path $stateFile -Value "$target" -Encoding ASCII   # 只有真正恢复了才前移
        Log "id=$target 已回到池链"
    } catch {
        # 指针【不前移】：下轮再试同一个，避免一次 API 抖动白等一整圈。
        Log "WARN: id=$target 解除排除失败（下轮重试同一项）: $($_.Exception.Message)"
    }
}

# 用 D:\ARTEX\jwt.key 现场签一个 HS256 令牌（与后端同源，无需交互式登录）。
function Get-ArtexAuthHeader {
    $keyFile = Join-Path $root 'jwt.key'
    if (-not (Test-Path $keyFile)) { return @{} }
    $secret = [System.Text.Encoding]::ASCII.GetBytes(([System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes($keyFile))).Trim())
    $b64 = { param([byte[]]$b) [Convert]::ToBase64String($b).TrimEnd('=').Replace('+','-').Replace('/','_') }
    $now = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    $h = & $b64 ([System.Text.Encoding]::UTF8.GetBytes('{"alg":"HS256","typ":"JWT"}'))
    $p2 = & $b64 ([System.Text.Encoding]::UTF8.GetBytes(([ordered]@{ sub = 'ARTEX'; exp = ($now + 300); iat = $now } | ConvertTo-Json -Compress)))
    $hm = New-Object System.Security.Cryptography.HMACSHA256
    $hm.Key = $secret
    $sig = & $b64 ($hm.ComputeHash([System.Text.Encoding]::UTF8.GetBytes("$h.$p2")))
    return @{ Authorization = "Bearer $h.$p2.$sig" }
}

# 系统设置里开启 HTTP Basic Auth 后，连 /api 都要先过 gate。拿 cookie 的入口是
# /api/basic-auth/check（与前端同一路径，免 JWT），凭据由开启时写入 data\basic-auth.cred。
# 未开启 Basic Auth 时该文件不存在，或 check 直接 204 无 cookie —— 两种都退化为 null，
# 调用方照旧只带 JWT。
function Get-ArtexGateSession {
    $credFile = Join-Path $root 'data\basic-auth.cred'
    if (-not (Test-Path $credFile)) { return $null }
    try {
        $c = Get-Content $credFile -Raw -Encoding UTF8 | ConvertFrom-Json
        if (-not $c.username -or -not $c.password) { return $null }
        $pair = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes("$($c.username):$($c.password)"))
        $sess = $null
        $null = Invoke-WebRequest 'http://127.0.0.1:8787/api/basic-auth/check' `
            -Headers @{ Authorization = "Basic $pair" } -SessionVariable sess -UseBasicParsing -TimeoutSec 15
        return $sess
    } catch {
        Log "WARN: Basic Auth 会话获取失败: $($_.Exception.Message)"
        return $null
    }
}

# 判定 artex 是否「进程活着但引擎卡死」（僵死）。
# 为什么需要它：2026-10-14 的僵死里 artex 进程健在、/api/health 照样返回 200
# （HTTP 服务本身没坏），但三个 worker 全卡在无超时的上游调用上 —— 日志不再增长、
# DB 不再写入、running 意图一直挂着。只看进程或只看 health 都发现不了，
# 所以必须四条件同时成立才判僵死，避免把「任务全部做完后的正常安静」误杀。
function Test-ArtexZombie([int]$ArtexPid) {
    $logFile = Join-Path $root 'data\artex.stderr.log'
    if (-not (Test-Path $logFile)) { return $false }

    # (a) 日志静默时长：正常推进时 worker 每次工具调用都会留痕
    $idleMin = ((Get-Date) - (Get-Item $logFile).LastWriteTime).TotalMinutes
    if ($idleMin -lt 8) { return $false }

    # (b) HTTP 服务是否还活着（僵死时它照样 200，所以这条只用来排除「服务已死」）
    $alive = $false
    try {
        $h = Invoke-WebRequest 'http://127.0.0.1:8787/api/health' -TimeoutSec 5 -UseBasicParsing
        $alive = ($h.StatusCode -eq 200)
    } catch { $alive = $false }
    if (-not $alive) { return $false }

    # (c) 是否还有 running 意图挂着（全做完 = 正常安静，不是僵死）
    $raw = docker exec artex-pg psql -U artex -d artex -t -A -c "SELECT count(*) FROM exploration_nodes WHERE kind='intent' AND state='running';" 2>$null
    $n = 0
    if (-not [int]::TryParse("$raw".Trim(), [ref]$n)) { return $false }
    if ($n -le 0) { return $false }

    Log "僵死判据成立: 日志静默 $([math]::Round($idleMin,1)) 分钟 + health 200 + $n 个 running 意图"
    return $true
}
# 日志盯盘：把「补丁该修的那几类故障」从后端日志里数出来，命中就记一条 WARN。
# 为什么需要：本地补丁把 stream stalled / 空完成 变成可观测事件（原来是静默卡死或
# 静默空转），但频繁出现仍说明上游在丢数据、需要人介入。看门狗本来每 5 分钟跑一次，
# 是现成的定时器。只扫增量（offset 记账），不反复重读整个文件。
$watchPatterns = [ordered]@{
    'stream stalled' = '上游流卡死（idle 看门狗已打断并转 failover）'
    '空完成'          = '空完成（HTTP 200 + error 帧，已转 failover）'
    '轮询链已耗尽'     = '轮询链耗尽（全部配置都失败）'
    '压缩调用失败'     = '压缩调用失败后重试'
    '僵死'            = '僵死判据成立并强制重启'
}
function Invoke-LogWatch {
    $logFile = Join-Path $root 'data\artex.stderr.log'
    if (-not (Test-Path $logFile)) { return }
    $stateFile = Join-Path $root 'data\log-watch.state'
    $prev = 0
    if (Test-Path $stateFile) { [void][int64]::TryParse((Get-Content $stateFile -Raw).Trim(), [ref]$prev) }
    $len = (Get-Item $logFile).Length
    if ($len -lt $prev) { $prev = 0 }   # 日志被轮转过，从头算
    if ($len -le $prev) { return }
    $text = ''
    $fs = [System.IO.File]::Open($logFile, 'Open', 'Read', 'ReadWrite')
    try {
        $null = $fs.Seek($prev, 'Begin')
        $buf = New-Object byte[] ($len - $prev)
        $read = $fs.Read($buf, 0, $buf.Length)
        if ($read -gt 0) { $text = [System.Text.Encoding]::UTF8.GetString($buf, 0, $read) }
    } finally { $fs.Close() }
    Set-Content -Path $stateFile -Value "$len" -Encoding ASCII
    foreach ($k in $watchPatterns.Keys) {
        $n = ([regex]::Matches($text, [regex]::Escape($k))).Count
        if ($n -gt 0) { Log "WARN(盯盘): $($watchPatterns[$k]) × $n" }
    }
}

Log '=== boot-stack 开始 ==='
Invoke-LogWatch
Invoke-LLMRecordRetention
Invoke-ExcludedChannelProbe

# --- 0) 已在运行则不动；但僵死必须强制重启 ----------------------------------
# 本脚本现在既被登录触发，也被看门狗每 5 分钟触发一次，所以这里是两道判断：
# 活着 → 直接返回（幂等）；僵死 → 拆掉再重建，否则引擎会一直挂着不推进。
$existing = Get-Process artex -ErrorAction SilentlyContinue
if ($existing) {
    if (-not (Test-ArtexZombie -ArtexPid $existing.Id)) {
        Log "artex 已在运行 (pid=$($existing.Id)) 且正常，跳过本次自举"
        return
    }
    Log "artex (pid=$($existing.Id)) 僵死 → 强制重启"
    # 先拆 start.bat 守护再杀 artex，否则守护会在我们重建前把残留进程再拉起来
    Get-CimInstance Win32_Process -Filter "Name='cmd.exe'" |
        Where-Object { $_.CommandLine -match 'start\.bat' } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    Stop-Process -Id $existing.Id -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 3
    Log "残留进程已清理，继续按序自举"
}
# --- 1) Docker Desktop（Postgres 宿主）--------------------------------------
$dockerReady = $false
$ver = docker version --format '{{.Server.Version}}' 2>$null
if ($LASTEXITCODE -eq 0 -and $ver) {
    $dockerReady = $true
    Log "Docker 守护已就绪 server=$ver"
} else {
    Log 'Docker 守护未运行，启动 Docker Desktop …'
    Start-Process 'C:\Program Files\Docker\Docker\Docker Desktop.exe'
    for ($i = 0; $i -lt 90; $i++) {          # 最多等 180s
        Start-Sleep -Seconds 2
        $ver = docker version --format '{{.Server.Version}}' 2>$null
        if ($LASTEXITCODE -eq 0 -and $ver) {
            $dockerReady = $true
            Log "Docker 守护就绪（约 $($i * 2)s）server=$ver"
            break
        }
    }
}
if (-not $dockerReady) { Log 'FATAL: Docker 守护 180s 内未就绪，放弃本次自举'; return }

# --- 2) Postgres 容器 -------------------------------------------------------
# 容器带 restart=unless-stopped，正常会随 Docker 自动起来；这里兜底。
$state = docker inspect -f '{{.State.Running}}' artex-pg 2>$null
if ("$state" -notmatch 'True') {
    Log 'artex-pg 未运行，执行 docker start'
    docker start artex-pg | Out-Null
}
# 必须双条件就绪：
#   (a) 容器内 pg_isready —— 证明 postmaster 活着；
#   (b) host 侧 TCP 127.0.0.1:<config.json 里的 database.port> —— 证明 Docker 用户态转发器
#       （com.docker.backend）已经把端口接上。只查 (a) 会误判：2026-10-13 22:01 那次 artex
#       就是 (a) 通过、(b) 未就绪，启动即 `open stores: ping postgres` 失败退出 code=1。
# 宿主侧探测端口取自 config.json：Docker 的 5432 用户态映射曾整条卡死（com.docker.backend
# 里的陈旧条目，重建容器/boot 都不清），PG 因此改发布到 15432。写死 5432 会让看门狗永远
# 等不到就绪，所以这里跟 artex 用同一个端口来源。
$pgHostPort = 5432
try {
    $cfgPath = Join-Path $root 'config.json'
    if (Test-Path $cfgPath) {
        $cfg = Get-Content $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($cfg.database -and $cfg.database.port) { $pgHostPort = [int]$cfg.database.port }
    }
} catch { }
$pgReady = $false
for ($i = 0; $i -lt 60; $i++) {              # 最多等 120s
    $probe = docker exec artex-pg pg_isready -U artex -d artex 2>&1
    $inContainer = ("$probe" -match 'accepting connections')
    $onHost = $false
    $tcp = New-Object System.Net.Sockets.TcpClient
    try {
        $tcp.Connect('127.0.0.1', $pgHostPort)
        $onHost = $tcp.Connected
    } catch { $onHost = $false } finally { $tcp.Close() }

    if ($inContainer -and $onHost) {
        $pgReady = $true
        Log "Postgres 就绪（约 $($i * 2)s，容器内+host:$pgHostPort 双探测通过）"
        break
    }
    if (($i % 10) -eq 9) {
        Log "等待 Postgres：容器内=$inContainer host:$pgHostPort=$onHost（已 $(($i + 1) * 2)s）"
    }
    Start-Sleep -Seconds 2
}
if (-not $pgReady) { Log 'FATAL: Postgres 120s 内未双就绪，放弃本次自举'; return }

# --- 3) 日志轮转：单个 stderr 日志超过 200MB 就先归档再启动（保留最近 2 份）---
# 只在 artex 未运行时执行（本脚本的启动分支），此时文件句柄已释放；运行中不碰它。
$stderrLog = Join-Path $root 'data\artex.stderr.log'
if (Test-Path $stderrLog) {
    $logSize = (Get-Item $stderrLog).Length
    if ($logSize -gt 200MB) {
        $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
        $rotated = Join-Path $root ("data\artex.stderr.$stamp.log")
        try {
            Move-Item -LiteralPath $stderrLog -Destination $rotated -Force
            Log "旧日志已轮转: $rotated ($([math]::Round($logSize / 1MB, 1)) MB)"
            Get-ChildItem (Join-Path $root 'data') -Filter 'artex.stderr.20*.log' |
                Sort-Object LastWriteTime -Descending |
                Select-Object -Skip 2 |
                Remove-Item -Force -ErrorAction SilentlyContinue
        } catch {
            Log "日志轮转失败（继续追加原文件）: $_"
        }
    }
}

# --- 3) 后端守护（start.bat 自带崩溃重启与换装）-----------------------------
Log '启动 artex 守护 start.bat'
Start-Process -FilePath 'cmd.exe' `
    -ArgumentList '/c "call D:\ARTEX\start.bat >> D:\ARTEX\data\artex.stderr.log 2>&1"' `
    -WorkingDirectory $root -WindowStyle Hidden

$healthy = $false
for ($i = 0; $i -lt 90; $i++) {              # 最多等 180s 变健康
    Start-Sleep -Seconds 2
    try {
        $h = Invoke-WebRequest 'http://127.0.0.1:8787/api/health' -TimeoutSec 5 -UseBasicParsing
        if ($h.StatusCode -eq 200) {
            $healthy = $true
            Log "artex 健康（约 $($i * 2)s）: $($h.Content)"
            break
        }
    } catch {}
}
# 这里不健康不代表失败：start.bat 自带退避重启（code=1 时 60s 后重来），
# 后面的看门狗周期触发也会再来一次，所以只记录不报错。
if (-not $healthy) { Log 'WARN: 180s 内未健康，交给 start.bat 退避重启 + 看门狗周期重试' }

Log '=== boot-stack 结束 ==='
