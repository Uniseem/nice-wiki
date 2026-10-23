import { Comment, Display, Globe, Magnifier, Moon, Sun } from '@gravity-ui/icons';
import { Button, Dropdown, Label, Modal, useTheme } from '@heroui/react';
import type { Key } from '@heroui/react';
import { rememberLanguage } from './enhance';

export type Language = { tag: string; href: string; label: string; current: boolean; prompt?: Record<string, string> };
export type Config = { kind: string; locale: string; documentId: string; aiEnabled: boolean; askUrl: string; languages: Language[] | null; ui: Record<string, string> };
export type Translate = (key: string) => string;

// 语言名用该语言自己的写法（English、日本語…），浏览器不支持时退回服务端给的名字。
function nativeName(language: Language): string {
  try { return new Intl.DisplayNames([language.tag], { type: 'language' }).of(language.tag) ?? language.label; } catch { return language.label; }
}

export function HeaderActions({ config, t, onAsk, onAskIntent, onSearch, onSearchIntent }: { config: Config; t: Translate; onAsk: () => void; onAskIntent?: () => void; onSearch: () => void; onSearchIntent: () => void }) {
  const languages = config.languages ?? [];
  const current = languages.find((language) => language.current);
  const mac = /Mac|iPhone|iPad/.test(navigator.platform);
  return (
    <>
      {config.kind !== 'home' && <Button aria-label={t('搜索文档')} className="md:min-w-44 md:justify-start" size="sm" variant="outline" onFocus={onSearchIntent} onHoverStart={onSearchIntent} onPress={onSearch}>
        <Magnifier />
        <span className="hidden text-muted md:inline">{t('搜索')}</span>
        <kbd className="ms-auto hidden rounded border border-border px-1 font-sans text-[11px] leading-4 text-muted md:inline">{mac ? '⌘K' : 'Ctrl K'}</kbd>
      </Button>}
      {config.aiEnabled && config.kind !== 'home' && (
        <Button size="sm" variant="primary" onFocus={onAskIntent} onHoverStart={onAskIntent} onPress={onAsk}>
          <Comment className="sm:hidden" />
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
              onAction={(key: Key) => {
                const target = languages.find((language) => language.tag === key);
                if (!target) return;
                rememberLanguage(target.tag);
                if (!target.current) location.href = target.href;
              }}
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

// 决策模型给出的语言建议：居中弹窗，用建议语言和当前语言各写一遍，选“切换”或“保持”都会记住。
export function LanguageSuggestion({ config, t, target, onClose }: { config: Config; t: Translate; target: Language; onClose: () => void }) {
  const tt = (key: string) => target.prompt?.[key] ?? key;
  const current = (config.languages ?? []).find((language) => language.current);
  const stay = () => { if (current) rememberLanguage(current.tag); onClose(); };
  const go = () => { rememberLanguage(target.tag); location.href = target.href; };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) stay(); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.Header className="flex flex-col items-center gap-2 text-center">
            <Globe className="size-6 text-accent" />
            <Modal.Heading lang={target.tag}>{nativeName(target)}</Modal.Heading>
          </Modal.Header>
          <Modal.Body className="flex flex-col gap-1 text-center">
            <p className="text-base font-medium" lang={target.tag}>{tt('要切换到这种语言阅读吗？')}</p>
            <p className="text-sm text-muted">{t('要切换到这种语言阅读吗？')}</p>
          </Modal.Body>
          <Modal.Footer className="flex flex-col gap-2 sm:flex-row sm:justify-center">
            <Button variant="tertiary" onPress={stay}>{t('保持当前语言')} / <span lang={target.tag}>{tt('保持当前语言')}</span></Button>
            <Button onPress={go}><span lang={target.tag}>{tt('切换')}</span> / {t('切换')}</Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
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
