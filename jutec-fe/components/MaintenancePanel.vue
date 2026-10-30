<template>
  <!-- Pemeliharaan data: menyelaraskan data lama dengan engine terbaru tanpa terminal -->
  <section class="space-y-3">
    <div>
      <h2 class="section-title">Pemeliharaan Data</h2>
      <p class="section-desc">Selaraskan data lama dengan engine terbaru. Situs yang sudah diverifikasi manual tidak pernah diubah.</p>
    </div>

    <div class="grid grid-cols-1 2xl:grid-cols-2 gap-6 items-start">
      <!-- 1. Rekening palsu -->
      <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
        <template #header>
          <div class="flex items-center gap-3">
            <div class="card-icon bg-violet-500/10 ring-violet-500/25"><UIcon name="i-lucide-landmark" class="w-[18px] h-[18px] text-violet-400" /></div>
            <div class="flex-1 min-w-0">
              <h3 class="card-title">Bersihkan Rekening Palsu</h3>
              <p class="card-desc">Angka yang dulu salah terbaca sebagai rekening: timestamp, tanggal, nomor registrasi, nomor HP</p>
            </div>
          </div>
        </template>

        <div v-if="!fake" class="flex flex-col items-start gap-3">
          <p class="text-[13px] text-slate-400">Periksa dulu daftarnya — tidak ada yang dihapus sebelum kamu konfirmasi. Yang dihapus disalin ke tabel cadangan.</p>
          <UButton icon="i-lucide-search-check" label="Pratinjau" :loading="loadingFake" @click="loadFake" />
        </div>
        <template v-else>
          <EmptyState v-if="!fake.artifacts.length" compact tone="good" icon="i-lucide-check" title="Tidak ada rekening palsu"
            :desc="`${fake.checked} rekening tersimpan sudah lolos aturan terbaru.`" />
          <template v-else>
            <div class="flex items-center justify-between mb-2">
              <p class="text-[13px] text-slate-300"><b class="text-white">{{ fake.artifacts.length }}</b> dari {{ fake.checked }} rekening tersimpan bukan rekening</p>
              <UButton size="2xs" color="gray" variant="ghost" :label="picked.size === fake.artifacts.length ? 'Hapus pilihan' : 'Pilih semua'"
                @click="picked = new Set(picked.size === fake.artifacts.length ? [] : fake.artifacts.map((a: any) => a.id))" />
            </div>
            <ul class="max-h-72 overflow-y-auto divide-y divide-slate-800/70 rounded-lg ring-1 ring-slate-800">
              <li v-for="a in fake.artifacts" :key="a.id" class="flex items-center gap-3 px-3 py-2 text-[13px] cursor-pointer hover:bg-slate-800/30" @click="togglePick(a.id)">
                <UCheckbox :model-value="picked.has(a.id)" @click.stop @update:model-value="togglePick(a.id)" />
                <span class="mono text-slate-100 w-40 shrink-0">{{ a.value }}</span>
                <span class="text-slate-500 w-12 shrink-0">{{ a.name }}</span>
                <UBadge color="gray" variant="subtle" size="xs" class="shrink-0">{{ a.reason }}</UBadge>
                <span class="text-[12px] text-slate-500 truncate ml-auto">{{ a.sites.map(host).join(', ') }}</span>
              </li>
            </ul>
            <div class="flex items-center gap-2 mt-3">
              <UButton color="rose" icon="i-lucide-trash-2" :label="`Hapus ${picked.size} (dicadangkan)`" :disabled="!picked.size" :loading="cleaning" @click="clean" />
              <UButton color="gray" variant="ghost" label="Batal" @click="fake = null" />
            </div>
          </template>
        </template>
      </UCard>

      <!-- 2. Scan ulang -->
      <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
        <template #header>
          <div class="flex items-center gap-3">
            <div class="card-icon bg-primary-500/10 ring-primary-500/25"><UIcon name="i-lucide-refresh-ccw-dot" class="w-[18px] h-[18px] text-primary-400" /></div>
            <div class="flex-1 min-w-0">
              <h3 class="card-title">Scan Ulang dengan Engine Terbaru</h3>
              <p class="card-desc">Hitung ulang skor & vonis — false positive lama terkoreksi, situs yang gagal diakses tidak kehilangan vonisnya</p>
            </div>
          </div>
        </template>

        <!-- Sedang/sudah berjalan -->
        <div v-if="rescan?.job" class="space-y-3">
          <div class="flex items-center justify-between text-[13px]">
            <span class="text-slate-300">{{ scopeLabel(rescan.job.scope) }} · {{ rescan.running ? 'berjalan' : 'selesai' }}</span>
            <span class="tabular-nums text-slate-400">{{ rescan.done }} / {{ rescan.job.queued }}</span>
          </div>
          <div class="h-2 rounded-full bg-slate-800 overflow-hidden">
            <div class="h-full rounded-full transition-all duration-700" :class="rescan.running ? 'bg-primary-500' : 'bg-emerald-500'"
              :style="{ width: `${rescan.job.queued ? (rescan.done / rescan.job.queued) * 100 : 0}%` }" />
          </div>
          <div class="grid grid-cols-3 gap-2 text-center">
            <div class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 py-2">
              <p class="text-lg font-semibold tabular-nums text-white">{{ rescan.done }}</p>
              <p class="text-[11px] text-slate-500">selesai di-scan</p>
            </div>
            <div class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 py-2">
              <p class="text-lg font-semibold tabular-nums text-emerald-400">{{ rescan.corrected }}</p>
              <p class="text-[11px] text-slate-500">dari {{ rescan.job.was_judol }} judol dikoreksi jadi bukan judol</p>
            </div>
            <div class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 py-2">
              <p class="text-lg font-semibold tabular-nums" :class="rescan.failed ? 'text-amber-400' : 'text-slate-300'">{{ rescan.failed }}</p>
              <p class="text-[11px] text-slate-500">gagal diakses (vonis lama tetap)</p>
            </div>
          </div>
          <p class="text-[12px] text-slate-500">Dimulai {{ fmtTime(rescan.job.started_at) }} oleh {{ rescan.job.by }} · berjalan di worker, halaman ini boleh ditutup.</p>
          <UButton v-if="!rescan.running" size="sm" color="gray" variant="soft" icon="i-lucide-rotate-ccw" label="Scan ulang lagi" @click="rescan = null" />
        </div>

        <!-- Pilih cakupan -->
        <div v-else class="space-y-2">
          <button v-for="s in scopes" :key="s.value" type="button" @click="scope = s.value"
            class="w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-left ring-1 transition"
            :class="scope === s.value ? 'bg-primary-500/10 ring-primary-500/40' : 'ring-slate-800 hover:ring-slate-700'">
            <span class="w-4 h-4 rounded-full ring-2 flex items-center justify-center shrink-0" :class="scope === s.value ? 'ring-primary-400' : 'ring-slate-600'">
              <span v-if="scope === s.value" class="w-2 h-2 rounded-full bg-primary-400" />
            </span>
            <span class="flex-1 min-w-0">
              <span class="block text-sm text-slate-100">{{ s.label }}</span>
              <span class="block text-[12px] text-slate-500">{{ s.desc }}</span>
            </span>
            <span class="text-sm font-semibold tabular-nums text-slate-300">{{ counts[s.value] ?? '—' }}</span>
          </button>
          <div class="flex items-center gap-3 pt-2">
            <UButton icon="i-lucide-play" :label="`Scan ulang ${counts[scope] ?? ''} situs`" :loading="starting" :disabled="!counts[scope] || !summary?.worker" @click="startRescan" />
            <span class="text-[12px] text-slate-500">±{{ estimate }} · 5 situs bersamaan</span>
          </div>
          <p v-if="summary && !summary.worker" class="text-[12px] text-amber-400">Worker background (Redis) tidak aktif — scan ulang tidak bisa dijalankan.</p>
        </div>
      </UCard>
    </div>
  </section>
