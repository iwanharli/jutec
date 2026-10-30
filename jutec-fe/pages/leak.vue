<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Data Enrichment" title="Pencarian Identitas"
      subtitle="Identifikasi pemilik nomor HP lewat data registrasi seluler (KomInfo) dan kependudukan (Dukcapil) untuk memperkaya investigasi."
      icon="i-lucide-fingerprint" />

    <div class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <!-- Left: forms -->
      <div class="xl:col-span-4 xl:sticky xl:top-24 space-y-6">
        <UCard>
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-cyan-500/10 ring-1 ring-cyan-500/25 flex items-center justify-center">
                <UIcon name="i-lucide-phone" class="w-5 h-5 text-cyan-400" />
              </div>
              <div>
                <h2 class="section-title">Cek Nomor HP</h2>
                <p class="section-desc !mt-0">Cross-reference data publik</p>
              </div>
            </div>
          </template>

          <div class="space-y-5">
            <UFormGroup label="Nomor seluler" size="xl" :error="phoneError || undefined"
              help="Format 08…, 62…, atau +62… dinormalisasi otomatis.">
              <UInput v-model="phone" icon="i-lucide-smartphone" placeholder="081234567890" :ui="{ base: 'font-mono' }"
                @keyup.enter="lookupPhone" />
            </UFormGroup>
            <UButton block size="xl" color="cyan" icon="i-lucide-search" :loading="loadingPhone" :disabled="!phone.trim()"
              :label="loadingPhone ? 'Memproses...' : 'Cek Identitas'" @click="lookupPhone" />
          </div>
        </UCard>

        <UCard>
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-amber-500/10 ring-1 ring-amber-500/25 flex items-center justify-center">
                <UIcon name="i-lucide-zap" class="w-5 h-5 text-amber-400" />
              </div>
              <div>
                <h2 class="section-title">Enrichment Massal</h2>
                <p class="section-desc !mt-0">Semua nomor HP di database</p>
              </div>
            </div>
          </template>

          <div class="space-y-5">
            <p class="text-sm text-slate-400 leading-relaxed">
              Proses otomatis seluruh artefak nomor HP yang belum pernah dicek. Hasilnya tersimpan di Repository → KomInfo / Dukcapil.
            </p>
            <UButton block size="lg" color="amber" variant="soft" icon="i-lucide-play" :loading="enriching"
              :label="enriching ? 'Memproses...' : 'Jalankan Enrichment'" @click="enrichAll" />

            <div v-if="enrichResult" class="grid grid-cols-2 gap-3 animate-fade-in">
              <div class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-4 text-center">
                <p class="text-xs text-slate-500">Data ditemukan</p>
                <p class="text-2xl font-semibold text-white tabular-nums mt-1">{{ (enrichResult.saved_dukcapil ?? 0) + (enrichResult.saved_kominfo ?? 0) }}</p>
              </div>
              <div class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-4 text-center">
                <p class="text-xs text-slate-500">Nomor diproses</p>
                <p class="text-2xl font-semibold text-white tabular-nums mt-1">{{ enrichResult.processed ?? 0 }}</p>
              </div>
              <p v-if="enrichResult.message" class="col-span-2 text-sm text-slate-400 text-center">
                {{ enrichResult.message === 'no new phone artifacts to enrich' ? 'Tidak ada nomor baru untuk diproses.' : enrichResult.message }}
              </p>
            </div>
          </div>
        </UCard>
      </div>

      <!-- Right: results -->
      <div class="xl:col-span-8 min-w-0 space-y-6">
        <template v-if="phoneResult">
          <!-- Subject summary -->
          <UCard v-if="hasData" :ui="{ ring: 'ring-1 ring-cyan-500/30' }" class="animate-fade-in">
            <div class="flex flex-col md:flex-row md:items-center gap-6">
              <div class="w-20 h-20 rounded-2xl bg-cyan-500/10 ring-1 ring-cyan-500/30 flex items-center justify-center shrink-0">
                <UIcon name="i-lucide-user-round-check" class="w-10 h-10 text-cyan-300" />
              </div>
              <div class="flex-1 min-w-0">
                <p class="page-eyebrow !text-cyan-400">Subjek teridentifikasi</p>
                <h2 class="text-3xl font-semibold text-white mt-1 capitalize">{{ subject.name || 'Nama tidak diketahui' }}</h2>
                <div class="flex flex-wrap items-center gap-2 mt-3">
                  <UBadge color="primary" variant="subtle" size="lg" class="mono"><UIcon name="i-lucide-id-card" class="w-4 h-4 mr-1.5" />{{ subject.nik || 'NIK tidak ada' }}</UBadge>
                  <UBadge color="gray" variant="subtle" size="lg"><UIcon name="i-lucide-map-pin" class="w-4 h-4 mr-1.5" />{{ subject.region || 'Wilayah tidak diketahui' }}</UBadge>
                  <UBadge color="gray" variant="subtle" size="lg" class="mono"><UIcon name="i-lucide-phone" class="w-4 h-4 mr-1.5" />{{ phoneResult.phone }}</UBadge>
                </div>
              </div>
            </div>
          </UCard>

          <UAlert v-else icon="i-lucide-search-x" color="gray" variant="subtle" title="Tidak ada data"
            :description="`Nomor ${phoneResult.phone} tidak ditemukan di data KomInfo maupun Dukcapil.`" />

          <!-- VflOSINT: pengecekan berantai lewat bot Telegram (nomor → NIK → nomor lain milik NIK itu) -->
          <UCard>
            <div class="flex flex-wrap items-center justify-between gap-4">
              <div class="flex items-center gap-3 min-w-0">
                <UIcon name="i-lucide-bot" class="w-5 h-5 text-cyan-400 shrink-0" />
                <div class="min-w-0">
                  <p class="text-base font-semibold text-white">Pengecekan VflOSINT</p>
                  <p class="text-sm text-slate-400">
                    <template v-if="phoneResult.vfl_checked">Sudah dicek dalam 30 hari terakhir · </template>
                    <template v-if="phoneResult.api_error">API LeakOSINT: {{ phoneResult.api_error }} · </template>
                    Dicek otomatis tiap jam untuk nomor situs judol; kuota hari ini {{ vflStatus.today ?? '—' }}/{{ vflStatus.daily_cap ?? '—' }}
                  </p>
                </div>
              </div>
              <UButton color="cyan" variant="soft" icon="i-lucide-link" :loading="vflRunning"
                :label="vflRunning ? 'Mengecek berantai…' : 'Cek Berantai Sekarang'" @click="runVfl" />
            </div>
            <div v-if="vflChain.length" class="mt-4 pt-4 border-t border-slate-800 space-y-2">
              <div v-for="q in vflChain" :key="q.id" class="flex flex-wrap items-center gap-2 text-sm" :style="{ paddingLeft: q.depth * 20 + 'px' }">
                <UIcon :name="q.kind === 'nik' ? 'i-lucide-id-card' : 'i-lucide-phone'" class="w-4 h-4 text-slate-500" />
                <span class="mono text-slate-200">{{ q.query }}</span>
                <UBadge :color="q.status === 'found' ? 'emerald' : q.status === 'error' ? 'rose' : 'gray'" variant="subtle" size="xs">
                  {{ q.status === 'found' ? `${q.result_count} data` : q.status === 'error' ? 'gagal' : 'tidak ada' }}
                </UBadge>
                <span v-if="q.error" class="text-rose-400 text-xs">{{ q.error }}</span>
              </div>
            </div>
          </UCard>

          <div class="grid grid-cols-1 2xl:grid-cols-2 gap-6">
            <!-- KomInfo -->
            <UCard>
              <template #header>
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-3">
                    <UIcon name="i-lucide-radio-tower" class="w-5 h-5 text-blue-400" />
                    <h2 class="section-title">Registrasi Seluler</h2>
                  </div>
                  <UBadge color="blue" variant="subtle">KomInfo · {{ phoneResult.kominfo?.length ?? 0 }}</UBadge>
                </div>
              </template>
              <div v-if="!phoneResult.kominfo?.length" class="empty-state !py-10"><p>Tidak ada data registrasi.</p></div>
              <div v-else class="space-y-3">
                <div v-for="(e, i) in phoneResult.kominfo" :key="i" class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-5 grid grid-cols-2 gap-5">
                  <div><p class="text-xs text-slate-500">Operator</p><p class="text-base font-semibold text-white mt-1">{{ e.Provider || '—' }}</p></div>
                  <div><p class="text-xs text-slate-500">Tanggal registrasi</p><p class="text-base font-semibold text-white mono mt-1">{{ e.RegDate || '—' }}</p></div>
                  <div v-if="e.Passport" class="col-span-2"><p class="text-xs text-slate-500">NIK terdaftar</p><p class="text-base text-slate-200 mono mt-1">{{ e.Passport }}</p></div>
                  <div v-if="e.Source" class="col-span-2"><UBadge color="gray" variant="subtle" size="xs">Sumber: {{ e.Source }}</UBadge></div>
                </div>
              </div>
              <div v-if="phoneResult.related?.length" class="mt-5 pt-5 border-t border-slate-800">
                <p class="text-sm font-semibold text-white mb-3">Nomor lain dengan NIK yang sama · {{ phoneResult.related.length }}</p>
                <div class="space-y-2">
                  <div v-for="(r, i) in phoneResult.related" :key="i" class="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
                    <NuxtLink :to="`/leak?q=${r.Phone}`" class="mono text-cyan-400 hover:underline" @click="openPhone(r.Phone)">{{ r.Phone }}</NuxtLink>
                    <span class="text-slate-400">{{ r.Provider }}</span>
                    <span class="mono text-slate-500">{{ r.RegDate }}</span>
                    <span class="mono text-slate-500">NIK {{ r.Passport }}</span>
                  </div>
                </div>
              </div>
            </UCard>

            <!-- Dukcapil -->
            <UCard>
              <template #header>
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-3">
                    <UIcon name="i-lucide-landmark" class="w-5 h-5 text-purple-400" />
                    <h2 class="section-title">Data Kependudukan</h2>
                  </div>
                  <UBadge color="purple" variant="subtle">Dukcapil · {{ phoneResult.dukcapil?.length ?? 0 }}</UBadge>
                </div>
              </template>
              <div v-if="!phoneResult.dukcapil?.length" class="empty-state !py-10"><p>Tidak ada data kependudukan.</p></div>
              <div v-else class="space-y-3">
                <div v-for="(e, i) in phoneResult.dukcapil" :key="i" class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-5 space-y-4">
                  <p class="text-lg font-semibold text-white capitalize">{{ e.FullName || '—' }}</p>
                  <div class="grid grid-cols-2 gap-5">
                    <div><p class="text-xs text-slate-500">NIK</p><p class="text-sm text-slate-200 mono mt-1">{{ e.Passport || '—' }}</p></div>
                    <div><p class="text-xs text-slate-500">Gender</p><p class="text-sm text-slate-200 mt-1 capitalize">{{ e.Gender || '—' }}</p></div>
                    <div><p class="text-xs text-slate-500">Tanggal lahir</p><p class="text-sm text-slate-200 mono mt-1">{{ e.BDay || '—' }}</p></div>
                    <div><p class="text-xs text-slate-500">Wilayah</p><p class="text-sm text-slate-200 mt-1 capitalize">{{ e.Region || '—' }}</p></div>
                  </div>
                  <div class="pt-4 border-t border-slate-800">
                    <p class="text-xs text-slate-500">Alamat</p>
                    <p class="text-sm text-slate-300 mt-1 leading-relaxed capitalize">{{ [e.Address, e.District, e.Location].filter(Boolean).join(', ') || '—' }}</p>
                  </div>
                </div>
              </div>
            </UCard>
          </div>
        </template>

        <UCard v-else-if="loadingPhone">
          <div class="empty-state !py-28">
            <UIcon name="i-lucide-loader-circle" class="w-8 h-8 text-cyan-400 animate-spin" />
            <p>Mencari di database KomInfo & Dukcapil...</p>
          </div>
        </UCard>

        <UCard v-else>
          <div class="empty-state !py-32">
            <div class="empty-icon"><UIcon name="i-lucide-fingerprint" class="w-6 h-6" /></div>
            <p class="text-base text-slate-300 font-medium">Siap mencari identitas</p>
            <p class="text-sm max-w-md">Masukkan nomor HP target di panel kiri untuk melihat data registrasi dan kependudukan yang terkait.</p>
          </div>
        </UCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const { get, post } = useApi()
