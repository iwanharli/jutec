<template>
  <div ref="wrapEl" class="relative w-full overflow-hidden bg-[#030a17]" :class="isFullscreen ? 'h-screen' : 'h-[calc(100vh-4rem)]'">
    <div class="absolute inset-0 pointer-events-none opacity-30"
      style="background-image: radial-gradient(circle, #1e293b 1px, transparent 1px); background-size: 28px 28px;" />
    <svg ref="svgRef" class="absolute inset-0 w-full h-full cursor-grab active:cursor-grabbing" @click.self="select(null)" />

    <div v-if="loading" class="absolute inset-0 z-30 flex flex-col items-center justify-center gap-3 bg-slate-950/50 backdrop-blur-sm">
      <UIcon name="i-lucide-loader-circle" class="w-8 h-8 text-primary-400 animate-spin" />
      <p class="text-sm text-slate-300">Membangun graf jaringan…</p>
    </div>

    <!-- ── Kiri: judul, pencarian, filter ── -->
    <div class="absolute top-4 left-4 z-20 w-[300px] max-h-[calc(100%-2rem)] flex flex-col gap-3 pointer-events-none">
      <div class="glass p-3 pointer-events-auto">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-waypoints" class="w-5 h-5 text-primary-400" />
          <h1 class="text-sm font-semibold text-white flex-1">Network Graph</h1>
          <UTooltip text="Muat ulang"><UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-refresh-cw" :loading="loading" @click="loadGraph" /></UTooltip>
        </div>
        <p class="text-xs text-slate-400 mt-1 tabular-nums">
          <b class="text-slate-200">{{ view.nodes.length.toLocaleString('id-ID') }}</b> / {{ totals.nodes.toLocaleString('id-ID') }} node ·
          <b class="text-slate-200">{{ view.edges.length.toLocaleString('id-ID') }}</b> / {{ totals.edges.toLocaleString('id-ID') }} relasi
        </p>

        <!-- Pencarian -->
        <div class="relative mt-2.5">
          <UInput v-model="query" size="sm" icon="i-lucide-search" placeholder="Cari domain, IP, NS, tracking, artefak…"
            :ui="{ icon: { trailing: { pointer: '' } } }" @keydown.enter.prevent="results[0] && pick(results[0])" @keydown.esc="query = ''">
            <template #trailing>
              <UButton v-show="query" color="gray" variant="link" icon="i-lucide-x" :padded="false" @click="query = ''" />
            </template>
          </UInput>
          <div v-if="query && results.length" class="absolute left-0 right-0 mt-1 glass !bg-slate-900/95 max-h-72 overflow-y-auto py-1 z-30">
            <button v-for="n in results" :key="n.id" type="button" @click="pick(n)"
              class="w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-slate-800/70">
              <span class="w-2 h-2 rounded-full shrink-0" :style="{ background: typeMeta[n.type]?.color }" />
              <span class="flex-1 min-w-0 truncate text-[13px] text-slate-200">{{ n.label }}</span>
              <span class="text-[11px] text-slate-500">{{ degree.get(n.id) ?? 0 }}</span>
            </button>
          </div>
          <p v-else-if="query" class="text-xs text-slate-500 mt-1.5 px-1">Tidak ada node yang cocok.</p>
        </div>
      </div>

      <!-- Filter -->
      <div class="glass pointer-events-auto flex flex-col min-h-0">
        <button type="button" class="flex items-center gap-2 px-3 py-2.5 text-left" @click="showFilters = !showFilters">
          <UIcon name="i-lucide-sliders-horizontal" class="w-4 h-4 text-slate-400" />
          <span class="text-[13px] font-semibold text-slate-200 flex-1">Filter</span>
          <UBadge v-if="activeFilterCount" color="primary" variant="subtle" size="xs">{{ activeFilterCount }}</UBadge>
          <UIcon :name="showFilters ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'" class="w-4 h-4 text-slate-500" />
        </button>
        <div v-if="showFilters" class="px-3 pb-3 space-y-3.5 overflow-y-auto min-h-0 border-t border-slate-800/70 pt-3">
          <div>
            <p class="flabel">Jenis node</p>
            <div class="flex flex-wrap gap-1">
              <button v-for="(m, t) in typeMeta" :key="t" type="button" @click="toggle(f.types, t)"
                class="chip" :class="f.types.has(t) ? 'text-slate-100' : 'chip-off'"
                :style="f.types.has(t) ? { background: m.color + '22', '--tw-ring-color': m.color + '66' } : {}">
                <span class="w-2 h-2 rounded-full" :style="f.types.has(t) ? { background: m.color } : { boxShadow: `inset 0 0 0 1.5px ${m.color}` }" />
                {{ m.label }} <span class="text-[10px] text-slate-400 tabular-nums">{{ typeCounts[t] ?? 0 }}</span>
              </button>
            </div>
          </div>
          <div>
            <p class="flabel">Jenis relasi</p>
            <div class="flex flex-wrap gap-1">
              <button v-for="(m, t) in edgeMeta" :key="t" type="button" @click="toggle(f.edges, t)"
                class="chip" :class="f.edges.has(t) ? 'bg-slate-800 ring-slate-600 text-slate-100' : 'chip-off'">
                <svg width="16" height="4"><line x1="0" y1="2" x2="16" y2="2" :stroke="m.color" stroke-width="2" :stroke-dasharray="m.dash" /></svg>
                {{ m.label }} <span class="text-[10px] text-slate-400 tabular-nums">{{ edgeCounts[t] ?? 0 }}</span>
              </button>
            </div>
          </div>
          <div>
            <p class="flabel">Prioritas situs</p>
            <div class="flex gap-1">
              <button v-for="p in ['High', 'Medium', 'Low']" :key="p" type="button" @click="toggle(f.priorities, p)"
                class="chip flex-1 justify-center" :class="f.priorities.has(p) ? prioClass[p] : 'chip-off'">{{ p }}</button>
            </div>
          </div>
          <div>
            <div class="flex justify-between"><p class="flabel">Confidence situs min.</p><span class="text-xs text-slate-300 tabular-nums">{{ f.minConf }}%</span></div>
            <URange v-model="f.minConf" :min="0" :max="100" :step="5" size="sm" />
          </div>
          <div>
            <div class="flex justify-between"><p class="flabel">Koneksi minimal per node</p><span class="text-xs text-slate-300 tabular-nums">≥ {{ f.minDegree }}</span></div>
            <URange v-model="f.minDegree" :min="0" :max="10" :step="1" size="sm" />
            <p class="text-[11px] text-slate-500 mt-1">Naikkan untuk menyisakan simpul yang menghubungkan banyak situs.</p>
          </div>
          <label class="flex items-center justify-between gap-2 text-[13px] text-slate-300 cursor-pointer">
            Sembunyikan node tanpa koneksi <UToggle v-model="f.hideIsolated" size="sm" />
          </label>
          <UButton block size="xs" color="gray" variant="soft" icon="i-lucide-rotate-ccw" label="Reset filter" :disabled="!activeFilterCount" @click="resetFilters" />
        </div>
      </div>
    </div>

    <!-- ── Kanan: detail node terpilih ── -->
    <transition name="panel">
      <div v-if="selected" class="absolute top-4 right-4 z-20 w-[320px] max-h-[calc(100%-7rem)] glass flex flex-col">
        <div class="p-3 border-b border-slate-800/70">
          <div class="flex items-start gap-2">
            <span class="w-2.5 h-2.5 rounded-full mt-1.5 shrink-0" :style="{ background: typeMeta[selected.type]?.color }" />
            <div class="flex-1 min-w-0">
              <p class="text-[11px] uppercase tracking-wider font-semibold" :style="{ color: typeMeta[selected.type]?.color }">{{ typeMeta[selected.type]?.label }}</p>
              <p class="text-sm font-semibold text-white break-all leading-snug">{{ selected.label }}</p>
            </div>
            <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-x" @click="select(null)" />
          </div>
          <div class="flex flex-wrap gap-1.5 mt-2 text-xs">
            <UBadge color="gray" variant="subtle" size="xs">{{ degree.get(selected.id) ?? 0 }} koneksi</UBadge>
            <template v-if="selected.meta">
              <UBadge v-if="selected.meta.priority" :color="selected.meta.priority === 'High' ? 'rose' : selected.meta.priority === 'Medium' ? 'amber' : 'gray'" variant="subtle" size="xs">{{ selected.meta.priority }}</UBadge>
              <UBadge v-if="selected.meta.confidence != null" color="gray" variant="subtle" size="xs">{{ Math.round(selected.meta.confidence * 100) }}%</UBadge>
            </template>
          </div>
          <div class="flex flex-wrap gap-1.5 mt-2.5">
            <UButton v-for="a in actionsFor(selected)" :key="a.label" size="xs" :color="a.primary ? 'primary' : 'gray'" variant="soft" :icon="a.icon" :label="a.label" :to="a.to" />
          </div>
          <div class="mt-3">
            <p class="flabel">Fokus tetangga</p>
            <div class="flex gap-1">
              <button v-for="h in [0, 1, 2, 3]" :key="h" type="button" @click="f.focusHops = h"
                class="chip flex-1 justify-center" :class="f.focusHops === h ? 'bg-primary-500/20 ring-primary-500/50 text-primary-100' : 'chip-off'">
                {{ h === 0 ? 'Semua' : `${h} langkah` }}
              </button>
            </div>
          </div>
        </div>
        <div class="overflow-y-auto min-h-0 p-2">
          <div v-for="grp in neighborGroups" :key="grp.type" class="mb-2">
            <p class="flabel px-1.5 flex items-center gap-1.5">
              <span class="w-2 h-2 rounded-full" :style="{ background: typeMeta[grp.type]?.color }" />{{ typeMeta[grp.type]?.label }} · {{ grp.nodes.length }}
            </p>
            <button v-for="n in grp.nodes" :key="n.id" type="button" @click="pick(n)"
              class="w-full flex items-center gap-2 px-1.5 py-1 rounded hover:bg-slate-800/60 text-left">
              <span class="flex-1 min-w-0 truncate text-[13px] text-slate-300">{{ n.label }}</span>
              <span v-if="n.meta?.priority === 'High'" class="w-1.5 h-1.5 rounded-full bg-rose-500" />
              <span class="text-[11px] text-slate-500 tabular-nums">{{ degree.get(n.id) ?? 0 }}</span>
            </button>
          </div>
          <p v-if="!neighborGroups.length" class="text-xs text-slate-500 p-2">Tidak ada tetangga pada filter ini.</p>
        </div>
      </div>
    </transition>

    <!-- Tooltip hover -->
    <div v-if="hover" class="absolute z-30 pointer-events-none glass !bg-slate-900/95 px-3 py-2 max-w-xs"
      :style="{ left: hoverPos.x + 14 + 'px', top: hoverPos.y + 14 + 'px' }">
      <p class="text-[13px] font-semibold text-white break-all">{{ hover.label }}</p>
      <p class="text-[11px] mt-0.5" :style="{ color: typeMeta[hover.type]?.color }">{{ typeMeta[hover.type]?.label }} · {{ degree.get(hover.id) ?? 0 }} koneksi</p>
    </div>

    <!-- ── Kanan bawah: kontrol & legenda ── -->
    <div class="absolute bottom-4 right-4 z-20 flex items-end gap-2">
      <div class="glass px-3 py-2 hidden md:block">
        <div class="flex items-center gap-3 text-[11px] text-slate-400">
          <span v-for="(m, t) in edgeMeta" :key="t" class="inline-flex items-center gap-1.5">
            <svg width="18" height="4"><line x1="0" y1="2" x2="18" y2="2" :stroke="m.color" stroke-width="2" :stroke-dasharray="m.dash" /></svg>{{ m.label }}
          </span>
        </div>
      </div>
      <div class="glass p-1 flex flex-col gap-0.5">
        <UTooltip text="Perbesar" :popper="{ placement: 'left' }"><UButton size="sm" color="gray" variant="ghost" icon="i-lucide-zoom-in" @click="zoomBy(1.4)" /></UTooltip>
        <UTooltip text="Perkecil" :popper="{ placement: 'left' }"><UButton size="sm" color="gray" variant="ghost" icon="i-lucide-zoom-out" @click="zoomBy(0.7)" /></UTooltip>
        <UTooltip text="Sesuaikan tampilan" :popper="{ placement: 'left' }"><UButton size="sm" color="gray" variant="ghost" icon="i-lucide-scan" @click="fit()" /></UTooltip>
        <UTooltip :text="isFullscreen ? 'Keluar layar penuh' : 'Layar penuh'" :popper="{ placement: 'left' }">
          <UButton size="sm" color="gray" variant="ghost" :icon="isFullscreen ? 'i-lucide-minimize' : 'i-lucide-maximize'" @click="toggleFullscreen" />
        </UTooltip>
      </div>
    </div>
    <p class="absolute bottom-4 left-4 z-10 text-[11px] text-slate-500 pointer-events-none hidden lg:block">
      Scroll untuk zoom · seret latar untuk menggeser · klik node untuk detail · klik dua kali situs untuk membuka halamannya
    </p>
  </div>
