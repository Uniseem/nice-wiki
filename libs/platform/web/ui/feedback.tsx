import { AlertDialog, Button, Description, FieldError, Form, Input, Label, Modal, TextArea, TextField, toast } from '@heroui/react';
import { useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';

// 统一的操作反馈：轻提示用 HeroUI toast，确认和输入用对话框，替代浏览器自带的 alert/confirm/prompt。

export const notify = {
  success: (title: string, description?: string) => { toast.success(title, description ? { description } : undefined); },
  error: (title: string, description?: string) => { toast.danger(title, { description, timeout: 8000 }); },
  info: (title: string, description?: string) => { toast.info(title, description ? { description } : undefined); },
  warning: (title: string, description?: string) => { toast.warning(title, { description, timeout: 6000 }); },
};

type Tone = 'danger' | 'warning' | 'accent';

type ConfirmOptions = {
  title: string;
  body?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: Tone;
};

type PromptOptions = {
  title: string;
  label: string;
  description?: string;
  defaultValue?: string;
  placeholder?: string;
  confirmLabel?: string;
  multiline?: boolean;
  type?: 'text' | 'password' | 'url';
  required?: boolean;
  // 返回错误说明表示不通过，返回 null 表示可以提交。
  validate?: (value: string) => string | null;
};

type Request =
  | ({ kind: 'confirm'; resolve: (ok: boolean) => void } & ConfirmOptions)
  | ({ kind: 'prompt'; resolve: (value: string | null) => void } & PromptOptions);

let queue: Request[] = [];
const listeners = new Set<() => void>();

function emit(): void {
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function confirmAction(options: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => { queue = [...queue, { kind: 'confirm', ...options, resolve }]; emit(); });
}

export function askText(options: PromptOptions): Promise<string | null> {
  return new Promise((resolve) => { queue = [...queue, { kind: 'prompt', ...options, resolve }]; emit(); });
}

// 挂在应用根部一次；按顺序逐个弹出排队的确认和输入框。
export function DialogHost() {
  const head = useSyncExternalStore(subscribe, () => queue[0] ?? null);
  const [open, setOpen] = useState(false);
  const [text, setText] = useState('');
  const [error, setError] = useState<string | null>(null);
  const settled = useRef<Request | null>(null);

  useEffect(() => {
    if (!head) return;
    settled.current = null;
    setText(head.kind === 'prompt' ? head.defaultValue ?? '' : '');
    setError(null);
    setOpen(true);
  }, [head]);

  const finish = (value: boolean | string | null) => {
    if (!head || settled.current === head) return;
    settled.current = head;
    setOpen(false);
    if (head.kind === 'confirm') head.resolve(value === true);
    else head.resolve(typeof value === 'string' ? value : null);
    // 等关闭动画结束再弹下一个。
    setTimeout(() => { queue = queue.slice(1); emit(); }, 180);
  };

  if (!head) return null;
  if (head.kind === 'confirm') {
    const tone = head.tone ?? 'accent';
    return (
      <AlertDialog.Backdrop isOpen={open} isKeyboardDismissDisabled={false} onOpenChange={(next) => { if (!next) finish(false); }}>
        <AlertDialog.Container size="sm">
          <AlertDialog.Dialog>
            <AlertDialog.Header>
              <AlertDialog.Icon status={tone} />
              <AlertDialog.Heading>{head.title}</AlertDialog.Heading>
            </AlertDialog.Header>
            {head.body && <AlertDialog.Body><div className="text-sm leading-6 text-muted">{head.body}</div></AlertDialog.Body>}
            <AlertDialog.Footer>
              <Button variant="tertiary" onPress={() => finish(false)}>{head.cancelLabel ?? '取消'}</Button>
              <Button variant={tone === 'danger' ? 'danger' : 'primary'} onPress={() => finish(true)}>{head.confirmLabel ?? '确认'}</Button>
            </AlertDialog.Footer>
          </AlertDialog.Dialog>
        </AlertDialog.Container>
      </AlertDialog.Backdrop>
    );
  }
  return (
    <Modal.Backdrop isOpen={open} onOpenChange={(next) => { if (!next) finish(null); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header>
            <Modal.Heading>{head.title}</Modal.Heading>
          </Modal.Header>
          <Form
            className="flex flex-col"
            onSubmit={(event) => {
              event.preventDefault();
              const problem = head.validate?.(text) ?? null;
              if (problem) { setError(problem); return; }
              finish(text);
            }}
          >
            <Modal.Body>
              <TextField
                autoFocus
                fullWidth
                isInvalid={!!error}
                isRequired={head.required ?? true}
                type={head.type ?? 'text'}
                value={text}
                onChange={(value) => { setText(value); setError(null); }}
              >
                <Label>{head.label}</Label>
                {head.multiline ? <TextArea placeholder={head.placeholder} rows={5} /> : <Input placeholder={head.placeholder} />}
                {error ? <FieldError>{error}</FieldError> : head.description ? <Description>{head.description}</Description> : null}
              </TextField>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={() => finish(null)}>取消</Button>
              <Button type="submit">{head.confirmLabel ?? '确定'}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
