import { MessageCircleIcon as Comment } from 'lucide-react';
import { Button } from '@heroui/react';
import { lazy, Suspense, useState } from 'react';
import { Matrix } from './loader';
import type { QaWelcome, Translate } from './QaChat';

export type { Translate };

// 对话、流式读取、人机验证和 Markdown 渲染都在 panel 里，第一次打开问答时才下载；
// 指针移到入口上或入口获得焦点时就开始预取，点开时通常已经就绪。
const loadPanel = () => import('./panel');
const QaPanel = lazy(() => loadPanel().then((module) => ({ default: module.QaPanel })));
const QaInlineChat = lazy(() => loadPanel().then((module) => ({ default: module.QaInlineChat })));

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
      {pending ? <Matrix className="text-lg" /> : <Comment className="size-5" />}
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

// 独立问答页按整页布局，加载过程也不引入卡片外壳：只在中间放一个点阵加载，代码到了淡入对话。
export function QaInline({ locale, t, welcome }: { locale: string; t: Translate; welcome?: QaWelcome }) {
  return (
    <Suspense fallback={<div className="nw-qa-loading flex min-h-0 flex-1 items-center justify-center text-muted"><Matrix className="text-2xl" /></div>}>
      <QaInlineChat locale={locale} t={t} welcome={welcome} />
    </Suspense>
  );
}
