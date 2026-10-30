<template>
  <UCard :ui="{ ring: 'ring-1 ring-blue-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-blue-500/10 ring-blue-500/25"><UIcon name="i-lucide-radar" class="w-[18px] h-[18px] text-blue-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">ID Analytics di Situs Lain</h2>
          <p class="card-desc">Domain di internet yang memakai ID Google Analytics / GTM / AdSense yang sama — sumber: HackerTarget</p>
        </div>
        <UCheckbox v-model="queue" label="Antrekan analisis domain baru" />
        <UButton size="sm" icon="i-lucide-search" :loading="busy" :label="results.some(r => r.fetched_at) ? 'Perbarui' : 'Cari di internet'" @click="$emit('refresh', queue)" />
      </div>
    </template>
    <div class="space-y-4">
      <div v-for="r in results" :key="r.value">
        <div class="flex flex-wrap items-center gap-2 mb-1.5">
          <span class="mono text-[15px] text-slate-100">{{ r.value }}</span>
          <UBadge color="gray" variant="subtle" size="xs">{{ r.platform }}</UBadge>
          <UBadge v-if="r.domains.length" color="blue" variant="subtle" size="xs">{{ r.domains.length }} domain</UBadge>
          <UBadge v-if="known(r)" color="rose" variant="subtle" size="xs">{{ known(r) }} judol di Jutec</UBadge>
          <span class="text-[13px] text-slate-500 ml-auto">{{ r.fetched_at ? `dicek ${fmt(r.fetched_at)}` : 'belum dicek' }}</span>
        </div>
        <p v-if="r.error" class="text-[13px] text-rose-300">{{ r.error }}</p>
        <p v-else-if="r.fetched_at && !r.domains.length" class="muted">Tidak ada domain lain yang tercatat memakai ID ini.</p>
        <ul v-else class="grid sm:grid-cols-2 gap-x-4 gap-y-1">
          <li v-for="d in r.domains.slice(0, open.has(r.value) ? undefined : 12)" :key="d.domain" class="flex items-center gap-2 text-sm min-w-0">
            <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="d.site ? (d.site.is_gambling ? 'bg-rose-500' : 'bg-slate-500') : 'bg-blue-400'" />
            <NuxtLink v-if="d.site" :to="`/site/${d.site.id}`" class="mono truncate hover:text-primary-300" :class="d.site.is_gambling ? 'text-rose-300' : 'text-slate-300'">{{ d.domain }}</NuxtLink>
            <a v-else :href="`/discovery?url=${encodeURIComponent('https://' + d.domain)}`" class="mono truncate text-slate-200 hover:text-primary-300">{{ d.domain }}</a>
            <span class="text-[12px] text-slate-500 shrink-0">{{ d.site ? (d.site.is_gambling ? 'judol' : 'normal') : 'baru' }}</span>
          </li>
        </ul>
        <UButton v-if="r.domains.length > 12" size="2xs" color="gray" variant="ghost" class="mt-1"
          :label="open.has(r.value) ? 'Lebih sedikit' : `+${r.domains.length - 12} lainnya`" @click="toggle(r.value)" />
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
defineProps<{ results: any[]; busy: boolean }>()
defineEmits<{ (e: 'refresh', queue: boolean): void }>()
const queue = ref(true)
const open = ref(new Set<string>())
function toggle(v: string) { const s = new Set(open.value); s.has(v) ? s.delete(v) : s.add(v); open.value = s }
const known = (r: any) => r.domains.filter((d: any) => d.site?.is_gambling).length
const fmt = (d: string) => new Date(d).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
</script>
