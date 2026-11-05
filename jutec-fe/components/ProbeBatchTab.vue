<template>
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-5' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-layers" class="w-[18px] h-[18px] text-rose-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Uji Massal</h2>
          <p class="card-desc">Tiap identitas mendaftar sekali di semua situs judol valid — form bercaptcha/OTP dilewati, tiap situs tidak diuji ulang oleh identitas yang sama</p>
        </div>
      </div>
    </template>

    <!-- Peringatan kewenangan -->
    <div class="rounded-xl bg-amber-500/10 ring-1 ring-amber-500/30 p-4 flex items-start gap-3">
      <UIcon name="i-lucide-shield-alert" class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
      <p class="text-sm text-amber-100/90 leading-relaxed">
        Uji massal mengirim identitas dummy ke banyak situs pelaku sekaligus — nomor HP & rekening uji akan tersebar ke semua operator itu.
        Pastikan ada dasar kewenangan (surat perintah / SOP unit) dan gunakan SIM & rekening khusus investigasi. Semua uji tercatat di log audit.
      </p>
    </div>

    <!-- Pengaturan batch -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-5">
      <UFormGroup label="Identitas dummy">
        <USelect v-model="profileId" :options="profileOpts" :disabled="running" />
      </UFormGroup>
      <UFormGroup label="Jeda antar-situs (detik)" help="Minimal 10 — makin pelan makin aman">
        <UInput v-model.number="delay" type="number" min="10" max="600" :disabled="running" icon="i-lucide-timer" />
      </UFormGroup>
    </div>

    <div class="flex flex-wrap items-center gap-3 mt-5 pt-4 border-t border-slate-800">
      <div class="flex-1 min-w-0 text-sm text-slate-400">
        <template v-if="preview">
          <b class="text-slate-100 tabular-nums">{{ preview.total }}</b> form pendaftaran bisa diujicoba identitas ini
          <span class="text-slate-500">· perkiraan ~{{ estMinutes }} menit</span>
        </template>
        <span v-else class="text-slate-500">Menghitung situs yang belum diuji identitas ini…</span>
      </div>
      <UButton v-if="!running" color="rose" icon="i-lucide-play" :loading="starting" :disabled="!profiles.length || !preview?.total"
        :label="profiles.length ? 'Jalankan uji massal' : 'Belum ada data dummy'" @click="start" />
      <UButton v-else color="gray" variant="soft" icon="i-lucide-square" label="Hentikan" @click="stop" />
    </div>

    <!-- Progres batch berjalan / terakhir -->
    <div v-if="batch" class="mt-5 rounded-xl bg-slate-950/40 ring-1 ring-slate-800 p-4">
      <div class="flex items-center gap-2">
        <UBadge :color="batch.status === 'running' ? 'sky' : batch.status === 'stopped' ? 'amber' : 'emerald'" variant="subtle">
          {{ batch.status === 'running' ? 'Berjalan' : batch.status === 'stopped' ? 'Dihentikan' : 'Selesai' }}
        </UBadge>
        <span class="text-sm text-slate-400">{{ batch.done }} / {{ batch.total }} diproses</span>
        <span class="ml-auto text-xs text-slate-500">{{ fmt(batch.created_at) }}<template v-if="batch.created_by"> · {{ batch.created_by }}</template></span>
      </div>
      <div class="h-2 rounded-full bg-slate-800 overflow-hidden mt-3">
        <div class="h-full rounded-full bg-rose-500 transition-all" :style="{ width: `${batch.total ? (batch.done / batch.total) * 100 : 0}%` }" />
      </div>
      <div class="flex flex-wrap gap-x-5 gap-y-1 mt-3 text-sm">
        <span class="text-emerald-300"><b class="tabular-nums">{{ batch.accepted }}</b> akun diterima</span>
        <span class="text-slate-300"><b class="tabular-nums">{{ batch.submitted }}</b> terkirim</span>
        <span class="text-sky-300"><b class="tabular-nums">{{ batch.otp ?? 0 }}</b> perlu OTP</span>
        <span class="text-slate-500"><b class="tabular-nums">{{ batch.skipped }}</b> dilewati (captcha)</span>
        <span class="text-rose-400"><b class="tabular-nums">{{ batch.failed }}</b> gagal</span>
        <NuxtLink to="/form-daftar?tab=accounts" class="ml-auto text-primary-400 hover:underline">Lihat akun terdaftar →</NuxtLink>
      </div>
    </div>

    <!-- Situs yang butuh captcha/OTP — dikerjakan manual lewat Mode Pantau -->
    <div v-if="preview?.captcha?.length" class="mt-5 rounded-xl bg-sky-500/5 ring-1 ring-sky-500/25 p-4">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-shield-question" class="w-4 h-4 text-sky-400" />
        <p class="text-sm font-medium text-sky-100 flex-1">{{ preview.captcha.length }} situs butuh captcha/OTP — dilewati uji otomatis</p>
      </div>
      <p class="text-xs text-sky-200/70 mt-1">Kerjakan manual: buka situs → kartu Uji Form → <b>Mode Pantau</b>, selesaikan captcha sendiri.</p>
      <div class="mt-3 divide-y divide-slate-800/60 max-h-72 overflow-y-auto">
        <div v-for="t in preview.captcha" :key="t.site_id" class="flex items-center gap-2 py-2">
          <span class="mono text-sm text-slate-200 flex-1 truncate">{{ host(t.page_url || t.url) }}</span>
          <UButton size="2xs" color="sky" variant="soft" icon="i-lucide-eye" :to="`/site/${t.site_id}`" label="Mode Pantau" />
        </div>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const { get, post } = useApi()
