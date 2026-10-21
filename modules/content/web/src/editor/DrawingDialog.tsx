import { Brush } from '@gravity-ui/icons';
import { Button, Modal, Spinner } from '@heroui/react';
import { assetURL, errorText, uploadAsset, type AssetRef } from '@ui';
import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react';
import type { BoardHandle } from './DrawingBoard';

const DrawingBoard = lazy(() => import('./DrawingBoard'));

export type DrawingAttrs = { scene: AssetRef; lightSVG: AssetRef; darkSVG: AssetRef; previewURL?: string };

// 全屏画板：打开时读取已有场景，完成后上传场景 JSON 与浅/深色 SVG 三个资源。
export function DrawingDialog({ initial, onSave, onClose }: { initial: DrawingAttrs | null; onSave: (attrs: DrawingAttrs) => void; onClose: () => void }) {
  const [scene, setScene] = useState<unknown>(undefined);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const board = useRef<BoardHandle | null>(null);
  const onReady = useCallback((handle: BoardHandle) => { board.current = handle; }, []);

  useEffect(() => {
    // Excalidraw 在加载时读取字体路径，必须先设；字体随后台静态文件发布，不走外网。
    (window as unknown as { EXCALIDRAW_ASSET_PATH: string }).EXCALIDRAW_ASSET_PATH = '/_admin/excalidraw/';
    if (!initial) { setScene(null); return; }
    fetch(assetURL(initial.scene)).then((response) => response.json()).then(setScene).catch((problem) => { setError(`画板读取失败：${errorText(problem)}`); setScene(null); });
  }, [initial]);

  async function save() {
    if (!board.current || busy) return;
    setBusy(true);
    setError('');
    try {
      const result = await board.current.snapshot();
      if (!result) { setError('画板是空的，先画点什么。'); return; }
      const sceneRef = await uploadAsset(new File([result.scene], 'scene.json', { type: 'application/json' }), 'drawing_scene');
      const lightSVG = await uploadAsset(new File([result.lightSVG], 'light.svg', { type: 'image/svg+xml' }), 'drawing_svg');
      const darkSVG = await uploadAsset(new File([result.darkSVG], 'dark.svg', { type: 'image/svg+xml' }), 'drawing_svg');
      onSave({ scene: sceneRef, lightSVG, darkSVG, previewURL: 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(result.lightSVG) });
    } catch (problem) {
      setError(`保存失败：${errorText(problem)}`);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal.Backdrop isOpen isDismissable={false} isKeyboardDismissDisabled onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container className="p-0" size="full">
        <Modal.Dialog aria-label="画板" className="flex h-dvh max-h-dvh flex-col gap-0 rounded-none p-0">
          <div className="flex shrink-0 flex-wrap items-center gap-3 border-b border-separator px-4 py-2.5">
            <Brush className="size-4 text-muted" />
            <strong className="text-sm font-semibold">画板</strong>
            {error && <span className="text-sm text-danger">{error}</span>}
            <span className="flex-1" />
            <Button isDisabled={busy} size="sm" variant="tertiary" onPress={onClose}>取消</Button>
            <Button isPending={busy} size="sm" onPress={save}>{busy ? <Spinner color="current" size="sm" /> : null}完成并插入</Button>
          </div>
          <div className="drawing-host min-h-0 flex-1">
            {scene === undefined ? (
              <div className="flex h-full items-center justify-center"><Spinner /></div>
            ) : (
              <Suspense fallback={<div className="flex h-full items-center justify-center"><Spinner /></div>}>
                <DrawingBoard initial={scene as never} onReady={onReady} />
              </Suspense>
            )}
          </div>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
