import { api } from './api';

// media 的内容寻址上传：声明 sha256 → 顺序分块（每块带自己的 sha256）→ 完成后得到不可变资源引用。
// 文档编辑器（图片、视频、附件、画板）和站点设置（站标、图标、分享图）共用。

export type AssetRef = { assetId: string; version: number; sha256: string; mediaType: string; byteSize: number };
export type AssetKind = 'image' | 'attachment' | 'video' | 'drawing_scene' | 'drawing_svg';
type AssetInfo = { ref: AssetRef; width: number | null; height: number | null; animated: boolean; revision: number; imageVariants: unknown[] };
type UploadState = { uploadId: string; revision: number };

const chunkSize = 4 << 20;
const variantWidths = [640, 1280, 1920];

async function sha256(data: ArrayBuffer): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

export function assetURL(asset: { assetId: string; sha256: string }): string {
  return `/v1/assets/${asset.assetId}/bytes?sha=${asset.sha256}`;
}

async function uploadFull(file: File, kind: AssetKind, onProgress?: (fraction: number) => void): Promise<AssetInfo> {
  const bytes = await file.arrayBuffer();
  const begin = await api<{ upload: UploadState }>('/v1/uploads', { method: 'POST', body: { sha256: await sha256(bytes), byteSize: bytes.byteLength, originalName: file.name.slice(0, 255), kind } });
  let upload = begin.upload;
  for (let offset = 0; offset < bytes.byteLength; offset += chunkSize) {
    const chunk = bytes.slice(offset, Math.min(bytes.byteLength, offset + chunkSize));
    const response = await api<{ upload: UploadState }>(`/v1/uploads/${upload.uploadId}/chunks`, {
      method: 'PUT', body: new Blob([chunk]), uploadChunk: { offset, sha256: await sha256(chunk), revision: upload.revision },
    });
    upload = response.upload;
    onProgress?.(Math.min(1, (offset + chunk.byteLength) / bytes.byteLength));
  }
  const done = await api<{ asset: AssetInfo }>(`/v1/uploads/${upload.uploadId}:complete`, { method: 'POST', body: { expectedRevision: upload.revision } });
  return done.asset;
}

export async function uploadAsset(file: File, kind: AssetKind, onProgress?: (fraction: number) => void): Promise<AssetRef> {
  return (await uploadFull(file, kind, onProgress)).ref;
}

// 图片：上传原图后在浏览器里按 640/1280/1920 宽生成 WebP 变体（不放大、动图不处理）并关联；变体失败不影响原图。
export async function uploadImage(file: File, onProgress?: (fraction: number) => void): Promise<AssetRef> {
  const original = await uploadFull(file, 'image', onProgress);
  try {
    if (original.animated || original.imageVariants.length || !original.width) return original.ref;
    const bitmap = await createImageBitmap(file);
    const variants: { width: number; height: number; asset: AssetRef }[] = [];
    for (const width of variantWidths.filter((w) => w < bitmap.width)) {
      const height = Math.max(1, Math.round((bitmap.height * width) / bitmap.width));
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      canvas.getContext('2d')!.drawImage(bitmap, 0, 0, width, height);
      const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', 0.82));
      if (!blob || blob.type !== 'image/webp') break;
      variants.push({ width, height, asset: await uploadAsset(new File([blob], `${width}.webp`, { type: 'image/webp' }), 'image') });
    }
    if (variants.length) await api(`/v1/assets/${original.ref.assetId}/variants`, { method: 'PUT', body: { original: original.ref, variants, expectedRevision: original.revision } });
  } catch { /* 变体只是优化，失败时原图照常使用 */ }
  return original.ref;
}

// 视频封面：浏览器截取开头附近一帧，作为普通图片资源上传；截不到就不要封面。
export async function captureVideoPoster(file: File): Promise<AssetRef | null> {
  const url = URL.createObjectURL(file);
  try {
    const video = document.createElement('video');
    video.muted = true;
    video.preload = 'auto';
    video.src = url;
    await new Promise<void>((resolve, reject) => { video.onloadeddata = () => resolve(); video.onerror = () => reject(new Error('无法读取视频')); });
    video.currentTime = Math.min(1, (video.duration || 1) / 10);
    await new Promise<void>((resolve) => { video.onseeked = () => resolve(); });
    const scale = Math.min(1, 1280 / video.videoWidth);
    const canvas = document.createElement('canvas');
    canvas.width = Math.round(video.videoWidth * scale);
    canvas.height = Math.round(video.videoHeight * scale);
    canvas.getContext('2d')!.drawImage(video, 0, 0, canvas.width, canvas.height);
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.82));
    return blob ? await uploadAsset(new File([blob], 'poster.jpg', { type: 'image/jpeg' }), 'image') : null;
  } catch { return null; } finally { URL.revokeObjectURL(url); }
}