</template>

<script setup lang="ts">
import * as d3 from 'd3'

const { get } = useApi()
const router = useRouter()

const wrapEl = ref<HTMLElement | null>(null)
const svgRef = ref<SVGSVGElement | null>(null)
const { isFullscreen, toggle: toggleFullscreen } = useFullscreen(wrapEl)

const typeMeta: Record<string, { label: string; color: string }> = {
  site: { label: 'Situs', color: '#6366f1' },
  ip: { label: 'IP', color: '#f59e0b' },
  ns: { label: 'Nameserver', color: '#10b981' },
  tracking: { label: 'Tracking', color: '#8b5cf6' },
  artifact: { label: 'Artefak', color: '#ef4444' },
}
const edgeMeta: Record<string, { label: string; color: string; dash: string }> = {
  shared_artifact: { label: 'Artefak', color: '#ef4444', dash: '6,2' },
  shared_tracking: { label: 'Tracking', color: '#8b5cf6', dash: '2,2' },
  shared_ip: { label: 'IP', color: '#f59e0b', dash: '' },
  shared_ns: { label: 'NS', color: '#10b981', dash: '4,3' },
}
const prioClass: Record<string, string> = {
  High: 'bg-rose-500/15 ring-rose-500/40 text-rose-200',
  Medium: 'bg-amber-500/15 ring-amber-500/40 text-amber-200',
  Low: 'bg-slate-700/40 ring-slate-600 text-slate-200',
}

