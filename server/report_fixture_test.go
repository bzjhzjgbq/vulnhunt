package server

// compliantFindingReport 生成一份满足固定 6 段结构的报告，供工作流/流量/归并等用例复用。
func compliantFindingReport(title, conclusion string) string {
	return "# " + title + "\n" +
		"- 漏洞等级：高危（测试用例）\n\n" +
		"## 一句话结论\n" + conclusion + "\n\n" +
		"## 成立前提\n匿名可达，对象 ID 可推算。\n\n" +
		"## 关键动作\nGET /api/example?id=1\n\n" +
		"## 影响范围\n读取他人私有数据。\n"
}
