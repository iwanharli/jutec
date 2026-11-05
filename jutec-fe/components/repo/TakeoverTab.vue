<template>
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-history" class="w-[18px] h-[18px] text-amber-400" /></div>
        <div class="flex-1 min-w-0"><h2 class="card-title">Domain Bekas</h2><p class="card-desc">Domain yang dulunya situs sah (bisnis, instansi, sekolah) dan kini dipakai judol — dari riwayat Wayback Machine</p></div>
        <UBadge color="gray" variant="subtle">{{ rows.length }}</UBadge>
      </div>
    </template>
    <div v-if="loading" class="p-5 space-y-2"><USkeleton v-for="i in 5" :key="i" class="h-14 w-full" /></div>
    <EmptyState v-else-if="!rows.length" icon="i-lucide-history" title="Belum ada domain bekas terdeteksi"
      desc="Riwayat domain situs judol dicek otomatis tiap 2 jam dari arsip Wayback Machine." />
    <div v-else class="divide-y divide-slate-800/60">
      <div v-for="r in rows" :key="r.id" class="px-5 py-4 flex flex-col md:flex-row md:items-center gap-3">
        <div class="flex-1 min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <NuxtLink :to="`/site/${r.id}`" class="mono text-sm text-slate-100 hover:text-primary-300 break-all">{{ host(r.url) }}</NuxtLink>
            <span v-if="r.lifecycle === 'inactive'" class="text-[11px] text-slate-500">tidak aktif</span>
            <span v-if="r.tp_listed != null" class="text-[11px]" :class="r.tp_listed ? 'text-emerald-400' : 'text-amber-400'">{{ r.tp_listed ? 'terblokir Komdigi' : 'belum diblokir' }}</span>
          </div>
          <p class="text-sm text-slate-400 mt-1">
            Dulu: <span class="text-slate-200">{{ r.takeover.legit_title || '—' }}</span>
          </p>
        </div>
        <div class="flex items-center gap-2 text-xs shrink-0">
          <span class="rounded px-2 py-1 bg-emerald-500/10 text-emerald-300 ring-1 ring-emerald-500/25">sah s.d. {{ ym(r.takeover.legit_until) }}</span>
          <UIcon name="i-lucide-arrow-right" class="w-3.5 h-3.5 text-slate-500" />
          <span class="rounded px-2 py-1 bg-rose-500/10 text-rose-300 ring-1 ring-rose-500/25">judol {{ ym(r.takeover.judol_since) }}</span>
        </div>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const { get } = useApi()
const rows = ref<any[]>([])
const loading = ref(true)
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/$/, '')
const ym = (ts?: string) => ts ? new Date(`${ts.slice(0, 4)}-${ts.slice(4, 6)}-01`).toLocaleDateString('id-ID', { month: 'short', year: 'numeric' }) : '—'
onMounted(async () => {
  try { rows.value = (await get<any>('/data/takeovers')).results ?? [] } catch {} finally { loading.value = false }
})
</script>
