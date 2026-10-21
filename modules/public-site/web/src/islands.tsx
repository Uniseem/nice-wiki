import { Display, Globe, Moon, Sparkles, Sun } from '@gravity-ui/icons';
import { Button, Dropdown, Label, useTheme } from '@heroui/react';
import type { Key } from '@heroui/react';

export type Language = { tag: string; href: string; label: string; current: boolean };
export type Config = { kind: string; locale: string; documentId: string; aiEnabled: boolean; askUrl: string; languages: Language[] | null; ui: Record<string, string> };
export type Translate = (key: string) => string;

// 语言名用该语言自己的写法（English、日本語…），浏览器不支持时退回服务端给的名字。
function nativeName(language: Language): string {
  try { return new Intl.DisplayNames([language.tag], { type: 'language' }).of(language.tag) ?? language.label; } catch { return language.label; }
}

export function HeaderActions({ config, t, onAsk, onAskIntent }: { config: Config; t: Translate; onAsk: () => void; onAskIntent?: () => void }) {
  const languages = config.languages ?? [];
  const current = languages.find((language) => language.current);
  return (
    <>
      {config.aiEnabled && (
        <Button size="sm" variant="secondary" onFocus={onAskIntent} onHoverStart={onAskIntent} onPress={onAsk}>
          <Sparkles />
          <span className="hidden sm:inline">{t('AI 问答')}</span>
        </Button>
      )}
      {languages.length > 1 && (
        <Dropdown>
          <Button aria-label={t('语言')} size="sm" variant="ghost">
            <Globe />
            <span className="hidden md:inline">{current ? nativeName(current) : config.locale}</span>
          </Button>
          <Dropdown.Popover placement="bottom end">
            <Dropdown.Menu
              selectedKeys={current ? [current.tag] : []}
              selectionMode="single"
              onAction={(key: Key) => { const target = languages.find((language) => language.tag === key); if (target && !target.current) location.href = target.href; }}
            >
              {languages.map((language) => (
                <Dropdown.Item key={language.tag} id={language.tag} textValue={nativeName(language)}>
                  <Label>{nativeName(language)}</Label>
                  <Dropdown.ItemIndicator />
                </Dropdown.Item>
              ))}
            </Dropdown.Menu>
          </Dropdown.Popover>
        </Dropdown>
      )}
      <ThemeSwitch t={t} />
    </>
  );
}

function ThemeSwitch({ t }: { t: Translate }) {
  const { theme, resolvedTheme, setTheme } = useTheme();
  const options = [['light', t('浅色'), Sun], ['dark', t('深色'), Moon], ['system', t('跟随系统'), Display]] as const;
  return (
    <Dropdown>
      <Button isIconOnly aria-label={t('亮 / 暗')} size="sm" variant="ghost">{resolvedTheme === 'dark' ? <Moon /> : <Sun />}</Button>
      <Dropdown.Popover placement="bottom end">
        <Dropdown.Menu selectedKeys={[theme]} selectionMode="single" onAction={(key: Key) => setTheme(String(key))}>
          {options.map(([id, label, Icon]) => (
            <Dropdown.Item key={id} id={id} textValue={label}>
              <Icon className="size-4 text-muted" />
              <Label>{label}</Label>
              <Dropdown.ItemIndicator />
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}
