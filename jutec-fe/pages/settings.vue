<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Sistem" title="Pengaturan" icon="i-lucide-settings"
      subtitle="Kunci API sumber data, parameter engine, dan pemeliharaan data." />

    <div class="grid grid-cols-1 lg:grid-cols-[240px_1fr] gap-6 items-start">
      <!-- ── Submenu ── -->
      <nav class="lg:sticky lg:top-24 rounded-2xl bg-slate-900/60 ring-1 ring-slate-800 p-2 flex lg:flex-col gap-1 overflow-x-auto">
        <button v-for="s in sections" :key="s.key" type="button" @click="go(s.key)"
          class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-left whitespace-nowrap transition-colors"
          :class="tab === s.key ? 'bg-primary-500/10 text-primary-200 ring-1 ring-primary-500/25' : 'text-slate-300 hover:bg-slate-800/60'">
          <UIcon :name="s.icon" class="w-[18px] h-[18px] shrink-0" :class="tab === s.key ? 'text-primary-400' : 'text-slate-500'" />
          <span class="flex-1 min-w-0">
            <span class="block text-sm font-medium">{{ s.label }}</span>
            <span class="hidden lg:block text-xs text-slate-500 truncate">{{ s.desc }}</span>
          </span>
          <span v-if="s.badge" class="rounded-full px-1.5 text-[11px] tabular-nums" :class="s.badgeCls">{{ s.badge }}</span>
        </button>
      </nav>

      <div class="min-w-0 space-y-6">
        <!-- ── Kunci API ── -->
        <template v-if="tab === 'api'">
          <div v-if="pendingCount" class="flex flex-wrap items-center gap-3 rounded-2xl bg-amber-500/10 ring-1 ring-amber-500/30 px-5 py-4">
            <UIcon name="i-lucide-refresh-ccw-dot" class="w-5 h-5 text-amber-400" />
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-amber-100">{{ pendingCount }} kunci baru belum diterapkan</p>
              <p class="text-xs text-amber-200/70">Sistem akan dimuat ulang beberapa detik; pekerjaan latar yang terpotong diulang otomatis.</p>
            </div>
            <UButton color="amber" icon="i-lucide-zap" :loading="applying" label="Terapkan sekarang" @click="apply" />
          </div>

          <UCard v-for="g in keyGroups" :key="g.key" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
            <template #header>
              <div class="flex items-center gap-3">
                <div class="card-icon" :class="g.iconBg"><UIcon :name="g.icon" class="w-[18px] h-[18px]" :class="g.iconColor" /></div>
                <div class="flex-1 min-w-0"><h2 class="card-title">{{ g.label }}</h2><p class="card-desc">{{ g.desc }}</p></div>
              </div>
            </template>
            <div class="divide-y divide-slate-800/70">
              <div v-for="k in g.items" :key="k.env" class="px-5 py-4">
                <div class="flex flex-col md:flex-row md:items-start gap-4">
                  <div class="flex-1 min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <p class="text-sm font-medium text-slate-100">{{ k.label }}</p>
                      <span class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] ring-1" :class="statusOf(k).cls">
                        <span class="w-1.5 h-1.5 rounded-full" :class="statusOf(k).dot" />{{ statusOf(k).label }}
                      </span>
                      <UBadge v-if="k.required" color="rose" variant="subtle" size="xs">Wajib</UBadge>
                    </div>
                    <p class="text-sm text-slate-400 mt-1">{{ k.desc }}</p>
                    <p class="text-xs text-slate-500 mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1">
                      <span v-if="k.preview" class="mono">{{ k.preview }}</span>
                      <span v-if="k.source === 'env'">dari file konfigurasi server</span>
                      <a v-if="k.signup" :href="k.signup" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 text-primary-400 hover:underline">
                        Dapatkan kunci <UIcon name="i-lucide-external-link" class="w-3 h-3" />
                      </a>
                    </p>
                  </div>
                  <div class="flex items-center gap-2 md:w-[380px] shrink-0">
                    <UInput v-model="drafts[k.env]" :type="k.secret && !reveal[k.env] ? 'password' : 'text'" class="flex-1"
                      :placeholder="k.set ? 'Ganti dengan kunci baru…' : 'Tempel kunci di sini…'" :ui="{ base: 'font-mono' }"
                      autocomplete="off" @keyup.enter="saveKey(k)">
                      <template v-if="k.secret" #trailing>
                        <UButton color="gray" variant="link" :padded="false" :icon="reveal[k.env] ? 'i-lucide-eye-off' : 'i-lucide-eye'"
                          @click="reveal[k.env] = !reveal[k.env]" />
                      </template>
                    </UInput>
                    <UButton icon="i-lucide-save" :disabled="!drafts[k.env]" :loading="savingKey === k.env" label="Simpan" @click="saveKey(k)" />
                    <UTooltip v-if="k.source === 'db'" text="Hapus kunci dari Pengaturan">
                      <UButton color="gray" variant="ghost" icon="i-lucide-trash-2" @click="removeKey(k)" />
                    </UTooltip>
                  </div>
                </div>
              </div>
            </div>
          </UCard>
        </template>

        <!-- ── Parameter per kelompok ── -->
        <template v-else-if="currentGroup">
          <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
            <template #header>
              <div class="flex items-center gap-3">
                <div class="card-icon" :class="currentGroup.iconBg"><UIcon :name="currentGroup.icon" class="w-[18px] h-[18px]" :class="currentGroup.iconColor" /></div>
                <div class="flex-1 min-w-0"><h2 class="card-title">{{ currentGroup.label }}</h2><p class="card-desc">Perubahan langsung berlaku</p></div>
              </div>
            </template>
            <div v-if="loading" class="p-5 space-y-3"><USkeleton v-for="i in 3" :key="i" class="h-14 w-full" /></div>
            <div v-else-if="!currentGroup.settings.length" class="empty-state !py-12"><p class="text-sm">Tidak ada parameter di bagian ini.</p></div>
            <div v-else class="divide-y divide-slate-800/70">
              <div v-for="s in currentGroup.settings" :key="s.key" class="flex flex-col sm:flex-row sm:items-center gap-4 px-5 py-4">
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-slate-100">{{ formatKey(s.key) }}</p>
                  <p class="text-sm text-slate-400 mt-1 leading-relaxed">{{ s.description || '—' }}</p>
                  <p class="text-xs text-slate-600 mt-1.5">Diubah {{ formatDate(s.updated_at) }}</p>
                </div>
                <div class="shrink-0 flex items-center gap-3">
                  <template v-if="isBool(s.value)">
                    <span class="text-sm font-medium" :class="s.value === 'true' ? 'text-primary-300' : 'text-slate-500'">{{ s.value === 'true' ? 'Aktif' : 'Nonaktif' }}</span>
                    <UToggle :model-value="s.value === 'true'" size="lg" :loading="saving === s.key" @update:model-value="toggleBool(s)" />
                  </template>
                  <template v-else>
                    <UInput v-model="s.value" :type="s.key.includes('password') ? 'password' : isNum(s.value) && !s.key.startsWith('probe') ? 'number' : 'text'"
                      :class="s.key.startsWith('probe') ? 'w-64' : 'w-36'" :ui="{ base: s.key.startsWith('probe') ? 'font-mono' : 'font-mono text-right' }" @keyup.enter="save(s)" />
                    <UButton icon="i-lucide-save" :loading="saving === s.key" label="Simpan" @click="save(s)" />
                  </template>
                </div>
              </div>
            </div>
          </UCard>
        </template>

        <!-- ── Pemeliharaan ── -->
        <template v-else-if="tab === 'maintenance'">
          <UCard :ui="{ header: { padding: 'px-5 py-4' } }">
            <template #header>
              <div class="flex items-center gap-3">
                <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-database-zap" class="w-[18px] h-[18px] text-amber-400" /></div>
                <div class="flex-1 min-w-0"><h2 class="card-title">Cache pencarian</h2><p class="card-desc">Hasil pencarian Google disimpan agar kuota tidak terbuang untuk kata kunci yang sama</p></div>
                <UButton color="amber" variant="soft" icon="i-lucide-trash-2" :loading="clearing" :label="`Bersihkan (${cacheCount})`" :disabled="!cacheCount" @click="clearCache" />
              </div>
            </template>
            <p class="text-sm text-slate-400">Bersihkan bila ingin Deep Sweep mengambil hasil pencarian terbaru untuk kata kunci yang sudah pernah dicari.</p>
          </UCard>
          <MaintenancePanel />
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const { get, put, patch, post } = useApi()
const route = useRoute()
const router = useRouter()
const { confirm } = useConfirm()

