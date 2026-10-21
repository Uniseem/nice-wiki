import { Button, Checkbox, CheckboxGroup, cn, Description, FieldError, Input, Label, ListBox, NumberField, Radio, RadioGroup, Select, Switch, TextArea, TextField } from '@heroui/react';
import type { Key } from '@heroui/react';
import { useEffect, useRef, type ReactNode } from 'react';
import { t } from './i18n';

// 表单控件：统一 HeroUI 的 标签 + 输入 + 说明 + 校验错误 组合。卡片里默认用 secondary（无阴影的填充样式）。

type Variant = 'primary' | 'secondary';
type Common = {
  label?: ReactNode;
  description?: ReactNode;
  error?: string | null;
  isRequired?: boolean;
  isDisabled?: boolean;
  className?: string;
  variant?: Variant;
  'aria-label'?: string;
};

// React Aria 的原生校验只在失焦/提交时刷新显示，而自定义错误期间不记录原生结果；
// 错误消失后会残留旧提示（如“请填写此字段”）。错误消失时补发一次 change，让它按当前值重新提交。
function useRecommit<T extends HTMLElement>(error: string | null | undefined) {
  const ref = useRef<T>(null);
  const had = useRef(false);
  useEffect(() => {
    if (had.current && !error) ref.current?.dispatchEvent(new Event('change'));
    had.current = !!error;
  }, [error]);
  return ref;
}

export function TextInput({ label, description, error, isRequired, isDisabled, className, variant = 'secondary', value, onChange, placeholder, type, maxLength, minLength, pattern, autoComplete, autoFocus, isReadOnly, name, onBlur, validate, ...rest }: Common & {
  value: string; onChange: (value: string) => void; placeholder?: string; type?: 'text' | 'password' | 'email' | 'url' | 'search' | 'tel';
  maxLength?: number; minLength?: number; pattern?: string; autoComplete?: string; autoFocus?: boolean; isReadOnly?: boolean; name?: string;
  onBlur?: () => void; validate?: (value: string) => string | null | undefined;
}) {
  const input = useRecommit<HTMLInputElement>(error);
  return (
    <TextField
      aria-label={rest['aria-label']}
      autoComplete={autoComplete}
      autoFocus={autoFocus}
      className={cn('w-full', className)}
      isDisabled={isDisabled}
      isInvalid={error ? true : undefined}
      isReadOnly={isReadOnly}
      isRequired={isRequired}
      maxLength={maxLength}
      minLength={minLength}
      name={name}
      pattern={pattern}
      type={type}
      validate={validate}
      value={value}
      onBlur={onBlur}
      onChange={onChange}
    >
      {label && <Label>{label}</Label>}
      <Input ref={input} placeholder={placeholder} variant={variant} />
      {description && <Description>{description}</Description>}
      <FieldError>{error || undefined}</FieldError>
    </TextField>
  );
}

export function TextAreaInput({ label, description, error, isRequired, isDisabled, className, variant = 'secondary', value, onChange, placeholder, rows = 4, maxLength, isReadOnly, ...rest }: Common & {
  value: string; onChange: (value: string) => void; placeholder?: string; rows?: number; maxLength?: number; isReadOnly?: boolean;
}) {
  const area = useRecommit<HTMLTextAreaElement>(error);
  return (
    <TextField
      aria-label={rest['aria-label']}
      className={cn('w-full', className)}
      isDisabled={isDisabled}
      isInvalid={error ? true : undefined}
      isReadOnly={isReadOnly}
      isRequired={isRequired}
      maxLength={maxLength}
      value={value}
      onChange={onChange}
    >
      {label && <Label>{label}</Label>}
      <TextArea ref={area} placeholder={placeholder} rows={rows} variant={variant} />
      {description && <Description>{description}</Description>}
      <FieldError>{error || undefined}</FieldError>
    </TextField>
  );
}

