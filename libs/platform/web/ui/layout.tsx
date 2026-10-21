import { CircleExclamation } from '@gravity-ui/icons';
import { Alert, Button, Card, Chip, cn, EmptyState, Skeleton, Spinner } from '@heroui/react';
import type { ReactNode } from 'react';
import { errorText } from './api';

// 各后台页面共用的版式：页头、分区卡片、空状态、加载与错误。

export function PageHeader({ title, description, actions, children }: { title: ReactNode; description?: ReactNode; actions?: ReactNode; children?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">{title}</h1>
        {description && <p className="mt-1.5 max-w-3xl text-sm leading-6 text-muted">{description}</p>}
        {children}
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function Panel({ title, description, actions, footer, children, className }: {
  title?: ReactNode; description?: ReactNode; actions?: ReactNode; footer?: ReactNode; children?: ReactNode; className?: string;
}) {
  return (
    <Card className={cn('gap-4 p-5', className)}>
      {(title || description || actions) && (
        <div className="flex flex-wrap items-start justify-between gap-3">
          <Card.Header className="min-w-0 flex-1 gap-1">
            {title && <Card.Title className="text-base font-semibold">{title}</Card.Title>}
            {description && <Card.Description className="leading-6">{description}</Card.Description>}
          </Card.Header>
          {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
        </div>
      )}
      {children !== undefined && <Card.Content className="gap-4">{children}</Card.Content>}
      {footer && <Card.Footer className="flex flex-wrap items-center justify-end gap-2 border-t border-separator pt-4">{footer}</Card.Footer>}
    </Card>
  );
}

export function EmptyBlock({ icon, title, description, action, className }: { icon?: ReactNode; title: ReactNode; description?: ReactNode; action?: ReactNode; className?: string }) {
  return (
    <EmptyState className={cn('flex flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-border px-6 py-10 text-center', className)}>
      {icon && <span className="flex size-11 items-center justify-center rounded-full bg-default text-muted [&>svg]:size-5">{icon}</span>}
      <div className="max-w-md space-y-1">
        <p className="text-sm font-medium text-foreground">{title}</p>
        {description && <p className="text-sm leading-6 text-muted">{description}</p>}
      </div>
      {action}
    </EmptyState>
  );
}

export function LoadingBlock({ lines = 4, className }: { lines?: number; className?: string }) {
  return (
    <div className={cn('flex flex-col gap-3', className)} aria-busy="true" aria-label="加载中">
      {Array.from({ length: lines }, (_, index) => (
        <Skeleton key={index} className={cn('h-4 rounded-md', index === 0 ? 'w-1/3' : index % 3 === 2 ? 'w-2/3' : 'w-full')} />
      ))}
    </div>
  );
}

export function CenteredSpinner({ label = '加载中…' }: { label?: string }) {
  return (
    <div className="flex min-h-40 flex-col items-center justify-center gap-3 text-sm text-muted">
      <Spinner />
      <span>{label}</span>
    </div>
  );
}

export function ErrorBlock({ error, title = '加载失败', onRetry }: { error: unknown; title?: string; onRetry?: () => void }) {
  return (
    <Alert status="danger">
      <Alert.Indicator><CircleExclamation /></Alert.Indicator>
      <Alert.Content>
        <Alert.Title>{title}</Alert.Title>
        <Alert.Description>{errorText(error)}</Alert.Description>
        {onRetry && <Button className="mt-2" size="sm" variant="secondary" onPress={onRetry}>重试</Button>}
      </Alert.Content>
    </Alert>
  );
}

export function Notice({ status = 'default', title, children, className }: { status?: 'default' | 'accent' | 'success' | 'warning' | 'danger'; title?: ReactNode; children?: ReactNode; className?: string }) {
  return (
    <Alert className={className} status={status}>
      <Alert.Indicator />
      <Alert.Content>
        {title && <Alert.Title>{title}</Alert.Title>}
        {children && <Alert.Description>{children}</Alert.Description>}
      </Alert.Content>
    </Alert>
  );
}

type Tone = 'default' | 'accent' | 'success' | 'warning' | 'danger';
export function StatusChip({ tone = 'default', children }: { tone?: Tone; children: ReactNode }) {
  return <Chip color={tone} size="sm" variant="soft">{children}</Chip>;
}

export function Stat({ label, value, hint, icon }: { label: ReactNode; value: ReactNode; hint?: ReactNode; icon?: ReactNode }) {
  return (
    <Card className="gap-2 p-5">
      <div className="flex items-center justify-between gap-2 text-sm text-muted">
        <span>{label}</span>
        {icon && <span className="text-muted [&>svg]:size-4">{icon}</span>}
      </div>
      <div className="text-3xl font-semibold tracking-tight text-foreground tabular-nums">{value}</div>
      {hint && <div className="text-xs text-muted">{hint}</div>}
    </Card>
  );
}

// 小号计数块：放在卡片里的并排数字。
export function MiniStat({ label, value, tone = 'default' }: { label: ReactNode; value: ReactNode; tone?: Tone }) {
  const color = { default: 'text-foreground', accent: 'text-accent', success: 'text-success', warning: 'text-warning', danger: 'text-danger' }[tone];
  return (
    <div className="rounded-xl border border-border bg-surface-secondary/60 px-4 py-3">
      <div className="text-xs text-muted">{label}</div>
      <div className={cn('mt-1 text-2xl font-semibold tabular-nums', color)}>{value}</div>
    </div>
  );
}

// 键值对列表，用于详情区。
export function Details({ items, className }: { items: [ReactNode, ReactNode][]; className?: string }) {
  return (
    <dl className={cn('grid grid-cols-[minmax(6rem,auto)_1fr] gap-x-4 gap-y-2 text-sm', className)}>
      {items.map(([key, value], index) => (
        <div key={index} className="contents">
          <dt className="text-muted">{key}</dt>
          <dd className="min-w-0 break-words text-foreground">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Mono({ children, className }: { children: ReactNode; className?: string }) {
  return <code className={cn('rounded-md bg-default px-1.5 py-0.5 font-mono text-xs text-foreground break-all', className)}>{children}</code>;
}
