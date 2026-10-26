import { cn } from '@heroui/react';

// 加载指示：3×3 点阵按顺时针逐格点亮，和首页背景的点阵同一种质感；大小随字号缩放（样式在 public-site 的 site.css）。
export function Matrix({ className }: { className?: string }) {
  return <span aria-hidden className={cn('nw-matrix', className)}>{Array.from({ length: 9 }, (_, index) => <i key={index} />)}</span>;
}
