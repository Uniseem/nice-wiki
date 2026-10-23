import { cn } from '@heroui/react';
import { api, assetURL } from '@ui';
import markSVG from '@ui/mark.svg?raw';
import { useEffect, useState } from 'react';

type Asset = { assetId: string; sha256: string };
type Brand = { name: string; logo?: Asset | null; themeColor?: string };

// 页头、后台和 favicon 共用平台的书页标志；只替换站点主题色。
const markURL = (color: string) => `data:image/svg+xml,${encodeURIComponent(markSVG.replaceAll('currentColor', color))}`;
const validColor = (color?: string) => (color && /^#[0-9A-Fa-f]{6}$/.test(color) ? color : '#171717');

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
      .catch(() => setBrand({ name: '', themeColor: '#171717' }));
  }, []);
  return brand;
}

// 配置读到之前只占位，标志不会先画成默认色再变成主题色。
export function SiteMark({ brand, className }: { brand: Brand; className: string }) {
  if (brand.logo) return <img alt="" className={cn(className, 'object-contain')} src={assetURL(brand.logo)} />;
  if (!brand.themeColor) return <span className={className} />;
  return <img alt="" className={cn(className, 'rounded-none')} src={markURL(validColor(brand.themeColor))} />;
}
