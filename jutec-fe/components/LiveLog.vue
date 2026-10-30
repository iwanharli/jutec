<template>
  <div ref="container" class="terminal h-64 space-y-0">
    <div v-if="!logs.length" class="flex items-center gap-2 text-zinc-600 italic py-1">
      <span class="text-zinc-700">$</span> Menunggu analisis dimulai...
    </div>

    <div v-for="(log, i) in logs" :key="i" class="log-line" :class="lineClass(log)">
      <span class="log-time">{{ fmt(log.ts) }}</span>
      <span class="text-zinc-600 shrink-0">{{ stepIcon(log.step) }}</span>
      <span class="break-all">{{ log.message }}</span>
    </div>

    <div v-if="running" class="log-line text-emerald-400">
      <span class="log-time">{{ fmt(Date.now()) }}</span>
      <span class="inline-flex gap-0.5">
        <span class="animate-bounce" style="animation-delay:0ms">·</span>
        <span class="animate-bounce" style="animation-delay:100ms">·</span>
        <span class="animate-bounce" style="animation-delay:200ms">·</span>
      </span>
      <span class="text-zinc-500">memproses</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ProgressEvent } from '~/composables/useAnalyzeStream'

const props = defineProps<{ logs: ProgressEvent[]; running: boolean }>()
const container = ref<HTMLDivElement>()

watch(() => props.logs.length, () =>
  nextTick(() => {
    if (container.value) container.value.scrollTop = container.value.scrollHeight
  })
)

function fmt(ts: number) {
  return new Date(ts).toLocaleTimeString('id-ID', { hour12: false })
}

const icons: Record<string, string> = {
  init: '→', crawling: '⟳', crawled: '✓', whois: '⟳', whois_done: '✓',
  simhash: '⟳', simhash_hit: '!', scoring: '⟳', scored: '✓',
  saving: '⟳', infra: '⟳', infra_done: '✓', subcrawl: '⟳',
  enrichment_queued: '⇢', complete: '●', failed: '✗',
}

function stepIcon(step: string) { return icons[step] ?? '›' }

function lineClass(log: ProgressEvent) {
  if (log.type === 'error') return 'text-red-400'
  if (log.type === 'done')  return 'text-emerald-400 font-semibold'
  if (log.step?.endsWith('_hit')) return 'text-amber-400'
  if (log.step?.endsWith('_done') || log.step === 'crawled' || log.step === 'scored')
    return 'text-emerald-300'
  return 'text-zinc-300'
}
</script>
