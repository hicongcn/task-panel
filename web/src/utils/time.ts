// fmtTime 统一时间显示:YYYY-MM-DD HH:mm:ss
export function fmtTime(t?: string): string {
  if (!t) return '-'
  return t.replace('T', ' ').slice(0, 19)
}
