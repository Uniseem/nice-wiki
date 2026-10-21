import { Node } from '@tiptap/core';
import type { AssetRef } from './upload';

// 画板块：保存场景 JSON 与浅/深色 SVG 三个不可变资源引用；编辑器里显示浅色 SVG，双击打开画板。
export const Drawing = Node.create({
  name: 'drawing',
  group: 'block',
  atom: true,
  draggable: true,
  addAttributes() {
    return {
      blockId: { default: null, rendered: false },
      scene: { default: null, rendered: false },
      lightSVG: { default: null, rendered: false },
      darkSVG: { default: null, rendered: false },
      widthPercent: { default: null, rendered: false },
      previewURL: { default: null, rendered: false }, // 刚画完、正文还没引用这些资源时用来预览，不保存
    };
  },
  parseHTML() { return [{ tag: 'img[data-drawing]' }]; },
  renderHTML({ node }) {
    const light = node.attrs.lightSVG as AssetRef | null;
    return ['img', { 'data-drawing': '', class: 'drawing', src: (node.attrs.previewURL as string | null) ?? (light ? `/v1/assets/${light.assetId}/bytes?sha=${light.sha256}` : ''), alt: '画板' }];
  },
});

// 服务端 schema 只登记了固定的一组元素字段（不含 0.18 新增的 index 等），保存前按类型逐字段筛选。
const base = ['id', 'type', 'x', 'y', 'width', 'height', 'angle', 'strokeColor', 'backgroundColor', 'fillStyle', 'strokeWidth', 'strokeStyle', 'roughness', 'opacity', 'groupIds', 'frameId', 'roundness', 'seed', 'version', 'versionNonce', 'isDeleted', 'boundElements', 'updated', 'link', 'locked'];
const extra: Record<string, string[]> = {
  rectangle: [], diamond: [], ellipse: [],
  line: ['points', 'startBinding', 'endBinding', 'startArrowhead', 'endArrowhead', 'elbowed'],
  arrow: ['points', 'startBinding', 'endBinding', 'startArrowhead', 'endArrowhead', 'elbowed'],
  freedraw: ['points', 'pressures', 'simulatePressure', 'lastCommittedPoint'],
  text: ['fontSize', 'fontFamily', 'lineHeight', 'baseline', 'text', 'originalText', 'autoResize', 'containerId', 'textAlign', 'verticalAlign'],
  image: ['fileId', 'status', 'scale'],
  frame: ['name'],
};

export function normalizeScene(elements: readonly any[], appState: any, files: Record<string, any>) {
  const kept = elements.filter((el) => !el.isDeleted && extra[el.type]).map((el) => {
    const out: Record<string, unknown> = {};
    for (const key of [...base, ...extra[el.type]]) if (el[key] !== undefined) out[key] = el[key];
    if (out.roundness && (out.roundness as any).value === undefined) out.roundness = { ...(out.roundness as object), value: null };
    if (el.type === 'text' && out.baseline === undefined) out.baseline = Math.round((el.fontSize ?? 20) * 0.9);
    if (el.type === 'line' || el.type === 'arrow') {
      if (out.elbowed === undefined) out.elbowed = false;
      for (const key of ['startBinding', 'endBinding']) {
        const b = out[key] as any;
        out[key] = b ? { elementId: b.elementId, focus: b.focus, gap: b.gap, fixedPoint: b.fixedPoint ?? null } : null;
      }
    }
    return out;
  });
  const used = new Set(kept.filter((el) => el.type === 'image').map((el) => el.fileId as string));
  const outFiles: Record<string, unknown> = {};
  for (const [id, file] of Object.entries(files ?? {})) {
    if (used.has(id)) outFiles[id] = { id, mimeType: (file as any).mimeType, dataURL: (file as any).dataURL, created: (file as any).created ?? Date.now(), lastRetrieved: (file as any).lastRetrieved ?? null };
  }
  return {
    type: 'excalidraw', version: 2, source: 'nice-wiki', elements: kept,
    appState: { viewBackgroundColor: appState.viewBackgroundColor ?? '#ffffff', gridSize: appState.gridSize ?? null, theme: 'light' },
    files: outFiles,
  };
}
