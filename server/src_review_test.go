package server

import "testing"

func TestStandardReviewReason(t *testing.T) {
	std := &SRCStandard{MinSeverity: "medium", RejectKeywords: []string{"CORS", "版本", "目录遍历"}}
	cases := []struct {
		name      string
		vulnclass string
		title     string
		summary   string
		severity  string
		wantFlag  bool
		wantKw    string
	}{
		{"cors low", "CORS", "CORS 配置不当", "Access-Control-Allow-Origin: *", "low", true, "cors"},
		{"version leak", "信息泄露", "版本号泄露", "Server: nginx 1.2.3", "medium", true, "版本"},
		{"sqli keeps", "SQL注入", "SQL 注入", "id 参数可注入", "high", false, ""},
		{"xss keeps", "XSS", "反射型 XSS", "q 参数未过滤", "medium", false, ""},
		{"low only", "越权", "水平越权", "可读取他人订单", "low", true, ""},
	}
	for _, c := range cases {
		reason, kw := standardReviewReason(std, c.vulnclass, c.title, c.summary, c.severity)
		flagged := reason != ""
		if flagged != c.wantFlag {
			t.Fatalf("%s: flagged=%v want=%v (reason=%q)", c.name, flagged, c.wantFlag, reason)
		}
		if kw != c.wantKw {
			t.Fatalf("%s: keyword=%q want=%q", c.name, kw, c.wantKw)
		}
	}
}

func TestStandardForTaskPicksPlatform(t *testing.T) {
	cases := []struct {
		task *Task
		want string
	}{
		{&Task{Name: "Airbnb爱彼迎｜漏洞盒子SRC"}, "vulbox"},
		{&Task{Name: "货讯通科技｜补天专属SRC"}, "butian"},
		{&Task{Name: "OPPO安全应急响应中心｜漏洞盒子SRC"}, "vulbox"},
		{&Task{Name: "某个未收录的项目"}, "generic-vendor"},
	}
	for _, c := range cases {
		if got := standardForTask(c.task).ID; got != c.want {
			t.Fatalf("%s: standard=%q want=%q", c.task.Name, got, c.want)
		}
	}
}
