import { Tabs } from '@heroui/react';
import { EmptyBlock, PageHeader, useCurrentSite, useHasGrant, t } from '@ui';
import { useState } from 'react';
import { BackupPanel } from './BackupPanel';
import { GitPanel } from './GitPanel';
import { ImportPanel } from './ImportPanel';
import { PlatformPanel } from './PlatformPanel';

// 数据与同步：把别处的文档搬进来、把站点内容备份出去、与 Git 仓库同步。
export default function DataPage() {
  const site = useCurrentSite();
  const canImport = useHasGrant('content.edit');
  const canBackup = useHasGrant('backup.manage');
  const canGit = useHasGrant('site.manage');
  const isOperator = useHasGrant('system.manage');
  // 网盘授权跳回来时直接落在“导出与备份”。
  let returning = false;
  try { returning = !!sessionStorage.getItem('nw_drive_return'); } catch { /* 存储被禁用时按默认页签 */ }
  const first = returning && canBackup ? 'backup' : canImport ? 'import' : canBackup ? 'backup' : canGit ? 'git' : 'platform';
  const [tab, setTab] = useState(first);
  if (!site && !isOperator) return <EmptyBlock title={t('先选一个站点')} description={t('在左上角切换到要管理的站点。')} />;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('数据与同步')} description={t('导入别处的文档，把站点内容备份到自己的存储，或与 Git 仓库互相同步。')} />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4 overflow-x-auto">
          <Tabs.List aria-label={t('数据与同步')}>
            {canImport && <Tabs.Tab id="import">{t('导入')}<Tabs.Indicator /></Tabs.Tab>}
            {canBackup && <Tabs.Tab id="backup">{t('导出与备份')}<Tabs.Indicator /></Tabs.Tab>}
            {canGit && <Tabs.Tab id="git">{t('Git 同步')}<Tabs.Indicator /></Tabs.Tab>}
            {isOperator && <Tabs.Tab id="platform">{t('平台备份')}<Tabs.Indicator /></Tabs.Tab>}
          </Tabs.List>
        </Tabs.ListContainer>
        {canImport && <Tabs.Panel id="import"><ImportPanel /></Tabs.Panel>}
        {canBackup && <Tabs.Panel id="backup"><BackupPanel site={site} /></Tabs.Panel>}
        {canGit && <Tabs.Panel id="git"><GitPanel /></Tabs.Panel>}
        {isOperator && <Tabs.Panel id="platform"><PlatformPanel /></Tabs.Panel>}
      </Tabs>
    </div>
  );
}