const toast = useToast()

const phone        = ref('')
const loadingPhone = ref(false)
const phoneResult  = ref<any>(null)
const phoneError   = ref('')

const hasData = computed(() => !!(phoneResult.value?.dukcapil?.length || phoneResult.value?.kominfo?.length))
const subject = computed(() => {
  const d = phoneResult.value?.dukcapil?.[0] ?? {}
  const k = phoneResult.value?.kominfo?.[0] ?? {}
  return { name: d.FullName || k.FullName, nik: d.Passport || k.Passport, region: d.Region || d.Location }
})

async function lookupPhone() {
  if (!phone.value.trim()) return
  loadingPhone.value = true
  phoneResult.value  = null
  phoneError.value   = ''
  try { phoneResult.value = await get<any>('/leak/phone', { q: phone.value.trim() }) }
  catch (e: any) { phoneError.value = e.message } finally { loadingPhone.value = false; loadVfl() }
}

// ── VflOSINT ──
const vflStatus  = ref<any>({})
const vflChain   = ref<any[]>([])
const vflRunning = ref(false)
const artifactId = ref<number | null>(null)

async function loadVfl() {
  get<any>('/leak/vfl/status').then(r => (vflStatus.value = r)).catch(() => {})
  const q: any = artifactId.value ? { artifact_id: artifactId.value } : { phone: phoneResult.value?.phone }
  if (q.artifact_id || q.phone) {
    const r = await get<any>('/leak/vfl/chain', q).catch(() => null)
    vflChain.value = r?.queries ?? []
  }
}

