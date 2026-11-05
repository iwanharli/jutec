<template>
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-repeat" class="w-[18px] h-[18px] text-rose-400" /></div>
        <div class="flex-1 min-w-0"><h2 class="card-title">Domain Respawn</h2><p class="card-desc">Domain baru dari klaster operator yang sama, muncul ≤30 hari setelah domain lamanya diblokir atau mati</p></div>
        <div v-if="rows.length" class="flex items-center gap-4 text-sm">
          <span class="text-slate-400">Median jeda <b class="text-slate-100 tabular-nums">{{ median }} hari</b></span>
          <span class="text-slate-400">Pengganti masih hidup <b class="text-rose-300 tabular-nums">{{ rows.filter(r => r.new_live).length }}</b></span>
        </div>
      </div>
    </template>

    <div v-if="loading" class="p-5 space-y-2"><USkeleton v-for="i in 5" :key="i" class="h-14 w-full" /></div>
    <EmptyState v-else-if="!rows.length" icon="i-lucide-repeat" title="Belum ada respawn terdeteksi"
      desc="Butuh klaster operator dan situs yang sudah diblokir/mati. Pola ini terlihat setelah pemantauan berjalan beberapa minggu." />
    <div v-else class="divide-y divide-slate-800/60">
      <div v-for="r in rows" :key="r.new.id" class="px-5 py-3.5 flex flex-col md:flex-row md:items-center gap-3">
        <span class="md:w-44 shrink-0 text-xs text-slate-400 truncate" :title="r.operator">
          <UIcon name="i-lucide-users" class="w-3.5 h-3.5 inline -mt-0.5" /> {{ r.operator }}
        </span>
        <div class="flex-1 min-w-0 flex flex-wrap items-center gap-2">
          <NuxtLink :to="`/site/${r.old.id}`" class="mono text-sm text-slate-500 line-through decoration-slate-600 hover:text-slate-300 break-all">{{ host(r.old.url) }}</NuxtLink>
          <span class="text-[11px] text-slate-500">{{ r.old_reason === 'blocked' ? 'diblokir' : 'mati' }} {{ day(r.old_down_at) }}</span>
          <UIcon name="i-lucide-arrow-right" class="w-4 h-4 text-rose-400" />
          <NuxtLink :to="`/site/${r.new.id}`" class="mono text-sm text-slate-100 hover:text-primary-300 break-all">{{ host(r.new.url) }}</NuxtLink>
          <UBadge :color="r.new_live ? 'rose' : 'gray'" variant="subtle" size="xs">{{ r.new_live ? 'masih hidup' : 'sudah mati' }}</UBadge>
        </div>
        <div class="text-right shrink-0"><p class="text-sm font-semibold tabular-nums" :class="r.gap_days <= 3 ? 'text-rose-300' : 'text-slate-200'">{{ fmtGap(r.gap_days) }}</p><p class="text-[11px] text-slate-500">jeda</p></div>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const { get } = useApi()
const rows = ref<any[]>([])
const loading = ref(true)
const median = computed(() => {
  const v = rows.value.map(r => r.gap_days).sort((a, b) => a - b)
  return v.length ? Math.max(0, Math.round(v[Math.floor(v.length / 2)] * 10) / 10) : 0
})
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/.*$/, '')
const day = (d: string) => new Date(d).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })
const fmtGap = (d: number) => d < 1 ? `${Math.max(1, Math.round(d * 24))} jam` : `${Math.round(d)} hari`
onMounted(async () => {
  try { rows.value = (await get<any>('/data/respawns')).results ?? [] } catch {} finally { loading.value = false }
})
</script>
