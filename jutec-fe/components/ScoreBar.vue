<template>
  <div class="space-y-3.5">
    <div v-for="(val, key) in breakdown" :key="key" class="space-y-1.5">
      <div class="flex items-center justify-between text-sm">
        <span class="text-slate-400">{{ labels[key] ?? key }}</span>
        <span class="font-mono text-slate-200 tabular-nums">{{ (val * 100).toFixed(0) }}%</span>
      </div>
      <div class="h-2 bg-slate-800 rounded-full overflow-hidden">
        <div class="h-full rounded-full transition-all duration-700 ease-out"
          :class="barColor(key)"
          :style="{ width: `${Math.min(val * 100, 100)}%` }" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ breakdown: Record<string, number> }>()

const labels: Record<string, string> = {
  content:  'Content',
  network:  'Network',
  domain:   'Domain',
  url_path: 'URL Path',
  tld_risk: 'TLD Risk',
  simhash:  'Template',
  enrichment: 'Enrichment',
  hostname: 'Hostname',
}

function barColor(key: string) {
  const m: Record<string, string> = {
    content:  'bg-blue-500',
    network:  'bg-purple-500',
    domain:   'bg-orange-500',
    url_path: 'bg-cyan-500',
    tld_risk: 'bg-red-500',
    simhash:  'bg-emerald-500',
  }
  return m[key] ?? 'bg-zinc-500'
}
</script>
