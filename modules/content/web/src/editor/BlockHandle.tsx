import { Grip } from '@gravity-ui/icons';
import { NodeSelection } from '@tiptap/pm/state';
import type { Editor } from '@tiptap/react';
import { t } from '@ui';
import { useEffect, useState, type DragEvent, type RefObject } from 'react';

type Block = { pos: number; top: number; dom: HTMLElement };

// 块拖动手柄：鼠标停在某个顶层块上时左侧出现抓手。拖动抓手把整块移到别处（交给 ProseMirror 自带的拖放，
// 稳定 ID 随块移动、不会变），点一下选中整块，之后可以复制、删除或用工具栏改格式。
export function BlockHandle({ editor, host }: { editor: Editor; host: RefObject<HTMLDivElement | null> }) {
  const [block, setBlock] = useState<Block | null>(null);

  useEffect(() => {
    const root = host.current;
    if (!root) return;
    const locate = (y: number) => {
      const view = editor.view;
      let found: Block | null = null;
      view.state.doc.forEach((_node, offset) => {
        if (found) return;
        const dom = view.nodeDOM(offset);
        if (!(dom instanceof HTMLElement)) return;
        const rect = dom.getBoundingClientRect();
        if (y >= rect.top - 6 && y <= rect.bottom + 6) found = { pos: offset, top: rect.top - root.getBoundingClientRect().top, dom };
      });
      setBlock((current) => (current?.pos === (found as Block | null)?.pos && current?.top === (found as Block | null)?.top ? current : found));
    };
    // 顶层块数量有限，直接逐块比对位置即可。
    const move = (event: MouseEvent) => {
      if (editor.isEditable && !(event.target as HTMLElement).closest('[data-block-handle]')) locate(event.clientY);
    };
    const leave = () => setBlock(null);
    root.addEventListener('mousemove', move);
    root.addEventListener('mouseleave', leave);
    return () => {
      root.removeEventListener('mousemove', move);
      root.removeEventListener('mouseleave', leave);
    };
  }, [editor, host]);

  // 内容变化后块的位置会变，先收起手柄，等鼠标再动时重新定位。
  useEffect(() => {
    const hide = () => setBlock(null);
    editor.on('update', hide);
    return () => { editor.off('update', hide); };
  }, [editor]);

  if (!block || !editor.isEditable) return null;

  const select = () => {
    const view = editor.view;
    view.dispatch(view.state.tr.setSelection(NodeSelection.create(view.state.doc, block.pos)));
    view.focus();
  };
  const start = (event: DragEvent<HTMLButtonElement>) => {
    const view = editor.view;
    view.dispatch(view.state.tr.setSelection(NodeSelection.create(view.state.doc, block.pos)));
    const slice = view.state.selection.content();
    const { dom, text } = view.serializeForClipboard(slice);
    event.dataTransfer.clearData();
    event.dataTransfer.setData('text/html', dom.innerHTML);
    event.dataTransfer.setData('text/plain', text);
    event.dataTransfer.effectAllowed = 'copyMove';
    event.dataTransfer.setDragImage(block.dom, 0, 0);
    view.dragging = { slice, move: true };
  };

  return (
    <button
      aria-label={t('拖动以移动，点击选中整块')}
      className="absolute -start-8 z-10 hidden h-6 w-5 sm:flex cursor-grab items-center justify-center rounded text-muted opacity-60 transition hover:bg-default hover:opacity-100 active:cursor-grabbing"
      data-block-handle=""
      draggable
      style={{ top: block.top + 2 }}
      title={t('拖动以移动，点击选中整块')}
      type="button"
      onClick={select}
      onDragEnd={() => { editor.view.dragging = null; setBlock(null); }}
      onDragStart={start}
      onMouseDown={(event) => event.stopPropagation()}
    >
      <Grip className="size-3.5" />
    </button>
  );
}
