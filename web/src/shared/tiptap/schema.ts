import type { JSONContent } from '@tiptap/core'

export const schemaVersion = 1
export const maximumDocumentBytes = 8 << 20
export const maximumNodes = 20_000
export const maximumDepth = 64

export const blockTypes = [
  'paragraph', 'heading', 'blockquote', 'horizontalRule', 'bulletList', 'orderedList',
  'listItem', 'taskList', 'taskItem', 'callout', 'table', 'tableRow', 'tableCell',
  'tableHeader', 'codeBlock', 'blockMath', 'image', 'attachment', 'drawing', 'video',
  'columns', 'column',
] as const
export const nodeTypes = ['doc', 'text', 'hardBreak', 'inlineMath', ...blockTypes] as const
export const markTypes = ['bold', 'italic', 'underline', 'strike', 'code', 'highlight', 'link', 'internalLink'] as const
export type BlockType = typeof blockTypes[number]

export interface WikiDocument { schemaVersion: number, content: JSONContent }
export interface ValidationIssue { path: string, message: string }

const blocks = new Set<string>(blockTypes)
const nodes = new Set<string>(nodeTypes)
const marks = new Set<string>(markTypes)
const validID = /^[A-Za-z0-9_-]{1,128}$/
const validHash = /^[a-f0-9]{64}$/
const validExtension = /^\.[a-z0-9]{1,16}$/

export function isBlockType(type: string | undefined): type is BlockType {
  return type !== undefined && blocks.has(type)
}
export function newBlockID(): string { return crypto.randomUUID().replaceAll('-', '') }

function cloneNode(source: JSONContent): JSONContent {
  return {
    ...source,
    attrs: source.attrs ? { ...source.attrs } : undefined,
    marks: source.marks?.map(mark => ({ ...mark, attrs: mark.attrs ? { ...mark.attrs } : undefined })),
    content: source.content?.map(cloneNode),
  }
}
function walk(node: JSONContent, visit: (node: JSONContent, path: string, depth: number) => void, path = 'content', depth = 0): void {
  visit(node, path, depth)
  node.content?.forEach((child, index) => walk(child, visit, `${path}.content[${index}]`, depth + 1))
}

// 复制块必须重新分配整棵子树的块 ID。
export function duplicateBlock(source: JSONContent): JSONContent {
  const result = cloneNode(source)
  walk(result, node => { if (isBlockType(node.type)) node.attrs = { ...node.attrs, id: newBlockID() } })
  return result
}

// 粘贴和载入旧编辑状态时只替换缺失、非法或重复的 ID，合法 ID 保持稳定。
export function ensureBlockIDs(source: JSONContent): JSONContent {
  const result = cloneNode(source)
  const seen = new Set<string>()
  walk(result, node => {
    if (!isBlockType(node.type)) return
    const id = typeof node.attrs?.id === 'string' ? node.attrs.id : ''
    if (!validID.test(id) || seen.has(id)) node.attrs = { ...node.attrs, id: newBlockID() }
    seen.add(String(node.attrs?.id))
  })
  return result
}

const stringAttr = (node: JSONContent, name: string) => typeof node.attrs?.[name] === 'string' ? String(node.attrs[name]) : ''
const numberAttr = (node: JSONContent, name: string, fallback = 0) => typeof node.attrs?.[name] === 'number' && Number.isFinite(node.attrs[name]) ? Number(node.attrs[name]) : fallback
const add = (result: ValidationIssue[], path: string, message: string) => result.push({ path, message })

function allowedChild(parent: string | undefined, child: string | undefined): boolean {
  if (parent === undefined) return child === 'doc'
  const inline = child === 'text' || child === 'hardBreak' || child === 'inlineMath'
  if (parent === 'paragraph' || parent === 'heading') return inline
  if (parent === 'codeBlock') return child === 'text'
  if (['text', 'hardBreak', 'horizontalRule', 'image', 'attachment', 'drawing', 'video', 'inlineMath', 'blockMath'].includes(parent)) return false
  if (parent === 'columns') return child === 'column'
  if (parent === 'table') return child === 'tableRow'
  if (parent === 'tableRow') return child === 'tableCell' || child === 'tableHeader'
  if (parent === 'bulletList' || parent === 'orderedList') return child === 'listItem'
  if (parent === 'taskList') return child === 'taskItem'
  if (inline) return false
  return !['doc', 'column', 'tableRow', 'tableCell', 'tableHeader', 'listItem', 'taskItem'].includes(child ?? '')
}