const loading = ref(true)
const raw = shallowRef<{ nodes: any[]; edges: any[] }>({ nodes: [], edges: [] })
const query = ref('')
const showFilters = ref(true)
const selected = shallowRef<any>(null)
const hover = shallowRef<any>(null)
const hoverPos = reactive({ x: 0, y: 0 })

const blank = () => ({
  types: new Set(Object.keys(typeMeta)), edges: new Set(Object.keys(edgeMeta)),
  priorities: new Set(['High', 'Medium', 'Low']), minConf: 0, minDegree: 0, hideIsolated: true, focusHops: 0,
})
const f = reactive(blank())
function toggle(set: Set<string>, v: string) { set.has(v) ? set.delete(v) : set.add(v) }
function resetFilters() { Object.assign(f, blank()) }
const activeFilterCount = computed(() =>
  (f.types.size < 5 ? 1 : 0) + (f.edges.size < 4 ? 1 : 0) + (f.priorities.size < 3 ? 1 : 0) +
  (f.minConf ? 1 : 0) + (f.minDegree ? 1 : 0) + (f.hideIsolated ? 0 : 1))

const totals = computed(() => ({ nodes: raw.value.nodes.length, edges: raw.value.edges.length }))
const typeCounts = computed(() => countBy(raw.value.nodes, (n: any) => n.type))
const edgeCounts = computed(() => countBy(raw.value.edges, (e: any) => e.type))
function countBy(arr: any[], key: (x: any) => string) {
  const c: Record<string, number> = {}
  for (const x of arr) c[key(x)] = (c[key(x)] ?? 0) + 1
  return c
}

