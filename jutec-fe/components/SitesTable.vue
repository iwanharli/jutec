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
          </div>
        </div>

        <!-- Bar filter: satu baris, tinggi seragam -->
        <div class="flex flex-wrap items-center gap-2">
          <UInput v-model="q" icon="i-lucide-search" placeholder="Cari domain…" class="w-full sm:w-72" />
          <!-- Siklus hidup: bawaannya hanya situs yang masih beroperasi agar situs mati tidak jadi noise -->
          <SegmentedControl v-model="lifecycle" :options="lifecycles" />
          <SegmentedControl v-model="f.priority" clearable :options="priorityOpts" />
          <FilterToggle v-model="grouped" label="Per domain" icon="i-lucide-layers"
            hint="Kelompokkan situs dengan domain induk yang sama (subdomain & halaman) — paginasi dihitung per domain" />
          <UTooltip text="Unduh semua situs sesuai filter (maks. 10.000) sebagai CSV" class="ml-auto">
            <UButton class="h-9" color="gray" variant="solid" icon="i-lucide-file-down" label="CSV" @click="exportCsv" />
          </UTooltip>
          <UButton class="h-9" :color="showFilters || activeCount ? 'primary' : 'gray'" :variant="showFilters || activeCount ? 'soft' : 'solid'"
            icon="i-lucide-sliders-horizontal" :trailing-icon="showFilters ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
            :label="activeCount ? `Filter lanjutan (${activeCount})` : 'Filter lanjutan'" @click="showFilters = !showFilters" />
        </div>

        <div v-if="showFilters" class="grid grid-cols-2 md:grid-cols-4 2xl:grid-cols-7 gap-3 rounded-xl bg-slate-950/40 ring-1 ring-slate-800 p-4">
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
          <UFormGroup label="Daftar blokir Komdigi">
            <USelect v-model="f.tp" :options="opt.tp" />
          </UFormGroup>
          <UFormGroup label="Aplikasi">
            <USelect v-model="f.app" :options="opt.app" />
          </UFormGroup>
          <UFormGroup label="Urutkan">
            <USelect v-model="sortKey" :options="opt.sort" />
          </UFormGroup>
          <div class="col-span-full flex justify-end -mb-1">
            <UButton size="sm" color="gray" variant="soft" icon="i-lucide-rotate-ccw" label="Reset filter"
              :disabled="!activeCount" @click="resetFilters" />
          </div>
        </div>
      </div>
    </template>

    <UTable v-model:sort="sort" sort-mode="manual" :rows="displayRows" :columns="columns" :loading="loading"
      :ui="{ wrapper: 'relative overflow-x-hidden', base: 'w-full table-fixed', td: { padding: 'px-3 py-3' }, th: { padding: 'px-3 py-3' } }"
      :empty-state="{ icon: 'i-lucide-search-x', label: 'Tidak ada domain yang cocok.' }">
      <template #url-data="{ row }">
        <!-- Baris domain (kelompok): klik untuk buka/tutup daftar situsnya -->
        <button v-if="row._header" type="button" class="w-full flex items-center gap-3 min-w-0 text-left group/g" @click="toggle(row.root)">
          <span class="w-6 h-6 rounded-md flex items-center justify-center shrink-0 ring-1 transition-colors"
            :class="open[row.root] ? 'bg-primary-500/15 ring-primary-500/30 text-primary-300' : 'bg-slate-800 ring-slate-700 text-slate-400 group-hover/g:text-slate-200'">
            <UIcon name="i-lucide-chevron-right" class="w-4 h-4 transition-transform" :class="open[row.root] && 'rotate-90'" />
          </span>
          <div class="min-w-0">
            <p class="text-[15px] font-semibold text-slate-100 truncate">{{ row.root }}</p>
            <div class="flex items-center gap-2 mt-0.5 text-xs text-slate-400">
              <span class="inline-flex items-center gap-1"><UIcon name="i-lucide-layers" class="w-3.5 h-3.5" />{{ row.members.length }} situs</span>
              <span v-if="row.judol" class="text-rose-400">· {{ row.judol }} judol</span>
              <span v-if="row.live < row.members.length" class="text-slate-500">· {{ row.members.length - row.live }} tidak aktif</span>
            </div>
          </div>
        </button>
        <div v-else class="flex items-center gap-3 min-w-0" :class="[row.lifecycle === 'inactive' && 'opacity-60', row._child && 'pl-9 relative']">
          <span v-if="row._child" class="absolute left-3 top-[-14px] bottom-1/2 w-4 border-l border-b border-slate-700 rounded-bl-md" />
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
      <template #category-header>
        <UDropdown :items="[categoryMenu]" :popper="{ placement: 'bottom-start' }" :ui="{ width: 'w-56' }">
          <button type="button" class="inline-flex items-center gap-1.5 font-semibold" :class="category ? 'text-primary-300' : ''">
            Jenis<UIcon :name="category ? 'i-lucide-filter-x' : 'i-lucide-filter'" class="w-3.5 h-3.5 opacity-70" />
          </button>
          <template #item="{ item }">
            <UIcon :name="item.icon" class="w-4 h-4 shrink-0" :class="item.iconClass" />
            <span class="flex-1 text-left truncate" :class="item.active ? 'text-white font-medium' : ''">{{ item.label }}</span>
            <span class="text-[11px] tabular-nums text-slate-500">{{ item.count }}</span>
          </template>
        </UDropdown>
      </template>
      <template #category-data="{ row }">
        <div v-if="row._header" class="flex flex-wrap gap-1">
          <UTooltip v-for="c in row.categories" :key="c" :text="catMeta[c]?.desc">
            <span class="inline-flex items-center justify-center w-6 h-6 rounded-md ring-1" :class="catMeta[c]?.tag"><UIcon :name="catMeta[c]?.icon" class="w-3.5 h-3.5" /></span>
          </UTooltip>
        </div>
        <span v-else-if="catMeta[row.category]" class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[12px] font-medium ring-1 whitespace-nowrap" :class="catMeta[row.category].tag">
          <UIcon :name="catMeta[row.category].icon" class="w-3.5 h-3.5" />{{ catMeta[row.category].label }}
        </span>
        <UTooltip v-if="!row._header && row.is_gambling && row.tp_listed != null" :text="row.tp_listed ? 'Sudah ada di daftar blokir TrustPositif (Komdigi)' : 'Belum ada di daftar blokir TrustPositif — bisa diajukan'" class="block mt-1">
          <span class="inline-flex items-center gap-1 text-[11px]" :class="row.tp_listed ? 'text-emerald-400' : 'text-amber-400'">
            <UIcon :name="row.tp_listed ? 'i-lucide-shield-check' : 'i-lucide-shield-alert'" class="w-3 h-3" />{{ row.tp_listed ? 'Terblokir Komdigi' : 'Belum diblokir' }}
          </span>
        </UTooltip>
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
          <p class="text-xs text-slate-500 mt-0.5" title="Pertama ditemukan">{{ row._header ? 'pertama' : 'ditemukan' }} {{ fmtDate(row.created_at) }}</p>
        </div>
      </template>
      <template #actions-data="{ row }">
        <div v-if="row._header" class="flex justify-end">
          <UButton size="sm" color="gray" variant="soft" :icon="open[row.root] ? 'i-lucide-chevrons-down-up' : 'i-lucide-chevrons-up-down'"
            :label="open[row.root] ? 'Tutup' : 'Buka'" @click="toggle(row.root)" />
        </div>
        <div v-else class="flex justify-end gap-1">
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
          <span class="muted">{{ grouped ? 'domain' : 'situs' }} per halaman</span>
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

