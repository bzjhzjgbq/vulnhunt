package agent

import "testing"

func TestIntentSimilarityNormalizesFormatting(t *testing.T) {
	a := "【SSRF】测试 api.airbnb.cn 的 graphql 取址参数，判断服务端是否消费 url"
	b := "【ssrf】测试 api.airbnb.cn 的 GraphQL 取址参数, 判断服务端是否消费 URL。"
	if got := intentSimilarity(a, b); got < 0.95 {
		t.Fatalf("reformatted duplicate scored %.2f, want >= 0.95", got)
	}
}

func TestIntentSimilarityKeepsDifferentDirectionsApart(t *testing.T) {
	a := "测试 OPPO 商城订单接口的越权读取（IDOR），用两个账号交叉验证"
	b := "检查 line3-game-open-platform-api 的 pvc_link_check 是否会服务端取址（SSRF）"
	if got := intentSimilarity(a, b); got >= 0.5 {
		t.Fatalf("different directions scored %.2f, want < 0.5", got)
	}
}

func TestIntentSimilarityShortTextNeverDupes(t *testing.T) {
	if got := intentSimilarity("测试 SSRF", "测试 SSRF"); got != 0 {
		t.Fatalf("short summaries should not be compared, got %.2f", got)
	}
}

func TestCheckDuplicateIntentWithoutStoreIsNoop(t *testing.T) {
	tool := &ToolSet{}
	if err := tool.checkDuplicateIntent("测试一个很长很长的方向描述用于绕过长度限制"); err != nil {
		t.Fatalf("nil store should never reject: %v", err)
	}
}