// ── Graf hasil filter ──
const view = computed(() => {
  const { nodes, edges } = raw.value
  let keep = new Set<string>()
  for (const n of nodes) {
    if (!f.types.has(n.type)) continue
    if (n.type === 'site') {
      if (!f.priorities.has(n.meta?.priority ?? 'Low')) continue
      if ((n.meta?.confidence ?? 0) * 100 < f.minConf) continue
    }
    keep.add(n.id)
  }
  let es = edges.filter(e => f.edges.has(e.type) && keep.has(e.source) && keep.has(e.target))
  // Koneksi minimal & node terisolasi (diulang agar node yang kehilangan tetangga ikut tersaring)
  for (let pass = 0; pass < 3; pass++) {
    const deg = new Map<string, number>()
    for (const e of es) { deg.set(e.source, (deg.get(e.source) ?? 0) + 1); deg.set(e.target, (deg.get(e.target) ?? 0) + 1) }
    const min = Math.max(f.minDegree, f.hideIsolated ? 1 : 0)
    if (!min) break
    const next = new Set([...keep].filter(id => (deg.get(id) ?? 0) >= min))
    if (next.size === keep.size) break
    keep = next
    es = es.filter(e => keep.has(e.source) && keep.has(e.target))
  }
  // Fokus: hanya tetangga node terpilih sampai N langkah
  if (f.focusHops && selected.value && keep.has(selected.value.id)) {
    const adj = adjacency(es)
    const seen = new Set([selected.value.id])
    let frontier = [selected.value.id]
    for (let h = 0; h < f.focusHops; h++) {
      const nx: string[] = []
      for (const id of frontier) for (const nb of adj.get(id) ?? []) if (!seen.has(nb)) { seen.add(nb); nx.push(nb) }
      frontier = nx
    }
    keep = seen
    es = es.filter(e => keep.has(e.source) && keep.has(e.target))
  }
  return { nodes: nodes.filter(n => keep.has(n.id)), edges: es }
})

