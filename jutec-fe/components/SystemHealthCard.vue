<template>
  <!-- Kesehatan sistem: apa yang sedang & akan dikerjakan mesin secara otomatis -->
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { base: 'flex flex-col gap-5' } }">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon" :class="healthy ? 'bg-emerald-500/10 ring-emerald-500/25' : 'bg-rose-500/10 ring-rose-500/25'">
          <UIcon :name="healthy ? 'i-lucide-heart-pulse' : 'i-lucide-triangle-alert'" class="w-[18px] h-[18px]" :class="healthy ? 'text-emerald-400' : 'text-rose-400'" />
        </div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Kesehatan Sistem</h2>
          <p class="card-desc">{{ sys.redis ? `${sys.servers} pemroses aktif · ${busy ? `${busy} tugas berjalan` : 'menganggur'}` : 'Antrean tugas tidak terhubung' }}</p>
        </div>
        <UBadge :color="healthy ? 'emerald' : 'rose'" variant="subtle">{{ healthy ? 'Sehat' : 'Bermasalah' }}</UBadge>
      </div>
    </template>

    <!-- Ruang disk: bukti forensik adalah pemakai terbesar -->
    <div v-if="disk" class="rounded-xl ring-1 px-3 py-2.5" :class="disk.low ? 'bg-rose-500/10 ring-rose-500/30' : 'bg-slate-950/50 ring-slate-800'">
      <div class="flex items-center gap-2 text-[12px]" :class="disk.low ? 'text-rose-300' : 'text-slate-400'">
        <UIcon :name="disk.low ? 'i-lucide-hard-drive-download' : 'i-lucide-hard-drive'" class="w-3.5 h-3.5" />
        <span class="flex-1">{{ disk.low ? 'Disk hampir penuh — scan & bukti forensik bisa gagal' : 'Ruang disk' }}</span>
        <span class="tabular-nums">{{ disk.free_gb.toFixed(1) }} GB bebas</span>
      </div>
      <div class="h-1.5 rounded-full bg-slate-800 overflow-hidden mt-2">
        <div class="h-full rounded-full" :class="disk.low ? 'bg-rose-500' : 'bg-primary-500'" :style="{ width: `${Math.min(100, 100 - (disk.free_gb / disk.total_gb) * 100)}%` }" />
      </div>
      <p class="text-[11px] text-slate-500 mt-1.5">Bukti forensik: {{ disk.evidence_mb >= 1024 ? `${(disk.evidence_mb / 1024).toFixed(1)} GB` : `${Math.round(disk.evidence_mb)} MB` }}</p>
    </div>

    <!-- Deep Sweep harian -->
    <div v-if="sys.daily_sweep" class="flex items-center gap-2.5 rounded-xl bg-slate-950/50 ring-1 ring-slate-800 px-3 py-2.5 text-[13px]">
      <UIcon name="i-lucide-moon" class="w-4 h-4 shrink-0" :class="sys.daily_sweep.enabled ? 'text-indigo-400' : 'text-slate-600'" />
      <span class="flex-1 text-slate-300">
        Deep Sweep harian
        <span class="text-slate-500">· {{ sys.daily_sweep.enabled ? `pukul ${String(sys.daily_sweep.hour).padStart(2, '0')}.00, ${sys.daily_sweep.keywords} keyword` : 'nonaktif' }}</span>
      </span>
      <span class="text-[11px] text-slate-500">{{ sys.daily_sweep.last ? `terakhir ${sys.daily_sweep.last}` : 'belum pernah' }}</span>
    </div>

    <!-- Antrean: ubin per jenis pekerjaan -->
    <div>
      <p class="label">Antrean pekerjaan</p>
      <EmptyState v-if="!queues.length" compact :tone="sys.redis ? 'good' : 'neutral'" :icon="sys.redis ? 'i-lucide-check' : 'i-lucide-unplug'"
        :title="sys.redis ? 'Tidak ada antrean' : 'Pemroses nonaktif'" :desc="sys.redis ? '' : 'Scan otomatis & pengayaan tidak berjalan.'" />
      <div v-else class="grid grid-cols-2 gap-2">
        <div v-for="q in queues" :key="q.name" class="rounded-xl bg-slate-950/50 ring-1 ring-slate-800 px-3 py-2.5">
          <div class="flex items-center gap-1.5 text-[12px] text-slate-400">
            <UIcon :name="q.icon" class="w-3.5 h-3.5" />{{ q.label }}
            <span v-if="q.active" class="ml-auto w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse" />
          </div>
          <p class="mt-1 flex items-baseline gap-1">
            <span class="text-xl font-semibold tabular-nums" :class="q.waiting ? 'text-white' : 'text-slate-500'">{{ q.waiting }}</span>
            <span class="text-[11px] text-slate-500">antre<template v-if="q.active"> · {{ q.active }} jalan</template></span>
          </p>
          <p class="text-[11px] text-slate-500 tabular-nums">{{ q.processed_today }} selesai<span v-if="q.failed_today" class="text-rose-400"> · {{ q.failed_today }} gagal</span> hari ini</p>
        </div>
      </div>
    </div>

    <!-- Jadwal otomatis -->
    <div v-if="schedule.length">
      <p class="label">Berjalan otomatis</p>
      <ul class="space-y-1.5">
        <li v-for="j in schedule" :key="j.type" class="flex items-center gap-2.5 text-[13px]">
          <UIcon :name="j.icon" class="w-4 h-4 text-slate-500 shrink-0" />
          <span class="text-slate-300 flex-1 truncate">{{ j.label }}</span>
          <span class="text-[11px] text-slate-600 shrink-0">{{ j.every }}</span>
          <span class="text-[11px] tabular-nums text-slate-400 shrink-0 w-20 text-right">{{ j.next }}</span>
        </li>
      </ul>
    </div>

    <!-- Kunci API -->
    <div>
      <p class="label">Sumber data</p>
      <div class="grid grid-cols-2 gap-1.5">
        <UTooltip v-for="k in sys.api_keys ?? []" :key="k.name" :text="k.set ? 'Kunci API terpasang' : k.free_tier ? 'Berjalan dengan kuota gratis' : 'Kunci belum diisi — sumber ini nonaktif'">
          <span class="w-full inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-[12px] ring-1"
            :class="k.set ? 'text-slate-300 ring-slate-800' : k.free_tier ? 'text-slate-300 ring-slate-800' : 'text-amber-300 ring-amber-500/30 bg-amber-500/5'">
            <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="k.set ? 'bg-emerald-500' : k.free_tier ? 'bg-sky-400' : 'bg-amber-500'" />
            <span class="truncate">{{ k.name.replace(/ \(.*\)/, '') }}</span>
            <span v-if="!k.set && k.free_tier" class="ml-auto text-[10px] text-sky-400/80 shrink-0">gratis</span>
          </span>
        </UTooltip>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const props = defineProps<{ sys: any }>()

