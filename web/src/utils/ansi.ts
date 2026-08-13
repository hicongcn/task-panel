// ANSI 转义 → HTML 渲染。
// 安全要点:所有文本段在拼接成 HTML 前必须先 HTML 转义,杜绝日志注入 XSS。

const ESC_MAP: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
}

function escapeHtml(text: string): string {
  return text.replace(/[&<>"]/g, (c) => ESC_MAP[c] ?? c)
}

const ANSI_COLORS: Record<number, string> = {
  30: '#000', 31: '#e06c75', 32: '#98c379', 33: '#e5c07b',
  34: '#61afef', 35: '#c678dd', 36: '#56b6c2', 37: '#abb2bf',
  90: '#5c6370', 91: '#ff6b6b', 92: '#9ae68d', 93: '#ffd866',
  94: '#74b8ff', 95: '#d6a8ff', 96: '#9ce6e6', 97: '#fff',
}

// ansiToHtml 把含 ANSI 颜色码的文本转成带 <span style> 的 HTML。
// 文本内容已先经 escapeHtml 转义,可安全 v-html。
export function ansiToHtml(text: string): string {
  if (!text) return ''
  // 先转义,再做颜色替换(颜色码不含 HTML 特殊字符,顺序安全)。
  const escaped = escapeHtml(text)
  if (escaped.indexOf('\x1b[') === -1) return escaped

  const regex = /\x1b\[([0-9;]*)m/g
  let result = ''
  let last = 0
  let fg = ''
  let open = false
  let m: RegExpExecArray | null

  while ((m = regex.exec(escaped)) !== null) {
    result += escaped.slice(last, m.index)
    last = m.index + m[0].length
    const codes = m[1] ? m[1].split(';').map(Number) : [0]
    for (const code of codes) {
      if (code === 0) { fg = ''; if (open) { result += '</span>'; open = false } }
      else if (ANSI_COLORS[code]) {
        if (open) result += '</span>'
        fg = ANSI_COLORS[code]
        result += `<span style="color:${fg}">`
        open = true
      }
    }
  }
  result += escaped.slice(last)
  if (open) result += '</span>'
  return result
}
