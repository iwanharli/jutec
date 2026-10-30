<template>
  <UCard :ui="{ body: { padding: '' } }">
    <template #header>
      <div class="space-y-4">
        <div class="flex flex-col lg:flex-row lg:items-center gap-4">
          <div class="shrink-0">
            <div class="flex items-center gap-2.5">
              <h2 class="section-title">{{ title }}</h2>
              <UBadge v-if="live" color="emerald" variant="subtle" size="xs">Live</UBadge>
            </div>
            <p class="section-desc">{{ total.toLocaleString('id-ID') }} domain cocok dengan filter</p>
            <!-- Siklus hidup: bawaannya hanya situs yang masih beroperasi agar situs mati tidak jadi noise -->
            <div class="flex gap-0.5 mt-2 p-0.5 rounded-lg bg-slate-950/60 ring-1 ring-slate-800 w-fit">
              <UTooltip v-for="l in lifecycles" :key="l.value" :text="l.hint">
                <UButton size="2xs" :label="l.label" :color="lifecycle === l.value ? 'primary' : 'gray'"
                  :variant="lifecycle === l.value ? 'soft' : 'ghost'" @click="lifecycle = l.value" />
              </UTooltip>
            </div>
          </div>
          <div class="flex-1 flex flex-col sm:flex-row gap-3 lg:justify-end">
            <UInput v-model="q" icon="i-lucide-search" placeholder="Cari domain..." class="sm:w-72" />
            <div class="flex gap-1.5">
              <UButton v-for="p in priorities" :key="p.value" :label="p.label"
                :color="f.priority === p.value ? p.color : 'gray'" :variant="f.priority === p.value ? 'soft' : 'ghost'"
                @click="f.priority = f.priority === p.value ? '' : p.value" />
            </div>
            <UButton :color="showFilters || activeCount ? 'primary' : 'gray'" variant="soft" icon="i-lucide-sliders-horizontal"
              :label="activeCount ? `Filter (${activeCount})` : 'Filter'" @click="showFilters = !showFilters" />
          </div>
        </div>

        <div v-if="showFilters" class="grid grid-cols-2 md:grid-cols-3 2xl:grid-cols-6 gap-3 pt-4 border-t border-slate-800">
          <UFormGroup v-if="!gamblingOnly" label="Klasifikasi">
            <USelect v-model="f.gambling" :options="opt.gambling" />
          </UFormGroup>
          <UFormGroup label="Status situs">
            <USelect v-model="f.status" :options="opt.status" />
          </UFormGroup>
          <UFormGroup label="Punya artefak">
            <USelect v-model="f.artifact" :options="opt.artifact" />
          </UFormGroup>
          <UFormGroup label="Confidence minimal">
            <USelect v-model="f.minConf" :options="opt.conf" />
          </UFormGroup>
          <UFormGroup label="Di-scan dalam">
            <USelect v-model="f.days" :options="opt.days" />
          </UFormGroup>
          <UFormGroup label="Aplikasi">
            <USelect v-model="f.app" :options="opt.app" />
          </UFormGroup>
          <UFormGroup label="Urutkan">
            <USelect v-model="sortKey" :options="opt.sort" />
          </UFormGroup>
          <div class="col-span-full flex justify-end">
            <UButton size="sm" color="gray" variant="ghost" icon="i-lucide-rotate-ccw" label="Reset filter"
              :disabled="!activeCount" @click="resetFilters" />
          </div>
        </div>
      </div>
    </template>

    <UTable v-model:sort="sort" sort-mode="manual" :rows="rows" :columns="columns" :loading="loading"
      :ui="{ wrapper: 'relative overflow-x-hidden', base: 'w-full table-fixed', td: { padding: 'px-3 py-3' }, th: { padding: 'px-3 py-3' } }"
      :empty-state="{ icon: 'i-lucide-search-x', label: 'Tidak ada domain yang cocok.' }">
      <template #url-data="{ row }">
        <div class="flex items-center gap-3 min-w-0" :class="row.lifecycle === 'inactive' && 'opacity-60'">
          <span class="w-1 h-9 rounded-full shrink-0"
            :class="row.priority === 'High' ? 'bg-rose-500' : row.priority === 'Medium' ? 'bg-amber-500' : 'bg-slate-700'" />
          <div class="min-w-0">
            <NuxtLink :to="`/site/${row.id}`" :title="row.url"
              class="block text-[15px] font-medium text-slate-200 hover:text-primary-300 truncate">{{ stripScheme(row.url) }}</NuxtLink>
            <div class="flex items-center gap-2 mt-0.5 text-xs">
              <span class="inline-flex items-center gap-1" :class="statusMeta[row.status]?.text">
                <span class="w-1.5 h-1.5 rounded-full" :class="statusMeta[row.status]?.dot" />{{ statusMeta[row.status]?.label ?? row.status }}
              </span>
              <span v-if="!gamblingOnly" :class="row.is_gambling ? 'text-rose-400' : 'text-slate-500'">· {{ row.is_gambling ? 'Judol' : 'Normal' }}</span>
              <UBadge v-if="row.lifecycle === 'inactive'" color="gray" variant="outline" size="xs">tidak aktif</UBadge>
              <UBadge v-else-if="row.lifecycle === 'uncertain'" color="amber" variant="outline" size="xs">belum pasti</UBadge>
            </div>
          </div>
        </div>
      </template>
      <template #confidence-data="{ row }">
        <div class="space-y-1.5">
          <PriorityBadge :priority="row.priority" />
          <div class="flex items-center gap-2">
            <UProgress :value="Math.round(row.confidence * 100)" size="xs"
              :color="row.confidence >= 0.75 ? 'rose' : row.confidence >= 0.45 ? 'amber' : 'primary'" class="flex-1" />
            <span class="text-xs font-semibold tabular-nums w-9 text-right">{{ Math.round(row.confidence * 100) }}%</span>
          </div>
        </div>
      </template>
      <template #artifact_count-data="{ row }">
        <div class="flex flex-col gap-0.5 text-sm tabular-nums">
          <span :class="row.artifact_count ? 'text-violet-300' : 'text-slate-600'"><UIcon name="i-lucide-wallet" class="w-3.5 h-3.5 align-[-2px] mr-1" />{{ row.artifact_count }} artefak</span>
          <span :class="row.app_count ? 'text-emerald-300' : 'text-slate-600'"><UIcon name="i-lucide-smartphone" class="w-3.5 h-3.5 align-[-2px] mr-1" />{{ row.app_count }} apps</span>
        </div>
      </template>
      <template #last_scanned_at-data="{ row }">
        <div class="leading-tight">
          <p class="text-sm text-slate-300" title="Terakhir scan">{{ fmtDate(row.last_scanned_at) }}</p>
          <p class="text-xs text-slate-500 mt-0.5" title="Pertama ditemukan">ditemukan {{ fmtDate(row.created_at) }}</p>
        </div>
      </template>
      <template #actions-data="{ row }">
        <div class="flex justify-end gap-1">
          <UTooltip text="Detail"><UButton :to="`/site/${row.id}`" size="sm" icon="i-lucide-panel-right-open" /></UTooltip>
          <UTooltip text="Analisis ulang"><UButton :to="`${analyzePath}?url=${encodeURIComponent(row.url)}`" size="sm" color="gray" variant="soft" icon="i-lucide-scan-line" /></UTooltip>
        </div>
      </template>
    </UTable>

    <template #footer>
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center gap-3">
          <span class="muted">Halaman {{ page }} dari {{ pages.toLocaleString('id-ID') }}</span>
          <USelect v-model="limit" :options="[10, 15, 25, 50, 100]" size="sm" class="w-20" />
          <span class="muted">per halaman</span>
        </div>
        <UPagination v-model="page" :page-count="limit" :total="total" :max="7" />
      </div>
    </template>
  </UCard>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  title?: string
  gamblingOnly?: boolean // Dashboard: hanya situs judol
  live?: boolean         // muat ulang berkala bila tanpa filter
  analyzePath?: string
  defaultLimit?: number
}>(), { title: 'Semua Situs', gamblingOnly: false, live: false, analyzePath: '/discovery', defaultLimit: 15 })