export function NumberInput({ label, description, error, isRequired, isDisabled, className, variant = 'secondary', value, onChange, minValue, maxValue, step, formatOptions, ...rest }: Common & {
  value: number; onChange: (value: number) => void; minValue?: number; maxValue?: number; step?: number; formatOptions?: Intl.NumberFormatOptions;
}) {
  return (
    <NumberField
      aria-label={rest['aria-label']}
      className={cn('w-full', className)}
      formatOptions={formatOptions}
      isDisabled={isDisabled}
      isInvalid={error ? true : undefined}
      isRequired={isRequired}
      maxValue={maxValue}
      minValue={minValue}
      step={step}
      value={value}
      variant={variant}
      onChange={(next) => onChange(typeof next === 'number' && Number.isFinite(next) ? next : (minValue ?? 0))}
    >
      {label && <Label>{label}</Label>}
      <NumberField.Group>
        <NumberField.DecrementButton />
        <NumberField.Input />
        <NumberField.IncrementButton />
      </NumberField.Group>
      {description && <Description>{description}</Description>}
      <FieldError>{error || undefined}</FieldError>
    </NumberField>
  );
}

export type Option<V extends string = string> = { value: V; label: ReactNode; text?: string; description?: ReactNode; disabled?: boolean };

export function SelectInput<V extends string>({ label, description, error, isRequired, isDisabled, className, variant = 'secondary', value, onChange, options, placeholder = t('请选择'), ...rest }: Common & {
  value: V | '' | null; onChange: (value: V) => void; options: Option<V>[]; placeholder?: string;
}) {
  return (
    <Select
      aria-label={rest['aria-label']}
      className={cn('w-full', className)}
      disabledKeys={options.filter((option) => option.disabled).map((option) => option.value)}
      isDisabled={isDisabled}
      isInvalid={error ? true : undefined}
      isRequired={isRequired}
      placeholder={placeholder}
      value={value || null}
      variant={variant}
      onChange={(key: Key | Key[] | null) => { if (key !== null && !Array.isArray(key)) onChange(String(key) as V); }}
    >
      {label && <Label>{label}</Label>}
      <Select.Trigger>
        <Select.Value />
        <Select.Indicator />
      </Select.Trigger>
      {description && <Description>{description}</Description>}
      <FieldError>{error || undefined}</FieldError>
      <Select.Popover>
        <ListBox>
          {options.map((option) => (
            <ListBox.Item key={option.value} id={option.value} textValue={option.text ?? (typeof option.label === 'string' ? option.label : option.value)}>
              {option.description ? (
                <div className="flex flex-col">
                  <Label>{option.label}</Label>
                  <Description>{option.description}</Description>
                </div>
              ) : option.label}
              <ListBox.ItemIndicator />
            </ListBox.Item>
          ))}
        </ListBox>
      </Select.Popover>
    </Select>
  );
}

export function SwitchInput({ label, description, isSelected, onChange, isDisabled, className, size = 'md' }: {
  label: ReactNode; description?: ReactNode; isSelected: boolean; onChange: (value: boolean) => void; isDisabled?: boolean; className?: string; size?: 'sm' | 'md' | 'lg';
}) {
  return (
    <Switch className={className} isDisabled={isDisabled} isSelected={isSelected} size={size} onChange={onChange}>
      <Switch.Content>
        <Switch.Control>
          <Switch.Thumb />
        </Switch.Control>
        <Label>{label}</Label>
      </Switch.Content>
      {description && <Description>{description}</Description>}
    </Switch>
  );
}

export function CheckInput({ children, description, isSelected, onChange, isDisabled, className, variant = 'secondary', ...rest }: {
  children?: ReactNode; description?: ReactNode; isSelected: boolean; onChange: (value: boolean) => void; isDisabled?: boolean; className?: string; variant?: Variant; 'aria-label'?: string;
}) {
  return (
    <Checkbox aria-label={rest['aria-label']} className={className} isDisabled={isDisabled} isSelected={isSelected} variant={variant} onChange={onChange}>
      <Checkbox.Content>
        <Checkbox.Control>
          <Checkbox.Indicator />
        </Checkbox.Control>
        {children}
      </Checkbox.Content>
      {description && <Description>{description}</Description>}
    </Checkbox>
  );
}

