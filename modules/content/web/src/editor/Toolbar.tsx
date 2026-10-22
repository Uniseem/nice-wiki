import { ArrowUturnCcwLeft, ArrowUturnCwRight, Bold, ChevronDown, Code, Italic, LayoutHeaderCells, Link as LinkIcon, ListCheck, ListOl, ListUl, Paintbrush, Picture, Plus, QuoteOpen, Strikethrough, Underline } from '@gravity-ui/icons';
import { Button, Description, Dropdown, Header, Label, Separator, ToggleButton, Toolbar as ToolbarRoot, Tooltip } from '@heroui/react';
import type { Key } from '@heroui/react';
import { askText, t } from '@ui';
import { NodeSelection } from '@tiptap/pm/state';
import { useEditorState, type Editor } from '@tiptap/react';
import type { ReactNode } from 'react';
import { blockCommands, codeLanguages, type EditorActions } from './commands';

type State = {
  bold: boolean; italic: boolean; underline: boolean; strike: boolean; code: boolean; highlight: boolean; link: boolean;
  bullet: boolean; ordered: boolean; task: boolean; quote: boolean; codeBlock: boolean; table: boolean; heading: number;
  language: string; canUndo: boolean; canRedo: boolean;
  media: '' | 'image' | 'video' | 'drawing'; mediaWidth: number | null;
};

const mediaLabel = { image: t('图片'), video: t('视频'), drawing: t('画板') } as const;
const mediaWidths = [25, 50, 75];

// 选中的图片/视频/画板节点（点一下媒体块就是节点选区）。
function selectedMedia(editor: Editor) {
  const { selection } = editor.state;
  const node = selection instanceof NodeSelection ? selection.node : null;
  return node && (node.type.name === 'image' || node.type.name === 'video' || node.type.name === 'drawing') ? node : null;
}

// 图注是行内内容数组；这里按纯文本编辑（原有格式在改写时不保留）。
function captionText(caption: unknown): string {
  return Array.isArray(caption) ? caption.map((item: { text?: string }) => item.text ?? '').join('') : '';
}

const blockTypes = [
  { id: '0', label: t('正文') },
  { id: '2', label: t('标题 2') },
  { id: '3', label: t('标题 3') },
  { id: '4', label: t('标题 4') },
];

