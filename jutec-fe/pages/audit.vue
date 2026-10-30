<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Keamanan" title="Log Audit"
      subtitle="Catatan setiap perubahan data, akses data pribadi, dan pengambilan/unduhan bukti. Log bersifat tambah-saja dan berantai hash, sehingga perubahan atau penghapusan baris lama akan terdeteksi."
      icon="i-lucide-scroll-text" />

    <!-- Status rantai -->
    <div class="rounded-2xl ring-1 p-5 flex flex-wrap items-center gap-4"
      :class="chain == null ? 'ring-slate-800 bg-slate-900/60' : chain.intact ? 'ring-emerald-500/30 bg-emerald-500/5' : 'ring-rose-500/40 bg-rose-500/10'">
      <div class="w-11 h-11 rounded-xl flex items-center justify-center ring-1 shrink-0"
        :class="chain == null ? 'bg-slate-800 ring-slate-700' : chain.intact ? 'bg-emerald-500/10 ring-emerald-500/30' : 'bg-rose-500/15 ring-rose-500/40'">
        <UIcon :name="chain == null ? 'i-lucide-shield-question' : chain.intact ? 'i-lucide-shield-check' : 'i-lucide-shield-x'" class="w-6 h-6"
          :class="chain == null ? 'text-slate-400' : chain.intact ? 'text-emerald-400' : 'text-rose-400'" />
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-base font-semibold text-white">
          {{ chain == null ? 'Memeriksa rantai hash…' : chain.intact ? 'Rantai log utuh' : `Rantai log RUSAK mulai baris #${chain.broken_at}` }}
        </p>
        <p class="text-sm text-slate-400 mt-0.5">
          <template v-if="chain">{{ chain.checked.toLocaleString('id-ID') }} baris diperiksa · {{ fmtDate(checkedAt) }}</template>
          <template v-if="chain && !chain.intact"> — ada baris yang diubah, dihapus, atau disisipkan di luar aplikasi.</template>
        </p>
      </div>
      <UButton color="gray" variant="soft" icon="i-lucide-refresh-cw" label="Periksa ulang" :loading="verifying" @click="verify" />
    </div>

    <UCard :ui="{ body: { padding: '' } }">
      <template #header>
        <div class="flex flex-col lg:flex-row lg:items-center gap-4">
          <div class="flex-1">
            <h2 class="section-title">Riwayat Aktivitas</h2>
            <p class="section-desc">{{ total.toLocaleString('id-ID') }} catatan</p>
          </div>
          <UInput v-model="q" icon="i-lucide-search" placeholder="Cari nama, aksi, atau target…" class="lg:w-80" />
          <USelect v-model="preset" :options="presets" class="lg:w-56" />
        </div>
      </template>

      <UTable :rows="rows" :columns="columns" :loading="loading"
        :ui="{ wrapper: 'relative overflow-x-hidden', base: 'w-full table-fixed', td: { padding: 'px-3 py-2.5' }, th: { padding: 'px-3 py-3' } }"
        :empty-state="{ icon: 'i-lucide-scroll-text', label: 'Belum ada catatan.' }">
        <template #at-data="{ row }">
          <p class="text-sm text-slate-200 whitespace-nowrap">{{ fmtDate(row.at) }}</p>
          <p class="text-xs text-slate-500 mono">#{{ row.id }}</p>
        </template>
        <template #actor-data="{ row }">
          <p class="text-sm truncate" :class="row.actor === 'anonymous' ? 'text-amber-300' : row.actor === 'system' ? 'text-slate-400' : 'text-slate-100'">
            {{ row.actor === 'anonymous' ? 'Tanpa nama' : row.actor === 'system' ? 'Sistem' : row.actor }}
          </p>
          <p class="text-xs text-slate-500 mono truncate">{{ row.ip || '—' }}</p>
        </template>
        <template #action-data="{ row }">
          <div class="flex items-start gap-2 min-w-0">
            <UIcon :name="actionMeta(row.action).icon" class="w-4 h-4 shrink-0 mt-0.5" :class="actionMeta(row.action).color" />
            <div class="min-w-0">
              <p class="text-sm text-slate-100">{{ actionMeta(row.action).label }}</p>
              <p class="text-xs text-slate-500 mono truncate" :title="row.action + (row.detail?.query ? '?' + row.detail.query : '')">
                {{ row.action }}<span v-if="row.detail?.query" class="text-slate-400">?{{ row.detail.query }}</span>
              </p>
            </div>
          </div>
        </template>
        <template #target-data="{ row }">
          <NuxtLink v-if="targetLink(row)" :to="targetLink(row)!" class="text-sm text-primary-300 hover:underline mono">{{ row.target }}</NuxtLink>
          <span v-else class="text-sm text-slate-400 mono">{{ row.target || '—' }}</span>
        </template>
        <template #status-data="{ row }">
          <UBadge :color="row.status >= 400 ? 'rose' : 'emerald'" variant="subtle" size="xs">{{ row.status || '—' }}</UBadge>
        </template>
        <template #hash-data="{ row }">
          <UTooltip :text="`hash ${row.hash} · sebelumnya ${row.prev_hash}`">
            <span class="mono text-xs text-slate-500">{{ row.hash.slice(0, 10) }}…</span>
          </UTooltip>
        </template>
      </UTable>

      <template #footer>
        <div class="flex items-center justify-between gap-4">
          <span class="muted">Halaman {{ page }} dari {{ Math.max(1, Math.ceil(total / limit)).toLocaleString('id-ID') }}</span>
          <UPagination v-model="page" :page-count="limit" :total="total" :max="7" />
        </div>
      </template>
    </UCard>
  </div>
