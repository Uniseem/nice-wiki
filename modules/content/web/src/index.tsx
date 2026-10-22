import { Bars, FileText, Plus } from '@gravity-ui/icons';
import { Button, Drawer } from '@heroui/react';
import { api, askText, confirmLeave, EmptyBlock, ErrorBlock, errorText, notify, useHasGrant, useSearch, t } from '@ui';
import { useCallback, useEffect, useState } from 'react';
import { DocumentEditor } from './DocumentEditor';
import { TreePanel } from './TreePanel';
import type { Meta, Tree } from './types';

// 文档管理：左侧文档树，右侧编辑区；窄屏时文档树收进抽屉。
export default function ContentPage() {
  const search = useSearch();
  const [tree, setTree] = useState<Tree | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [selected, setSelected] = useState<string | null>(() => new URLSearchParams(search).get('doc'));
  const [session, setSession] = useState(0);
  const [listOpen, setListOpen] = useState(false);
  const canEdit = useHasGrant('content.edit');

  const loadTree = useCallback(async () => {
    const next = await api<Tree>('/v1/tree');
    setTree(next);
  }, []);

  useEffect(() => { loadTree().catch(setError); }, [loadTree]);

  const show = useCallback((id: string | null) => {
    setSelected(id);
    setListOpen(false);
    history.replaceState(history.state, '', id ? `/admin/content?doc=${id}` : '/admin/content');
  }, []);

  // 换文档前先让当前编辑器保存，保存失败且用户不放弃时留在原文档。
  const select = useCallback(async (id: string | null) => {
    if (id === selected) setListOpen(false);
    else if (await confirmLeave()) show(id);
  }, [selected, show]);

  const create = async (parentId: string | null) => {
    const parent = parentId ? tree?.documents.find((doc) => doc.documentId === parentId) : null;
    const title = await askText({ title: parent ? t('在“{0}”下新建', parent.title) : t('新建文档'), label: t('标题'), placeholder: t('例如：快速开始'), confirmLabel: t('创建'), description: t('标题随时可以改；页面路径在首次发布时确定。') });
    if (!title?.trim()) return;
    try {
      const created = await api<{ metadata: Meta }>('/v1/documents', { method: 'POST', body: { title: title.trim(), parentId, beforeDocumentId: null } });
      await loadTree();
      await select(created.metadata.documentId);
    } catch (problem) { notify.error(t('新建失败'), errorText(problem)); }
  };

  // 已发布文档的公开地址：/<语言>/<祖先路径段…>/<路径段>；祖先可以是草稿，但都要有路径段。
  const publicPath = useCallback((doc: Meta): string | null => {
    if (!tree) return null;
    const byId = new Map(tree.documents.map((item) => [item.documentId, item]));
    const segments: string[] = [];
    let current: Meta | undefined = byId.get(doc.documentId) ?? doc;
    for (let depth = 0; current && depth < 32; depth++) {
      if (!current.pathSegment) return null;
      segments.unshift(current.pathSegment);
      current = current.parentId ? byId.get(current.parentId) : undefined;
    }
    return `/${doc.sourceLanguage ?? 'zh-Hans'}/${segments.join('/')}`;
  }, [tree]);

  if (error) return <div className="p-6"><ErrorBlock error={error} onRetry={() => { setError(null); loadTree().catch(setError); }} /></div>;

  const panel = tree && (
    <TreePanel docs={tree.documents} selected={selected} treeRevision={tree.treeRevision} onChanged={loadTree} onCreate={create} onSelect={select} />
  );
  const exists = !!tree && !!selected && tree.documents.some((doc) => doc.documentId === selected && (doc.state === 'draft' || doc.state === 'published'));

  return (
    <div className="flex h-[calc(100dvh-3.5rem)] min-h-0">
      <aside className="hidden w-72 shrink-0 flex-col border-e border-separator bg-surface md:flex">{panel}</aside>
      <Drawer.Backdrop isOpen={listOpen} onOpenChange={setListOpen}>
        <Drawer.Content placement="left">
          <Drawer.Dialog aria-label={t('文档列表')} className="h-full w-80 max-w-[90vw] p-0">{panel}</Drawer.Dialog>
        </Drawer.Content>
      </Drawer.Backdrop>
      <section className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-2 border-b border-separator px-3 py-2 md:hidden">
          <Button size="sm" variant="secondary" onPress={() => setListOpen(true)}><Bars />{t('文档列表')}</Button>
        </div>
        {exists ? (
          <DocumentEditor
            key={`${selected}:${session}`}
            docs={tree!.documents}
            documentId={selected!}
            publicPath={publicPath}
            treeRevision={tree!.treeRevision}
            onClosed={() => show(null)}
            onReopen={() => setSession((value) => value + 1)}
            onTreeChanged={loadTree}
          />
        ) : (
          <div className="flex flex-1 items-center justify-center p-6">
            <EmptyBlock
              className="w-full max-w-md"
              icon={<FileText />}
              title={tree ? t('选择一篇文档开始编辑') : t('正在加载文档…')}
              description={t('从左侧文档树里选择，或者新建一篇。编辑会自动保存为草稿，点“发布”后访客才能看到。')}
              action={canEdit && tree ? <Button onPress={() => create(null)}><Plus />{t('新建文档')}</Button> : undefined}
            />
          </div>
        )}
      </section>
    </div>
  );
}
