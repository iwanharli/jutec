<template>
  <div ref="wrap" class="relative w-full h-full">
    <svg ref="svgRef" class="w-full h-full cursor-grab active:cursor-grabbing" />

    <transition name="slide-up">
      <div v-if="hover" class="absolute z-20 pointer-events-none rounded-xl bg-slate-900/95 ring-1 ring-slate-700 shadow-2xl px-4 py-3 max-w-md"
        :style="{ left: hover.x + 16 + 'px', top: hover.y - 12 + 'px' }">
        <p class="text-xs font-semibold uppercase tracking-wider" :style="{ color: hover.color }">{{ hover.kind }}</p>
        <p class="text-sm font-semibold text-white mt-1 break-all">{{ hover.label }}</p>
        <p v-if="hover.sub" class="text-sm text-slate-400 mt-1">{{ hover.sub }}</p>
        <p class="text-xs text-slate-500 mt-2">{{ hover.hint }}</p>
      </div>
    </transition>

    <div class="absolute bottom-4 right-4 z-10 flex gap-1.5 rounded-xl bg-slate-900/90 ring-1 ring-slate-700 p-1.5">
      <UTooltip text="Perbesar"><UButton icon="i-lucide-zoom-in" color="gray" variant="ghost" @click="zoomBy(1.4)" /></UTooltip>
      <UTooltip text="Perkecil"><UButton icon="i-lucide-zoom-out" color="gray" variant="ghost" @click="zoomBy(0.7)" /></UTooltip>
      <UTooltip text="Sesuaikan"><UButton icon="i-lucide-maximize" color="gray" variant="ghost" @click="fit" /></UTooltip>
    </div>
  </div>
</template>

<script setup lang="ts">
import * as d3 from 'd3'

const props = defineProps<{
  relations: any
  kindMeta: Record<string, { label: string; color: string }>
  showIsolated: boolean
  highlight?: string // teks pencarian: node yang cocok disorot, lainnya diredupkan
}>()
const emit = defineEmits<{ (e: 'open-site', id: number): void; (e: 'open-attr', attr: any): void }>()

const wrap   = ref<HTMLDivElement | null>(null)
const svgRef = ref<SVGSVGElement | null>(null)
const hover  = ref<any>(null)

let sim: d3.Simulation<any, any> | null = null
let zoom: d3.ZoomBehavior<SVGSVGElement, unknown> | null = null
let nodesRef: any[] = []
// Label situs terkait baru muncul saat diperbesar — mencegah teks bertumpuk
const SITE_LABEL_ZOOM = 1.3