async function runVfl() {
  vflRunning.value = true
  try {
    const r = await post<any>('/leak/vfl', { phone: phoneResult.value?.phone || phone.value })
    artifactId.value = r.artifact_id
    toast.add({ title: 'Pengecekan diantrekan', description: r.message, color: 'cyan', icon: 'i-lucide-bot' })
    // rantai butuh ±1–3 menit (jeda 30 dtk antar kueri) — pantau hingga tak ada kueri baru
    let lastLen = -1, idle = 0
    for (let i = 0; i < 40 && idle < 6; i++) {
      await new Promise(res => setTimeout(res, 8000))
      await loadVfl()
      idle = vflChain.value.length === lastLen ? idle + 1 : 0
      lastLen = vflChain.value.length
    }
    await lookupPhone()
  } catch (e: any) {
    toast.add({ title: 'Tidak bisa dicek', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { vflRunning.value = false }
}

function openPhone(p: string) { phone.value = p; artifactId.value = null; vflChain.value = []; lookupPhone() }

// Dibuka dari halaman detail situs: /leak?q=628… → langsung cek
const route = useRoute()
onMounted(() => {
  const q = route.query.q as string
  if (q) { phone.value = q; lookupPhone() }
})

const enriching    = ref(false)
const enrichResult = ref<any>(null)

async function enrichAll() {
  enriching.value = true
  enrichResult.value = null
  try { enrichResult.value = await post<any>('/leak/enrich', {}) }
  catch (e: any) { toast.add({ title: 'Enrichment gagal', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' }) }
  finally { enriching.value = false }
}
</script>