function validateHash(result: ValidationIssue[], node: JSONContent, path: string, name: string, required = false): void {
  const value = stringAttr(node, name)
  if ((required || value !== '') && !validHash.test(value)) add(result, `${path}.attrs.${name}`, '资源哈希无效')
}
function validateExtension(result: ValidationIssue[], node: JSONContent, path: string, required = false): void {
  const value = stringAttr(node, 'extension')
  if ((required || value !== '') && !validExtension.test(value)) add(result, `${path}.attrs.extension`, '资源扩展名无效')
}
function validateAttrs(result: ValidationIssue[], node: JSONContent, path: string): void {
  switch (node.type) {
    case 'heading':
      if (![2, 3, 4].includes(numberAttr(node, 'level'))) add(result, `${path}.attrs.level`, '正文标题仅允许 H2 到 H4')
      break
    case 'orderedList':
      if (!Number.isInteger(numberAttr(node, 'start', 1)) || numberAttr(node, 'start', 1) < 1) add(result, `${path}.attrs.start`, '有序列表起点无效')
      break
    case 'callout':
      if (!['info', 'tip', 'warning', 'danger'].includes(stringAttr(node, 'kind'))) add(result, `${path}.attrs.kind`, '提示框类型无效')
      break
    case 'codeBlock':
      if (!/^[A-Za-z0-9_+-]{0,32}$/.test(stringAttr(node, 'language'))) add(result, `${path}.attrs.language`, '代码语言无效')
      break
    case 'inlineMath': case 'blockMath':
      if (stringAttr(node, 'latex').length > 100_000) add(result, `${path}.attrs.latex`, '公式内容过长')
      break
    case 'image':
      validateHash(result, node, path, 'hash', true); validateExtension(result, node, path, true)
      if (numberAttr(node, 'width', 100) < 10 || numberAttr(node, 'width', 100) > 100) add(result, `${path}.attrs.width`, '图片宽度无效')
      break
    case 'attachment':
      validateHash(result, node, path, 'hash', true); validateExtension(result, node, path, true)
      if (stringAttr(node, 'name').trim() === '') add(result, `${path}.attrs.name`, '附件名称不能为空')
      if (!Number.isInteger(numberAttr(node, 'size')) || numberAttr(node, 'size') < 0) add(result, `${path}.attrs.size`, '附件大小无效')
      break
    case 'drawing':
      validateHash(result, node, path, 'sceneHash', true); validateHash(result, node, path, 'lightHash', true); validateHash(result, node, path, 'darkHash')
      break
    case 'video':
      if (!['upload', 'platform'].includes(stringAttr(node, 'source'))) add(result, `${path}.attrs.source`, '视频来源无效')
      if (stringAttr(node, 'source') === 'upload') { validateHash(result, node, path, 'hash', true); validateExtension(result, node, path, true) }
      else if (stringAttr(node, 'provider') === '' || stringAttr(node, 'providerId') === '') add(result, path, '平台视频缺少提供方或视频 ID')
      validateHash(result, node, path, 'posterHash')
      break
    case 'columns': {
      const widths = node.attrs?.widths
      if (!Array.isArray(widths) || widths.length !== node.content?.length || widths.some(value => typeof value !== 'number' || !Number.isFinite(value) || value <= 0 || value > 100)) add(result, `${path}.attrs.widths`, '栏宽比例无效')
      break
    }
  }
}

