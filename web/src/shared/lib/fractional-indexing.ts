// 算法语义与 Go 端 roci.dev/fracdex 一致：在两个按字典序排列的键之间取键。
// 本实现只负责前端预览排序，服务端仍会校验并生成最终键。
const alphabet = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz'

function digit(value: string): number {
  const index = alphabet.indexOf(value)
  if (index < 0) throw new Error('排序键包含未知字符')
  return index
}

export function keyBetween(before = '', after = ''): string {
  if (before && after && before >= after) throw new Error('排序边界顺序无效')
  let prefix = ''
  let index = 0
  while (true) {
    const lower = index < before.length ? digit(before[index]!) : -1
    const upper = index < after.length ? digit(after[index]!) : alphabet.length
    if (upper - lower > 1) return prefix + alphabet[Math.floor((lower + upper) / 2)]
    prefix += lower < 0 ? alphabet[0] : alphabet[lower]
    index += 1
  }
}
