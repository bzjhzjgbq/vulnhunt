package server

import "testing"

func TestCheckEvidence(t *testing.T) {
	longSummary := "这是一个足够长的漏洞描述，说明影响范围、触发条件与可利用性，供审核人员快速判断。"
	longEvidence := "GET /api/v1/users?id=1 HTTP/1.1\nHost: example.com\n\nHTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
		"{\"id\":1,\"phone\":\"138****0000\",\"email\":\"a@example.com\"}\n"
	for len([]rune(longEvidence)) < 320 {
		longEvidence += "GET /api/v1/users?id=1 HTTP/1.1\n\nHTTP/1.1 200 OK\n"
	}
	longReport := "## 漏洞详情\n\n## 复现步骤\n1. 登录\n2. 请求接口\n\n## 影响\n可读取其他用户信息。\n"
	for len([]rune(longReport)) < 550 {
		longReport += "补充说明：该接口未校验当前用户与目标用户的关系。\n"
	}

	checks, score, ready := checkEvidence("越权读取用户信息", "IDOR", longSummary, longEvidence, longReport, "high")
	if !ready || score != len(checks) {
		t.Fatalf("expected ready, got score=%d/%d", score, len(checks))
	}

	weakChecks, _, weakReady := checkEvidence("", "", "", "", "", "low")
	if weakReady {
		t.Fatalf("empty finding must not be ready")
	}
	if len(weakChecks) == 0 {
		t.Fatalf("expected checks")
	}
	for _, c := range weakChecks {
		if c.OK {
			t.Fatalf("check %s should fail on empty finding", c.Key)
		}
	}
}

func TestCheckEvidenceClientSide(t *testing.T) {
	summary := "导出的 ContentProvider 未校验调用方，任意第三方 App 可读取当前登录用户的凭证与设备标识，导致账号被接管。"
	evidence := "调用链：com.bili.sdk -> BiliStatusProvider.query()\nadb shell content query --uri content://com.bili/status\n" +
		"返回结果：uid=12345 buvid=XXXX access_key=abcd，证明任意应用可读取。\n"
	for len([]rune(evidence)) < 320 {
		evidence += "反编译后确认 query() 未调用 checkCallingPermission。\n"
	}
	report := "## 漏洞详情\n## 复现步骤\n1. 安装恶意 App\n2. 调用 query\n## 影响\n账号接管\n"
	for len([]rune(report)) < 550 {
		report += "补充：该组件在 AndroidManifest 中 exported=true。\n"
	}
	_, score, ready := checkEvidence("Android 导出 ContentProvider 凭证泄露", "Android 组件", summary, evidence, report, "high")
	if !ready {
		t.Fatalf("client-side evidence should be ready, score=%d", score)
	}
}

func TestCheckEvidenceLowSeverityFails(t *testing.T) {
	long := make([]rune, 600)
	for i := range long {
		long[i] = 'x'
	}
	evidence := "GET /a HTTP/1.1\nHTTP/1.1 200 OK\n" + string(long)
	report := "## 复现\n## 影响\n" + string(long)
	_, _, ready := checkEvidence("t", "c", string(long), evidence, report, "low")
	if ready {
		t.Fatalf("low severity should not be ready for submission")
	}
}
