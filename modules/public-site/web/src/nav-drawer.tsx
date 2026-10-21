import { Drawer } from '@heroui/react';
import { useState } from 'react';
import type { Translate } from './islands';

// 窄屏的文档树（按需加载）：复用服务端已经输出的导航（同一份 HTML），放进左侧抽屉。
export default function MobileNav({ open, onOpenChange, t }: { open: boolean; onOpenChange: (open: boolean) => void; t: Translate }) {
  const [html] = useState(() => document.getElementById('nw-tree')?.innerHTML ?? '');
  if (!html) return null;
  return (
    <Drawer.Backdrop isOpen={open} onOpenChange={onOpenChange}>
      <Drawer.Content placement="left">
        <Drawer.Dialog aria-label={t('文档树')} className="flex h-full w-80 max-w-[85vw] flex-col p-0">
          <Drawer.CloseTrigger aria-label={t('关闭')} />
          <Drawer.Header className="px-4 py-3"><Drawer.Heading className="text-base">{t('文档树')}</Drawer.Heading></Drawer.Header>
          <nav aria-label={t('文档树')} className="nw-tree min-h-0 flex-1 overflow-y-auto px-3 pb-6 text-sm" dangerouslySetInnerHTML={{ __html: html }} />
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}
