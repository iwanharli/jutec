<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Sistem" title="Settings"
      subtitle="Atur parameter engine secara langsung — perubahan berlaku tanpa perlu restart server."
      icon="i-lucide-settings">
      <template #actions>
        <UButton size="lg" color="amber" variant="soft" icon="i-lucide-trash-2" :loading="clearing"
          :label="`Bersihkan Cache Serper (${cacheCount})`" @click="clearCache" />
        <UButton size="lg" color="gray" variant="soft" icon="i-lucide-refresh-cw" label="Muat Ulang" @click="load" />
      </template>
    </PageHeader>

    <MaintenancePanel />

    <div v-if="loading" class="grid grid-cols-1 2xl:grid-cols-2 gap-6">
      <USkeleton v-for="i in 2" :key="i" class="h-64 w-full rounded-2xl" />
    </div>

    <div v-else-if="groupedSettings.length" class="grid grid-cols-1 2xl:grid-cols-2 gap-6 items-start">
      <UCard v-for="group in groupedSettings" :key="group.label" :ui="{ body: { padding: '' } }">
        <template #header>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl flex items-center justify-center ring-1" :class="group.iconBg">
              <UIcon :name="group.icon" class="w-5 h-5" :class="group.iconColor" />
            </div>
            <div>
              <h2 class="section-title">{{ group.label }}</h2>
              <p class="section-desc !mt-0">{{ group.settings.length }} parameter</p>
            </div>
          </div>
        </template>

        <div class="divide-y divide-slate-800/70">
          <div v-for="s in group.settings" :key="s.key" class="flex flex-col sm:flex-row sm:items-center gap-4 px-6 py-5">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <p class="text-base font-medium text-white">{{ formatKey(s.key) }}</p>
                <UBadge color="gray" variant="subtle" size="xs" class="mono">{{ s.key }}</UBadge>
              </div>
              <p class="text-sm text-slate-400 mt-1 leading-relaxed">{{ s.description || '—' }}</p>
              <p class="text-xs text-slate-600 mt-1.5">Diubah {{ formatDate(s.updated_at) }}</p>
            </div>

            <div class="shrink-0 flex items-center gap-3">
              <template v-if="isBool(s.value)">
                <span class="text-sm font-medium" :class="s.value === 'true' ? 'text-primary-300' : 'text-slate-500'">
                  {{ s.value === 'true' ? 'Aktif' : 'Nonaktif' }}
                </span>
                <UToggle :model-value="s.value === 'true'" size="lg" :loading="saving === s.key" @update:model-value="toggleBool(s)" />
              </template>
              <template v-else>
                <UInput v-model="s.value" :type="isNum(s.value) ? 'number' : 'text'" class="w-36" :ui="{ base: 'font-mono text-right' }"
                  @keyup.enter="save(s)" />
                <UButton icon="i-lucide-save" :loading="saving === s.key" label="Simpan" @click="save(s)" />
              </template>
            </div>
          </div>
        </div>
      </UCard>
    </div>

    <UCard v-else>
      <div class="empty-state">
        <div class="empty-icon"><UIcon name="i-lucide-sliders-horizontal" class="w-6 h-6" /></div>
        <p>Tidak ada setting.</p>
      </div>
    </UCard>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const { get, patch } = useApi()

const settings   = ref([])
const loading    = ref(true)
const saving     = ref(null)
const clearing   = ref(false)
const cacheCount = ref(0)

const groupDefs = [
  { prefix: 'serper',   label: 'Serper / Pencarian', icon: 'i-lucide-search',             iconBg: 'bg-blue-500/10 ring-blue-500/25',     iconColor: 'text-blue-400' },
  { prefix: 'telegram', label: 'Telegram',           icon: 'i-lucide-send',               iconBg: 'bg-sky-500/10 ring-sky-500/25',       iconColor: 'text-sky-400' },
  { prefix: 'crawler',  label: 'Crawler',            icon: 'i-lucide-bug',                iconBg: 'bg-amber-500/10 ring-amber-500/25',   iconColor: 'text-amber-400' },
  { prefix: 'sweep',    label: 'Deep Sweep',         icon: 'i-lucide-satellite-dish',     iconBg: 'bg-indigo-500/10 ring-indigo-500/25', iconColor: 'text-indigo-400' },
  { prefix: '',         label: 'Umum',               icon: 'i-lucide-sliders-horizontal', iconBg: 'bg-slate-700/40 ring-slate-600/40',   iconColor: 'text-slate-300' },
]

const groupedSettings = computed(() => {
  const result = []
  for (const def of groupDefs) {
    const items = settings.value.filter(s =>
      def.prefix ? s.key.startsWith(def.prefix) : !groupDefs.slice(0, -1).some(d => s.key.startsWith(d.prefix))
    )
    if (items.length) result.push({ ...def, settings: items })
  }
  return result
})

async function load() {
  loading.value = true
  try {
    const [s, c] = await Promise.all([
      get('/settings'),
      get('/discovery/cache/stats').catch(() => null),
    ])
    settings.value = s ?? []
    cacheCount.value = c?.total ?? 0
  } finally { loading.value = false }
}

async function clearCache() {
  clearing.value = true
  try {
    await fetch(useRuntimeConfig().public.apiBase + '/discovery/cache', { method: 'DELETE' })
    cacheCount.value = 0
    showToast('success', 'Cache Serper berhasil dihapus')
  } catch { showToast('error', 'Gagal menghapus cache') }
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

function formatKey(key) {
  return key.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}
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

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity .2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