const { get, apiBase } = useApi()

const priorityOpts = [
  { value: 'High', label: 'High', color: 'rose', dot: 'bg-rose-500' },
  { value: 'Medium', label: 'Medium', color: 'amber', dot: 'bg-amber-500' },
  { value: 'Low', label: 'Low', dot: 'bg-slate-500' },
]

const opt = {
  gambling: [{ label: 'Semua', value: '' }, { label: 'Judol', value: 'true' }, { label: 'Normal', value: 'false' }],
  status: [{ label: 'Semua', value: '' }, { label: 'Aktif', value: 'active' }, { label: 'Diblokir (semua)', value: 'blocked' }, { label: 'Diblokir tapi masih hidup', value: 'blocked_alive' }, { label: 'Domain hilang', value: 'nxdomain' }, { label: 'Offline', value: 'offline' }, { label: 'Tak terjangkau', value: 'unreachable' }],
  artifact: [{ label: 'Semua', value: '' }, { label: 'Artefak apa pun', value: 'any' }, { label: 'Rekening bank', value: 'Bank' },
    { label: 'E-Wallet', value: 'E-Wallet' }, { label: 'QRIS', value: 'QRIS' }, { label: 'Kontak (WA/TG)', value: 'Contact' },
    { label: 'Nomor HP', value: 'Phone' }, { label: 'Kripto', value: 'Crypto' }],
  conf: [{ label: 'Semua', value: 0 }, { label: '≥ 45%', value: 0.45 }, { label: '≥ 75%', value: 0.75 }, { label: '≥ 90%', value: 0.9 }],
  days: [{ label: 'Kapan pun', value: 0 }, { label: '24 jam', value: 1 }, { label: '7 hari', value: 7 }, { label: '30 hari', value: 30 }, { label: '90 hari', value: 90 }],
  app: [{ label: 'Semua', value: '' }, { label: 'Punya aplikasi/APK', value: 'true' }],
  tp: [{ label: 'Semua', value: '' }, { label: 'Belum terdaftar — perlu diajukan', value: 'not_listed' }, { label: 'Sudah terdaftar', value: 'listed' }, { label: 'Belum dicek', value: 'unchecked' }],
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
  { key: 'category', label: 'Jenis', class: 'w-44' },
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

// Kategori situs (dihitung backend) — definisi bersama dengan halaman detail
const catMeta = SITE_CATEGORY
const category = ref('')
const catCounts = ref<Record<string, number>>({})
// Menu filter di header kolom "Jenis" — tabel khusus judol (Dashboard) tanpa opsi "Bukan Judi"
const categoryMenu = computed(() => [
  { label: 'Semua jenis', icon: 'i-lucide-list', iconClass: 'text-slate-400', active: !category.value,
    count: Object.entries(catCounts.value).filter(([k]) => !(props.gamblingOnly && k === 'clean')).reduce((n, [, v]) => n + v, 0).toLocaleString('id-ID'),
    click: () => (category.value = '') },
  ...Object.entries(catMeta).filter(([k]) => !(props.gamblingOnly && k === 'clean')).map(([value, m]) => ({
    label: m.label, icon: m.icon, iconClass: m.tag.split(' ').find(c => c.startsWith('text-')), active: category.value === value,
    count: (catCounts.value[value] ?? 0).toLocaleString('id-ID'), click: () => (category.value = value) })),
])

const blank = () => ({ priority: '', gambling: '', status: '', artifact: '', minConf: 0, days: 0, app: '', tp: '' })
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

// ── Kelompok per domain induk (backend: paginasi per domain, semua situs domain ikut dikirim) ──
const grouped = ref(true)
try { grouped.value = localStorage.getItem('jutec.sites.grouped') !== '0' } catch {}
watch(grouped, v => { try { localStorage.setItem('jutec.sites.grouped', v ? '1' : '0') } catch {} })
const open = reactive<Record<string, boolean>>({})
const toggle = (root: string) => { open[root] = !open[root] }
const PRIO = { High: 3, Medium: 2, Low: 1 } as Record<string, number>
const displayRows = computed(() => {
  if (!grouped.value) return rows.value
  const groups = new Map<string, any[]>()
  for (const r of rows.value) {
    const k = r.root || r.url
    if (!groups.has(k)) groups.set(k, [])
    groups.get(k)!.push(r)
  }
  const out: any[] = []
  for (const [root, members] of groups) {
    if (members.length === 1) { out.push(members[0]); continue }
    const top = [...members].sort((a, b) => (PRIO[b.priority] ?? 0) - (PRIO[a.priority] ?? 0) || b.confidence - a.confidence)[0]
    out.push({
      _header: true, id: `g:${root}`, root, members,
      priority: top.priority, confidence: top.confidence,
      categories: [...new Set(members.map(m => m.category).filter(Boolean))],
      judol: members.filter(m => m.is_gambling).length,
      live: members.filter(m => m.lifecycle !== 'inactive').length,
      artifact_count: members.reduce((n, m) => n + m.artifact_count, 0),
      app_count: members.reduce((n, m) => n + m.app_count, 0),
      last_scanned_at: members.map(m => m.last_scanned_at).filter(Boolean).sort().at(-1),
      created_at: members.map(m => m.created_at).filter(Boolean).sort()[0],
    })
    if (open[root]) out.push(...members.map(m => ({ ...m, _child: true })))
  }
  return out
})

const activeCount = computed(() =>
  [f.gambling, f.status, f.artifact, f.minConf, f.days, f.app, f.tp].filter(Boolean).length)
const pages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))
const isFiltered = computed(() => !!(q.value || f.priority || activeCount.value || page.value > 1))

