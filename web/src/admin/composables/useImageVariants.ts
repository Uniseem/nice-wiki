import { ref } from 'vue'

export interface ImageVariant { width: number, file: File }

const workerSource = `self.onmessage=async({data})=>{try{const bitmap=await createImageBitmap(data.file);const width=Math.min(data.width,bitmap.width);const height=Math.round(bitmap.height*width/bitmap.width);const canvas=new OffscreenCanvas(width,height);canvas.getContext('2d').drawImage(bitmap,0,0,width,height);const blob=await canvas.convertToBlob({type:'image/webp',quality:.84});bitmap.close();self.postMessage({width,blob})}catch(error){self.postMessage({error:String(error)})}}`

export function useImageVariants() {
  const processing = ref(false); const progress = ref(0)
  async function create(file: File, widths = [640, 1280, 1920]): Promise<ImageVariant[]> {
    if (!file.type.startsWith('image/') || file.type === 'image/gif') return []
    if (!('Worker' in window) || !('OffscreenCanvas' in window) || !('createImageBitmap' in window)) return []
    processing.value = true; progress.value = 0
    const result: ImageVariant[] = []
    try {
      for (const width of widths) {
        const url = URL.createObjectURL(new Blob([workerSource], { type: 'text/javascript' }))
        const value = await new Promise<{ width: number, blob: Blob }>((resolve, reject) => {
          const worker = new Worker(url)
          worker.onmessage = event => {
            worker.terminate()
            if (event.data.error) reject(new Error(event.data.error))
            else resolve(event.data)
          }
          worker.onerror = event => { worker.terminate(); reject(new Error(event.message)) }
          worker.postMessage({ file, width })
        }).finally(() => URL.revokeObjectURL(url))
        result.push({ width: value.width, file: new File([value.blob], `${file.name}-${value.width}.webp`, { type: 'image/webp' }) })
        progress.value = Math.round(result.length / widths.length * 100)
      }
      return result.filter((value, index, all) => all.findIndex(other => other.width === value.width) === index)
    } finally { processing.value = false }
  }
  return { processing, progress, create }
}
