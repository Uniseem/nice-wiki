import { createApp, h, type App } from "vue";
import type { ModuleUIBootstrap } from "../../../../libs/platform/web/module-api";
import OverviewView from "./views/OverviewView.vue";
import "./style.css";
const mounts = new WeakMap<HTMLElement, { app: App; revision: string }>();

// portal持有认证与CSRF；本域只接非序列化API端口。
export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
 if (!rootElement.isConnected || rootElement.ownerDocument !== document || bootstrap.module !== "knowledge" || !bootstrap.apiClient?.request) throw new Error("VALIDATION");
 const mounted = mounts.get(rootElement);
 if (mounted?.revision === bootstrap.buildRevision) return;
 mounted?.app.unmount();
 const app = createApp({ render: () => bootstrap.capabilities.includes("knowledge.manage") ? h(OverviewView, { client: bootstrap.apiClient }) : h("p", { role: "status" }, "没有知识管理权限。") });
 app.mount(rootElement);
 mounts.set(rootElement, { app, revision: bootstrap.buildRevision });
}
