<template>
  <!-- Deep Sweep yang sedang berjalan di server (dari halaman Discovery maupun terminal) -->
  <div v-if="job" class="relative overflow-hidden rounded-2xl ring-1 p-5"
    :class="job.status === 'paused' ? 'bg-amber-500/[0.06] ring-amber-500/30' : 'bg-primary-500/[0.07] ring-primary-500/30'">
    <div class="absolute -right-10 -top-10 w-40 h-40 rounded-full blur-3xl opacity-30" :class="job.status === 'paused' ? 'bg-amber-500' : 'bg-primary-500'" />

    <div class="relative flex flex-col lg:flex-row lg:items-center gap-4">
      <div class="flex items-center gap-4 min-w-0 flex-1">
        <div class="relative w-12 h-12 rounded-xl flex items-center justify-center ring-1 shrink-0"
          :class="job.status === 'paused' ? 'bg-amber-500/15 ring-amber-500/30' : 'bg-primary-500/15 ring-primary-500/30'">
          <UIcon name="i-lucide-satellite-dish" class="w-6 h-6" :class="job.status === 'paused' ? 'text-amber-300' : 'text-primary-300'" />
          <span v-if="job.status === 'running'" class="absolute inset-0 rounded-xl ring-2 ring-primary-400/40 animate-ping" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <p class="text-[15px] font-semibold text-white">Deep Sweep {{ job.status === 'paused' ? 'dijeda' : 'sedang berjalan' }}</p>
            <UBadge :color="job.status === 'paused' ? 'amber' : 'primary'" variant="subtle" size="xs">{{ phase }}</UBadge>
          </div>
          <!-- Progres tahap saat ini -->
          <div class="flex items-center gap-3 mt-2">
            <div class="flex-1 h-1.5 rounded-full bg-slate-800/80 overflow-hidden">
              <div class="h-full rounded-full transition-all duration-700" :class="job.status === 'paused' ? 'bg-amber-400' : 'bg-primary-400'"
                :style="{ width: job.total ? `${Math.min(100, (job.processed / job.total) * 100)}%` : '6%' }" />
            </div>
            <span class="text-[12px] tabular-nums text-slate-400 shrink-0">{{ job.total ? `${job.processed} / ${job.total} situs tahap ini` : 'menyiapkan…' }}</span>
          </div>
        </div>
      </div>

      <!-- Angka ringkas -->
      <div class="flex items-center gap-5 lg:px-5 lg:border-x border-slate-700/50">
        <div v-for="s in stats" :key="s.label" class="text-center">
          <p class="text-xl font-semibold tabular-nums" :class="s.color">{{ s.value }}</p>
          <p class="text-[11px] uppercase tracking-wider text-slate-500">{{ s.label }}</p>
        </div>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <UButton v-if="job.status === 'running'" size="sm" color="gray" variant="soft" icon="i-lucide-pause" label="Jeda" :loading="busy" @click="act('pause')" />
        <UButton v-else size="sm" color="amber" variant="soft" icon="i-lucide-play" label="Lanjut" :loading="busy" @click="act('resume')" />
        <UButton size="sm" to="/discovery?mode=crawler" trailing-icon="i-lucide-arrow-right" label="Lihat log" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const { get, post } = useApi()
const job = ref<any>(null)
const live = ref<any>(null)
const found = ref(0)
const busy = ref(false)
let since = 0
let timer: any = null

const phase = computed(() => live.value?.phase || 'Menyiapkan')
function elapsed() {
  if (!job.value?.started_at) return '—'
  const s = Math.max(0, Math.round((Date.now() - new Date(job.value.started_at).getTime()) / 1000))
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60)
  return h ? `${h}j ${m}m` : `${m}m ${s % 60}d`
}
const now = ref(Date.now())
const stats = computed(() => (now.value, [
  { label: 'judol', value: found.value, color: 'text-emerald-400' },
  { label: 'dianalisis', value: analyzed.value, color: 'text-white' },
  { label: 'berjalan', value: elapsed(), color: 'text-slate-200' },
]))
const analyzed = ref(0)

async function poll() {
  now.value = Date.now()
  const active = (await get<any>('/sweep/active').catch(() => null))?.job
  if (!active) { job.value = null; since = 0; found.value = 0; analyzed.value = 0; return }
  if (active.id !== job.value?.id) { since = 0; found.value = 0; analyzed.value = 0 }
  const d = await get<any>(`/sweep/${active.id}`, { since }).catch(() => null)
  if (!d) return
  job.value = d.job
  live.value = d.live
  for (const e of d.events ?? []) {
    if (e.kind === 'found') found.value++
    if (/^[✓·✗]/.test(e.text)) analyzed.value++
    since = e.seq
  }
}
async function act(a: 'pause' | 'resume') {
  if (!job.value) return
  busy.value = true
  try { await post(`/sweep/${job.value.id}/${a}`, {}); await poll() } finally { busy.value = false }
}
onMounted(() => { poll(); timer = setInterval(poll, 3000) })
onUnmounted(() => clearInterval(timer))
</script>