export function validateWikiDocument(document: WikiDocument): ValidationIssue[] {
  const result: ValidationIssue[] = []
  if (document.schemaVersion !== schemaVersion) add(result, 'schemaVersion', `不支持文档版本 ${document.schemaVersion}`)
  const seen = new Set<string>()
  let count = 0
  const visit = (node: JSONContent, parent: string | undefined, path: string, depth: number, inColumns: boolean) => {
    if (++count > maximumNodes) { if (count === maximumNodes + 1) add(result, path, '文档节点数量超限'); return }
    if (depth > maximumDepth) { add(result, path, '文档嵌套深度超限'); return }
    if (!nodes.has(node.type ?? '')) add(result, `${path}.type`, `未知节点类型：${node.type ?? ''}`)
    if (!allowedChild(parent, node.type)) add(result, path, `${parent ?? '根'} 不能包含 ${node.type ?? '未命名节点'}`)
    if (isBlockType(node.type)) {
      const id = stringAttr(node, 'id')
      if (!validID.test(id)) add(result, `${path}.attrs.id`, '块 ID 缺失或无效')
      else if (seen.has(id)) add(result, `${path}.attrs.id`, '块 ID 重复')
      else seen.add(id)
    }
    if (node.type === 'text' && node.content?.length) add(result, `${path}.content`, '文本节点不能包含子节点')
    if (node.type !== 'text' && node.text) add(result, `${path}.text`, '只有文本节点可以包含 text')
    if (node.type === 'columns' && inColumns) add(result, path, '超级块不能嵌套')
    if (node.type === 'columns' && ((node.content?.length ?? 0) < 2 || (node.content?.length ?? 0) > 4)) add(result, `${path}.content`, '超级块需要 2 到 4 栏')
    validateAttrs(result, node, path)
    const usedMarks = new Set<string>()
    node.marks?.forEach((mark, index) => {
      const markPath = `${path}.marks[${index}]`
      if (node.type !== 'text') add(result, markPath, '行内格式只能附在文本节点')
      if (!marks.has(mark.type)) add(result, `${markPath}.type`, `未知行内格式：${mark.type}`)
      if (usedMarks.has(mark.type)) add(result, markPath, '同一行内格式不能重复')
      usedMarks.add(mark.type)
      if (mark.type === 'link' && typeof mark.attrs?.href !== 'string') add(result, `${markPath}.attrs.href`, '链接地址无效')
      if (mark.type === 'internalLink' && (typeof mark.attrs?.documentId !== 'string' || !validID.test(mark.attrs.documentId))) add(result, `${markPath}.attrs.documentId`, '站内链接文档 ID 无效')
    })
    node.content?.forEach((child, index) => visit(child, node.type, `${path}.content[${index}]`, depth + 1, inColumns || node.type === 'columns'))
  }
  visit(document.content, undefined, 'content', 0, false)
  return result
}

export function parseWikiDocument(raw: string): WikiDocument {
  if (new TextEncoder().encode(raw).byteLength > maximumDocumentBytes) throw new Error('文档超过大小限制')
  const value: unknown = JSON.parse(raw)
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('文档根对象无效')
  const record = value as Record<string, unknown>
  if (typeof record.schemaVersion !== 'number' || typeof record.content !== 'object' || record.content === null || Array.isArray(record.content)) throw new Error('文档结构无效')
  const document = { schemaVersion: record.schemaVersion, content: record.content as JSONContent }
  const issues = validateWikiDocument(document)
  const firstIssue = issues[0]
  if (firstIssue) throw new Error(`${firstIssue.path}：${firstIssue.message}`)
  return document
}

// 新版本必须在此逐级追加迁移，不能覆盖或跳过旧版本。
const migrations: Record<number, (document: WikiDocument) => WikiDocument> = {}
export function migrateWikiDocument(source: WikiDocument): WikiDocument {
  let result = { schemaVersion: source.schemaVersion, content: cloneNode(source.content) }
  if (!Number.isInteger(result.schemaVersion) || result.schemaVersion < 1 || result.schemaVersion > schemaVersion) throw new Error(`不支持文档版本 ${result.schemaVersion}`)
  while (result.schemaVersion < schemaVersion) {
    const previous = result.schemaVersion
    const migrate = migrations[previous]
    if (!migrate) throw new Error(`缺少版本 ${previous} 的迁移`)
    result = migrate(result)
    if (result.schemaVersion !== previous + 1) throw new Error('文档迁移没有前进一个版本')
  }
  const issues = validateWikiDocument(result)
  const firstIssue = issues[0]
  if (firstIssue) throw new Error(`${firstIssue.path}：${firstIssue.message}`)
  return result
}