const settings   = ref([])
const apiKeys    = ref([])
const loading    = ref(true)
const saving     = ref(null)
const savingKey  = ref(null)
const applying   = ref(false)
const clearing   = ref(false)
const cacheCount = ref(0)
const drafts     = reactive({})
const reveal     = reactive({})

const groupDefs = [
  { key: 'search',   prefix: 'serper',   label: 'Pencarian',  desc: 'Halaman & kuota pencarian Google', icon: 'i-lucide-search',          iconBg: 'bg-blue-500/10 ring-blue-500/25',     iconColor: 'text-blue-400' },
  { key: 'crawler',  prefix: 'crawler',  label: 'Crawler',    desc: 'Kecepatan & batas pemindaian',      icon: 'i-lucide-bug',             iconBg: 'bg-amber-500/10 ring-amber-500/25',   iconColor: 'text-amber-400' },
  { key: 'sweep',    prefix: 'sweep',    label: 'Deep Sweep', desc: 'Kedalaman & batas sweep',           icon: 'i-lucide-satellite-dish',  iconBg: 'bg-indigo-500/10 ring-indigo-500/25', iconColor: 'text-indigo-400' },
  { key: 'telegram', prefix: 'telegram', label: 'Telegram',   desc: 'Pemantauan channel',                icon: 'i-lucide-send',            iconBg: 'bg-sky-500/10 ring-sky-500/25',       iconColor: 'text-sky-400' },
  { key: 'general',  prefix: '',         label: 'Umum',       desc: 'Parameter lainnya',                 icon: 'i-lucide-sliders-horizontal', iconBg: 'bg-slate-700/40 ring-slate-600/40', iconColor: 'text-slate-300' },
]
const groups = computed(() => groupDefs.map(def => ({
  ...def,
  // Data dummy uji form (probe_*) diatur di halaman Form Daftar & Login
  settings: settings.value.filter(s => !s.key.startsWith('probe_') && !['sweep_daily_last', 'sweep_daily_offset'].includes(s.key)).filter(s => def.prefix ? s.key.startsWith(def.prefix) : !groupDefs.some(d => d.prefix && s.key.startsWith(d.prefix))),
})))

