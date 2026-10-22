import { Sparkles, Xmark } from '@gravity-ui/icons';
import { Button, Card, cn, Drawer, useMediaQuery } from '@heroui/react';
import { useEffect } from 'react';
import { QaChat, type Translate } from './QaChat';

type PanelProps = { open: boolean; onOpenChange: (open: boolean) => void; locale: string; t: Translate; documentId?: string; pageTitle?: string };

// 问答面板本体，由 index.tsx 按需加载：宽屏是右下角浮出的对话窗（不遮挡正文，可以边看边问），窄屏从底部弹出。
export function QaPanel(props: PanelProps) {
  const wide = useMediaQuery('(min-width: 640px)', { defaultValue: true });
  return wide ? <FloatingWindow {...props} /> : <BottomSheet {...props} />;
}

function FloatingWindow({ open, onOpenChange, locale, t, documentId, pageTitle }: PanelProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onOpenChange(false); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onOpenChange]);
  // 关上时只是隐藏，对话和滚动位置都保留；打开、关闭带缩放和淡入淡出。
  return (
    <section
      aria-label={t('AI 问答')}
      className={cn(
        'nw-qa-window fixed bottom-5 end-5 z-50 flex h-[min(44rem,calc(100dvh-2.5rem))] w-[27rem] max-w-[calc(100vw-2.5rem)] origin-bottom-right flex-col overflow-hidden rounded-[1.75rem] border border-border bg-overlay shadow-overlay transition-[opacity,transform] duration-200 ease-out rtl:origin-bottom-left',
        open ? 'translate-y-0 scale-100 opacity-100' : 'pointer-events-none translate-y-4 scale-95 opacity-0',
      )}
      inert={!open}
      role="dialog"
    >
      <header className="flex items-center gap-2.5 border-b border-separator px-4 py-3">
        <span className="flex size-8 items-center justify-center rounded-full bg-accent-soft text-accent-soft-foreground"><Sparkles className="size-4" /></span>
        <h2 className="flex-1 text-sm font-semibold text-foreground">{t('AI 问答')}</h2>
        <Button isIconOnly aria-label={t('关闭')} size="sm" variant="ghost" onPress={() => onOpenChange(false)}><Xmark /></Button>
      </header>
      <QaChat autoFocus={open} documentId={documentId} locale={locale} pageTitle={pageTitle} t={t} />
    </section>
  );
}

function BottomSheet({ open, onOpenChange, locale, t, documentId, pageTitle }: PanelProps) {
  return (
    <Drawer.Backdrop isOpen={open} variant="transparent" onOpenChange={onOpenChange}>
      <Drawer.Content placement="bottom">
        <Drawer.Dialog aria-label={t('AI 问答')} className="flex h-[88dvh] flex-col gap-0 p-0">
          <Drawer.Handle />
          <Drawer.CloseTrigger aria-label={t('关闭')} />
          <Drawer.Header className="border-b border-separator px-4 py-3">
            <Drawer.Heading className="flex items-center gap-2 text-base"><Sparkles className="size-4 text-accent" />{t('AI 问答')}</Drawer.Heading>
          </Drawer.Header>
          <QaChat autoFocus={open} documentId={documentId} locale={locale} pageTitle={pageTitle} t={t} />
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}

// 独立问答页里的内嵌形态，和面板共用同一个会话。
export function QaInlineChat({ locale, t }: { locale: string; t: Translate }) {
  return (
    <Card className="flex min-h-[32rem] flex-1 flex-col gap-0 overflow-hidden p-0">
      <QaChat autoFocus locale={locale} t={t} />
    </Card>
  );
}
