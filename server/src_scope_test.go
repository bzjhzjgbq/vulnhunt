package server

import "testing"

func TestExtractScopeCandidates(t *testing.T) {
	html := `<html><script src="https://cdn.example-cdn.net/app.js"></script>
<p>范围：*.target-a.com、api.target-b.cn、10.0.0.0/24、192.168.1.1</p>
<a href="https://www.w3.org/TR/html">spec</a>
<img src="/logo.png"><span>beian.miit.gov.cn</span>
<p>更多：shop.target-c.com:443 与 172.16.5.0/24</p>
<script>if (a.active && seajs.use) { location.href = m.iswechat }</script>`
	got := extractScopeCandidates(html)
	want := map[string]bool{
		"10.0.0.0/24":   true,
		"172.16.5.0/24": true,
		"target-a.com":  true,
		"target-b.cn":   true,
		"target-c.com":  true,
	}
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.Value] = true
		if c.Kind != "cidr" && c.Kind != "root_domain" {
			t.Errorf("unexpected kind %q for %q", c.Kind, c.Value)
		}
	}
	for v := range want {
		if !seen[v] {
			t.Errorf("missing candidate %q in %v", v, got)
		}
	}
	for _, noise := range []string{"w3.org", "beian.miit.gov.cn", "logo.png"} {
		if seen[noise] {
			t.Errorf("noise domain %q was returned", noise)
		}
	}
}
