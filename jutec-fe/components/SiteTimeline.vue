<template>
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-sky-500/10 ring-sky-500/25"><UIcon name="i-lucide-calendar-range" class="w-[18px] h-[18px] text-sky-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Timeline Situs</h2>
          <p class="card-desc">Domain, scan, perubahan, blokir, arsip, bukti & aktivitas analis dalam satu urutan</p>
        </div>
        <div class="flex flex-wrap gap-1">
          <UButton v-for="k in kinds" :key="k.key" size="2xs" :icon="k.icon" :label="`${k.label} ${k.n}`"
            :color="active.has(k.key) ? 'primary' : 'gray'" :variant="active.has(k.key) ? 'soft' : 'ghost'" @click="toggle(k.key)" />
        </div>
      </div>
    </template>
    <p v-if="!shown.length" class="muted">Belum ada kejadian tercatat.</p>
    <ol v-else class="relative max-h-[560px] overflow-y-auto pr-1">
      <li v-for="(e, i) in shown" :key="i" class="relative pl-7 pb-4 last:pb-0">
        <span class="absolute left-[7px] top-4 bottom-0 w-px bg-slate-800" />
        <span class="absolute left-0 top-1 w-[15px] h-[15px] rounded-full ring-4 ring-slate-900 flex items-center justify-center" :class="DOT[e.tone]" />
        <p v-if="yearBreak(i)" class="text-xs font-semibold text-slate-500 -mt-1 mb-1.5">{{ new Date(e.at).getFullYear() }}</p>
        <div class="flex flex-wrap items-baseline gap-x-2">
          <span class="text-[13px] tabular-nums text-slate-500 w-[120px] shrink-0">{{ fmt(e.at) }}</span>
          <span class="text-sm text-slate-100 font-medium">{{ e.title }}</span>
          <UBadge v-if="e.future" color="gray" variant="outline" size="xs">akan datang</UBadge>
        </div>
        <p v-if="e.detail" class="text-[13px] text-slate-400 mt-0.5 sm:pl-[128px] break-words [overflow-wrap:anywhere]">{{ e.detail }}</p>
      </li>
    </ol>
  </UCard>
</template>

<script setup lang="ts">
import type { TimelineEvent, TimelineKind } from '~/utils/siteAnalysis'
const props = defineProps<{ events: TimelineEvent[] }>()
const DOT: Record<string, string> = { rose: 'bg-rose-500', amber: 'bg-amber-500', emerald: 'bg-emerald-500', sky: 'bg-sky-500', violet: 'bg-violet-500', gray: 'bg-slate-500' }
const kinds = computed(() => (Object.entries(TIMELINE_KINDS) as [TimelineKind, any][])
  .map(([key, m]) => ({ key, ...m, n: props.events.filter(e => e.kind === key).length })).filter(k => k.n))
const active = ref(new Set<TimelineKind>(Object.keys(TIMELINE_KINDS) as TimelineKind[]))
function toggle(k: TimelineKind) {
  const s = new Set(active.value)
  s.has(k) ? s.delete(k) : s.add(k)
  active.value = s
}
const shown = computed(() => props.events.filter(e => active.value.has(e.kind)))
const yearBreak = (i: number) => i === 0 || new Date(shown.value[i].at).getFullYear() !== new Date(shown.value[i - 1].at).getFullYear()
const fmt = (t: number) => new Date(t).toLocaleString('id-ID', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
</script>
