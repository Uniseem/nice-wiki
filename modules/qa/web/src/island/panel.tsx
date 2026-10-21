import { Sparkles } from '@gravity-ui/icons';
import { Card, Drawer, useMediaQuery } from '@heroui/react';
import { QaChat, type Translate } from './QaChat';

// 问答面板本体，由 index.tsx 按需加载：宽屏从右侧滑出，窄屏从底部弹出。
export function QaPanel({ open, onOpenChange, locale, t }: { open: boolean; onOpenChange: (open: boolean) => void; locale: string; t: Translate }) {
  const wide = useMediaQuery('(min-width: 640px)', { defaultValue: true });
  return (
    <Drawer.Backdrop isOpen={open} variant="transparent" onOpenChange={onOpenChange}>
      <Drawer.Content placement={wide ? 'right' : 'bottom'}>
        <Drawer.Dialog aria-label={t('AI 问答')} className={`flex flex-col gap-0 p-0 ${wide ? 'h-full w-[26rem] max-w-[92vw]' : 'h-[85dvh]'}`}>
          {!wide && <Drawer.Handle />}
          <Drawer.CloseTrigger aria-label={t('关闭')} />
          <Drawer.Header className="border-b border-separator px-4 py-3">
            <Drawer.Heading className="flex items-center gap-2 text-base"><Sparkles className="size-4 text-accent" />{t('AI 问答')}</Drawer.Heading>
          </Drawer.Header>
          <QaChat autoFocus={open} locale={locale} t={t} />
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
