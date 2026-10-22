import { FolderTree } from '@gravity-ui/icons';
import { Button, Drawer } from '@heroui/react';
import { api, confirmLeave, ErrorBlock, errorText, notify, useSearch, useStoredState, t } from '@ui';
import { useCallback, useEffect, useState } from 'react';
import { DocumentEditor } from './DocumentEditor';
import { Overview } from './Overview';
import { TreePanel } from './TreePanel';
import type { Meta, Tree } from './types';

// 文档管理：左侧文档树（宽屏可收起，窄屏收进抽屉），右侧编辑区；没选文档时显示概览。
export default function ContentPage() {
  const search = useSearch();
  const [tree, setTree] = useState<Tree | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [selected, setSelected] = useState<string | null>(() => new URLSearchParams(search).get('doc'));
  const [session, setSession] = useState(0);
  const [listOpen, setListOpen] = useState(false);
  const [treeOpen, setTreeOpen] = useStoredState('nw-content-tree-open', true);
  // 刚新建的文档：打开后聚焦并选中标题，方便直接改名。
  const [fresh, setFresh] = useState<string | null>(null);

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

  // 新建直接生成“无标题文档”并打开，标题在编辑区里改（同常见 wiki 的做法），不再先弹窗问标题。
  const create = async (parentId: string | null) => {
    if (!(await confirmLeave())) return;
    try {
      const created = await api<{ metadata: Meta }>('/v1/documents', { method: 'POST', body: { title: t('无标题文档'), parentId, beforeDocumentId: null } });
      await loadTree();
      setFresh(created.metadata.documentId);
      show(created.metadata.documentId);
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
    <TreePanel docs={tree.documents} publicPath={publicPath} selected={selected} treeRevision={tree.treeRevision} onChanged={loadTree} onCreate={create} onSelect={select} />
  );
  const exists = !!tree && !!selected && tree.documents.some((doc) => doc.documentId === selected && (doc.state === 'draft' || doc.state === 'published'));
  // 宽屏切换侧边文档树，窄屏打开抽屉。
  const toggle = (
    <>
      <Button isIconOnly aria-label={t('文档列表')} className="md:hidden" size="sm" variant="ghost" onPress={() => setListOpen(true)}><FolderTree className="rtl:-scale-x-100" /></Button>
      <Button isIconOnly aria-label={treeOpen ? t('收起文档树') : t('展开文档树')} className="hidden md:inline-flex" size="sm" variant="ghost" onPress={() => setTreeOpen((value) => !value)}>
        <FolderTree className="rtl:-scale-x-100" />
      </Button>
    </>
  );

  return (
    <div className="flex h-[calc(100dvh-3.5rem)] min-h-0">
      {treeOpen && <aside className="hidden w-72 shrink-0 flex-col border-e border-separator bg-surface md:flex">{panel}</aside>}
      <Drawer.Backdrop isOpen={listOpen} onOpenChange={setListOpen}>
        <Drawer.Content placement="left">
          <Drawer.Dialog aria-label={t('文档列表')} className="h-full w-80 max-w-[90vw] p-0">{panel}</Drawer.Dialog>
        </Drawer.Content>
      </Drawer.Backdrop>
      <section className="@container flex min-w-0 flex-1 flex-col">
        {exists ? (
          <DocumentEditor
            key={`${selected}:${session}`}
            docs={tree!.documents}
            documentId={selected!}
            fresh={fresh === selected}
            leading={toggle}
            publicPath={publicPath}
            treeRevision={tree!.treeRevision}
            onClosed={() => show(null)}
            onReopen={() => { setFresh(null); setSession((value) => value + 1); }}
            onSelect={select}
            onTreeChanged={loadTree}
          />
        ) : (
          <Overview docs={tree?.documents ?? null} leading={toggle} onCreate={() => create(null)} onSelect={select} />
        )}
      </section>
    </div>
  );
}