const missingRequired = computed(() => apiKeys.value.filter(k => k.required && !k.set).length)
const pendingCount = computed(() => apiKeys.value.filter(k => k.pending).length)
const sections = computed(() => [
  { key: 'api', label: 'Kunci API', desc: 'Sumber data eksternal', icon: 'i-lucide-key-round',
    badge: missingRequired.value || pendingCount.value || null,
    badgeCls: missingRequired.value ? 'bg-rose-500/20 text-rose-300' : 'bg-amber-500/20 text-amber-300' },
  ...groups.value.filter(g => g.settings.length || g.key === 'general').map(g => ({ key: g.key, label: g.label, desc: g.desc, icon: g.icon })),
  { key: 'maintenance', label: 'Pemeliharaan', desc: 'Cache, scan ulang & pembersihan', icon: 'i-lucide-wrench' },
])
const tab = computed(() => sections.value.some(s => s.key === route.query.tab) ? route.query.tab : 'api')
const currentGroup = computed(() => groups.value.find(g => g.key === tab.value))
function go(key) { router.replace({ query: { ...route.query, tab: key } }) }

const keyGroupDefs = [
  { key: 'pencarian',  label: 'Pencarian',            desc: 'Menemukan situs judol baru',                 icon: 'i-lucide-search',       iconBg: 'bg-blue-500/10 ring-blue-500/25',     iconColor: 'text-blue-400' },
  { key: 'reputasi',   label: 'Reputasi & Intelijen', desc: 'Memperkaya data situs dari layanan luar',    icon: 'i-lucide-shield-question', iconBg: 'bg-emerald-500/10 ring-emerald-500/25', iconColor: 'text-emerald-400' },
  { key: 'identitas',  label: 'Identitas',            desc: 'Pemilik nomor & rekening',                   icon: 'i-lucide-fingerprint',  iconBg: 'bg-rose-500/10 ring-rose-500/25',     iconColor: 'text-rose-400' },
  { key: 'blockchain', label: 'Blockchain',           desc: 'Penelusuran dompet kripto',                  icon: 'i-lucide-coins',        iconBg: 'bg-amber-500/10 ring-amber-500/25',   iconColor: 'text-amber-400' },
  { key: 'telegram',   label: 'Telegram',             desc: 'Akses baca channel Telegram',                icon: 'i-lucide-send',         iconBg: 'bg-sky-500/10 ring-sky-500/25',       iconColor: 'text-sky-400' },
]
const keyGroups = computed(() => keyGroupDefs.map(g => ({ ...g, items: apiKeys.value.filter(k => k.group === g.key) })).filter(g => g.items.length))

