import { Display, Globe, Moon, Sun } from '@gravity-ui/icons';
import { Button, Dropdown, Label, useTheme } from '@heroui/react';
import type { Key } from '@heroui/react';
import { adminLanguage, setAdminLanguage } from './i18n';
import { siteLanguages } from './languages';
import { t } from './i18n';

// 浅色 / 深色 / 跟随系统：用 HeroUI 自带的 useTheme（localStorage 的 heroui-theme，html 上的 .dark 类），
// 后台和公开站同源，所以偏好互通。页面首屏由 theme-init 脚本提前套上，避免闪烁。
export function ThemeMenu({ size = 'sm' }: { size?: 'sm' | 'md' }) {
  const { theme, resolvedTheme, setTheme } = useTheme();
  return (
    <Dropdown>
      <Button isIconOnly aria-label={t('切换明暗主题')} size={size} variant="ghost">
        {resolvedTheme === 'dark' ? <Moon /> : <Sun />}
      </Button>
      <Dropdown.Popover placement="bottom end">
        <Dropdown.Menu selectedKeys={[theme]} selectionMode="single" onAction={(key: Key) => setTheme(String(key))}>
          <Dropdown.Item id="light" textValue={t('浅色')}>
            <Sun className="size-4 text-muted" />
            <Label>{t('浅色')}</Label>
            <Dropdown.ItemIndicator />
          </Dropdown.Item>
          <Dropdown.Item id="dark" textValue={t('深色')}>
            <Moon className="size-4 text-muted" />
            <Label>{t('深色')}</Label>
            <Dropdown.ItemIndicator />
          </Dropdown.Item>
          <Dropdown.Item id="system" textValue={t('跟随系统')}>
            <Display className="size-4 text-muted" />
            <Label>{t('跟随系统')}</Label>
            <Dropdown.ItemIndicator />
          </Dropdown.Item>
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}

// 后台界面语言：只在这个浏览器里记住，选完整页刷新。
export function LanguageMenu({ size = 'sm' }: { size?: 'sm' | 'md' }) {
  return (
    <Dropdown>
      <Button isIconOnly aria-label={t('切换界面语言')} size={size} variant="ghost">
        <Globe />
      </Button>
      <Dropdown.Popover placement="bottom end">
        <Dropdown.Menu selectedKeys={[adminLanguage()]} selectionMode="single" onAction={(key: Key) => { if (String(key) !== adminLanguage()) setAdminLanguage(String(key)); }}>
          {siteLanguages.map((language) => (
            <Dropdown.Item key={language.tag} id={language.tag} textValue={language.native}>
              <Label lang={language.tag}>{language.native}</Label>
              <Dropdown.ItemIndicator />
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}
