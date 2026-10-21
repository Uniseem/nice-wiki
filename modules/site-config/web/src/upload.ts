import { api } from './client';

export type AssetRef = { assetId: string; version: number; sha256: string; mediaType: string; byteSize: number };

async function sha256(data: ArrayBuffer): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

const chunkSize = 4 << 20;

// 站标、图标、分享图都是图片资源：内容寻址上传（声明 sha256 → 分块 → 完成），返回不可变资源引用。
export async function uploadImage(file: File): Promise<AssetRef> {
  const bytes = await file.arrayBuffer();
  const begin = await api<{ upload: any }>('/v1/uploads', { method: 'POST', body: { sha256: await sha256(bytes), byteSize: bytes.byteLength, originalName: file.name.slice(0, 255), kind: 'image' } });
  let upload = begin.upload;
  for (let offset = 0; offset < bytes.byteLength; offset += chunkSize) {
    const chunk = bytes.slice(offset, Math.min(bytes.byteLength, offset + chunkSize));
    const response = await api<{ upload: any }>(`/v1/uploads/${upload.uploadId}/chunks`, {
      method: 'PUT', body: new Blob([chunk]), uploadChunk: { offset, sha256: await sha256(chunk), revision: upload.revision },
    });
    upload = response.upload;
  }
  const done = await api<{ asset: { ref: AssetRef } }>(`/v1/uploads/${upload.uploadId}:complete`, { method: 'POST', body: { expectedRevision: upload.revision } });
  return done.asset.ref;
}
