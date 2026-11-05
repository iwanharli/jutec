<template>
  <!-- Atribut yang dipakai bersama banyak situs judol: infrastruktur, referral, banner -->
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="space-y-4">
        <div class="flex items-center gap-3">
          <div class="card-icon" :class="iconBg"><UIcon :name="icon" class="w-[18px] h-[18px]" :class="iconColor" /></div>
          <div class="flex-1 min-w-0"><h2 class="card-title">{{ title }}</h2><p class="card-desc">{{ desc }}</p></div>
          <UBadge color="gray" variant="subtle">{{ rows.length }}</UBadge>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UInput v-model="q" icon="i-lucide-search" :placeholder="placeholder" class="w-full sm:w-72" />
          <SegmentedControl v-if="kinds.length > 1" v-model="kind" :options="kinds" />
          <SegmentedControl v-model="min" :options="minOpts" />
        </div>
      </div>
    </template>

    <div v-if="loading" class="p-5 space-y-2"><USkeleton v-for="i in 6" :key="i" class="h-12 w-full" /></div>
    <EmptyState v-else-if="!rows.length" :icon="icon" title="Belum ada yang dipakai bersama"
      :desc="min > 1 ? 'Coba turunkan batas jumlah situs.' : 'Data muncul setelah situs judol di-scan.'" />
    <div v-else :class="kind === 'image' ? 'grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3 p-4' : 'divide-y divide-slate-800/60'">
      <div v-for="r in rows" :key="r.value"
        :class="kind === 'image' ? 'rounded-xl bg-slate-950/40 ring-1 ring-slate-800 overflow-hidden' : 'px-5 py-3.5 hover:bg-slate-800/20'">
        <a v-if="kind === 'image'" :href="r.value" target="_blank" rel="noopener noreferrer" class="block h-36 bg-slate-900">
          <img :src="r.value" loading="lazy" referrerpolicy="no-referrer" class="w-full h-full object-contain" @error="($event.target as HTMLImageElement).style.display = 'none'">
        </a>
        <div :class="kind === 'image' ? 'p-3' : 'flex flex-col lg:flex-row lg:items-center gap-3'">
          <div class="flex-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="mono text-sm text-slate-100 break-all" :class="kind === 'image' && 'text-xs text-slate-400 line-clamp-1'">{{ display(r) }}</span>
              <UBadge v-if="r.label && kind !== 'image'" color="gray" variant="subtle" size="xs">{{ labelOf(r) }}</UBadge>
              <UBadge v-if="r.extra?.cdn" color="sky" variant="subtle" size="xs">CDN {{ r.extra.cdn }}</UBadge>
              <span v-if="r.extra?.country" class="text-xs text-slate-500">{{ r.extra.country }}</span>
            </div>
            <p v-if="r.extra?.abuse?.length" class="text-xs text-slate-500 mt-1">
              Lapor penyalahgunaan: <a v-for="(e, i) in r.extra.abuse" :key="e" :href="`mailto:${e}`" class="text-primary-400 hover:underline">{{ e }}<span v-if="i < r.extra.abuse.length - 1">, </span></a>
            </p>
            <div class="flex flex-wrap gap-1 mt-1.5">
              <NuxtLink v-for="s in r.samples" :key="s.id" :to="`/site/${s.id}`"
                class="rounded px-1.5 py-0.5 text-[11px] ring-1 hover:ring-slate-500" :class="s.is_gambling ? 'text-rose-300 ring-rose-500/25' : 'text-slate-400 ring-slate-700'">{{ host(s.url) }}</NuxtLink>
              <span v-if="r.sites > r.samples.length" class="text-[11px] text-slate-500 self-center">+{{ r.sites - r.samples.length }}</span>
            </div>
          </div>
          <div class="flex items-center gap-4 shrink-0" :class="kind === 'image' && 'mt-2'">
            <div class="text-right"><p class="text-lg font-semibold tabular-nums text-rose-300">{{ r.judol }}</p><p class="text-[11px] text-slate-500">judol</p></div>
            <div class="text-right"><p class="text-lg font-semibold tabular-nums text-slate-200">{{ r.live }}</p><p class="text-[11px] text-slate-500">masih hidup</p></div>
            <UTooltip v-if="lookupType" text="Cari semua situs dengan atribut ini">
              <UButton size="sm" color="gray" variant="soft" icon="i-lucide-scan-search" :to="`/reverse-lookup?type=${lookupType}&v=${encodeURIComponent(r.value)}`" />
            </UTooltip>
          </div>
        </div>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const props = defineProps<{
  title: string; desc: string; icon: string; iconBg: string; iconColor: string; placeholder?: string
  kinds: { value: string; label: string }[]
}>()
const { get } = useApi()
const kind = ref(props.kinds[0].value)
const q = ref('')
const min = ref(2)
const minOpts = [{ value: 1, label: 'Semua' }, { value: 2, label: '≥2 situs' }, { value: 5, label: '≥5 situs' }]
const rows = ref<any[]>([])
const loading = ref(true)

const LOOKUP: Record<string, string> = { ip: 'IP', ns: 'NS', ssl: 'SSL', fingerprint: 'FP', referral: 'FP' }
const lookupType = computed(() => LOOKUP[kind.value] ?? '')
const FP: Record<string, string> = { cdn: 'CDN', favicon: 'Favicon', api: 'API backend', apk_cert: 'Sertifikat APK', apk_pkg: 'Paket APK' }
const labelOf = (r: any) => kind.value === 'fingerprint' ? FP[r.label] ?? r.label : r.label
const display = (r: any) => kind.value === 'image' ? r.value.replace(/^https?:\/\//, '') : r.value
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/.*$/, '')

let t: any
async function load() {
  loading.value = true
  try { rows.value = (await get<any>('/data/shared', { kind: kind.value, q: q.value || undefined, min: min.value })).results ?? [] }
  catch { rows.value = [] } finally { loading.value = false }
}
watch([kind, min], load)
watch(q, () => { clearTimeout(t); t = setTimeout(load, 300) })
onMounted(load)
</script>