const { get } = useApi()

const priorities = [
  { value: '', label: 'Semua', color: 'primary' },
  { value: 'High', label: 'High', color: 'rose' },
  { value: 'Medium', label: 'Medium', color: 'amber' },
  { value: 'Low', label: 'Low', color: 'gray' },
] as const

const opt = {
  gambling: [{ label: 'Semua', value: '' }, { label: 'Judol', value: 'true' }, { label: 'Normal', value: 'false' }],
  status: [{ label: 'Semua', value: '' }, { label: 'Aktif', value: 'active' }, { label: 'Diblokir (semua)', value: 'blocked' }, { label: 'Diblokir tapi masih hidup', value: 'blocked_alive' }, { label: 'Domain hilang', value: 'nxdomain' }, { label: 'Offline', value: 'offline' }, { label: 'Tak terjangkau', value: 'unreachable' }],
  artifact: [{ label: 'Semua', value: '' }, { label: 'Artefak apa pun', value: 'any' }, { label: 'Rekening bank', value: 'Bank' },
    { label: 'E-Wallet', value: 'E-Wallet' }, { label: 'QRIS', value: 'QRIS' }, { label: 'Kontak (WA/TG)', value: 'Contact' },
    { label: 'Nomor HP', value: 'Phone' }, { label: 'Kripto', value: 'Crypto' }],
  conf: [{ label: 'Semua', value: 0 }, { label: '≥ 45%', value: 0.45 }, { label: '≥ 75%', value: 0.75 }, { label: '≥ 90%', value: 0.9 }],
  days: [{ label: 'Kapan pun', value: 0 }, { label: '24 jam', value: 1 }, { label: '7 hari', value: 7 }, { label: '30 hari', value: 30 }, { label: '90 hari', value: 90 }],
  app: [{ label: 'Semua', value: '' }, { label: 'Punya aplikasi/APK', value: 'true' }],
  sort: [
    { label: 'Scan terbaru', value: 'last_scanned_at:desc' }, { label: 'Scan terlama', value: 'last_scanned_at:asc' },
    { label: 'Baru ditemukan', value: 'created_at:desc' }, { label: 'Paling lama ditemukan', value: 'created_at:asc' },
    { label: 'Confidence tertinggi', value: 'confidence:desc' }, { label: 'Prioritas tertinggi', value: 'priority:desc' },
    { label: 'Artefak terbanyak', value: 'artifact_count:desc' }, { label: 'Apps terbanyak', value: 'app_count:desc' },
    { label: 'Domain A–Z', value: 'url:asc' },
  ],
}

