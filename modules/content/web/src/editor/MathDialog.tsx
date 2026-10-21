import { Button, Description, Label, Modal, TextArea, TextField } from '@heroui/react';
import katex from 'katex';
import 'katex/dist/katex.min.css';
import { useEffect, useMemo, useState } from 'react';

// 公式：输入 LaTeX，下方实时预览；公开页同样用 KaTeX 渲染。
export function renderMath(latex: string): { html: string; error: string } {
  try {
    return { html: katex.renderToString(latex || '\\;', { displayMode: true, throwOnError: true }), error: '' };
  } catch (problem) {
    return { html: '', error: problem instanceof Error ? problem.message.replace(/^KaTeX parse error: /, '') : '公式有误' };
  }
}

export function MathDialog({ initial, onSave, onClose }: { initial: string | null; onSave: (latex: string) => void; onClose: () => void }) {
  const [latex, setLatex] = useState(initial ?? '');
  useEffect(() => setLatex(initial ?? ''), [initial]);
  const preview = useMemo(() => renderMath(latex), [latex]);
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger />
          <Modal.Header>
            <Modal.Heading>{initial === null ? '插入公式' : '编辑公式'}</Modal.Heading>
          </Modal.Header>
          <Modal.Body className="flex flex-col gap-4">
            <TextField autoFocus fullWidth value={latex} onChange={setLatex}>
              <Label>LaTeX</Label>
              <TextArea className="font-mono" placeholder="例如：E = mc^2 或 \int_0^1 x^2\,dx" rows={4} variant="secondary" />
              <Description>不用写 $$，直接写公式内容。</Description>
            </TextField>
            <div className="min-h-16 overflow-x-auto rounded-xl border border-border bg-surface-secondary px-4 py-3">
              {latex.trim() === '' ? <p className="text-center text-sm text-muted">预览会显示在这里</p>
                : preview.error ? <p className="text-sm text-danger">{preview.error}</p>
                : <div dangerouslySetInnerHTML={{ __html: preview.html }} />}
            </div>
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>取消</Button>
            <Button isDisabled={!latex.trim() || !!preview.error} onPress={() => onSave(latex.trim())}>{initial === null ? '插入' : '保存'}</Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
