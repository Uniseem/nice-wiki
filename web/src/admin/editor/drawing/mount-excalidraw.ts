import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'

export interface DrawingScene { elements: readonly unknown[], appState: Record<string, unknown>, files: Record<string, unknown> }
export interface MountedDrawing { scene: () => DrawingScene, exportSVG: (dark: boolean) => Promise<Blob>, destroy: () => void }

export async function mountExcalidraw(target: HTMLElement, initial?: DrawingScene): Promise<MountedDrawing> {
  const [{ createElement }, { createRoot }, module] = await Promise.all([import('react'), import('react-dom/client'), import('@excalidraw/excalidraw')])
  await import('@excalidraw/excalidraw/index.css')
  let api: ExcalidrawImperativeAPI | null = null
  const root = createRoot(target)
  root.render(createElement(module.Excalidraw, {
    initialData: initial as never,
    excalidrawAPI: (value: ExcalidrawImperativeAPI) => { api = value },
    langCode: 'zh-CN',
  }))
  const current = () => {
    if (!api) throw new Error('画板尚未就绪')
    return api
  }
  return {
    scene: () => ({ elements: current().getSceneElements(), appState: current().getAppState() as unknown as Record<string, unknown>, files: current().getFiles() as unknown as Record<string, unknown> }),
    exportSVG: async dark => {
      const scene = current()
      const svg = await module.exportToSvg({ elements: scene.getSceneElements(), appState: { ...scene.getAppState(), exportWithDarkMode: dark }, files: scene.getFiles() })
      return new Blob([svg.outerHTML], { type: 'image/svg+xml' })
    },
    destroy: () => root.unmount(),
  }
}
