import packageJson from "../../package.json";

const currentYear = new Date().getFullYear();

export const APP_CONFIG = {
  name: "猎洞",
  latinName: "VulnHunt",
  version: packageJson.version,
  copyright: `© ${currentYear}, 猎洞 VulnHunt.`,
  meta: {
    title: "猎洞 · SRC 赏金挖掘工作台",
    description: "选 SRC 目标、按各家收录标准自动挖洞、提交并记账的赏金工作台",
  },
};
