<template>
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-gauge" class="w-[18px] h-[18px] text-rose-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Kenapa Skor Ini?</h2>
          <p class="card-desc">Kontribusi tiap sinyal pada scan lengkap {{ data ? fmt(data.scannedAt) : '' }}</p>
        </div>
        <span v-if="data" class="text-2xl font-semibold tabular-nums" :class="data.final >= 0.75 ? 'text-rose-400' : data.final >= 0.45 ? 'text-amber-400' : 'text-emerald-400'">{{ pct(data.final) }}</span>
      </div>
    </template>

    <p v-if="!data" class="muted">Belum ada scan lengkap dengan rincian skor. Scan ulang situs ini untuk melihat dasar penilaiannya.</p>
    <template v-else>
      <ul class="space-y-3">
        <li v-for="r in data.rows" :key="r.key">
          <div class="flex items-baseline gap-2 text-sm">
            <span class="text-slate-200 font-medium">{{ r.label }}</span>
            <span class="text-xs text-slate-500 tabular-nums">{{ pct(r.raw) }} {{ r.weight }}</span>
            <span class="ml-auto tabular-nums font-semibold" :class="r.contrib > 0.005 ? 'text-slate-100' : 'text-slate-600'">+{{ pct(r.contrib) }}</span>
          </div>
          <div class="h-1.5 rounded-full bg-slate-800 mt-1.5 overflow-hidden">
            <div class="h-full rounded-full" :class="r === data.top && r.contrib > 0 ? 'bg-rose-500' : 'bg-primary-500/70'" :style="{ width: `${Math.min(100, r.contrib * 100)}%` }" />
          </div>
          <p class="text-[13px] text-slate-500 mt-1 break-words [overflow-wrap:anywhere]">{{ r.detail }}</p>
        </li>
      </ul>

      <div class="mt-4 rounded-lg bg-slate-950/60 ring-1 ring-slate-800 p-3 text-sm space-y-1.5">
        <div class="flex items-center gap-2">
          <span class="text-slate-400">Jumlah sinyal</span><span class="ml-auto tabular-nums text-slate-200">{{ pct(data.sum) }}</span>
        </div>
        <div v-if="data.rule" class="flex items-start gap-2">
          <UIcon name="i-lucide-scale" class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
          <span class="text-slate-300">{{ data.ruleText }}</span>
        </div>
        <p v-for="n in data.notes" :key="n" class="flex items-start gap-2 text-[13px] text-slate-400">
          <UIcon name="i-lucide-info" class="w-4 h-4 shrink-0 mt-0.5" />{{ n }}
        </p>
      </div>

      <div v-if="data.keywords.length || data.patterns.length" class="mt-3 flex flex-wrap gap-1">
        <UBadge v-for="k in data.keywords" :key="'k' + k" color="rose" variant="subtle" size="xs">{{ k }}</UBadge>
        <UBadge v-for="p in data.patterns" :key="'p' + p" color="gray" variant="subtle" size="xs">{{ p }}</UBadge>
      </div>
    </template>
  </UCard>
</template>

<script setup lang="ts">
defineProps<{ data: ReturnType<typeof explainScore> }>()
const pct = (n: number) => `${Math.round(n * 100)}%`
const fmt = (d?: string) => d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : ''
</script>
