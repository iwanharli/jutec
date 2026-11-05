<template>
  <!-- Uji Form Terkendali: daftar → login → deposit dengan data dummy milik unit (manual, atas perintah analis) -->
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-flask-conical" class="w-[18px] h-[18px] text-rose-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Uji Form Terkendali</h2>
          <p class="card-desc">Daftar & login dengan data dummy unit, lalu buka halaman deposit — merekam respons server & rekening penampung</p>
        </div>
        <USelect v-if="profiles.length > 1" v-model="profileId" :options="profiles.map(p => ({ value: p.id, label: p.label + (p.is_default ? ' (bawaan)' : '') }))" class="w-52" />
        <UButton v-if="!profiles.length" color="gray" variant="soft" icon="i-lucide-contact" to="/form-daftar?tab=dummy" label="Atur data dummy" />
        <UButton v-if="profiles.length" color="gray" variant="soft" icon="i-lucide-eye" :disabled="running || !!attended" label="Mode Pantau" @click="openAttended" />
        <UButton v-if="profiles.length" color="rose" variant="soft" icon="i-lucide-play" :loading="starting" :disabled="running || !!attended" label="Uji otomatis" @click="start" />
      </div>
    </template>

    <!-- Panel Mode Pantau: jendela Chrome terbuka di layar -->
    <div v-if="attended" class="m-5 rounded-xl bg-sky-500/10 ring-1 ring-sky-500/30 p-4">
      <div class="flex items-start gap-3">
        <UIcon name="i-lucide-eye" class="w-5 h-5 text-sky-400 shrink-0 mt-0.5" />
        <div class="flex-1 min-w-0">
          <p class="text-sm font-medium text-sky-100">Jendela Chrome terbuka di layar Anda — username <span class="mono">{{ attended.username }}</span></p>
          <p class="text-sm text-sky-100/80 mt-1 leading-relaxed">
            Data dummy sudah terisi. Selesaikan captcha/OTP bila ada, tekan tombol DAFTAR di jendela itu, dan amati hasilnya.
            Setelah selesai (atau sudah login & melihat halaman deposit), klik <b>Simpan hasil</b> — Jutec merekam keadaan jendela saat itu.
          </p>
          <p v-if="attended.filled && Object.keys(attended.filled).length" class="text-xs text-sky-200/70 mt-1.5">Terisi otomatis: {{ Object.keys(attended.filled).join(', ') }}</p>
          <div class="flex flex-wrap gap-2 mt-3">
            <UButton color="sky" icon="i-lucide-save" :loading="capturing" label="Simpan hasil" @click="captureAttended" />
            <UButton color="gray" variant="soft" icon="i-lucide-x" :disabled="capturing" label="Batal (tutup jendela)" @click="cancelAttended" />
          </div>
        </div>
      </div>
    </div>

    <div v-if="!probes.length" class="empty-state !py-10">
      <div class="empty-icon"><UIcon name="i-lucide-flask-conical" class="w-6 h-6" /></div>
      <p class="text-sm">Belum pernah diuji.</p>
      <p class="text-xs text-slate-500 mt-1 max-w-md">Form bercaptcha/OTP otomatis dilewati. Data dummy diatur di Form Daftar & Login → Data Dummy.</p>
    </div>
    <div v-else class="divide-y divide-slate-800/60">
      <div v-for="p in probes" :key="p.id" class="px-5 py-4">
        <div class="flex flex-wrap items-center gap-2">
          <UBadge :color="statusMeta(p).color" variant="subtle">{{ statusMeta(p).label }}</UBadge>
          <UBadge v-if="p.login_status" :color="p.login_status === 'logged_in' ? 'emerald' : 'gray'" variant="subtle">
            {{ p.login_status === 'logged_in' ? 'Berhasil login' : p.login_status === 'skipped' ? 'Login dilewati' : 'Login gagal' }}
          </UBadge>
          <span class="mono text-xs text-slate-400">{{ p.username }}</span>
          <span class="ml-auto text-xs text-slate-500">{{ fmt(p.created_at) }}<template v-if="p.requested_by"> · {{ p.requested_by }}</template></span>
        </div>
        <p v-if="p.reason || p.result?.reason" class="text-sm text-slate-400 mt-1.5">{{ p.reason || p.result?.reason }}</p>
        <p v-if="p.result?.message" class="text-sm mt-1.5"><span class="text-slate-500">Pesan situs:</span> <span class="text-slate-200">“{{ p.result.message }}”</span></p>
        <p v-if="p.result?.login?.message" class="text-sm mt-1"><span class="text-slate-500">Pesan login:</span> <span class="text-slate-200">“{{ p.result.login.message }}”</span></p>

        <!-- Rekening penampung: hasil paling berharga -->
        <div v-if="moneyOf(p).length" class="mt-3 rounded-xl bg-amber-500/5 ring-1 ring-amber-500/25 p-3">
          <p class="text-xs uppercase tracking-wider text-amber-300/80 mb-1.5">Rekening / kontak setelah daftar & login</p>
          <div class="flex flex-wrap gap-1.5">
            <span v-for="a in moneyOf(p)" :key="a.type + a.value" class="rounded-md px-2 py-1 text-sm ring-1 ring-amber-500/30 text-amber-100">
              <span class="text-amber-300/80 text-xs mr-1">{{ a.name || a.type }}</span><span class="mono">{{ a.value }}</span>
            </span>
          </div>
        </div>

        <div class="flex flex-wrap gap-x-4 gap-y-1 mt-3 text-xs text-slate-500">
          <span v-if="Object.keys(p.result?.filled ?? {}).length">Diisi: {{ Object.keys(p.result.filled).join(', ') }}</span>
          <span v-if="p.result?.missing?.length" class="text-amber-400">Tanpa data: {{ p.result.missing.join(', ') }}</span>
          <span v-if="p.result?.platform?.length">Platform: <span class="text-slate-300">{{ p.result.platform.join(' · ') }}</span></span>
          <span v-if="p.result?.login?.deposit_urls?.length">Halaman deposit: {{ p.result.login.deposit_urls.length }}</span>
        </div>

        <div v-if="filesOf(p).length" class="flex flex-wrap gap-1.5 mt-3">
          <UButton v-for="f in filesOf(p)" :key="f.name" size="2xs" color="gray" variant="soft" :icon="f.name.endsWith('.png') ? 'i-lucide-image' : 'i-lucide-file-code'"
            :to="`${apiBase}/form-probes/${p.id}/files/${f.name}`" target="_blank" :label="f.name" />
        </div>

        <!-- Pertukaran HTTP: request yang dikirim & respons server -->
        <details v-if="exchangesOf(p).length" class="mt-3 group">
          <summary class="cursor-pointer text-xs text-slate-400 hover:text-slate-200 select-none">Respons server ({{ exchangesOf(p).length }} permintaan)</summary>
          <div class="mt-2 space-y-2">
            <div v-for="(x, i) in exchangesOf(p)" :key="i" class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 p-2.5">
              <div class="flex items-center gap-2 text-xs">
                <span class="mono px-1 rounded bg-slate-800 text-slate-300">{{ x.method }}</span>
                <span class="mono tabular-nums" :class="x.status >= 400 ? 'text-rose-400' : 'text-emerald-400'">{{ x.status || '—' }}</span>
                <span class="mono text-slate-400 truncate">{{ x.url }}</span>
              </div>
              <p v-if="x.request" class="mono text-[11px] text-slate-500 mt-1.5 break-all"><span class="text-slate-400">dikirim:</span> {{ x.request }}</p>
              <p v-if="x.headers && Object.keys(x.headers).length" class="mono text-[11px] text-slate-500 mt-1 break-all">
                <span v-for="(v, k) in x.headers" :key="k" class="mr-3"><span class="text-slate-400">{{ k }}:</span> {{ v }}</span>
              </p>
              <pre v-if="x.body" class="mono text-[11px] text-slate-400 mt-1.5 max-h-40 overflow-auto whitespace-pre-wrap break-all">{{ x.body }}</pre>
            </div>
          </div>
        </details>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const props = defineProps<{ siteId: number; siteUrl: string }>()
