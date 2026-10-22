import { Modal } from '@heroui/react';
import { t } from '@ui';

const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);
const mod = mac ? '⌘' : 'Ctrl';
const shift = mac ? '⇧' : 'Shift';

// 编辑器里实际生效的快捷键与 Markdown 输入方式（按 ⌘/ 打开）。
const groups: { title: string; items: [string, string[]][] }[] = [
  {
    title: t('常用'),
    items: [
      [t('立即保存'), [mod, 'S']], [t('撤销'), [mod, 'Z']], [t('重做'), [shift, mod, 'Z']],
      [t('插入块'), ['/']], [t('插入或编辑链接'), [mod, 'K']], [t('快捷键一览'), [mod, '/']],
    ],
  },
  {
    title: t('文字格式'),
    items: [
      [t('加粗'), [mod, 'B']], [t('斜体'), [mod, 'I']], [t('下划线'), [mod, 'U']],
      [t('删除线'), [shift, mod, 'S']], [t('行内代码'), [mod, 'E']], [t('高亮'), [shift, mod, 'H']],
      [t('列表缩进 / 取消缩进'), ['Tab', `${shift} Tab`]],
    ],
  },
  {
    title: t('Markdown 输入'),
    items: [
      [t('标题 2 / 3 / 4'), ['##', '###', '####']], [t('无序列表'), ['-']], [t('有序列表'), ['1.']], [t('任务列表'), ['[ ]']],
      [t('引用'), ['>']], [t('代码块'), ['```']], [t('分割线'), ['---']], [t('加粗、斜体、行内代码'), ['**…**', '*…*', '`…`']],
    ],
  },
];

export function ShortcutsDialog({ onClose }: { onClose: () => void }) {
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="lg">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('快捷键')}</Modal.Heading></Modal.Header>
          <Modal.Body className="grid gap-6 sm:grid-cols-2">
            {groups.map((group) => (
              <section key={group.title} className={group.title === t('Markdown 输入') ? 'sm:col-span-2' : undefined}>
                <h3 className="mb-2 text-xs font-medium text-muted">{group.title}</h3>
                <dl className={group.title === t('Markdown 输入') ? 'grid gap-x-8 sm:grid-cols-2' : 'flex flex-col'}>
                  {group.items.map(([label, keys]) => (
                    <div key={label} className="flex items-center justify-between gap-3 border-b border-separator py-1.5 text-sm last:border-b-0">
                      <dt className="text-foreground">{label}</dt>
                      <dd className="flex shrink-0 gap-1">{keys.map((key) => <kbd key={key} className="min-w-6 rounded-md border border-border bg-surface-secondary px-1.5 text-center font-sans text-xs leading-5 text-muted">{key}</kbd>)}</dd>
                    </div>
                  ))}
                </dl>
              </section>
            ))}
            <p className="text-xs text-muted sm:col-span-2">{t('标题、列表、引用等在行首打出符号再按空格即可转换；加粗等用符号把文字包住。')}</p>
          </Modal.Body>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
