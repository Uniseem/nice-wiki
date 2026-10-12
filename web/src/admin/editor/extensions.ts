import { VueNodeViewRenderer } from '@tiptap/vue-3'
import { schemaExtensions } from '../../shared/tiptap/extensions'
import CalloutView from './nodeviews/CalloutView.vue'
import CodeBlockView from './nodeviews/CodeBlockView.vue'
import MathView from './nodeviews/MathView.vue'
import ColumnsView from './nodeviews/ColumnsView.vue'
import ImageView from './nodeviews/ImageView.vue'
import AttachmentView from './nodeviews/AttachmentView.vue'
import VideoView from './nodeviews/VideoView.vue'
import DrawingView from './nodeviews/DrawingView.vue'

const views: Record<string, unknown> = {
  callout: CalloutView, codeBlock: CodeBlockView, blockMath: MathView, columns: ColumnsView,
  image: ImageView, attachment: AttachmentView, video: VideoView, drawing: DrawingView,
}

export const editorExtensions = schemaExtensions.map(extension => {
  const component = views[extension.name]
  if (!component) return extension
  const base = extension as unknown as { extend: (config: Record<string, unknown>) => typeof extension }
  return base.extend({ addNodeView: () => VueNodeViewRenderer(component as Parameters<typeof VueNodeViewRenderer>[0]) })
})