const { confirm } = useConfirm()
const toast = useToast()

const profiles = ref<any[]>([])
const profileId = ref<number>(0)
const delay = ref(25)
const preview = ref<any>(null)
const batch = ref<any>(null)
const starting = ref(false)

const profileOpts = computed(() => profiles.value.map(p => ({ value: p.id, label: p.label + (p.is_default ? ' (bawaan)' : '') })))
const running = computed(() => batch.value?.status === 'running')
const estMinutes = computed(() => Math.max(1, Math.round(((preview.value?.total ?? 0) * (delay.value + 35)) / 60)))

async function loadPreview() {
  try { preview.value = await get('/form-probe-batch/preview', { profile_id: profileId.value }) } catch {}
}
async function loadBatch() {
  try { batch.value = (await get<any>('/form-probe-batch/latest')).batch } catch {}
}
let timer: any
watch(running, r => { clearInterval(timer); if (r) timer = setInterval(loadBatch, 4000) })
watch(profileId, loadPreview)

async function start() {
  const n = preview.value?.total ?? 0
  const ok = await confirm({
    title: `Jalankan uji massal ke ${n} situs?`, tone: 'warning', confirmLabel: `Jalankan (${n} situs)`, icon: 'i-lucide-layers',
    description: `Identitas “${profiles.value.find(p => p.id === profileId.value)?.label ?? 'bawaan'}” didaftarkan ke ${n} situs judol, 1 situs tiap ${delay.value} detik (~${estMinutes.value} menit).`,
    details: ['Form bercaptcha/OTP dilewati otomatis', 'Nomor HP & rekening dummy tersebar ke semua situs itu', 'Bisa dihentikan kapan saja', 'Tercatat di log audit'],
  })
  if (!ok) return
  starting.value = true
  try {
    const r = await post<any>('/form-probe-batch', { profile_id: profileId.value, delay_sec: delay.value })
    toast.add({ title: `Uji massal dimulai — ${r.queued} situs`, color: 'emerald', icon: 'i-lucide-layers' })
    await loadBatch()
  } catch (e: any) {
    toast.add({ title: e?.data?.error || 'Gagal memulai', color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { starting.value = false }
}
async function stop() {
  if (!batch.value) return
  await post(`/form-probe-batch/${batch.value.id}/stop`, {})
  toast.add({ title: 'Uji massal dihentikan — sisa situs dilewati', color: 'amber', icon: 'i-lucide-square' })
  await loadBatch()
}

const host = (u: string) => (u || '').replace(/^https?:\/\//, '').replace(/\/.*$/, '')
const fmt = (d: string) => new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
onMounted(async () => {
  try {
    profiles.value = (await get<any>('/probe-profiles')).results ?? []
    profileId.value = profiles.value.find(p => p.is_default)?.id ?? profiles.value[0]?.id ?? 0
  } catch {}
  await Promise.all([loadPreview(), loadBatch()])
})
onBeforeUnmount(() => clearInterval(timer))
</script>