export function EditorToolbar({ editor, editable, actions }: { editor: Editor; editable: boolean; actions: EditorActions }) {
  const state = useEditorState({
    editor,
    selector: ({ editor: e }): State => ({
      bold: e.isActive('bold'), italic: e.isActive('italic'), underline: e.isActive('underline'), strike: e.isActive('strike'),
      code: e.isActive('code'), highlight: e.isActive('highlight'), link: e.isActive('link'),
      bullet: e.isActive('bulletList'), ordered: e.isActive('orderedList'), task: e.isActive('taskList'), quote: e.isActive('blockquote'),
      codeBlock: e.isActive('codeBlock'), table: e.isActive('table'),
      heading: [2, 3, 4].find((level) => e.isActive('heading', { level })) ?? 0,
      language: e.isActive('codeBlock') ? (e.getAttributes('codeBlock').language as string | null) ?? '' : '',
      canUndo: e.can().undo(), canRedo: e.can().redo(),
      media: (selectedMedia(e)?.type.name ?? '') as State['media'], mediaWidth: (selectedMedia(e)?.attrs.widthPercent as number | null | undefined) ?? null,
    }),
  });
  const chain = () => editor.chain().focus();
  const disabled = !editable;

  return (
    <ToolbarRoot aria-label={t('编辑工具')} className="flex items-center gap-0.5 overflow-x-auto [scrollbar-width:none] *:shrink-0 @3xl:flex-wrap @3xl:overflow-visible">
      <Tool label={t('撤销')} shortcut="⌘Z" isDisabled={disabled || !state.canUndo} onPress={() => chain().undo().run()}><ArrowUturnCcwLeft /></Tool>
      <Tool label={t('重做')} shortcut="⇧⌘Z" isDisabled={disabled || !state.canRedo} onPress={() => chain().redo().run()}><ArrowUturnCwRight /></Tool>
      <Divider />
      <Dropdown>
        <Button className="min-w-24 justify-between" isDisabled={disabled} size="sm" variant="ghost">
          {blockTypes.find((type) => Number(type.id) === state.heading)?.label ?? t('正文')}
          <ChevronDown className="size-3.5 text-muted" />
        </Button>
        <Dropdown.Popover placement="bottom start">
          <Dropdown.Menu
            selectedKeys={[String(state.heading)]}
            selectionMode="single"
            onAction={(key: Key) => {
              const level = Number(key);
              if (level === 0) chain().setParagraph().run();
              else chain().setHeading({ level: level as 2 | 3 | 4 }).run();
            }}
          >
            {blockTypes.map((type) => (
              <Dropdown.Item key={type.id} id={type.id} textValue={type.label}>
                <Label className={type.id === '2' ? 'text-lg font-semibold' : type.id === '3' ? 'text-base font-semibold' : type.id === '4' ? 'font-semibold' : ''}>{type.label}</Label>
                <Dropdown.ItemIndicator />
              </Dropdown.Item>
            ))}
          </Dropdown.Menu>
        </Dropdown.Popover>
      </Dropdown>
      <Divider />
      <Toggle label={t('加粗')} shortcut="⌘B" isDisabled={disabled} isSelected={state.bold} onPress={() => chain().toggleBold().run()}><Bold /></Toggle>
      <Toggle label={t('斜体')} shortcut="⌘I" isDisabled={disabled} isSelected={state.italic} onPress={() => chain().toggleItalic().run()}><Italic /></Toggle>
      <Toggle label={t('下划线')} shortcut="⌘U" isDisabled={disabled} isSelected={state.underline} onPress={() => chain().toggleUnderline().run()}><Underline /></Toggle>
      <Toggle label={t('删除线')} isDisabled={disabled} isSelected={state.strike} onPress={() => chain().toggleStrike().run()}><Strikethrough /></Toggle>
      <Toggle label={t('行内代码')} shortcut="⌘E" isDisabled={disabled} isSelected={state.code} onPress={() => chain().toggleCode().run()}><Code /></Toggle>
      <Toggle label={t('高亮')} isDisabled={disabled} isSelected={state.highlight} onPress={() => chain().toggleHighlight().run()}><Paintbrush /></Toggle>
      <Toggle label={t('链接')} shortcut="⌘K" isDisabled={disabled} isSelected={state.link} onPress={actions.editLink}><LinkIcon /></Toggle>
      <Divider />
      <Toggle label={t('无序列表')} isDisabled={disabled} isSelected={state.bullet} onPress={() => chain().toggleBulletList().run()}><ListUl /></Toggle>
      <Toggle label={t('有序列表')} isDisabled={disabled} isSelected={state.ordered} onPress={() => chain().toggleOrderedList().run()}><ListOl /></Toggle>
      <Toggle label={t('任务列表')} isDisabled={disabled} isSelected={state.task} onPress={() => chain().toggleTaskList().run()}><ListCheck /></Toggle>
      <Toggle label={t('引用')} isDisabled={disabled} isSelected={state.quote} onPress={() => chain().toggleBlockquote().run()}><QuoteOpen /></Toggle>
      <Divider />
      <Dropdown>
        <Button isDisabled={disabled} size="sm" variant="ghost"><Plus />{t('插入')}<ChevronDown className="size-3.5 text-muted" /></Button>
        <Dropdown.Popover className="max-h-[70vh] min-w-64" placement="bottom start">
          <Dropdown.Menu onAction={(key: Key) => blockCommands.find((command) => command.id === key)?.run(editor, actions)}>
            {([t('结构'), t('媒体'), t('文本'), t('列表')] as const).map((group) => (
              <Dropdown.Section key={group}>
                <Header>{group}</Header>
                {blockCommands.filter((command) => command.group === group).map((command) => (
                  <Dropdown.Item key={command.id} id={command.id} textValue={command.label}>
                    <command.icon className="size-4 shrink-0 text-muted" />
                    <div className="flex flex-col">
                      <Label>{command.label}</Label>
                      <Description>{command.hint}</Description>
                    </div>
                  </Dropdown.Item>
                ))}
              </Dropdown.Section>
            ))}
          </Dropdown.Menu>
        </Dropdown.Popover>
      </Dropdown>
      {state.table && (
        <Dropdown>
          <Button isDisabled={disabled} size="sm" variant="ghost"><LayoutHeaderCells />{t('表格')}<ChevronDown className="size-3.5 text-muted" /></Button>
          <Dropdown.Popover placement="bottom start">
            <Dropdown.Menu
              onAction={(key: Key) => {
                const commands: Record<string, () => boolean> = {
                  rowBefore: () => chain().addRowBefore().run(), rowAfter: () => chain().addRowAfter().run(),
                  columnBefore: () => chain().addColumnBefore().run(), columnAfter: () => chain().addColumnAfter().run(),
                  deleteRow: () => chain().deleteRow().run(), deleteColumn: () => chain().deleteColumn().run(),
                  header: () => chain().toggleHeaderRow().run(), deleteTable: () => chain().deleteTable().run(),
                };
                commands[String(key)]?.();
              }}
            >
              <Dropdown.Item id="rowBefore" textValue={t('上方插入行')}><Label>{t('上方插入行')}</Label></Dropdown.Item>
              <Dropdown.Item id="rowAfter" textValue={t('下方插入行')}><Label>{t('下方插入行')}</Label></Dropdown.Item>
              <Dropdown.Item id="columnBefore" textValue={t('左侧插入列')}><Label>{t('左侧插入列')}</Label></Dropdown.Item>
              <Dropdown.Item id="columnAfter" textValue={t('右侧插入列')}><Label>{t('右侧插入列')}</Label></Dropdown.Item>
              <Dropdown.Item id="header" textValue={t('切换表头行')}><Label>{t('切换表头行')}</Label></Dropdown.Item>
              <Dropdown.Item id="deleteRow" textValue={t('删除行')}><Label>{t('删除当前行')}</Label></Dropdown.Item>
              <Dropdown.Item id="deleteColumn" textValue={t('删除列')}><Label>{t('删除当前列')}</Label></Dropdown.Item>
              <Dropdown.Item id="deleteTable" textValue={t('删除表格')} variant="danger"><Label>{t('删除整个表格')}</Label></Dropdown.Item>
            </Dropdown.Menu>
          </Dropdown.Popover>
        </Dropdown>
      )}
      {state.media && (
        <Dropdown>
          <Button isDisabled={disabled} size="sm" variant="ghost"><Picture />{mediaLabel[state.media]}<ChevronDown className="size-3.5 text-muted" /></Button>
          <Dropdown.Popover placement="bottom start">
            <Dropdown.Menu
              onAction={async (key: Key) => {
                const node = selectedMedia(editor);
                if (!node) return;
                const type = node.type.name;
                if (String(key).startsWith('w')) { chain().updateAttributes(type, { widthPercent: key === 'wfull' ? null : Number(String(key).slice(1)) }).run(); return; }
                if (key === 'alt') {
                  const alt = await askText({ title: t('替代文字'), label: t('替代文字'), defaultValue: (node.attrs.alt as string | null) ?? '', required: false, description: t('给看不到图片的读者和搜索引擎看；留空则发布时由 AI 自动生成。') });
                  if (alt !== null) chain().updateAttributes(type, { alt: alt.trim() || null, altSource: alt.trim() ? 'manual' : 'auto' }).run();
                }
                if (key === 'caption') {
                  const caption = await askText({ title: t('图注'), label: t('图注'), defaultValue: captionText(node.attrs.caption), required: false, description: t('显示在图片下方，留空则不显示。') });
                  if (caption !== null) chain().updateAttributes(type, { caption: caption.trim() ? [{ type: 'text', text: caption.trim(), marks: [] }] : [] }).run();
                }
                if (key === 'title') {
                  const title = await askText({ title: t('视频标题'), label: t('视频标题'), defaultValue: (node.attrs.title as string | null) ?? '', required: false, validate: (value) => value.length <= 500 ? null : t('最多 500 个字符'), description: t('显示在视频下方，也用于无障碍朗读。') });
                  if (title !== null) chain().updateAttributes(type, { title: title.trim() }).run();
                }
                if (key === 'delete') chain().deleteSelection().run();
              }}
            >
              <Dropdown.Section>
                <Header>{t('显示宽度')}</Header>
                {[...mediaWidths.map((width) => (
                  <Dropdown.Item key={width} id={`w${width}`} textValue={`${width}%`}><Label>{`${width}%`}</Label>{state.mediaWidth === width && <Dropdown.ItemIndicator />}</Dropdown.Item>
                )), <Dropdown.Item key="full" id="wfull" textValue={t('原始宽度')}><Label>{t('原始宽度')}</Label>{state.mediaWidth === null && <Dropdown.ItemIndicator />}</Dropdown.Item>]}
              </Dropdown.Section>
              <Dropdown.Section>
                {[
                  ...(state.media === 'image' ? [<Dropdown.Item key="alt" id="alt" textValue={t('替代文字')}><Label>{t('替代文字…')}</Label></Dropdown.Item>] : []),
                  ...(state.media !== 'video' ? [<Dropdown.Item key="caption" id="caption" textValue={t('图注')}><Label>{t('图注…')}</Label></Dropdown.Item>] : []),
                  ...(state.media === 'video' ? [<Dropdown.Item key="title" id="title" textValue={t('视频标题')}><Label>{t('视频标题…')}</Label></Dropdown.Item>] : []),
                  <Dropdown.Item key="delete" id="delete" textValue={t('删除')} variant="danger"><Label>{t('删除')}</Label></Dropdown.Item>,
                ]}
              </Dropdown.Section>
            </Dropdown.Menu>
          </Dropdown.Popover>
        </Dropdown>
      )}
      {state.codeBlock && (
        <Dropdown>
          <Button isDisabled={disabled} size="sm" variant="ghost"><Code />{state.language || t('纯文本')}<ChevronDown className="size-3.5 text-muted" /></Button>
          <Dropdown.Popover className="max-h-80" placement="bottom start">
            <Dropdown.Menu
              selectedKeys={[state.language || 'plaintext']}
              selectionMode="single"
              onAction={async (key: Key) => {
                let language: string | null = String(key);
                if (language === '__other') language = await askText({ title: t('代码语言'), label: t('语言名称'), placeholder: t('如 kotlin、lua'), description: t('用于公开页语法高亮。'), required: true });
                if (language === null) return;
                chain().updateAttributes('codeBlock', { language: language === 'plaintext' ? null : language.trim().toLowerCase() || null }).run();
              }}
            >
              {[...codeLanguages.map((language) => (
                <Dropdown.Item key={language} id={language} textValue={language}>
                  <Label>{language === 'plaintext' ? t('纯文本') : language}</Label>
                  <Dropdown.ItemIndicator />
                </Dropdown.Item>
              )), <Dropdown.Item key="__other" id="__other" textValue={t('其他')}><Label>{t('其他…')}</Label></Dropdown.Item>]}
            </Dropdown.Menu>
          </Dropdown.Popover>
        </Dropdown>
      )}
    </ToolbarRoot>
  );
}