function adjacency(es: any[]) {
  const adj = new Map<string, string[]>()
  for (const e of es) {
    if (!adj.has(e.source)) adj.set(e.source, [])
    if (!adj.has(e.target)) adj.set(e.target, [])
    adj.get(e.source)!.push(e.target)
    adj.get(e.target)!.push(e.source)
  }
  return adj
}
const fullAdj = computed(() => adjacency(raw.value.edges))
const degree = computed(() => new Map([...fullAdj.value].map(([k, v]) => [k, v.length])))
const byId = computed(() => new Map(raw.value.nodes.map(n => [n.id, n])))

const results = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (q.length < 2) return []
  return raw.value.nodes.filter(n => n.label.toLowerCase().includes(q))
    .sort((a, b) => (degree.value.get(b.id) ?? 0) - (degree.value.get(a.id) ?? 0)).slice(0, 12)
})

const neighborGroups = computed(() => {
  if (!selected.value) return []
  const ids = fullAdj.value.get(selected.value.id) ?? []
  const groups: Record<string, any[]> = {}
  for (const id of new Set(ids)) {
    const n = byId.value.get(id)
    if (n) (groups[n.type] ??= []).push(n)
  }
  return Object.keys(typeMeta).filter(t => groups[t]).map(t => ({
    type: t, nodes: groups[t].sort((a, b) => (degree.value.get(b.id) ?? 0) - (degree.value.get(a.id) ?? 0)),
  }))
})