</template>

<script setup lang="ts">
const { get, post } = useApi()
const toast = useToast()

const summary = ref<any>(null)
const counts = computed(() => summary.value?.rescan_counts ?? {})
const fake = ref<any>(null)
const picked = ref(new Set<number>())
const loadingFake = ref(false)
const cleaning = ref(false)
const rescan = ref<any>(null)
const scope = ref('judol')
const starting = ref(false)
let timer: any = null

const scopes = [
  { value: 'judol', label: 'Situs judol yang belum diverifikasi', desc: 'Disarankan — memperbaiki false positive di daftar judol' },
  { value: 'suspect', label: 'Judol + prioritas High/Medium', desc: 'Termasuk situs mencurigakan yang belum dinilai judol' },
  { value: 'all', label: 'Semua situs yang belum diverifikasi', desc: 'Paling lengkap, paling lama' },
]
const scopeLabel = (v: string) => scopes.find(s => s.value === v)?.label ?? v
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/$/, '')
const fmtTime = (d: string) => new Date(d).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
// ±40 detik per situs, 5 bersamaan
const estimate = computed(() => {
  const m = Math.ceil(((counts.value[scope.value] ?? 0) * 40) / 5 / 60)
  return m < 60 ? `${m} menit` : `${Math.floor(m / 60)} jam ${m % 60} menit`
})