function Divider() {
  return <Separator className="mx-1 h-5" orientation="vertical" />;
}

function hint(label: string, shortcut?: string): ReactNode {
  return shortcut ? <span className="flex items-center gap-2">{label}<kbd className="font-sans text-xs opacity-70">{shortcut}</kbd></span> : label;
}

function Tool({ label, shortcut, isDisabled, onPress, children }: { label: string; shortcut?: string; isDisabled?: boolean; onPress: () => void; children: ReactNode }) {
  return (
    <Tooltip delay={400}>
      <Button isIconOnly aria-label={label} isDisabled={isDisabled} size="sm" variant="ghost" onPress={onPress}>{children}</Button>
      <Tooltip.Content>{hint(label, shortcut)}</Tooltip.Content>
    </Tooltip>
  );
}

function Toggle({ label, shortcut, isDisabled, isSelected, onPress, children }: { label: string; shortcut?: string; isDisabled?: boolean; isSelected: boolean; onPress: () => void; children: ReactNode }) {
  return (
    <Tooltip delay={400}>
      <ToggleButton isIconOnly aria-label={label} isDisabled={isDisabled} isSelected={isSelected} size="sm" variant="ghost" onPress={onPress}>{children}</ToggleButton>
      <Tooltip.Content>{hint(label, shortcut)}</Tooltip.Content>
    </Tooltip>
  );
}
