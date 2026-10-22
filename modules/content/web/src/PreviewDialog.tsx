import { ArrowUpRightFromSquare, Display, Eye, Smartphone } from '@gravity-ui/icons';
import { Button, buttonVariants, cn, Modal, ToggleButton, ToggleButtonGroup } from '@heroui/react';
import type { Key } from '@heroui/react';
import { t } from '@ui';
import { useState } from 'react';

// 草稿预览：在后台里用全屏对话框嵌入预览页（同源），不弹新窗口，也就不受浏览器弹窗拦截影响；
// 可切换桌面/手机宽度，需要时用普通链接在新标签页打开。
export function PreviewDialog({ url, title, onClose }: { url: string; title: string; onClose: () => void }) {
  const [width, setWidth] = useState<'desktop' | 'mobile'>('desktop');
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container className="p-0" size="full">
        <Modal.Dialog aria-label={t('草稿预览')} className="flex h-dvh max-h-dvh flex-col gap-0 rounded-none p-0">
          <div className="flex shrink-0 flex-wrap items-center gap-3 border-b border-separator px-4 py-2.5">
            <Eye className="size-4 text-muted" />
            <strong className="min-w-0 truncate text-sm font-semibold">{t('草稿预览：{0}', title)}</strong>
            <span className="hidden text-xs text-muted md:inline">{t('访客看不到这个预览，发布后才会公开。')}</span>
            <span className="flex-1" />
            <ToggleButtonGroup aria-label={t('预览宽度')} selectedKeys={[width]} selectionMode="single" size="sm" onSelectionChange={(keys: Set<Key>) => { const next = [...keys][0]; if (next) setWidth(next as 'desktop' | 'mobile'); }}>
              <ToggleButton id="desktop" aria-label={t('桌面宽度')}><Display /></ToggleButton>
              <ToggleButton id="mobile" aria-label={t('手机宽度')}><Smartphone /></ToggleButton>
            </ToggleButtonGroup>
            <a className={buttonVariants({ size: 'sm', variant: 'tertiary' })} href={url} rel="noopener" target="_blank"><ArrowUpRightFromSquare />{t('在新标签页打开')}</a>
            <Button size="sm" onPress={onClose}>{t('关闭')}</Button>
          </div>
          <div className="flex min-h-0 flex-1 justify-center overflow-auto bg-surface-secondary">
            <iframe className={cn('h-full border-0 bg-background', width === 'mobile' ? 'my-4 h-[calc(100%-2rem)] w-[390px] rounded-2xl shadow-overlay' : 'w-full')} src={url} title={t('草稿预览')} />
          </div>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
