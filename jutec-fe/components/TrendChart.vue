<template>
  <div>
    <!-- Legenda: identitas tidak hanya lewat warna (juga label langsung di ujung garis) -->
    <div class="flex flex-wrap items-center gap-x-4 gap-y-1 mb-2 text-[13px] text-slate-300">
      <span v-for="s in series" :key="s.key" class="inline-flex items-center gap-1.5">
        <span class="w-3 h-0.5 rounded-full" :style="{ background: s.color }" />{{ s.label }}
        <span class="text-slate-500 tabular-nums">{{ total(s.key).toLocaleString('id-ID') }}</span>
      </span>
      <UButton class="ml-auto" size="2xs" color="gray" variant="ghost" :icon="asTable ? 'i-lucide-line-chart' : 'i-lucide-table'"
        :label="asTable ? 'Grafik' : 'Tabel'" @click="asTable = !asTable" />
    </div>

    <div v-if="asTable" class="max-h-72 overflow-y-auto">
      <table class="w-full text-sm tabular-nums">
        <thead class="text-xs text-slate-500 uppercase tracking-wider"><tr><th class="text-left py-1.5">Tanggal</th><th v-for="s in series" :key="s.key" class="text-right">{{ s.label }}</th></tr></thead>
        <tbody class="divide-y divide-slate-800/70">
          <tr v-for="d in [...data].reverse()" :key="d.day" class="text-slate-300"><td class="py-1">{{ fmtDay(d.day) }}</td><td v-for="s in series" :key="s.key" class="text-right">{{ d[s.key] }}</td></tr>
        </tbody>
      </table>
    </div>

    <div v-else ref="wrap" class="relative" @mousemove="onMove" @mouseleave="hover = null">
      <svg :viewBox="`0 0 ${W} ${H}`" class="w-full h-64 block" preserveAspectRatio="none" role="img" :aria-label="ariaLabel">
        <!-- Grid & sumbu: resesif -->
        <g v-for="t in yTicks" :key="t">
          <line :x1="PL" :x2="W - PR" :y1="y(t)" :y2="y(t)" stroke="#1e293b" stroke-width="1" vector-effect="non-scaling-stroke" />
        </g>
        <!-- Crosshair -->
        <line v-if="hover != null" :x1="x(hover)" :x2="x(hover)" :y1="PT" :y2="H - PB" stroke="#475569" stroke-width="1" vector-effect="non-scaling-stroke" />
        <path v-for="s in series" :key="s.key" :d="path(s.key)" fill="none" :stroke="s.color" stroke-width="2"
          stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke" />
      </svg>
      <!-- Label sumbu & titik di HTML agar tidak ikut melar oleh preserveAspectRatio -->
      <span v-for="t in yTicks" :key="'y' + t" class="absolute left-0 -translate-y-1/2 text-[11px] text-slate-500 tabular-nums" :style="{ top: pct(y(t), H) }">{{ t }}</span>
      <span v-for="(d, i) in data" v-show="(i % 7 === 0 && data.length - 1 - i >= 4) || i === data.length - 1" :key="'x' + d.day"
        class="absolute bottom-0 -translate-x-1/2 text-[11px] text-slate-500 whitespace-nowrap" :style="{ left: pct(x(i), W) }">{{ fmtDay(d.day) }}</span>
      <!-- Label langsung di ujung tiap garis -->
      <span v-for="s in endLabels" :key="'l' + s.key" class="absolute -translate-y-1/2 text-[11px] text-slate-300 whitespace-nowrap pl-1.5"
        :style="{ left: pct(W - PR, W), top: pct(s.y, H) }">{{ s.label }}</span>
      <template v-if="hover != null">
        <span v-for="s in series" :key="'m' + s.key" class="absolute w-2 h-2 rounded-full -translate-x-1/2 -translate-y-1/2 ring-2 ring-slate-900 pointer-events-none"
          :style="{ left: pct(x(hover), W), top: pct(y(data[hover][s.key]), H), background: s.color }" />
        <div class="absolute z-10 pointer-events-none rounded-lg bg-slate-900/95 ring-1 ring-slate-700 shadow-xl px-3 py-2 text-[13px] min-w-[150px]"
          :style="tipStyle">
          <p class="text-slate-400 mb-1">{{ fmtDay(data[hover].day, true) }}</p>
          <p v-for="s in series" :key="'t' + s.key" class="flex items-center gap-2 text-slate-200">
            <span class="w-2 h-2 rounded-full" :style="{ background: s.color }" />{{ s.label }}
            <span class="ml-auto tabular-nums font-semibold">{{ data[hover][s.key] }}</span>
          </p>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ data: Record<string, any>[]; series: { key: string; label: string; color: string }[] }>()

const W = 600, H = 240, PL = 28, PR = 70, PT = 10, PB = 22
const asTable = ref(false)
const wrap = ref<HTMLElement | null>(null)
const hover = ref<number | null>(null)

const max = computed(() => Math.max(1, ...props.data.flatMap(d => props.series.map(s => d[s.key] ?? 0))))
const niceMax = computed(() => { const m = max.value; const step = m <= 5 ? 1 : Math.ceil(m / 4 / 5) * 5 || 1; return Math.ceil(m / step) * step })
const yTicks = computed(() => { const n = niceMax.value <= 5 ? niceMax.value : 4; return Array.from({ length: n + 1 }, (_, i) => Math.round((niceMax.value / n) * i)) })
const x = (i: number) => PL + (i / Math.max(1, props.data.length - 1)) * (W - PL - PR)
const y = (v: number) => PT + (1 - v / niceMax.value) * (H - PT - PB)
const pct = (v: number, of: number) => `${(v / of) * 100}%`
const path = (k: string) => props.data.map((d, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(d[k] ?? 0).toFixed(1)}`).join('')
const total = (k: string) => props.data.reduce((s, d) => s + (d[k] ?? 0), 0)

// Label ujung garis — bila bertumpuk, digeser ke ATAS dari yang paling bawah agar tetap di dalam area plot
const endLabels = computed(() => {
  const last = props.data[props.data.length - 1]
  if (!last) return []
  const ls = props.series.map(s => ({ ...s, y: y(last[s.key] ?? 0) })).sort((a, b) => b.y - a.y)
  for (let i = 1; i < ls.length; i++) if (ls[i - 1].y - ls[i].y < 14) ls[i].y = ls[i - 1].y - 14
  return ls
})

function onMove(e: MouseEvent) {
  const r = wrap.value!.getBoundingClientRect()
  const sx = ((e.clientX - r.left) / r.width) * W
  const i = Math.round(((sx - PL) / (W - PL - PR)) * (props.data.length - 1))
  hover.value = Math.max(0, Math.min(props.data.length - 1, i))
}
const tipStyle = computed(() => {
  if (hover.value == null) return {}
  const left = x(hover.value) / W
  return left > 0.6 ? { right: `${(1 - left) * 100 + 2}%`, top: '8px' } : { left: `${left * 100 + 2}%`, top: '8px' }
})
const fmtDay = (d: string, long = false) => new Date(d + 'T00:00:00').toLocaleDateString('id-ID', long ? { weekday: 'short', day: 'numeric', month: 'short' } : { day: 'numeric', month: 'short' })
const ariaLabel = computed(() => `Tren 30 hari: ${props.series.map(s => `${s.label} ${total(s.key)}`).join(', ')}`)
</script>
