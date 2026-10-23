import { ChevronsCollapseUpRight, ChevronsExpandUpRight, Xmark } from '@gravity-ui/icons';
import { Button, Card, cn, Drawer, Tooltip, useMediaQuery } from '@heroui/react';
import { useEffect, useState } from 'react';
import { QaChat, type Translate } from './QaChat';

type PanelProps = { open: boolean; onOpenChange: (open: boolean) => void; locale: string; t: Translate; documentId?: string; pageTitle?: string };

// 问答面板本体，由 index.tsx 按需加载：宽屏是右下角浮出的对话窗（不遮挡正文，可以边看边问），窄屏从底部弹出。
export function QaPanel(props: PanelProps) {
  const wide = useMediaQuery('(min-width: 640px)', { defaultValue: true });
  return wide ? <FloatingWindow {...props} /> : <BottomSheet {...props} />;
}

// 放大状态在本标签页内记住，重新打开面板时保持。
const largeKey = 'nw-qa-large';

function FloatingWindow({ open, onOpenChange, locale, t, documentId, pageTitle }: PanelProps) {
  const [large, setLarge] = useState(() => { try { return sessionStorage.getItem(largeKey) === '1'; } catch { return false; } });
  const toggle = () => {
    setLarge(!large);
    try { sessionStorage.setItem(largeKey, large ? '0' : '1'); } catch { /* 隐私模式 */ }
  };
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onOpenChange(false); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onOpenChange]);
  // 关上时只是隐藏，对话和滚动位置都保留；打开、关闭带缩放和淡入淡出。
  // 左上角的按钮把窗口放大到约 3/4 屏宽、3/4 屏高：右下角固定，向左上展开（从右到左的语言里向右上），尺寸变化带过渡。
  const label = large ? t('还原') : t('放大');
  return (
    <section
      aria-label={t('AI 问答')}
      className={cn(
        'nw-qa-window fixed bottom-5 end-5 z-50 flex max-h-[calc(100dvh-2.5rem)] max-w-[calc(100vw-2.5rem)] origin-bottom-right flex-col overflow-hidden rounded-[1.75rem] border border-border bg-overlay shadow-overlay transition-[opacity,transform,width,height] duration-300 ease-[cubic-bezier(.2,.8,.2,1)] motion-reduce:transition-none rtl:origin-bottom-left',
        large ? 'h-[max(75dvh,min(44rem,calc(100dvh-2.5rem)))] w-[max(75vw,27rem)]' : 'h-[min(44rem,calc(100dvh-2.5rem))] w-[27rem]',
        open ? 'translate-y-0 scale-100 opacity-100' : 'pointer-events-none translate-y-4 scale-95 opacity-0',
      )}
      inert={!open}
      role="dialog"
    >
      <header className="flex items-center gap-2.5 border-b border-separator py-3 pe-4 ps-2.5">
        <Tooltip delay={400}>
          <Button isIconOnly aria-label={label} aria-pressed={large} size="sm" variant="ghost" onPress={toggle}>
            {large ? <ChevronsCollapseUpRight className="-scale-x-100 rtl:scale-x-100" /> : <ChevronsExpandUpRight className="-scale-x-100 rtl:scale-x-100" />}
          </Button>
          <Tooltip.Content>{label}</Tooltip.Content>
        </Tooltip>
        <h2 className="flex-1 text-sm font-semibold text-foreground">{t('AI 问答')}</h2>
        <Button isIconOnly aria-label={t('关闭')} size="sm" variant="ghost" onPress={() => onOpenChange(false)}><Xmark /></Button>
      </header>
      <QaChat autoFocus={open} documentId={documentId} locale={locale} pageTitle={pageTitle} t={t} />
    </section>
  );
}

// 窄屏的底部面板：只能从把手和标题栏下拉关闭。对话放在 Drawer.Body 里，抽屉才把这里的上下滑动当成滚动，
// 否则滑动对话会被当成拖动、面板随之偏移。键盘弹出时面板高度收到可视区域以内，标题栏不会被顶出屏幕。
function BottomSheet({ open, onOpenChange, locale, t, documentId, pageTitle }: PanelProps) {
  return (
    <Drawer.Backdrop isOpen={open} variant="transparent" onOpenChange={onOpenChange}>
      <Drawer.Content placement="bottom">
        <Drawer.Dialog aria-label={t('AI 问答')} className="flex h-[min(88dvh,var(--visual-viewport-height,88dvh))] max-h-none flex-col gap-0 p-0">
          <Drawer.Handle />
          <Drawer.CloseTrigger aria-label={t('关闭')} />
          <Drawer.Header className="border-b border-separator px-4 py-3">
            <Drawer.Heading className="text-base">{t('AI 问答')}</Drawer.Heading>
          </Drawer.Header>
          <Drawer.Body className="m-0 flex flex-col overflow-hidden p-0 text-foreground">
            <QaChat documentId={documentId} locale={locale} pageTitle={pageTitle} t={t} />
          </Drawer.Body>
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}

// 首页主视觉里的“问问 AI”：还没提问时只是一个输入框；开始提问后原地展开成对话卡片（多轮，和面板、问答页共用会话），
// 对话在卡片里滚动。卡片高度由 :has() 按是否已有对话切换。
export function QaHomeChat({ locale, t }: { locale: string; t: Translate }) {
  return (
    <div className="nw-home-ask-card flex min-h-0 flex-col">
      <QaChat compact locale={locale} suggestions={[t('这个站点有哪些内容？'), t('我应该从哪里开始？'), t('帮我梳理主要概念')]} t={t} />
    </div>
  );
}

// 独立问答页里的内嵌形态，和面板共用同一个会话；高度固定在一屏之内，对话在卡片里滚动，页面本身不跟着动。
export function QaInlineChat({ locale, t }: { locale: string; t: Translate }) {
  return (
    <Card className="flex h-[max(28rem,calc(100dvh-16rem))] flex-col gap-0 overflow-hidden p-0">
      <QaChat autoFocus locale={locale} t={t} />
    </Card>
  );
}