function nodeKey(n: any) { return String(n.id).split(':') }
function actionsFor(n: any) {
  const [, id] = nodeKey(n)
  const val = n.label.includes(': ') ? n.label.split(': ').slice(1).join(': ') : n.label
  if (n.type === 'site') return [{ label: 'Detail situs', icon: 'i-lucide-panel-right-open', to: `/site/${id}`, primary: true }]
  if (n.type === 'ip') return [{ label: 'Lacak IP', icon: 'i-lucide-server-cog', to: `/ip/${encodeURIComponent(n.label)}`, primary: true }]
  const type = n.type === 'ns' ? 'NS' : n.type === 'tracking' ? (/GTM-/.test(n.label) ? 'GTM' : /^\d{10,}$/.test(val) ? 'FB' : 'GA') : (n.label.split(':')[0] || 'Contact')
  return [{ label: 'Reverse Lookup', icon: 'i-lucide-scan-search', to: `/reverse-lookup?type=${encodeURIComponent(type)}&v=${encodeURIComponent(val)}`, primary: true }]
}

// ── Render d3 ──
let sim: d3.Simulation<any, any> | null = null
let zoom: d3.ZoomBehavior<SVGSVGElement, unknown> | null = null
let gRoot: d3.Selection<SVGGElement, unknown, null, undefined> | null = null
const pos = new Map<string, { x: number; y: number }>() // posisi dipertahankan antar filter
let fitted = false

const radius = (n: any) => {
  const d = degree.value.get(n.id) ?? 0
  return (n.type === 'site' ? 5 : 3.5) + Math.min(9, Math.sqrt(d) * 1.4)
}