</template>

<script setup lang="ts">
const { get } = useApi()

const columns = [
  { key: 'at', label: 'Waktu', class: 'w-44' },
  { key: 'actor', label: 'Pengguna', class: 'w-40' },
  { key: 'action', label: 'Aksi' },
  { key: 'target', label: 'Target', class: 'w-36' },
  { key: 'status', label: 'Status', class: 'w-20' },
  { key: 'hash', label: 'Hash', class: 'w-28' },
]
const presets = [
  { label: 'Semua aktivitas', value: '' },
  { label: 'Bukti forensik', value: 'evidence' },
  { label: 'Akses data identitas', value: 'leak' },
  { label: 'Data KomInfo/Dukcapil', value: 'data/' },
  { label: 'Perubahan blacklist', value: 'blacklist' },
  { label: 'Scan & discovery', value: 'discovery' },
]

const rows = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const limit = 50
const q = ref('')
const preset = ref('')
const loading = ref(true)
const chain = ref<any>(null)
const checkedAt = ref('')
const verifying = ref(false)

const ACTIONS: { re: RegExp; label: string; icon: string; color: string }[] = [
  { re: /^evidence\.captured/, label: 'Bukti forensik tersimpan', icon: 'i-lucide-file-lock-2', color: 'text-sky-400' },
  { re: /^evidence\.failed/, label: 'Pengambilan bukti gagal', icon: 'i-lucide-file-x', color: 'text-rose-400' },
  { re: /^POST \/sites\/:id\/evidence/, label: 'Meminta pengambilan bukti', icon: 'i-lucide-camera', color: 'text-sky-400' },
  { re: /\/evidence\/:id\/verify/, label: 'Memverifikasi bukti', icon: 'i-lucide-shield-check', color: 'text-emerald-400' },
  { re: /\/evidence\/:id\/zip/, label: 'Mengunduh paket bukti', icon: 'i-lucide-file-archive', color: 'text-sky-300' },
  { re: /\/evidence\/:id\/files/, label: 'Membuka file bukti', icon: 'i-lucide-file-search', color: 'text-sky-300' },
  { re: /\/sites\/:id\/evidence$/, label: 'Melihat daftar bukti', icon: 'i-lucide-files', color: 'text-slate-400' },
  { re: /\/leak\/vfl/, label: 'Pengecekan identitas (VflOSINT)', icon: 'i-lucide-bot', color: 'text-amber-400' },
  { re: /\/leak/, label: 'Pencarian data identitas', icon: 'i-lucide-fingerprint', color: 'text-amber-400' },
  { re: /\/identities/, label: 'Melihat identitas terkait situs', icon: 'i-lucide-user-search', color: 'text-amber-400' },
  { re: /\/data\/(kominfo|dukcapil)/, label: 'Membuka data KomInfo/Dukcapil', icon: 'i-lucide-id-card', color: 'text-amber-400' },
  { re: /\/blacklist/, label: 'Mengubah blacklist', icon: 'i-lucide-ban', color: 'text-rose-300' },
  { re: /\/audit/, label: 'Membuka log audit', icon: 'i-lucide-scroll-text', color: 'text-slate-400' },
  { re: /\/discovery|\/scan|\/analyze/, label: 'Menjalankan scan / discovery', icon: 'i-lucide-radar', color: 'text-primary-400' },
  { re: /^DELETE/, label: 'Menghapus data', icon: 'i-lucide-trash-2', color: 'text-rose-400' },
]
function actionMeta(a: string) {
  return ACTIONS.find(x => x.re.test(a)) ?? { label: 'Perubahan data', icon: 'i-lucide-pencil', color: 'text-slate-400' }
}
function targetLink(row: any) {
  const [kind, id] = (row.target ?? '').split(':')
  if (kind === 'site' && id) return `/site/${id}`
  const sid = row.detail?.site_id
  if (kind === 'evidence' && sid) return `/site/${sid}`
  return null
}
function fmtDate(d?: string) { return d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'medium' }) : '—' }

async function load() {
  loading.value = true
  try {
    const d = await get<any>('/audit', { q: q.value || undefined, action: preset.value || undefined, limit, offset: (page.value - 1) * limit })
    rows.value = d?.results ?? []
    total.value = d?.total ?? 0
  } catch {} finally { loading.value = false }
}
async function verify() {
  verifying.value = true
  try {
    chain.value = await get<any>('/audit/verify')
    checkedAt.value = new Date().toISOString()
  } catch {} finally { verifying.value = false }
}

let timer: any = null
watch(q, () => { clearTimeout(timer); timer = setTimeout(() => { page.value === 1 ? load() : (page.value = 1) }, 350) })
watch(preset, () => { page.value === 1 ? load() : (page.value = 1) })
watch(page, load)
onMounted(() => { load(); verify() })
useHead({ title: 'Log Audit — Jutec' })
</script>
