// 展示用格式化：时间按浏览器本地时区，大小按 1024 进位，费用按百万分之一（micros）存储。

const dateTime = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
const dateOnly = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' });
const timeOnly = new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

function parse(value: string | number | Date | null | undefined): Date | null {
  if (value === null || value === undefined || value === '') return null;
  const date = value instanceof Date ? value : new Date(value);
  return Number.isFinite(date.getTime()) ? date : null;
}

export function fmtDateTime(value: string | number | Date | null | undefined): string {
  const date = parse(value);
  return date ? dateTime.format(date) : '—';
}

export function fmtDate(value: string | number | Date | null | undefined): string {
  const date = parse(value);
  return date ? dateOnly.format(date) : '—';
}

export function fmtTime(value: string | number | Date | null | undefined): string {
  const date = parse(value);
  return date ? timeOnly.format(date) : '—';
}

// “3 分钟前”式的相对时间，超过一周回到具体日期。
export function fmtRelative(value: string | number | Date | null | undefined): string {
  const date = parse(value);
  if (!date) return '—';
  const seconds = Math.round((Date.now() - date.getTime()) / 1000);
  const future = seconds < 0;
  const abs = Math.abs(seconds);
  const unit = abs < 60 ? `${abs} 秒` : abs < 3600 ? `${Math.round(abs / 60)} 分钟` : abs < 86400 ? `${Math.round(abs / 3600)} 小时` : abs < 7 * 86400 ? `${Math.round(abs / 86400)} 天` : '';
  if (!unit) return dateTime.format(date);
  if (abs < 10) return '刚刚';
  return future ? `${unit}后` : `${unit}前`;
}

export function fmtBytes(bytes: number | null | undefined): string {
  if (bytes === null || bytes === undefined || !Number.isFinite(bytes)) return '—';
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KiB', 'MiB', 'GiB', 'TiB'];
  let value = bytes / 1024;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) { value /= 1024; index++; }
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[index]}`;
}

export function fmtNumber(value: number | null | undefined): string {
  return value === null || value === undefined || !Number.isFinite(value) ? '—' : value.toLocaleString('zh-CN');
}

// 费用：micros 是百万分之一个货币单位，显示到小数点后最多 6 位（去掉末尾 0）。
export function fmtMoney(micros: number | bigint | null | undefined, currency = 'USD'): string {
  if (micros === null || micros === undefined) return '—';
  const value = BigInt(micros);
  const negative = value < 0n;
  const abs = negative ? -value : value;
  const whole = abs / 1000000n;
  const fraction = (abs % 1000000n).toString().padStart(6, '0').replace(/0+$/, '');
  const symbol = currency === 'USD' ? '$' : `${currency} `;
  return `${negative ? '-' : ''}${symbol}${whole.toLocaleString('en-US')}${fraction ? '.' + fraction.padEnd(2, '0') : '.00'}`;
}

export function shortId(id: string | null | undefined, length = 8): string {
  if (!id) return '—';
  return id.length > length ? `${id.slice(0, length)}…` : id;
}
