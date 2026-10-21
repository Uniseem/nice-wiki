import { Sparkles } from '@gravity-ui/icons';
import { Button, Card, Drawer, useMediaQuery } from '@heroui/react';
import { QaChat, type Translate } from './QaChat';

export type { Translate };

// 文档页右下角的问答入口：宽屏从右侧滑出，窄屏从底部弹出。
export function QaFloating({ open, onOpenChange, locale, t }: { open: boolean; onOpenChange: (open: boolean) => void; locale: string; t: Translate }) {
  const wide = useMediaQuery('(min-width: 640px)', { defaultValue: true });
  return (
    <>
      {!open && (
        <Button className="fixed bottom-5 end-5 z-40 h-12 rounded-full px-5 shadow-overlay" aria-label={t('AI 问答')} onPress={() => onOpenChange(true)}>
          <Sparkles />
          <span className="hidden sm:inline">{t('AI 问答')}</span>
        </Button>
      )}
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
    </>
  );
}

// 独立问答页：同一套对话，嵌在页面正文里。
export function QaInline({ locale, t }: { locale: string; t: Translate }) {
  return (
    <Card className="flex min-h-[32rem] flex-1 flex-col gap-0 overflow-hidden p-0">
      <QaChat autoFocus locale={locale} t={t} />
    </Card>
  );
}
