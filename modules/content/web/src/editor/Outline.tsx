import { cn } from '@heroui/react';
import { t } from '@ui';
import { useEditorState, type Editor } from '@tiptap/react';
import { useEffect, useRef, useState, type RefObject } from 'react';

type Heading = { level: number; text: string };

// 右侧大纲：列出正文里的 H2–H4，点一下滚到对应位置，滚动时高亮当前所在的小节；编辑区够宽时才显示。
export function Outline({ editor, scroller }: { editor: Editor; scroller: RefObject<HTMLDivElement | null> }) {
  const headings = useEditorState({
    editor,
    selector: ({ editor: e }): Heading[] => {
      const list: Heading[] = [];
      e.state.doc.descendants((node) => {
        if (node.type.name !== 'heading') return true;
        list.push({ level: Number(node.attrs.level), text: node.textContent });
        return false;
      });
      return list;
    },
  });
  const [active, setActive] = useState(-1);
  // 点过的标题：滚到底后它可能到不了顶部，仍以它为当前小节。
  const picked = useRef(-1);

  // 正文 DOM 里的标题与 headings 一一对应（同为文档顺序）；越过可视区上方三分之一的最后一个标题就是当前小节。
  useEffect(() => {
    const host = scroller.current;
    if (!host) return;
    const update = () => {
      const box = host.getBoundingClientRect();
      if (picked.current >= 0 && host.scrollTop + host.clientHeight >= host.scrollHeight - 2) { setActive(picked.current); return; }
      const top = box.top + Math.max(96, box.height / 3);
      let current = -1;
      editor.view.dom.querySelectorAll('h2, h3, h4').forEach((element, index) => { if (element.getBoundingClientRect().top <= top) current = index; });
      setActive(current);
    };
    // 用户自己滚动时不再沿用点过的标题。
    const release = () => { picked.current = -1; };
    update();
    host.addEventListener('scroll', update, { passive: true });
    host.addEventListener('wheel', release, { passive: true });
    host.addEventListener('touchmove', release, { passive: true });
    return () => {
      host.removeEventListener('scroll', update);
      host.removeEventListener('wheel', release);
      host.removeEventListener('touchmove', release);
    };
  }, [editor, scroller, headings.length]);

  const jump = (index: number) => {
    picked.current = index;
    setActive(index);
    editor.view.dom.querySelectorAll('h2, h3, h4')[index]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  return (
    <aside aria-label={t('大纲')} className="sticky top-16 hidden max-h-[calc(100dvh-9rem)] w-52 shrink-0 self-start overflow-y-auto pb-6 pt-10 @5xl:block">
      <div className="mb-2 text-xs font-medium text-muted">{t('大纲')}</div>
      {headings.length ? (
        <ul className="flex flex-col border-s border-separator">
          {headings.map((heading, index) => (
            <li key={index}>
              <button
                className={cn('-ms-px block w-full truncate border-s py-1 pe-1 text-start text-[13px] leading-5 transition-colors', index === active ? 'border-accent font-medium text-accent' : 'border-transparent text-muted hover:text-foreground')}
                style={{ paddingInlineStart: `${(heading.level - 2) * 12 + 12}px` }}
                title={heading.text}
                type="button"
                onClick={() => jump(index)}
              >
                {heading.text || t('（空标题）')}
              </button>
            </li>
          ))}
        </ul>
      ) : <p className="text-xs leading-5 text-muted">{t('正文里的标题会显示在这里。')}</p>}
    </aside>
  );
}