function shortUrl(u: string) { return u.replace(/^https?:\/\//, '').replace(/\/$/, '') }
function siteColor(s: any) { return s.is_gambling ? (s.priority === 'High' ? '#f43f5e' : '#f59e0b') : '#64748b' }

function build() {
  const rel = props.relations
  if (!rel || !svgRef.value || !wrap.value) return
  sim?.stop()

  const nodes: any[] = []
  const links: any[] = []
  const center = { id: `s${rel.site.id}`, type: 'center', label: shortUrl(rel.site.url), site: rel.site }
  nodes.push(center)
  const siteNodes = new Map<number, any>()

  for (const a of rel.attributes) {
    if (!props.showIsolated && !a.sites.length && !a.common) continue
    const meta = props.kindMeta[a.kind] ?? { label: a.kind, color: '#94a3b8' }
    const an = { id: `a${a.kind}${a.id}`, type: 'attr', attr: a, color: meta.color, kindLabel: meta.label,
      label: a.label && a.kind === 'artifact' ? `${a.label}: ${a.value}` : a.value }
    nodes.push(an)
    links.push({ source: center.id, target: an.id, common: a.common })
    for (const s of a.sites) {
      let sn = siteNodes.get(s.id)
      if (!sn) {
        sn = { id: `s${s.id}`, type: 'site', label: shortUrl(s.url), site: s }
        siteNodes.set(s.id, sn)
        nodes.push(sn)
      }
      links.push({ source: an.id, target: sn.id })
    }
  }
  nodesRef = nodes

  const width = wrap.value.clientWidth, height = wrap.value.clientHeight
  const svg = d3.select(svgRef.value)
  svg.selectAll('*').remove()
  const g = svg.append('g')
  zoom = d3.zoom<SVGSVGElement, unknown>().scaleExtent([0.2, 4]).on('zoom', e => {
    g.attr('transform', e.transform)
    g.selectAll('.site-label').attr('opacity', e.transform.k >= SITE_LABEL_ZOOM ? 1 : 0)
  })
  svg.call(zoom)

  const link = g.append('g').selectAll('line').data(links).join('line')
    .attr('stroke', (d: any) => d.common ? '#475569' : '#334155')
    .attr('stroke-width', (d: any) => d.common ? 1 : 1.4)
    .attr('stroke-dasharray', (d: any) => d.common ? '4,4' : null)

  const node = g.append('g').selectAll('g').data(nodes).join('g').attr('class', 'node').attr('cursor', 'pointer')
    .on('mouseover', (ev: MouseEvent, d: any) => {
      const r = wrap.value!.getBoundingClientRect()
      if (d.type === 'attr') {
        hover.value = { x: ev.clientX - r.left, y: ev.clientY - r.top, color: d.color, kind: d.kindLabel + (d.attr.group ? ` · ${d.attr.group}` : ''),
          label: d.label, sub: d.attr.common ? `Atribut umum — dipakai ${d.attr.shared_sites} situs lain` : `Dipakai ${d.attr.shared_sites} situs lain`,
          hint: 'Klik untuk Reverse Lookup' }
      } else {
        hover.value = { x: ev.clientX - r.left, y: ev.clientY - r.top, color: siteColor(d.site), kind: d.type === 'center' ? 'Situs dianalisis' : 'Situs terkait',
          label: d.site.url, sub: `${d.site.is_gambling ? 'Judol' : 'Normal'} · ${d.site.priority} · ${Math.round(d.site.confidence * 100)}%`,
          hint: d.type === 'center' ? '' : 'Klik untuk membuka detail' }
      }
    })
    .on('mousemove', (ev: MouseEvent) => {
      if (!hover.value) return
      const r = wrap.value!.getBoundingClientRect()
      hover.value.x = ev.clientX - r.left; hover.value.y = ev.clientY - r.top
    })
    .on('mouseout', () => { hover.value = null })
    .on('mouseenter', (ev: MouseEvent) => d3.select(ev.currentTarget as Element).select('.site-label').attr('opacity', 1))
    .on('mouseleave', (ev: MouseEvent) => {
      const k = d3.zoomTransform(svgRef.value!).k
      d3.select(ev.currentTarget as Element).select('.site-label').attr('opacity', k >= SITE_LABEL_ZOOM ? 1 : 0)
    })
    .on('click', (_: MouseEvent, d: any) => {
      if (d.type === 'site') emit('open-site', d.site.id)
      if (d.type === 'attr') emit('open-attr', d.attr)
    })
    .call(d3.drag<SVGGElement, any>()
      .on('start', (ev, d) => { if (!ev.active) sim?.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y })
      .on('drag',  (ev, d) => { d.fx = ev.x; d.fy = ev.y })
      .on('end',   (ev, d) => { if (!ev.active) sim?.alphaTarget(0); if (d.type !== 'center') { d.fx = null; d.fy = null } }))

  node.append('circle')
    .attr('r', (d: any) => d.type === 'center' ? 18 : d.type === 'attr' ? 9 : 7)
    .attr('fill', (d: any) => d.type === 'attr' ? d.color + (d.attr.common ? '33' : 'cc') : siteColor(d.site) + (d.type === 'center' ? 'ff' : 'bb'))
    .attr('stroke', (d: any) => d.type === 'center' ? '#e0e7ff' : d.type === 'attr' ? d.color : siteColor(d.site))
    .attr('stroke-width', (d: any) => d.type === 'center' ? 3 : 1.5)
    .attr('stroke-dasharray', (d: any) => d.type === 'attr' && d.attr.common ? '2,2' : null)

  node.append('text')
    .attr('class', (d: any) => d.type === 'site' ? 'site-label' : '')
    .attr('opacity', (d: any) => d.type === 'site' ? 0 : 1)
    .text((d: any) => d.label.length > 30 ? d.label.slice(0, 30) + '…' : d.label)
    .attr('x', (d: any) => d.type === 'center' ? 24 : 13).attr('y', 4)
    .attr('font-size', (d: any) => d.type === 'center' ? '15px' : '12.5px')
    .attr('font-weight', (d: any) => d.type === 'center' ? 700 : 500)
    .attr('font-family', (d: any) => d.type === 'attr' ? 'JetBrains Mono, monospace' : 'Inter, sans-serif')
    .attr('fill', (d: any) => d.type === 'attr' ? '#cbd5e1' : '#e2e8f0')
    .attr('paint-order', 'stroke').attr('stroke', '#030a17').attr('stroke-width', 3)
    .attr('pointer-events', 'none')

  center.fx = width / 2; center.fy = height / 2
  sim = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id((d: any) => d.id).distance((l: any) => l.source.type === 'center' ? 130 : 90).strength(0.7))
    .force('charge', d3.forceManyBody().strength((d: any) => d.type === 'center' ? -900 : d.type === 'attr' ? -320 : -140))
    .force('collide', d3.forceCollide().radius((d: any) => d.type === 'center' ? 40 : 22))
    .force('radial', d3.forceRadial((d: any) => d.type === 'attr' ? 150 : d.type === 'site' ? 300 : 0, width / 2, height / 2).strength(0.35))
    .on('tick', () => {
      link.attr('x1', (d: any) => d.source.x).attr('y1', (d: any) => d.source.y)
          .attr('x2', (d: any) => d.target.x).attr('y2', (d: any) => d.target.y)
      node.attr('transform', (d: any) => `translate(${d.x},${d.y})`)
    })
    .on('end', fit)
  applyHighlight()
}

function fit() {
  if (!svgRef.value || !zoom || !wrap.value || !nodesRef.length) return
  const w = wrap.value.clientWidth, h = wrap.value.clientHeight
  const xs = nodesRef.map(n => n.x), ys = nodesRef.map(n => n.y)
  const [x0, x1, y0, y1] = [Math.min(...xs), Math.max(...xs), Math.min(...ys), Math.max(...ys)]
  const k = Math.min(1.6, 0.85 / Math.max((x1 - x0 + 200) / w, (y1 - y0 + 80) / h))
  const t = d3.zoomIdentity.translate(w / 2, h / 2).scale(k).translate(-(x0 + x1) / 2, -(y0 + y1) / 2)
  d3.select(svgRef.value).transition().duration(600).call(zoom.transform, t)
}
function zoomBy(f: number) { if (svgRef.value && zoom) d3.select(svgRef.value).transition().call(zoom.scaleBy, f) }

watch(() => [props.relations, props.showIsolated], () => nextTick(build))
watch(() => props.highlight, applyHighlight)

// Sorot node yang label/URL-nya cocok dengan pencarian; redupkan sisanya
function applyHighlight() {
  if (!svgRef.value) return
  const q = (props.highlight ?? '').trim().toLowerCase()
  d3.select(svgRef.value).selectAll<SVGGElement, any>('g.node').each(function (d: any) {
    const text = `${d.label} ${d.site?.url ?? ''} ${d.attr?.value ?? ''}`.toLowerCase()
    const hit = !q || text.includes(q)
    const g = d3.select(this)
    g.attr('opacity', hit ? 1 : 0.15)
    if (q && hit) g.select('.site-label').attr('opacity', 1)
  })
}

// Ukuran kanvas berubah (mis. masuk/keluar layar penuh) → bangun ulang agar memenuhi area
let resizeObs: ResizeObserver | null = null
let lastW = 0, lastH = 0
onMounted(() => {
  if (!wrap.value) return
  resizeObs = new ResizeObserver(([e]) => {
    const { width, height } = e.contentRect
    if (Math.abs(width - lastW) > 40 || Math.abs(height - lastH) > 40) {
      lastW = width; lastH = height
      build()
    }
  })
  resizeObs.observe(wrap.value)
})
onUnmounted(() => resizeObs?.disconnect())
onMounted(() => nextTick(build))
onUnmounted(() => sim?.stop())
</script>
