# 外观包标准（schemaVersion 1）

外观（皮肤）决定公开站长什么样：配色、字体、正文排版、圆角、版式开关，以及一份受限的补充样式表。外观只换样式，不能带脚本、模板或外部资源；页面结构、交互岛和后台不受外观影响。单站点版与多用户版用同一套标准，同一个外观包两边都能装。

实现：校验、渲染和内置外观在 `libs/platform/theme`；theme.json 的 JSON Schema 见 [contracts/theme.schema.json](../contracts/theme.schema.json)。

## 1 包结构

外观包是一个 zip（压缩包不超过 2MiB，解压后合计不超过 4MiB，最多 64 个文件，单个文件不超过 512KiB）：

```text
theme.json        必需，外观清单
theme.css         可选，补充样式表（不超过 64KiB），在 theme.json 的 stylesheet 里声明
preview.png       可选，预览图（也可以是 preview.jpg / preview.webp），在 preview 里声明
README.md         可选，说明，不对外提供
LICENSE           可选（也可以是 LICENSE.md / LICENSE.txt），不对外提供
assets/           可选，图片（png jpg webp gif avif）与字体（woff2 woff），文件名只用小写字母、数字、点、下划线、连字符，不能有子目录
```

其他文件一律拒收。图片与字体按文件头核对实际格式。把整个文件夹打包（压缩包里只有一个顶层文件夹）也可以；macOS 生成的 `__MACOSX/`、`.DS_Store` 自动忽略。

外观包的**摘要**是对“路径 + 每个文件内容的 SHA-256”排序后再做 SHA-256 得到的 64 位十六进制串，只和文件内容有关，与压缩方式无关。站点按摘要引用外观，公开地址也带摘要，所以外观文件可以长期缓存；改了内容就是一个新摘要，要发新版本号。

## 2 theme.json

```json
{
  "schemaVersion": 1,
  "id": "paper",
  "version": "1.0.0",
  "name": { "zh": "纸本", "en": "Paper" },
  "description": { "zh": "暖色纸面、衬线字体和较窄的阅读栏。", "en": "Warm paper tones and serif type." },
  "author": "nice-wiki",
  "license": "MIT",
  "homepage": "https://example.com/themes/paper",
  "colorSchemes": ["light", "dark"],
  "tokens": {
    "light": { "background": "#f4efe6", "foreground": "#2b2620", "accent": "#8a4b2a" },
    "dark": { "background": "#1f1b17", "foreground": "#ece4d8", "accent": "#e0a77f" }
  },
  "typography": { "sans": "Georgia, serif", "body": "Georgia, serif", "mono": "Menlo, monospace", "bodySize": 18, "lineHeight": 1.85 },
  "shape": { "radius": "small" },
  "layout": { "contentWidth": "narrow", "sidebar": "left", "toc": "right", "header": "static" },
  "fonts": [{ "family": "Literata", "src": "assets/literata.woff2", "weight": "200 900", "style": "normal" }],
  "stylesheet": "theme.css",
  "preview": "preview.png"
}
```

| 字段 | 必需 | 说明 |
|---|---|---|
| `schemaVersion` | 是 | 固定为 `1`。格式有不兼容变化时才升版本 |
| `id` | 是 | 外观标识：小写字母开头，小写字母、数字、连字符，3–40 个字符。市场里同一个 id 只属于第一个发布者；内置外观的 id 保留 |
| `version` | 是 | `主.次.修订`，如 `1.2.0`。同一 id 的每次发布版本号都要不同 |
| `name` | 是 | 按语言写的名称（键为语言标签，如 `zh`、`zh-Hant`、`en`），每项 1–60 个字符。界面按当前语言、主语言、`en`、`zh` 的顺序取 |
| `description` | 否 | 按语言写的简介，每项不超过 280 个字符 |
| `author` | 是 | 作者，1–80 个字符 |
| `license` | 是 | SPDX 许可证标识，如 `MIT`、`CC-BY-4.0` |
| `homepage` | 否 | https 地址 |
| `colorSchemes` | 是 | 支持的配色：`["light"]`、`["dark"]` 或 `["light", "dark"]`。只支持一种时访客不能切换明暗，页头的明暗按钮隐藏 |
| `tokens` | 否 | 每种配色的颜色，见第 3 节。没写的颜色沿用默认外观 |
| `typography` | 否 | `sans`（界面、导航和标题）、`body`（正文）、`mono`（代码）三组字体名列表；`bodySize` 正文字号 14–22（px）；`lineHeight` 正文行高 1.3–2.2 |
| `shape.radius` | 否 | 控件和卡片的圆角：`none`、`small`、`medium`（默认）、`large` |
| `layout.contentWidth` | 否 | 正文栏宽：`narrow`（680px）、`normal`（800px，默认）、`wide`（960px） |
| `layout.sidebar` | 否 | 文档树位置：`left`（默认）或 `right`（此时本页目录移到左侧） |
| `layout.toc` | 否 | 本页目录：`right`（默认）或 `hidden` |
| `layout.header` | 否 | 页头：`sticky`（默认，滚动时固定在顶部）或 `static` |
| `fonts` | 否 | 最多 8 个网页字体：`family` 字体名，`src` 指向 `assets/` 下的 woff2/woff，`weight` 如 `400` 或可变字体的 `100 900`，`style` 为 `normal`/`italic`。声明后可以在 `typography` 里用这个字体名 |
| `stylesheet` | 否 | 只能是 `"theme.css"`。包里有 theme.css 时必须声明 |
| `preview` | 否 | `preview.png`、`preview.jpg` 或 `preview.webp`。没有预览图时后台用配色画一张示意图 |