export function CheckGroupInput<V extends string>({ label, description, value, onChange, options, isDisabled, className, columns = 2 }: {
  label?: ReactNode; description?: ReactNode; value: V[]; onChange: (value: V[]) => void; options: Option<V>[]; isDisabled?: boolean; className?: string; columns?: 1 | 2 | 3;
}) {
  return (
    <CheckboxGroup className={cn('flex flex-col gap-2', className)} isDisabled={isDisabled} value={value} onChange={(next) => onChange(next as V[])}>
      {label && <Label>{label}</Label>}
      {description && <Description>{description}</Description>}
      <div className={cn('grid gap-x-6 gap-y-2', columns === 1 ? 'grid-cols-1' : columns === 2 ? 'sm:grid-cols-2' : 'sm:grid-cols-2 lg:grid-cols-3')}>
        {options.map((option) => (
          <Checkbox key={option.value} isDisabled={option.disabled} value={option.value} variant="secondary">
            <Checkbox.Content>
              <Checkbox.Control>
                <Checkbox.Indicator />
              </Checkbox.Control>
              {option.label}
            </Checkbox.Content>
            {option.description && <Description>{option.description}</Description>}
          </Checkbox>
        ))}
      </div>
    </CheckboxGroup>
  );
}

export function RadioInput<V extends string>({ label, description, value, onChange, options, isDisabled, className, orientation = 'horizontal' }: {
  label?: ReactNode; description?: ReactNode; value: V; onChange: (value: V) => void; options: Option<V>[]; isDisabled?: boolean; className?: string; orientation?: 'horizontal' | 'vertical';
}) {
  return (
    <RadioGroup className={className} isDisabled={isDisabled} orientation={orientation} value={value} variant="secondary" onChange={(next) => onChange(next as V)}>
      {label && <Label>{label}</Label>}
      {description && <Description>{description}</Description>}
      {options.map((option) => (
        <Radio key={option.value} isDisabled={option.disabled} value={option.value}>
          <Radio.Content>
            <Radio.Control>
              <Radio.Indicator />
            </Radio.Control>
            {option.label}
          </Radio.Content>
          {option.description && <Description>{option.description}</Description>}
        </Radio>
      ))}
    </RadioGroup>
  );
}

// 选择文件的按钮：隐藏原生 input，用 HeroUI 按钮触发。
export function FileButton({ accept, onFile, children, isDisabled, isPending, variant = 'secondary', size = 'md', className }: {
  accept?: string; onFile: (file: File) => void; children: ReactNode; isDisabled?: boolean; isPending?: boolean;
  variant?: 'primary' | 'secondary' | 'tertiary' | 'outline' | 'ghost'; size?: 'sm' | 'md' | 'lg'; className?: string;
}) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <Button className={className} isDisabled={isDisabled} isPending={isPending} size={size} variant={variant} onPress={() => input.current?.click()}>{children}</Button>
      <input
        ref={input}
        accept={accept}
        className="hidden"
        type="file"
        onChange={(event) => {
          const file = event.target.files?.[0];
          event.target.value = '';
          if (file) onFile(file);
        }}
      />
    </>
  );
}

// 颜色：左侧色块是浏览器取色器，右侧可直接输入 #RRGGBB。
export function ColorInput({ label, description, value, onChange, placeholder = '#2563EB', isRequired, isDisabled, className }: {
  label: string; description?: ReactNode; value: string; onChange: (value: string) => void; placeholder?: string; isRequired?: boolean; isDisabled?: boolean; className?: string;
}) {
  const valid = /^#[0-9A-Fa-f]{6}$/.test(value);
  return (
    <div className={cn('flex flex-col gap-1', className)}>
      <Label>{label}</Label>
      <div className="flex items-start gap-2">
        <label className="relative size-9 shrink-0 cursor-pointer overflow-hidden rounded-xl border border-border shadow-field" style={{ background: valid ? value : 'transparent' }} title={t('选择颜色')}>
          <input aria-label={t('{0}：选择颜色', label)} className="absolute inset-0 cursor-pointer opacity-0" disabled={isDisabled} type="color" value={valid ? value : '#000000'} onChange={(event) => onChange(event.target.value.toUpperCase())} />
        </label>
        <TextInput
          aria-label={label}
          className="min-w-0 flex-1"
          error={value && !valid ? t('请输入 #RRGGBB 格式，例如 #2563EB') : null}
          isDisabled={isDisabled}
          isRequired={isRequired}
          placeholder={placeholder}
          value={value}
          onChange={(next) => onChange(next.trim())}
        />
      </div>
      {description && <Description>{description}</Description>}
    </div>
  );
}

export function FormGrid({ children, className, columns = 2 }: { children: ReactNode; className?: string; columns?: 1 | 2 | 3 }) {
  return <div className={cn('grid gap-4', columns === 2 && 'sm:grid-cols-2', columns === 3 && 'sm:grid-cols-2 lg:grid-cols-3', className)}>{children}</div>;
}
