import { createApp } from "vue";
import type { App } from "vue";
import type { ModuleUIBootstrap } from "../../../../libs/platform/web/module-api";
import OverviewView from "./views/OverviewView.vue";

interface MountedModuleUI {
  app: App;
  abort: AbortController;
  revision: string;
  capabilities: string;
  apiClient: ModuleUIBootstrap["apiClient"];
  stop: () => void;
}

const mountedInstances = new WeakMap<HTMLElement, MountedModuleUI>();

export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
  if (!(rootElement instanceof HTMLElement) || rootElement.ownerDocument !== document || !rootElement.isConnected || bootstrap.module !== "model-gateway" || !bootstrap.buildRevision || !Array.isArray(bootstrap.capabilities) || typeof bootstrap.apiClient?.request !== "function") {
    throw new Error("VALIDATION");
  }
  const existing = mountedInstances.get(rootElement);
  const capabilities = JSON.stringify([...bootstrap.capabilities].sort());
  if (existing?.revision === bootstrap.buildRevision && existing.capabilities === capabilities && existing.apiClient === bootstrap.apiClient) return;
  existing?.stop();
  const abort = new AbortController();
  const app = createApp(OverviewView, { bootstrap, parentSignal: abort.signal });
  const stop = () => {
    abort.abort();
    window.removeEventListener("pagehide", stop);
    app.unmount();
    mountedInstances.delete(rootElement);
  };
  app.provide("moduleUIBootstrap", bootstrap);
  app.mount(rootElement);
  window.addEventListener("pagehide", stop, { once: true });
  mountedInstances.set(rootElement, { app, abort, revision: bootstrap.buildRevision, capabilities, apiClient: bootstrap.apiClient, stop });
}
