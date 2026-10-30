// Gambar statis peta keterkaitan (SVG) untuk ekspor — tidak bergantung pada graf interaktif yang sedang tampil.
// Layout memakai gaya simulasi yang sama dengan SiteRelationGraph, dijalankan sampai stabil tanpa animasi.
import * as d3 from 'd3'

type Theme = 'light' | 'dark'
const THEMES = {
  dark: { bg: '#030a17', link: '#334155', linkCommon: '#475569', text: '#e2e8f0', attrText: '#cbd5e1', halo: '#030a17', center: '#e0e7ff', legend: '#94a3b8' },
  light: { bg: '#ffffff', link: '#cbd5e1', linkCommon: '#e2e8f0', text: '#0f172a', attrText: '#334155', halo: '#ffffff', center: '#0f172a', legend: '#475569' },
}
const esc = (v: any) => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' } as any)[c])
const short = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/$/, '')
const cut = (s: string, n: number) => (s.length > n ? s.slice(0, n) + '…' : s)
const siteColor = (s: any) => (s.is_gambling ? (s.priority === 'High' ? '#f43f5e' : '#f59e0b') : '#64748b')

export function buildRelationSvg(rel: any, kindMeta: Record<string, { label: string; color: string }>,
  opts: { theme?: Theme; showIsolated?: boolean; width?: number; maxSiteLabels?: number } = {}) {
  if (!rel?.site) return ''
  const th = THEMES[opts.theme ?? 'light']
  const nodes: any[] = [], links: any[] = []
  const center = { id: 'c', type: 'center', label: short(rel.site.url), site: rel.site, fx: 0, fy: 0 }
  nodes.push(center)
  const siteNodes = new Map<number, any>()
  const usedKinds = new Set<string>()
  for (const a of rel.attributes ?? []) {
    if (!opts.showIsolated && !a.sites.length && !a.common) continue
    const meta = kindMeta[a.kind] ?? { label: a.kind, color: '#94a3b8' }
    usedKinds.add(a.kind)
    const an = { id: `a${a.kind}${a.id}`, type: 'attr', attr: a, color: meta.color, label: a.label && a.kind === 'artifact' ? `${a.label}: ${a.value}` : a.value }
    nodes.push(an)
    links.push({ source: 'c', target: an.id, common: a.common })
    for (const s of a.sites) {
      let sn = siteNodes.get(s.id)
      if (!sn) { sn = { id: `s${s.id}`, type: 'site', label: short(s.url), site: s, deg: 0 }; siteNodes.set(s.id, sn); nodes.push(sn) }
      sn.deg++
      links.push({ source: an.id, target: sn.id })
    }
  }
  if (nodes.length === 1) return ''

  const sim = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id((d: any) => d.id).distance((l: any) => (l.source.type === 'center' ? 130 : 90)).strength(0.7))
    .force('charge', d3.forceManyBody().strength((d: any) => (d.type === 'center' ? -900 : d.type === 'attr' ? -320 : -140)))
    .force('collide', d3.forceCollide().radius((d: any) => (d.type === 'center' ? 40 : 22)))
    .force('radial', d3.forceRadial((d: any) => (d.type === 'attr' ? 150 : d.type === 'site' ? 300 : 0), 0, 0).strength(0.35))
    .stop()
  for (let i = 0; i < 400; i++) sim.tick()

  // Label situs: hanya yang paling banyak berbagi atribut / judol, agar tidak bertumpuk
  const maxLabels = opts.maxSiteLabels ?? 25
  const labeled = new Set([...siteNodes.values()]
    .sort((a, b) => b.deg - a.deg || Number(b.site.is_gambling) - Number(a.site.is_gambling)).slice(0, maxLabels).map(n => n.id))

  const pad = 30, labelRoom = 190
  const xs = nodes.map(n => n.x), ys = nodes.map(n => n.y)
  const x0 = Math.min(...xs) - pad, x1 = Math.max(...xs) + labelRoom, y0 = Math.min(...ys) - pad, y1 = Math.max(...ys) + pad
  const legendH = 28 + Math.ceil((usedKinds.size + 3) / 5) * 18
  const W = x1 - x0, H = y1 - y0 + legendH

  const out: string[] = []
  out.push(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="${x0} ${y0} ${W} ${H}" width="${opts.width ?? W}" height="${opts.width ? (opts.width * H) / W : H}" font-family="Inter, -apple-system, Segoe UI, sans-serif">`)
  out.push(`<rect x="${x0}" y="${y0}" width="${W}" height="${H}" fill="${th.bg}"/>`)
  for (const l of links) {
    out.push(`<line x1="${l.source.x.toFixed(1)}" y1="${l.source.y.toFixed(1)}" x2="${l.target.x.toFixed(1)}" y2="${l.target.y.toFixed(1)}" stroke="${l.common ? th.linkCommon : th.link}" stroke-width="${l.common ? 1 : 1.4}"${l.common ? ' stroke-dasharray="4,4"' : ''}/>`)
  }
  for (const d of nodes) {
    const r = d.type === 'center' ? 18 : d.type === 'attr' ? 9 : 7
    const col = d.type === 'attr' ? d.color : siteColor(d.site)
    const fillOp = d.type === 'attr' ? (d.attr.common ? 0.2 : 0.8) : d.type === 'center' ? 1 : 0.75
    out.push(`<circle cx="${d.x.toFixed(1)}" cy="${d.y.toFixed(1)}" r="${r}" fill="${col}" fill-opacity="${fillOp}" stroke="${d.type === 'center' ? th.center : col}" stroke-width="${d.type === 'center' ? 3 : 1.5}"${d.type === 'attr' && d.attr.common ? ' stroke-dasharray="2,2"' : ''}/>`)
    if (d.type === 'site' && !labeled.has(d.id)) continue
    out.push(`<text x="${(d.x + (d.type === 'center' ? 24 : 13)).toFixed(1)}" y="${(d.y + 4).toFixed(1)}" font-size="${d.type === 'center' ? 15 : 11.5}" font-weight="${d.type === 'center' ? 700 : 500}"${d.type === 'attr' ? ' font-family="ui-monospace, Menlo, Consolas, monospace"' : ''} fill="${d.type === 'attr' ? th.attrText : th.text}" stroke="${th.halo}" stroke-width="3" paint-order="stroke">${esc(cut(d.label, 30))}</text>`)
  }

  // Legenda
  const items = [
    ...[...usedKinds].map(k => ({ color: kindMeta[k]?.color ?? '#94a3b8', label: kindMeta[k]?.label ?? k })),
    { color: '#f43f5e', label: 'Situs judol (High)' }, { color: '#f59e0b', label: 'Situs judol (Medium)' }, { color: '#64748b', label: 'Situs normal' },
  ]
  const ly = y1 + 14, colW = W / 5
  items.forEach((it, i) => {
    const x = x0 + 16 + (i % 5) * colW, y = ly + Math.floor(i / 5) * 18
    out.push(`<circle cx="${x}" cy="${y}" r="5" fill="${it.color}"/><text x="${x + 10}" y="${y + 4}" font-size="11" fill="${th.legend}">${esc(it.label)}</text>`)
  })
  out.push('</svg>')
  return out.join('')
}

export function downloadBlob(blob: Blob, name: string) {
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = name
  a.click()
  setTimeout(() => URL.revokeObjectURL(a.href), 1000)
}

// SVG → PNG lewat canvas (skala 2× agar tajam saat ditempel di dokumen)
export async function svgToPng(svg: string, scale = 2): Promise<Blob> {
  const img = new Image()
  const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }))
  try {
    await new Promise((res, rej) => { img.onload = res; img.onerror = rej; img.src = url })
    const canvas = document.createElement('canvas')
    canvas.width = img.width * scale
    canvas.height = img.height * scale
    const ctx = canvas.getContext('2d')!
    ctx.scale(scale, scale)
    ctx.drawImage(img, 0, 0)
    return await new Promise((res, rej) => canvas.toBlob(b => (b ? res(b) : rej(new Error('gagal membuat PNG'))), 'image/png'))
  } finally { URL.revokeObjectURL(url) }
}
