import { ref } from 'vue'

export function useVideoPoster() {
  const extracting = ref(false)
  async function capture(file: File, at = 0.1): Promise<File> {
    if (!file.type.startsWith('video/')) throw new Error('请选择视频文件')
    extracting.value = true
    const source = URL.createObjectURL(file)
    try {
      const video = document.createElement('video'); video.preload = 'metadata'; video.muted = true; video.src = source
      await new Promise<void>((resolve, reject) => { video.onloadedmetadata = () => { video.currentTime = Math.min(Math.max(0, video.duration * at), Math.max(0, video.duration - .05)) }; video.onseeked = () => resolve(); video.onerror = () => reject(new Error('无法读取视频')) })
      const canvas = document.createElement('canvas'); canvas.width = video.videoWidth; canvas.height = video.videoHeight
      canvas.getContext('2d')?.drawImage(video, 0, 0)
      const blob = await new Promise<Blob>((resolve, reject) => canvas.toBlob(value => value ? resolve(value) : reject(new Error('无法生成视频封面')), 'image/webp', .86))
      return new File([blob], `${file.name}-poster.webp`, { type: 'image/webp' })
    } finally { URL.revokeObjectURL(source); extracting.value = false }
  }
  return { extracting, capture }
}