const statusMeta: Record<string, { label: string; dot: string; text: string }> = {
  active: { label: 'Aktif', dot: 'bg-emerald-500', text: 'text-slate-300' },
  offline: { label: 'Offline', dot: 'bg-amber-500', text: 'text-amber-300' },
  unreachable: { label: 'Tak terjangkau', dot: 'bg-rose-500', text: 'text-rose-300' },
  blocked_alive: { label: 'Diblokir · masih hidup', dot: 'bg-orange-500', text: 'text-orange-300' },
  blocked: { label: 'Diblokir', dot: 'bg-slate-500', text: 'text-slate-400' },
  nxdomain: { label: 'Domain hilang', dot: 'bg-slate-600', text: 'text-slate-500' },
}

const columns = [
  { key: 'url', label: 'Domain', sortable: true },
  { key: 'confidence', label: 'Skor', sortable: true, class: 'w-40' },
  { key: 'artifact_count', label: 'Temuan', sortable: true, class: 'w-32' },
  { key: 'last_scanned_at', label: 'Waktu', sortable: true, class: 'w-48' },
  { key: 'actions', label: '', class: 'w-24' },
]

const lifecycles = [
  { value: 'live', label: 'Beroperasi', hint: 'Aktif, diblokir tapi masih hidup, dan yang baru gagal diakses (<14 hari)' },
  { value: 'inactive', label: 'Tidak aktif', hint: 'Domain hilang, atau tidak bisa diakses ≥14 hari — data tetap tersimpan' },
  { value: '', label: 'Semua', hint: 'Termasuk situs yang sudah tidak aktif' },
]
const lifecycle = ref('live')

const blank = () => ({ priority: '', gambling: '', status: '', artifact: '', minConf: 0, days: 0, app: '' })
const f = reactive(blank())
const q = ref('')
const sort = ref<{ column: string; direction: 'asc' | 'desc' }>({ column: 'last_scanned_at', direction: 'desc' })
// Pilihan "Urutkan" di panel filter ↔ urutan tabel (judul kolom juga bisa diklik)
const sortKey = computed({
  get: () => `${sort.value?.column}:${sort.value?.direction}`,
  set: (v: string) => { const [column, direction] = v.split(':'); sort.value = { column, direction: direction as 'asc' | 'desc' } },
})
const page = ref(1)
const limit = ref(props.defaultLimit)
const rows = ref<any[]>([])
const total = ref(0)
const loading = ref(true)
const showFilters = ref(false)

const activeCount = computed(() =>
  [f.gambling, f.status, f.artifact, f.minConf, f.days, f.app].filter(Boolean).length)
const pages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))
const isFiltered = computed(() => !!(q.value || f.priority || activeCount.value || page.value > 1))

function resetFilters() { Object.assign(f, { ...blank(), priority: f.priority }) }
function stripScheme(u: string) { return u.replace(/^https?:\/\//, '').replace(/\/$/, '') }
function fmtDate(d?: string) { return d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—' }

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const d = await get<any>('/data/sites', {
      q: q.value || undefined,
      priority: f.priority || undefined,
      is_gambling: props.gamblingOnly ? true : (f.gambling || undefined),
      status: f.status || undefined,
      lifecycle: lifecycle.value || undefined,
      artifact_type: f.artifact || undefined,
      min_conf: f.minConf || undefined,
      scanned_days: f.days || undefined,
      has_app: f.app || undefined,
      sort: sort.value?.column || 'last_scanned_at',
      order: sort.value?.direction || 'desc',
      limit: limit.value,
      offset: (page.value - 1) * limit.value,
    })
    rows.value = d?.results ?? []
    total.value = d?.total ?? 0
  } catch {} finally { loading.value = false }
}

let timer: any = null
watch(q, () => { clearTimeout(timer); timer = setTimeout(() => { page.value === 1 ? load() : (page.value = 1) }, 350) })
watch([f, sort, limit, lifecycle], () => { page.value === 1 ? load() : (page.value = 1) }, { deep: true })
watch(page, () => load())

let poll: any = null
onMounted(() => {
  load()
  if (props.live) poll = setInterval(() => { if (!isFiltered.value) load(true) }, 5000)
})
onUnmounted(() => clearInterval(poll))

defineExpose({ reload: () => load() })
</script>