function render() {
  const svgEl = svgRef.value
  if (!svgEl) return
  const { nodes: vn, edges: ve } = view.value
  const W = svgEl.clientWidth, H = svgEl.clientHeight
  const svg = d3.select(svgEl)
  sim?.stop()
  if (!gRoot) {
    gRoot = svg.append('g')
    zoom = d3.zoom<SVGSVGElement, unknown>().scaleExtent([0.08, 6]).on('zoom', e => {
      gRoot!.attr('transform', e.transform)
      gRoot!.classed('zoomed', e.transform.k >= 1.4)
    })
    svg.call(zoom).on('dblclick.zoom', null)
  }
  gRoot.selectAll('*').remove()

  const nodes = vn.map(n => ({ ...n, ...(pos.get(n.id) ?? { x: W / 2 + (Math.random() - 0.5) * 400, y: H / 2 + (Math.random() - 0.5) * 400 }) }))
  const edges = ve.map(e => ({ ...e }))

  const link = gRoot.append('g').attr('class', 'links').selectAll('line').data(edges).join('line')
    .attr('stroke', (d: any) => edgeMeta[d.type]?.color ?? '#334155')
    .attr('stroke-dasharray', (d: any) => edgeMeta[d.type]?.dash ?? '')
    .attr('stroke-width', 0.8).attr('stroke-opacity', 0.28)

  const node = gRoot.append('g').attr('class', 'nodes').selectAll('g').data(nodes, (d: any) => d.id).join('g')
    .attr('class', 'node').attr('cursor', 'pointer')
    .on('mouseenter', (ev: MouseEvent, d: any) => { hover.value = d; highlight(d.id) })
    .on('mousemove', (ev: MouseEvent) => {
      const r = wrapEl.value!.getBoundingClientRect()
      hoverPos.x = ev.clientX - r.left; hoverPos.y = ev.clientY - r.top
    })
    .on('mouseleave', () => { hover.value = null; highlight(selected.value?.id ?? null) })
    .on('click', (ev: MouseEvent, d: any) => { ev.stopPropagation(); select(byId.value.get(d.id)) })
    .on('dblclick', (ev: MouseEvent, d: any) => { if (d.type === 'site') router.push(`/site/${nodeKey(d)[1]}`) })
    .call(d3.drag<SVGGElement, any>()
      .on('start', (ev, d) => { if (!ev.active) sim?.alphaTarget(0.25).restart(); d.fx = d.x; d.fy = d.y })
      .on('drag', (ev, d) => { d.fx = ev.x; d.fy = ev.y })
      .on('end', (ev, d) => { if (!ev.active) sim?.alphaTarget(0); d.fx = null; d.fy = null }))

  node.append('circle')
    .attr('r', (d: any) => radius(d))
    .attr('fill', (d: any) => typeMeta[d.type]?.color + (d.type === 'site' ? 'cc' : '88'))
    .attr('stroke', (d: any) => d.type === 'site' && d.meta?.priority === 'High' ? '#f43f5e' : typeMeta[d.type]?.color)
    .attr('stroke-width', (d: any) => d.type === 'site' && d.meta?.priority === 'High' ? 1.8 : 1)

  node.append('text').attr('class', (d: any) => `lbl ${d.type === 'site' || (degree.value.get(d.id) ?? 0) >= 8 ? 'lbl-main' : ''}`)
    .text((d: any) => d.label.length > 30 ? d.label.slice(0, 30) + '…' : d.label)
    .attr('x', (d: any) => radius(d) + 3).attr('y', 3.5)

  sim = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(edges).id((d: any) => d.id).distance(42).strength(0.35))
    .force('charge', d3.forceManyBody().strength(-70).distanceMax(380))
    .force('x', d3.forceX(W / 2).strength(0.035)).force('y', d3.forceY(H / 2).strength(0.035))
    .force('collide', d3.forceCollide().radius((d: any) => radius(d) + 2))
    .alpha(pos.size ? 0.35 : 1)
    .on('tick', () => {
      link.attr('x1', (d: any) => d.source.x).attr('y1', (d: any) => d.source.y)
        .attr('x2', (d: any) => d.target.x).attr('y2', (d: any) => d.target.y)
      node.attr('transform', (d: any) => `translate(${d.x},${d.y})`)
    })
    .on('end', () => {
      for (const n of nodes) pos.set(n.id, { x: n.x, y: n.y })
      if (!fitted) { fitted = true; fit() }
    })
  highlight(selected.value?.id ?? null)
}

// Terangkan node & tetangganya; redupkan sisanya
function highlight(id: string | null) {
  if (!gRoot) return
  const nb = new Set(id ? [id, ...(fullAdj.value.get(id) ?? [])] : [])
  gRoot.selectAll<SVGGElement, any>('g.node')
    .classed('dim', d => !!id && !nb.has(d.id))
    .classed('hl', d => !!id && nb.has(d.id))
    .classed('sel', d => d.id === selected.value?.id)
  gRoot.selectAll<SVGLineElement, any>('g.links line')
    .attr('stroke-opacity', (d: any) => !id ? 0.28 : (d.source.id === id || d.target.id === id) ? 0.9 : 0.04)
    .attr('stroke-width', (d: any) => id && (d.source.id === id || d.target.id === id) ? 1.6 : 0.8)
}

function select(n: any) {
  selected.value = n ?? null
  if (!n) f.focusHops = 0
  highlight(n?.id ?? null)
}