function statusOf(k) {
  if (k.pending) return { label: 'Belum diterapkan', cls: 'text-amber-300 ring-amber-500/30', dot: 'bg-amber-400' }
  if (k.set) return { label: 'Aktif', cls: 'text-emerald-300 ring-emerald-500/30', dot: 'bg-emerald-400' }
  if (k.free_tier) return { label: 'Kuota gratis', cls: 'text-sky-300 ring-sky-500/30', dot: 'bg-sky-400' }
  return { label: 'Belum diisi', cls: k.required ? 'text-rose-300 ring-rose-500/30' : 'text-slate-400 ring-slate-700', dot: k.required ? 'bg-rose-400' : 'bg-slate-500' }
}

async function load() {
  loading.value = true
  try {
    const [s, k, c] = await Promise.all([
      get('/settings'),
      get('/settings/api-keys').catch(() => []),
      get('/discovery/cache/stats').catch(() => null),
    ])
    settings.value = s ?? []
    apiKeys.value = k ?? []
    cacheCount.value = c?.total ?? 0
  } finally { loading.value = false }
}
async function loadKeys() { apiKeys.value = await get('/settings/api-keys').catch(() => apiKeys.value) }

async function saveKey(k) {
  savingKey.value = k.env
  try {
    await put(`/settings/api-keys/${k.env}`, { value: drafts[k.env] })
    drafts[k.env] = ''
    reveal[k.env] = false
    await loadKeys()
    showToast('success', `${k.label} disimpan — klik "Terapkan sekarang" agar dipakai`)
  } catch { showToast('error', 'Gagal menyimpan kunci') }
  finally { savingKey.value = null }
}

async function removeKey(k) {
  const ok = await confirm({ title: `Hapus kunci ${k.label}?`, description: 'Sumber ini akan memakai kunci dari file konfigurasi server bila ada, atau nonaktif.', confirmLabel: 'Hapus', tone: 'danger' })
  if (!ok) return
  try {
    await put(`/settings/api-keys/${k.env}`, { value: '' })
    await loadKeys()
    showToast('success', `${k.label} dihapus — terapkan agar berlaku`)
  } catch { showToast('error', 'Gagal menghapus kunci') }
}

async function apply() {
  applying.value = true
  try {
    await post('/settings/apply', {})
    // Tunggu sistem kembali hidup (maks. ±60 detik)
    for (let i = 0; i < 30; i++) {
      await new Promise(r => setTimeout(r, 2000))
      try { apiKeys.value = await get('/settings/api-keys'); break } catch {}
    }
    showToast('success', pendingCount.value ? 'Sebagian kunci belum terterapkan — coba lagi' : 'Kunci baru sudah dipakai')
  } catch { showToast('error', 'Gagal menerapkan') }
  finally { applying.value = false }
}

async function clearCache() {
  clearing.value = true
  try {
    await fetch(useRuntimeConfig().public.apiBase + '/discovery/cache', { method: 'DELETE' })
    cacheCount.value = 0
    showToast('success', 'Cache pencarian dibersihkan')
  } catch { showToast('error', 'Gagal membersihkan cache') }
  finally { clearing.value = false }
}

async function save(s) {
  saving.value = s.key
  try {
    await patch(`/settings/${s.key}`, { value: String(s.value) })
    showToast('success', `${formatKey(s.key)} disimpan`)
    s.updated_at = new Date().toISOString()
  } catch { showToast('error', 'Gagal menyimpan') }
  finally { saving.value = null }
}
async function toggleBool(s) {
  s.value = s.value === 'true' ? 'false' : 'true'
  await save(s)
}

function isBool(v) { return v === 'true' || v === 'false' }
function isNum(v)  { return !isNaN(Number(v)) && v !== '' }
function formatKey(key) { return key.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase()) }
function formatDate(d) {
  if (!d) return '—'
  return new Date(d).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' })
}
const notify = useToast()
function showToast(type, msg) {
  notify.add({ title: msg, color: type === 'success' ? 'emerald' : 'rose', icon: type === 'success' ? 'i-lucide-check' : 'i-lucide-circle-alert' })
}

onMounted(load)
</script>