function resetFilters() { Object.assign(f, { ...blank(), priority: f.priority }) }
function stripScheme(u: string) { return u.replace(/^https?:\/\//, '').replace(/\/$/, '') }
function fmtDate(d?: string) { return d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—' }

function queryParams(): Record<string, any> {
  return {
    q: q.value || undefined,
    priority: f.priority || undefined,
    is_gambling: props.gamblingOnly ? true : (f.gambling || undefined),
    judol_only: props.gamblingOnly ? 1 : undefined,
    group: grouped.value ? 'domain' : undefined,
    status: f.status || undefined,
    lifecycle: lifecycle.value || undefined,
    category: category.value || undefined,
    tp: f.tp || undefined,
    artifact_type: f.artifact || undefined,
    min_conf: f.minConf || undefined,
    scanned_days: f.days || undefined,
    has_app: f.app || undefined,
    sort: sort.value?.column || 'last_scanned_at',
    order: sort.value?.direction || 'desc',
    limit: limit.value,
    offset: (page.value - 1) * limit.value,
  }
}

// Ekspor CSV sesuai filter aktif (mis. judol yang belum ada di daftar blokir → lampiran pengajuan)
function exportCsv() {
  const p = { ...queryParams(), format: 'csv' }
  const qs = new URLSearchParams(Object.entries(p).filter(([, v]) => v !== undefined && v !== '').map(([k, v]) => [k, String(v)]))
  window.open(`${apiBase}/data/sites?${qs}`, '_blank')
}

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const d = await get<any>('/data/sites', queryParams())
    rows.value = d?.results ?? []
    total.value = d?.total ?? 0
    catCounts.value = d?.category_counts ?? {}
  } catch {} finally { loading.value = false }
}

let timer: any = null
watch(q, () => { clearTimeout(timer); timer = setTimeout(() => { page.value === 1 ? load() : (page.value = 1) }, 350) })
watch([f, sort, limit, lifecycle, category, grouped], () => { page.value === 1 ? load() : (page.value = 1) }, { deep: true })
watch(page, () => load())

let poll: any = null
onMounted(() => {
  load()
  if (props.live) poll = setInterval(() => { if (!isFiltered.value) load(true) }, 5000)
})
onUnmounted(() => clearInterval(poll))

defineExpose({ reload: () => load() })
</script>