theme.json 不接受未列出的字段。字体名列表只能用字母、数字、空格、引号（成对）、逗号、点、下划线、连字符。

## 3 配色

`tokens.light` 和 `tokens.dark` 里可写下列键，值为 `#rgb`、`#rgba`、`#rrggbb`、`#rrggbbaa`，或 `rgb()`、`rgba()`、`hsl()`、`hsla()`、`oklch()`、`oklab()`。它们对应公开站与后台共用的界面变量：

| 键 | 用途 |
|---|---|
| `background` / `foreground` | 页面底色与主文字 |
| `surface` / `surfaceSecondary` / `surfaceTertiary` | 卡片、代码块、表头等一到三级面 |
| `overlay` | 弹出层、下拉菜单 |
| `muted` | 次要文字（正文段落、目录、页脚） |
| `default` / `defaultForeground` | 默认按钮、行内代码、悬停底色及其文字 |
| `fieldBackground` | 输入框底色 |
| `border` / `separator` | 边框与分隔线 |
| `accent` / `accentForeground` | 强调色（正文链接、主按钮、当前语言）及其上的文字 |
| `focus` | 键盘焦点框 |

站点设置里的“主题色”仍只用于站标，和外观无关。

## 4 theme.css

theme.css 用来补充配色和版式开关做不到的细节，在站点默认样式和外观变量之后加载，可以覆盖它们。限制：

- 不能用 `@import`、转义符 `\`、`expression(`、`javascript:`、`vbscript:`、`image-set(`、`src(`、`</`。
- `url()` 只能指向包里 `assets/` 下存在的文件，写相对路径，如 `url(assets/grain.webp)`。
- 不能引用外部字体、图片或样式表。

建议只用下列稳定钩子写选择器，其余类名可能随版本调整：

| 钩子 | 位置 |
|---|---|
| `html[data-nw-theme="<id>"]` | 当前外观（按 id 区分自己的样式） |
| `body[data-page="home"\|"doc"\|"ask"]` | 首页、文档页、问答页 |
| `.nw-site-header`、`.nw-brand`、`.nw-site-footer`、`.nw-footer-brand` | 页头、站标、页脚 |
| `.nw-reading-layout`、`.nw-sidebar`、`.nw-tree`、`#nw-toc`、`.nw-toc-link` | 文档页三栏、文档树、本页目录 |
| `.nw-document`、`.nw-document-title`、`.nw-prose`、`.nw-adjacent` | 正文区、标题、正文排版、上一篇/下一篇 |
| `.nw-welcome-*`、`.nw-qa-*` | 首页欢迎区、问答界面 |
| `[data-theme="light"\|"dark"]` | 当前明暗 |

外观不能隐藏或改写“举报此站点”、来源链接等平台必需元素；市场审核会检查。

## 5 加载方式

公开站渲染页面时：

1. `<html>` 带 `data-nw-theme`、`data-nw-sidebar`、`data-nw-toc`、`data-nw-header`、`data-nw-schemes`；只支持一种配色时直接写好 `data-theme`。
2. `<head>` 内联一段由 theme.json 生成的样式（`@font-face`、字体与排版变量、圆角、栏宽、两套配色）。
3. 有 theme.css 时以 `<link>` 引入 `/_theme/<摘要>/theme.css`；字体和图片在 `/_theme/<摘要>/assets/…`。这些地址带摘要，按不可变资源缓存。

站点的外观选择（摘要、访客默认明暗）存在 site-config，切换后公开页缓存随即清掉。

## 6 来源与市场

- **内置外观**：classic（默认）、paper、forest、ocean、midnight，随程序发布，不能删除。后台可以导出任一外观的 zip，作为自己做外观的起点。
- **上传**：站点管理员可以直接上传外观包，只装在本站，不需要审核（多用户版和单站点版相同）。
- **外观市场**（多用户平台提供）：登录用户把外观包提交到市场，平台管理员审核通过后所有人可见；站点从市场安装时把包复制到本站，市场后来下架不影响已装的站点。市场的公开接口：
  - `GET /v1/theme-market`：已上架外观列表（每个 id 取最新通过的版本）。
  - `GET /v1/theme-market/{themeId}/{version}/package`：下载外观包。
  - `GET /v1/theme-market/{themeId}/{version}/preview`：预览图。
- **单站点版**在外观页填一个市场地址（多用户平台的地址），由站点服务端去拉列表和外观包，再按同样的规则校验安装。
