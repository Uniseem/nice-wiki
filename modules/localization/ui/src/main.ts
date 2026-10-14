import { createApp, onUnmounted } from "vue";

export interface ModuleUIBootstrap {
  module: "localization";
  apiBaseURL: string;
  locale: string;
  mountSelector: string;
  capabilities: string[];
  buildRevision: string;
}

interface MountedModuleUI {
  abortController: AbortController;
  buildRevision: string;
  unmount: () => void;
}

const mountedInstances = new WeakMap<HTMLElement, MountedModuleUI>();

function assertBootstrap(rootElement: HTMLElement | null, bootstrap: ModuleUIBootstrap): asserts rootElement is HTMLElement {
  if (!(rootElement instanceof HTMLElement) || rootElement.ownerDocument !== document || !document.documentElement.contains(rootElement)) {
    throw new Error("VALIDATION");
  }
  if (bootstrap.module !== "localization" || !Array.isArray(bootstrap.capabilities) || bootstrap.buildRevision === "") {
    throw new Error("VALIDATION");
  }
}

export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
  assertBootstrap(rootElement, bootstrap);

  const existing = mountedInstances.get(rootElement);
  if (existing?.buildRevision === bootstrap.buildRevision) {
    return;
  }
  existing?.abortController.abort();
  existing?.unmount();

  const abortController = new AbortController();
  const app = createApp({
    name: "LocalizationModuleRoot",
    setup() {
      onUnmounted(() => abortController.abort());
      return () => null;
    },
  });
  app.provide("moduleUIBootstrap", bootstrap);
  app.provide("moduleUIAbortSignal", abortController.signal);
  app.mount(rootElement);

  mountedInstances.set(rootElement, {
    abortController,
    buildRevision: bootstrap.buildRevision,
    unmount: () => app.unmount(),
  });
}
