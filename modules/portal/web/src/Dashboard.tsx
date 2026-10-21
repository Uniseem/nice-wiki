import { ArrowRight, ChartColumn, CircleCheck, Circle, FileText, Globe, Plus, TrashBin } from '@gravity-ui/icons';
import { buttonVariants, Card, Chip, cn } from '@heroui/react';
import { api, fmtNumber, navigate, PageHeader, Stat, useGrants } from '@ui';
import { useEffect, useState, type ReactNode } from 'react';
import { useAccount } from './AdminLayout';

type Doc = { documentId: string; title: string; state: string; parentId: string | null };
type Tiles = { total?: number; published?: number; drafts?: number; trash?: number; visits?: number; languages?: number };
type Step = { key: string; title: string; description: string; done: boolean; href: string };

export default function Dashboard() {
  const account = useAccount();
  const grants = useGrants();
  const [tiles, setTiles] = useState<Tiles>({});
  const [recent, setRecent] = useState<Doc[]>([]);
  const [steps, setSteps] = useState<Step[]>([]);

  useEffect(() => {
    const controller = new AbortController();
    const signal = controller.signal;
    const next: Tiles = {};
    const tasks: Promise<unknown>[] = [];
    const checklist: Step[] = [];
    if (grants.includes('content.read')) {
      tasks.push(api<{ documents: Doc[] }>('/v1/tree', { signal }).then((tree) => {
        const live = tree.documents.filter((doc) => doc.state === 'draft' || doc.state === 'published');
        next.total = live.length;
        next.published = live.filter((doc) => doc.state === 'published').length;
        next.drafts = live.length - next.published;
        next.trash = tree.documents.filter((doc) => doc.state === 'trash').length;
        setRecent(live.slice(-6).reverse());
        checklist.push({ key: 'doc', title: '写第一篇文档并发布', description: '在“文档”里新建、编辑并发布，访客就能在公开站看到。', done: next.published > 0, href: '/admin/content' });
      }).catch(() => {}));
    }
    if (grants.includes('stats.read')) {
      const now = new Date();
      const iso = (date: Date) => date.toISOString().slice(0, 13) + ':00:00Z';
      tasks.push(api<{ rows: { visits?: number }[] }>(`/v1/statistics/pages?from=${iso(new Date(now.getTime() - 7 * 86400_000))}&to=${iso(new Date(now.getTime() + 3600_000))}`, { signal })
        .then((stats) => { next.visits = stats.rows.reduce((sum, row) => sum + (row.visits ?? 0), 0); }).catch(() => {}));
    }
    tasks.push(api<{ languages: { enabled: boolean; isDefault: boolean }[] }>('/v1/languages?includeDisabled=true', { signal }).then((result) => {
      next.languages = result.languages.filter((language) => language.enabled).length;
      if (grants.includes('localization.languages')) {
        checklist.push({ key: 'lang', title: '按需启用其他语言', description: '在“多语言”里添加并启用语言，文档可以翻译成多种语言。', done: result.languages.some((language) => language.enabled && !language.isDefault), href: '/admin/localization' });
      }
    }).catch(() => {}));
    if (grants.includes('model.manage')) {
      tasks.push(api<{ models: unknown[]; taskAssignments: unknown[] }>('/v1/admin/catalog', { signal }).then((catalog) => {
        checklist.push({ key: 'model', title: '接入模型', description: '配置供应方、密钥和模型，再一键生成档位和任务分配。AI 问答和翻译都依赖它。', done: catalog.models.length > 0 && catalog.taskAssignments.length > 0, href: '/admin/models' });
      }).catch(() => {}));
    }
    if (grants.includes('qa.manage')) {
      tasks.push(api<{ policy: { enabled?: boolean } }>('/v1/admin/policy', { signal }).then((result) => {
        checklist.push({ key: 'qa', title: '开启公开问答', description: '在“问答”里打开公开问答，访客就能向文档提问。', done: !!result.policy.enabled, href: '/admin/qa' });
      }).catch(() => {}));
    }
    void Promise.all(tasks).then(() => {
      if (signal.aborted) return;
      setTiles({ ...next });
      const order = ['doc', 'lang', 'model', 'qa'];
      setSteps(checklist.sort((a, b) => order.indexOf(a.key) - order.indexOf(b.key)));
    });
    return () => controller.abort();
  }, [grants]);

  const greeting = new Date().getHours() < 12 ? '上午好' : new Date().getHours() < 18 ? '下午好' : '晚上好';
  const done = steps.filter((step) => step.done).length;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={`${greeting}，${account.accountName}`}
        description="这里汇总站点的整体情况。从左侧进入各个功能页面。"
        actions={grants.includes('content.edit') ? (
          <a className={buttonVariants({ variant: 'primary' })} href="/admin/content" onClick={(event) => { event.preventDefault(); navigate('/admin/content'); }}>
            <Plus />写文档
          </a>
        ) : undefined}
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {tiles.total !== undefined && <Stat icon={<FileText />} label="文档" value={fmtNumber(tiles.total)} hint={`${tiles.published} 篇已发布 · ${tiles.drafts} 篇草稿`} />}
        {tiles.trash !== undefined && <Stat icon={<TrashBin />} label="回收站" value={fmtNumber(tiles.trash)} hint="可以恢复或彻底删除" />}
        {tiles.visits !== undefined && <Stat icon={<ChartColumn />} label="近 7 天访问" value={fmtNumber(tiles.visits)} hint="按小时去重的访次之和" />}
        {tiles.languages !== undefined && <Stat icon={<Globe />} label="启用的语言" value={fmtNumber(tiles.languages)} hint="含基准语言" />}
      </div>

      <div className="grid gap-6 lg:grid-cols-5">
        {steps.length > 0 && (
          <Card className="gap-4 p-5 lg:col-span-3">
            <div className="flex items-center justify-between gap-3">
              <Card.Header className="gap-1">
                <Card.Title className="text-base font-semibold">上手清单</Card.Title>
                <Card.Description>按顺序完成，站点就能完整运转。</Card.Description>
              </Card.Header>
              <Chip color={done === steps.length ? 'success' : 'default'} size="sm" variant="soft">{done} / {steps.length}</Chip>
            </div>
            <ol className="flex flex-col divide-y divide-separator">
              {steps.map((step) => (
                <li key={step.key}>
                  <LinkRow href={step.href}>
                    {step.done ? <CircleCheck className="mt-0.5 size-5 shrink-0 text-success" /> : <Circle className="mt-0.5 size-5 shrink-0 text-muted" />}
                    <span className="min-w-0 flex-1">
                      <span className={cn('block text-sm font-medium', step.done ? 'text-muted line-through' : 'text-foreground')}>{step.title}</span>
                      <span className="block text-sm text-muted">{step.description}</span>
                    </span>
                    <ArrowRight className="mt-0.5 size-4 shrink-0 text-muted" />
                  </LinkRow>
                </li>
              ))}
            </ol>
          </Card>
        )}
        {recent.length > 0 && (
          <Card className={cn('gap-3 p-5', steps.length ? 'lg:col-span-2' : 'lg:col-span-5')}>
            <Card.Header className="gap-1">
              <Card.Title className="text-base font-semibold">最近的文档</Card.Title>
              <Card.Description>点开继续编辑。</Card.Description>
            </Card.Header>
            <ul className="flex flex-col">
              {recent.map((doc) => (
                <li key={doc.documentId}>
                  <LinkRow href={`/admin/content?doc=${doc.documentId}`}>
                    <FileText className="mt-0.5 size-4 shrink-0 text-muted" />
                    <span className="min-w-0 flex-1 truncate text-sm text-foreground">{doc.title}</span>
                    <Chip color={doc.state === 'published' ? 'success' : 'default'} size="sm" variant="soft">{doc.state === 'published' ? '已发布' : '草稿'}</Chip>
                  </LinkRow>
                </li>
              ))}
            </ul>
          </Card>
        )}
      </div>
    </div>
  );
}

function LinkRow({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a className="-mx-2 flex items-start gap-3 rounded-xl px-2 py-3 transition-colors hover:bg-default" href={href} onClick={(event) => { event.preventDefault(); navigate(href); }}>
      {children}
    </a>
  );
}
