import { cn } from '@heroui/react';
import { api, assetURL } from '@ui';
import { useEffect, useState } from 'react';

type Brand = { name: string; logo?: { assetId: string; sha256: string } | null };

// 站名和标志取自公开首页配置（匿名可读），登录页和后台侧栏共用；站点还没初始化时退回 nice-wiki。
export function useSiteBrand(): Brand {
  const [brand, setBrand] = useState<Brand>({ name: '' });
  useEffect(() => {
    api<{ siteName: string; branding?: { logo?: Brand['logo'] } }>('/v1/home/public')
      .then((result) => setBrand({ name: result.siteName, logo: result.branding?.logo }))
      .catch(() => {});
  }, []);
  return brand;
}

export function SiteMark({ brand, className }: { brand: Brand; className: string }) {
  return brand.logo
    ? <img alt="" className={cn(className, 'object-contain')} src={assetURL(brand.logo)} />
    : <span className={cn(className, 'flex items-center justify-center bg-accent font-bold text-accent-foreground')}>{(brand.name || 'N').slice(0, 1).toUpperCase()}</span>;
}