const QUEUES: Record<string, { label: string; icon: string }> = {
  default: { label: 'Scan situs', icon: 'i-lucide-scan-line' },
  ocr: { label: 'OCR & APK', icon: 'i-lucide-scan-text' },
  enrichment: { label: 'Reputasi & identitas', icon: 'i-lucide-shield-question' },
  low: { label: 'Otomasi latar', icon: 'i-lucide-bot' },
}
const JOBS: Record<string, { label: string; icon: string }> = {
  'rescan:stale': { label: 'Scan ulang situs lama', icon: 'i-lucide-refresh-ccw' },
  'auto:reach': { label: 'Cek akses & blokir ISP', icon: 'i-lucide-shield-ban' },
  'auto:evidence': { label: 'Ambil bukti forensik', icon: 'i-lucide-camera' },
  'auto:cluster': { label: 'Bentuk klaster operator', icon: 'i-lucide-network' },
  'auto:origin': { label: 'Cari IP asli', icon: 'i-lucide-server' },
  'auto:wayback': { label: 'Arsip Wayback', icon: 'i-lucide-archive' },
  'auto:reverse': { label: 'Domain lain se-ID analytics', icon: 'i-lucide-radar' },
  'auto:crypto': { label: 'Jejak dana USDT', icon: 'i-lucide-bitcoin' },
  'auto:trustpositif': { label: 'Cek daftar blokir Komdigi', icon: 'i-lucide-shield-ban' },
  'auto:commoncrawl': { label: 'Cari situs susupan (arsip web)', icon: 'i-lucide-archive' },
  'auto:takeover': { label: 'Cek riwayat domain (domain bekas)', icon: 'i-lucide-history' },
  'auto:evidence-retention': { label: 'Kebijakan simpan bukti', icon: 'i-lucide-archive-x' },
  'mirror:watch': { label: 'Cari mirror brand', icon: 'i-lucide-copy-plus' },
  'telegram:resolve': { label: 'Cek akun Telegram', icon: 'i-lucide-send' },
  'telegram:scrape': { label: 'Baca pesan kanal Telegram', icon: 'i-lucide-messages-square' },
  'leak:vfl-pending': { label: 'Cek identitas nomor HP', icon: 'i-lucide-fingerprint' },
}

const queues = computed(() => (props.sys.queues ?? []).map((q: any) => ({
  ...q, ...(QUEUES[q.name] ?? { label: q.name, icon: 'i-lucide-list' }), waiting: q.pending + q.scheduled + q.retry,
})))
const busy = computed(() => queues.value.reduce((n: number, q: any) => n + q.active, 0))
const healthy = computed(() => !!props.sys.redis && props.sys.servers > 0 && !queues.value.some((q: any) => q.failed_today > 10) && !props.sys.disk?.low)
const disk = computed(() => props.sys?.disk)

function every(spec: string) {
  const m = /@every\s+(\d+)([hm])/.exec(spec)
  return m ? `tiap ${m[1]} ${m[2] === 'h' ? 'jam' : 'mnt'}` : spec
}
function until(d: string) {
  const m = Math.round((new Date(d).getTime() - Date.now()) / 60000)
  return m <= 0 ? 'sebentar lagi' : m < 60 ? `${m} mnt lagi` : `${Math.floor(m / 60)} j ${m % 60} m lagi`
}
const schedule = computed(() => (props.sys.scheduler?.entries ?? [])
  .map((e: any) => ({ type: e.type, ...(JOBS[e.type] ?? { label: e.type, icon: 'i-lucide-clock' }), every: every(e.spec), next: until(e.next_at), at: new Date(e.next_at).getTime() }))
  .sort((a: any, b: any) => a.at - b.at))
</script>

<style scoped>
.label { @apply text-[11px] font-semibold uppercase tracking-wider text-slate-500 mb-2; }
</style>
