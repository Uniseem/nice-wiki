import { Sparkles } from '@gravity-ui/icons';
import { Button, Card, Spinner } from '@heroui/react';
import { lazy, Suspense, useState } from 'react';
import type { Translate } from './QaChat';

export type { Translate };

// 对话、流式读取、人机验证和 Markdown 渲染都在 panel 里，第一次打开问答时才下载；
// 指针移到入口上或入口获得焦点时就开始预取，点开时通常已经就绪。
const loadPanel = () => import('./panel');
const QaPanel = lazy(() => loadPanel().then((module) => ({ default: module.QaPanel })));
const QaInlineChat = lazy(() => loadPanel().then((module) => ({ default: module.QaInlineChat })));

export function preloadQa() {
  void loadPanel();
}

// 文档页右下角的问答入口；面板代码还在下载时，按钮原地显示加载状态。
export function QaFloating({ open, onOpenChange, locale, t }: { open: boolean; onOpenChange: (open: boolean) => void; locale: string; t: Translate }) {
  const [used, setUsed] = useState(open);
  if (open && !used) setUsed(true);
  const launcher = (pending: boolean) => (
    <Button
      aria-label={t('AI 问答')}
      className="fixed bottom-5 end-5 z-40 h-12 rounded-full px-5 shadow-overlay"
      isPending={pending}
      onFocus={preloadQa}
      onHoverStart={preloadQa}
      onPress={() => onOpenChange(true)}
    >
      {pending ? <Spinner color="current" size="sm" /> : <Sparkles />}
      <span className="hidden sm:inline">{t('AI 问答')}</span>
    </Button>
  );
  return (
    <>
      {!open && launcher(false)}
      {used && (
        <Suspense fallback={open ? launcher(true) : null}>
          <QaPanel locale={locale} open={open} t={t} onOpenChange={onOpenChange} />
        </Suspense>
      )}
    </>
  );
}

// 独立问答页：同一套对话，嵌在页面正文里。
export function QaInline({ locale, t }: { locale: string; t: Translate }) {
  return (
    <Suspense fallback={<Card className="flex min-h-[32rem] flex-1 items-center justify-center"><Spinner /></Card>}>
      <QaInlineChat locale={locale} t={t} />
    </Suspense>
  );
}
