import { ArrowUp, Comment } from '@gravity-ui/icons';
import { Button, Card, Spinner } from '@heroui/react';
import { lazy, Suspense, useState } from 'react';
import type { Translate } from './QaChat';

export type { Translate };

// 对话、流式读取、人机验证和 Markdown 渲染都在 panel 里，第一次打开问答时才下载；
// 指针移到入口上或入口获得焦点时就开始预取，点开时通常已经就绪。
const loadPanel = () => import('./panel');
const QaPanel = lazy(() => loadPanel().then((module) => ({ default: module.QaPanel })));
const QaInlineChat = lazy(() => loadPanel().then((module) => ({ default: module.QaInlineChat })));
const QaHomeChat = lazy(() => loadPanel().then((module) => ({ default: module.QaHomeChat })));

export function preloadQa() {
  void loadPanel();
}

// 文档页右下角的问答入口：品牌色渐变的胶囊按钮，带柔光和呼吸光圈（样式在 public-site 的 site.css）；
// 面板代码还在下载时，按钮原地显示加载状态。documentId 是当前页面，AI 会先读它。
export function QaFloating({ open, onOpenChange, locale, t, documentId, pageTitle }: { open: boolean; onOpenChange: (open: boolean) => void; locale: string; t: Translate; documentId?: string; pageTitle?: string }) {
  const [used, setUsed] = useState(open);
  if (open && !used) setUsed(true);
  const launcher = (pending: boolean) => (
    <Button
      aria-label={t('AI 问答')}
      className="nw-qa-launcher fixed bottom-6 end-6 z-40 h-12 min-w-12 gap-2 px-3.5 text-[15px] sm:px-5"
      isPending={pending}
      onFocus={preloadQa}
      onHoverStart={preloadQa}
      onPress={() => onOpenChange(true)}
    >
      {pending ? <Spinner color="current" size="sm" /> : <Comment className="size-5" />}
      <span className="hidden sm:inline">{t('AI 问答')}</span>
    </Button>
  );
  return (
    <>
      {!open && launcher(false)}
      {used && (
        <Suspense fallback={open ? launcher(true) : null}>
          <QaPanel documentId={documentId} locale={locale} open={open} pageTitle={pageTitle} t={t} onOpenChange={onOpenChange} />
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

// 首页的“问问 AI”：面板代码下载完之前先放一个同样大小的输入框占位，下载完换成可以多轮对话的输入框。
export function QaHome({ locale, t }: { locale: string; t: Translate }) {
  return (
    <Suspense fallback={<div className="flex min-h-[6.75rem] flex-col justify-between rounded-3xl border border-border bg-field p-4 text-[15px] text-muted">{t('问问这个站点的文档…')}<span className="ms-auto flex size-9 items-center justify-center rounded-full bg-foreground text-background"><ArrowUp className="size-4" /></span></div>}>
      <QaHomeChat locale={locale} t={t} />
    </Suspense>
  );
}
