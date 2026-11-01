import packageJson from "../../package.json";

const currentYear = new Date().getFullYear();

export const APP_CONFIG = {
  name: "ARTEX",
  version: packageJson.version,
  copyright: `© ${currentYear}, ARTEX.`,
  meta: {
    title: "ARTEX · SRC 赏金挖掘工作台",
    description: "从 SRC 平台选目标、自动挖洞、提交并记账的赏金工作台",
  },
};
