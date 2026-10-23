import { cn } from '@heroui/react';
import { api, assetURL } from '@ui';
import { useEffect, useState } from 'react';

type Asset = { assetId: string; sha256: string };
type Brand = { name: string; logo?: Asset | null; themeColor?: string };

// nice-wiki 的默认标志（与 public-site/internal/mark.go 同一图形）：主题色圆角方块里一张折角的白色文档，左下角带对话气泡的尾巴。
const markSVG = (color: string) => `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'><rect width='64' height='64' rx='16' fill='${color}'/>`
  + `<path fill='#fff' d='M24 12h12l11 11v16a7 7 0 0 1-7 7H29.5l-8.6 6.9A1.8 1.8 0 0 1 18 51.5V19a7 7 0 0 1 6-7Z'/>`
  + `<path fill='${color}' fill-opacity='.4' d='M36 12v5.5a5.5 5.5 0 0 0 5.5 5.5H47Z'/>`
  + `<rect x='25' y='27' width='15' height='4' rx='2' fill='${color}'/><rect x='25' y='35' width='9' height='4' rx='2' fill='${color}'/></svg>`;
const markURL = (color: string) => `data:image/svg+xml,${encodeURIComponent(markSVG(color))}`;
const validColor = (color?: string) => (color && /^#[0-9A-Fa-f]{6}$/.test(color) ? color : '#050505');

// 站名、标志和主题色取自公开首页配置（匿名可读），登录页和后台侧栏共用；站点还没初始化时退回 nice-wiki。
// 后台标签页的图标跟站点一致：上传了图标就用它，否则用按主题色着色的默认标志。
export function useSiteBrand(): Brand {
  const [brand, setBrand] = useState<Brand>({ name: '' });
  useEffect(() => {
    api<{ siteName: string; branding?: { logo?: Asset | null; favicon?: Asset | null; themeColor?: string } }>('/v1/home/public')
      .then((result) => {
        const branding = result.branding ?? {};
        setBrand({ name: result.siteName, logo: branding.logo, themeColor: validColor(branding.themeColor) });
        let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
        if (!link) {
          link = document.createElement('link');
          link.rel = 'icon';
          document.head.append(link);
        }
        link.href = branding.favicon ? assetURL(branding.favicon) : markURL(validColor(branding.themeColor));
      })
      .catch(() => setBrand({ name: '', themeColor: '#050505' }));
  }, []);
  return brand;
}

// 配置读到之前只占位，标志不会先画成默认色再变成主题色。
export function SiteMark({ brand, className }: { brand: Brand; className: string }) {
  if (brand.logo) return <img alt="" className={cn(className, 'object-contain')} src={assetURL(brand.logo)} />;
  if (!brand.themeColor) return <span className={className} />;
  return <img alt="" className={cn(className, 'rounded-none')} src={markURL(validColor(brand.themeColor))} />;
}