function togglePick(id: number) {
  const s = new Set(picked.value)
  s.has(id) ? s.delete(id) : s.add(id)
  picked.value = s
}
async function loadSummary() { summary.value = await get<any>('/maintenance/summary').catch(() => null) }
async function loadFake() {
  loadingFake.value = true
  try {
    fake.value = await get<any>('/maintenance/artifacts/fake')
    picked.value = new Set(fake.value.artifacts.map((a: any) => a.id))
  } catch (e: any) { toast.add({ title: 'Gagal memuat pratinjau', description: e.message, color: 'rose' }) }
  finally { loadingFake.value = false }
}
async function clean() {
  if (!confirm(`Hapus ${picked.value.size} rekening palsu? Datanya disalin dulu ke tabel cadangan (artifact_cleanup_backup).`)) return
  cleaning.value = true
  try {
    const r = await post<any>('/maintenance/artifacts/clean', { ids: [...picked.value] })
    toast.add({ title: `${r.removed} rekening palsu dihapus`, description: 'Cadangan tersimpan di tabel artifact_cleanup_backup.', color: 'emerald', icon: 'i-lucide-check' })
    await loadFake()
  } catch (e: any) { toast.add({ title: 'Gagal menghapus', description: e.message, color: 'rose' }) }
  finally { cleaning.value = false }
}
async function loadRescan() {
  rescan.value = await get<any>('/maintenance/rescan').catch(() => null)
  if (!rescan.value?.job) rescan.value = null
}
async function startRescan() {
  if (!confirm(`Scan ulang ${counts.value[scope.value]} situs dengan engine terbaru? Berjalan di latar belakang (±${estimate.value}).`)) return
  starting.value = true
  try {
    const r = await post<any>('/maintenance/rescan', { scope: scope.value })
    toast.add({ title: `${r.queued} situs diantrekan`, icon: 'i-lucide-refresh-ccw-dot', color: 'primary' })
    await loadRescan()
  } catch (e: any) { toast.add({ title: 'Tidak bisa memulai', description: e.message, color: 'rose' }) }
  finally { starting.value = false }
}

onMounted(() => { loadSummary(); loadRescan(); timer = setInterval(() => { if (rescan.value?.running) loadRescan() }, 5000) })
onUnmounted(() => clearInterval(timer))
</script>
