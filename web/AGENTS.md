# 前端约定

遵守根目录 AGENTS.md。admin 与 site 不互相导入，shared 不导入两个入口。site 只让问答和语言建议使用 Vue，其余使用原生 TypeScript；不得导入编辑器、React 或 Excalidraw。

Vue 使用 `<script setup lang="ts">`；后台固定中文。样式使用 Tailwind 和设计变量、逻辑方向属性，块样式以 nw- 开头。shadcn-vue 组件需确认具体新增文件，再用 CLI 生成，不手改生成代码。

API 统一通过 shared/api/client.ts 的 openapi-fetch 客户端，类型来自契约生成。先改 api/openapi.yaml 再 make gen；不要自行写一套接口类型。401 和强制改密码集中处理。

TypeScript 固定 6.0.x，严格模式。运行 lint、typecheck 和已有普通测试；新增样例、快照与 E2E 先确认。前端权限仅改善交互，后端仍须校验。