const { get, post, apiBase } = useApi()
const { confirm } = useConfirm()
const toast = useToast()
const probes = ref<any[]>([])
const profiles = ref<any[]>([])
const profileId = ref<number>(0)
const starting = ref(false)
const attended = ref<any>(null)
const capturing = ref(false)

async function openAttended() {
  const label = profiles.value.find(p => p.id === profileId.value)?.label ?? 'bawaan'
  const ok = await confirm({
    title: 'Buka Mode Pantau?', tone: 'warning', confirmLabel: 'Buka jendela', icon: 'i-lucide-eye',
    description: `Jendela Chrome akan terbuka di layar Anda, membuka halaman daftar ${props.siteUrl.replace(/^https?:\/\//, '')} dan mengisi data dummy “${label}”. Anda yang menyelesaikan captcha & menekan daftar.`,
    details: ['Captcha/OTP Anda selesaikan sendiri di jendela itu', 'Tidak ada submit otomatis — Anda yang menekan daftar', 'Klik “Simpan hasil” setelah selesai agar Jutec merekam bukti'],
  })
  if (!ok) return
  try {
    attended.value = await post<any>(`/sites/${props.siteId}/form-probe/attended`, { profile_id: profileId.value })
    toast.add({ title: 'Jendela Chrome dibuka', color: 'sky', icon: 'i-lucide-eye' })
  } catch (e: any) {
    toast.add({ title: e?.data?.error || 'Gagal membuka jendela', color: 'rose', icon: 'i-lucide-circle-alert' })
  }
}
async function captureAttended() {
  if (!attended.value) return
  capturing.value = true
  try {
    const r = await post<any>(`/form-probes/${attended.value.probe_id}/attended/capture`, {})
    toast.add({ title: `Hasil tersimpan — ${r.outcome}`, color: 'emerald', icon: 'i-lucide-check' })
    attended.value = null
    await load()
  } catch (e: any) {
    toast.add({ title: e?.data?.error || 'Gagal menyimpan', color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { capturing.value = false }
}
async function cancelAttended() {
  if (!attended.value) return
  try { await post(`/form-probes/${attended.value.probe_id}/attended/cancel`, {}) } catch {}
  attended.value = null
}
const running = computed(() => probes.value.some(p => p.status === 'pending'))

async function load() {
  try { probes.value = (await get<any>(`/sites/${props.siteId}/form-probes`)).probes ?? [] } catch {}
}
let timer: any
watch(running, r => { clearInterval(timer); if (r) timer = setInterval(load, 5000) })
onMounted(async () => {
  load()
  try {
    profiles.value = (await get<any>('/probe-profiles')).results ?? []
    profileId.value = profiles.value.find(p => p.is_default)?.id ?? profiles.value[0]?.id ?? 0
  } catch {}
})
onBeforeUnmount(() => clearInterval(timer))

async function start(force = false) {
  const label = profiles.value.find(p => p.id === profileId.value)?.label ?? 'bawaan'
  const ok = await confirm({
    title: force ? 'Uji ulang (paksa) situs ini?' : 'Jalankan uji daftar & login?', tone: 'warning',
    confirmLabel: force ? 'Uji ulang' : 'Jalankan uji', icon: 'i-lucide-flask-conical',
    description: `Identitas “${label}” akan dikirim ke ${props.siteUrl.replace(/^https?:\/\//, '')}. Ini interaksi aktif dengan situs pelaku dan tercatat di log audit.`,
    details: force
      ? ['Identitas ini sudah pernah berhasil mendaftar di sini — akun baru akan dibuat lagi', 'Mendaftar berulang dengan data sama bisa memicu blokir', 'Form bercaptcha/OTP dilewati otomatis']
      : ['Form bercaptcha/OTP dilewati otomatis', 'Setelah daftar, sistem login dengan akun yang sama lalu membuka halaman deposit',
         'Nomor HP & rekening dummy akan terlihat oleh pengelola situs — pastikan memakai data khusus investigasi', 'Tidak ada deposit yang dilakukan'],
  })
  if (!ok) return
  starting.value = true
  try {
    const r = await post<any>(`/sites/${props.siteId}/form-probe`, { profile_id: profileId.value, force })
    toast.add({ title: `Uji dimulai — username ${r.username}`, color: 'emerald', icon: 'i-lucide-flask-conical' })
    await load()
  } catch (e: any) {
    // Sudah pernah berhasil → tawarkan uji ulang paksa
    if (e?.data?.can_force) {
      const f = await confirm({ title: 'Sudah pernah berhasil di backend ini', tone: 'warning', confirmLabel: 'Uji ulang (paksa)',
        description: e.data.error })
      if (f) return start(true)
    } else {
      toast.add({ title: e?.data?.error || e?.message || 'Gagal memulai uji', color: 'rose', icon: 'i-lucide-circle-alert' })
    }
  } finally { starting.value = false }
}

const statusMeta = (p: any) => p.status === 'pending' ? { label: 'Berjalan…', color: 'sky' }
  : p.status === 'skipped' ? { label: 'Dilewati', color: 'gray' }
  : p.status === 'error' ? { label: 'Gagal', color: 'rose' }
  : p.outcome === 'accepted' ? { label: 'Pendaftaran diterima', color: 'emerald' }
  : p.outcome === 'otp_required' ? { label: 'Perlu verifikasi OTP', color: 'sky' }
  : p.outcome === 'rejected' ? { label: 'Pendaftaran ditolak', color: 'amber' }
  : { label: 'Terkirim · respons tak jelas', color: 'gray' }
const moneyOf = (p: any) => {
  const all = [...(p.result?.artifacts ?? []), ...(p.result?.login?.artifacts ?? [])]
  return all.filter((a: any, i: number) => all.findIndex((b: any) => b.type === a.type && b.value === a.value) === i)
}
const filesOf = (p: any) => Array.isArray(p.files) ? p.files : []
const exchangesOf = (p: any) => [...(p.result?.exchanges ?? []), ...(p.result?.login?.exchanges ?? [])]
const fmt = (d: string) => new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
</script>
