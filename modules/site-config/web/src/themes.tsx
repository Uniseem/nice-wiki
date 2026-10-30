import { adminLanguage, api } from '@ui';
import type { CSSProperties } from 'react';

// 外观包（docs/theme-standard.md）在后台的共用部分：清单类型、按界面语言取名称、用配色画的示意图。外观页和平台审核都用。

export type Palette = Partial<Record<'background' | 'foreground' | 'surface' | 'muted' | 'default' | 'border' | 'accent' | 'accentForeground', string>>;
export type Manifest = {
  id: string; version: string; name: Record<string, string>; description?: Record<string, string>; author: string; license: string; homepage?: string;
  colorSchemes: ('light' | 'dark')[]; tokens?: { light?: Palette; dark?: Palette };
  typography?: { sans?: string; body?: string }; shape?: { radius?: string }; layout?: { contentWidth?: string; sidebar?: string; toc?: string; header?: string };
  preview?: string;
};

export function localized(values: Record<string, string> | undefined): string {
  if (!values) return '';
  const language = adminLanguage();
  return values[language] ?? values[language.split('-')[0]] ?? values.en ?? values.zh ?? Object.values(values)[0] ?? '';
}

// 默认外观（classic）的配色，外观没写的颜色用它补齐，和 palette.css 一致。
const defaults: Record<'light' | 'dark', Required<Palette>> = {
  light: { background: '#e6e6e6', foreground: '#171717', surface: '#f8f8f8', muted: '#626262', default: '#dcdcdc', border: '#d2d2d2', accent: '#171717', accentForeground: '#fcfcfc' },
  dark: { background: '#17191b', foreground: '#ededed', surface: '#222426', muted: '#a5a9ac', default: '#303436', border: 'rgb(252 252 252 / 0.09)', accent: '#fcfcfc', accentForeground: '#050505' },
};
const radii: Record<string, string> = { none: '0', small: '3px', medium: '6px', large: '10px' };

// ThemeSwatch 用外观的配色、字体、圆角和文档树位置画一张缩略页面；支持明暗两套时左右各画一半。
export function ThemeSwatch({ manifest, className = '' }: { manifest: Manifest; className?: string }) {
  const radius = radii[manifest.shape?.radius ?? 'medium'] ?? radii.medium;
  const right = manifest.layout?.sidebar === 'right';
  return (
    <div aria-hidden className={`flex h-28 overflow-hidden rounded-xl border border-border ${className}`}>
      {manifest.colorSchemes.map((scheme) => {
        const p = { ...defaults[scheme], ...manifest.tokens?.[scheme] };
        const line = (width: string, color: string, opacity = 1): CSSProperties => ({ width, height: 4, borderRadius: 2, background: color, opacity });
        return (
          <div key={scheme} className="flex min-w-0 flex-1 flex-col" style={{ background: p.background, color: p.foreground, fontFamily: manifest.typography?.sans }}>
            <div className="flex h-5 shrink-0 items-center gap-1.5 px-2" style={{ borderBottom: `1px solid ${p.border}` }}>
              <span className="size-2 rounded-full" style={{ background: p.foreground }} />
              <span style={line('2rem', p.muted, 0.6)} />
            </div>
            <div className={`flex min-h-0 flex-1 gap-2 p-2 ${right ? 'flex-row-reverse' : ''}`}>
              <div className="flex w-1/4 flex-col gap-1.5 pt-1">
                <span style={line('100%', p.default)} />
                <span style={line('80%', p.muted, 0.45)} />
                <span style={line('90%', p.muted, 0.45)} />
              </div>
              <div className="flex min-w-0 flex-1 flex-col gap-1.5 p-1.5" style={{ background: p.surface, borderRadius: radius }}>
                <span className="text-[11px] font-semibold leading-none" style={{ fontFamily: manifest.typography?.body ?? manifest.typography?.sans }}>Aa</span>
                <span style={line('100%', p.muted, 0.5)} />
                <span style={line('75%', p.muted, 0.5)} />
                <span className="mt-auto h-3 w-9" style={{ background: p.accent, borderRadius: radius }} />
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

// downloadFile 把接口返回的文件存到本机。
export async function downloadFile(path: string, filename: string): Promise<void> {
  const blob = await api<Blob>(path, { responseType: 'blob' });
  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 10_000);
}

export function zipBody(file: Blob): Blob {
  return new Blob([file], { type: 'application/octet-stream' });
}