function pick(n: any) {
  query.value = ''
  // Pastikan node terlihat: longgarkan filter yang menyembunyikannya
  if (!view.value.nodes.some(x => x.id === n.id)) {
    f.types.add(n.type)
    if (n.type === 'site') { f.priorities.add(n.meta?.priority ?? 'Low'); f.minConf = 0 }
    f.minDegree = 0
    if (!(degree.value.get(n.id) ?? 0)) f.hideIsolated = false
  }
  select(n)
  nextTick(() => setTimeout(() => centerOn(n.id), 60))
}

function centerOn(id: string) {
  const p = pos.get(id)
  const svgEl = svgRef.value
  if (!p || !svgEl || !zoom) return
  const k = Math.max(1.6, d3.zoomTransform(svgEl).k)
  d3.select(svgEl).transition().duration(600)
    .call(zoom.transform, d3.zoomIdentity.translate(svgEl.clientWidth / 2, svgEl.clientHeight / 2).scale(k).translate(-p.x, -p.y))
}

function fit() {
  const svgEl = svgRef.value
  const pts = view.value.nodes.map(n => pos.get(n.id)).filter(Boolean) as { x: number; y: number }[]
  if (!svgEl || !zoom || !pts.length) return
  const xs = pts.map(p => p.x), ys = pts.map(p => p.y)
  const [x0, x1, y0, y1] = [Math.min(...xs), Math.max(...xs), Math.min(...ys), Math.max(...ys)]
  const W = svgEl.clientWidth, H = svgEl.clientHeight
  const k = Math.min(2.5, 0.85 / Math.max((x1 - x0 || 1) / W, (y1 - y0 || 1) / H))
  d3.select(svgEl).transition().duration(600)
    .call(zoom.transform, d3.zoomIdentity.translate(W / 2, H / 2).scale(k).translate(-(x0 + x1) / 2, -(y0 + y1) / 2))
}
function zoomBy(s: number) { if (svgRef.value && zoom) d3.select(svgRef.value).transition().call(zoom.scaleBy, s) }

async function loadGraph() {
  loading.value = true
  try {
    const d = await get<any>('/network/graph')
    raw.value = { nodes: d?.nodes ?? [], edges: d?.edges ?? [] }
    pos.clear(); fitted = false // render dipicu watch(view)
  } finally { loading.value = false }
}

let rTimer: any = null
watch(view, () => { clearTimeout(rTimer); rTimer = setTimeout(render, 120) })
watch(() => f.focusHops, () => setTimeout(() => fit(), 900))
watch(isFullscreen, () => nextTick(() => setTimeout(() => { render(); fit() }, 150)))

onMounted(loadGraph)
onUnmounted(() => sim?.stop())
useHead({ title: 'Network Graph — Jutec' })
</script>

<style scoped>
.glass { @apply rounded-xl bg-slate-950/70 backdrop-blur-md ring-1 ring-slate-700/60 shadow-2xl shadow-black/40; }
.flabel { @apply text-[11px] font-semibold uppercase tracking-wider text-slate-500 mb-1.5; }
.chip { @apply inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium ring-1 transition; }
.chip-off { @apply text-slate-500 ring-slate-800 hover:text-slate-300 hover:ring-slate-700; }

:deep(.lbl) { font: 600 9px Inter, sans-serif; fill: #e2e8f0; paint-order: stroke; stroke: #030a17; stroke-width: 3px; pointer-events: none; opacity: 0; transition: opacity .15s; }
:deep(.zoomed .lbl-main) { opacity: .9; }
:deep(.node.hl .lbl), :deep(.node.sel .lbl) { opacity: 1; }
:deep(.node) { transition: opacity .15s; }
:deep(.node.dim) { opacity: .12; }
:deep(.node.sel circle) { stroke: #fff; stroke-width: 2.5px; }

.panel-enter-active, .panel-leave-active { transition: all .18s ease; }
.panel-enter-from, .panel-leave-to { opacity: 0; transform: translateX(12px); }
</style>
