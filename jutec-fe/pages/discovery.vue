<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Penemuan" title="Discovery"
      subtitle="Temukan domain judol baru lewat crawler rekursif, pencarian Google, Certificate Transparency, atau analisis mendalam satu domain."
      icon="i-lucide-radar" />

    <!-- Mode selector -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <button v-for="m in modes" :key="m.value" @click="activeMode = m.value"
        class="text-left rounded-2xl p-5 ring-1 transition flex items-start gap-4"
        :class="activeMode === m.value ? 'bg-primary-500/10 ring-primary-500/40' : 'bg-slate-900/60 ring-slate-800 hover:ring-slate-700'">
        <div class="w-11 h-11 rounded-xl flex items-center justify-center shrink-0 ring-1"
          :class="activeMode === m.value ? 'bg-primary-500/20 ring-primary-500/40' : 'bg-slate-800/70 ring-slate-700'">
          <UIcon :name="m.icon" class="w-5 h-5" :class="activeMode === m.value ? 'text-primary-300' : 'text-slate-400'" />
        </div>
        <div class="min-w-0">
          <p class="font-semibold" :class="activeMode === m.value ? 'text-white' : 'text-slate-200'">{{ m.label }}</p>
          <p class="text-sm text-slate-400 mt-0.5 leading-snug">{{ m.desc }}</p>
        </div>
      </button>
    </div>

    <!-- ── ANALYZE ── -->
    <template v-if="activeMode === 'analyze'">
      <UCard>
        <div class="flex flex-col md:flex-row gap-3">
          <UInput v-model="analyzeUrl" size="xl" icon="i-lucide-globe" :disabled="analyzing" class="flex-1"
            placeholder="domain-target.com atau https://domain-target.com" @keyup.enter="startAnalysis" />
          <UButton size="xl" icon="i-lucide-scan-line" :loading="analyzing" :disabled="!analyzeUrl"
            :label="analyzing ? 'Menganalisis...' : 'Mulai Analisis'" @click="startAnalysis" />
        </div>
        <p class="text-sm text-slate-500 mt-3">Crawl headless browser, WHOIS, pencocokan template, dan composite scoring. Biasanya 10–40 detik.</p>
      </UCard>

      <div v-if="analyzing || analyzeLog.length" class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
        <!-- Verdict -->
        <UCard class="xl:col-span-4" :ui="{ ring: analyzeResult ? (analyzeResult.is_gambling ? 'ring-1 ring-rose-500/40' : 'ring-1 ring-emerald-500/40') : 'ring-1 ring-slate-800' }">
          <div v-if="!analyzeResult" class="empty-state !py-14">
            <UIcon name="i-lucide-loader-circle" class="w-8 h-8 text-primary-400" :class="analyzing && 'animate-spin'" />
            <p>{{ analyzing ? 'Menunggu hasil...' : 'Analisis gagal — lihat log.' }}</p>
          </div>
          <div v-else class="space-y-6">
            <div class="flex items-center gap-4">
              <div class="w-14 h-14 rounded-2xl flex items-center justify-center ring-1"
                :class="analyzeResult.is_gambling ? 'bg-rose-500/10 ring-rose-500/30' : 'bg-emerald-500/10 ring-emerald-500/30'">
                <UIcon :name="analyzeResult.is_gambling ? 'i-lucide-shield-alert' : 'i-lucide-shield-check'" class="w-7 h-7"
                  :class="analyzeResult.is_gambling ? 'text-rose-400' : 'text-emerald-400'" />
              </div>
              <div>
                <p class="text-sm text-slate-400">Klasifikasi</p>
                <p class="text-2xl font-semibold" :class="analyzeResult.is_gambling ? 'text-rose-300' : 'text-emerald-300'">
                  {{ analyzeResult.is_gambling ? 'Situs Judol' : 'Domain Normal' }}
                </p>
              </div>
            </div>
            <div>
              <div class="flex items-end justify-between mb-2">
                <span class="text-sm text-slate-400">Confidence</span>
                <span class="text-3xl font-semibold text-white tabular-nums">{{ Math.round(analyzeResult.confidence * 100) }}%</span>
              </div>
              <UProgress :value="Math.round(analyzeResult.confidence * 100)" :color="analyzeResult.is_gambling ? 'rose' : 'emerald'" />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 px-4 py-3">
                <p class="text-xs text-slate-500">Prioritas</p>
                <div class="mt-1.5"><PriorityBadge :priority="analyzeResult.priority" /></div>
              </div>
              <div class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 px-4 py-3">
                <p class="text-xs text-slate-500">Durasi</p>
                <p class="text-lg font-semibold text-white mono mt-1">{{ analyzeResult.time_taken }}</p>
              </div>
            </div>
            <div class="flex flex-col gap-2">
              <UButton v-if="analyzedSiteId" :to="`/site/${analyzedSiteId}`" block size="lg" icon="i-lucide-panel-right-open"
                label="Lihat Detail Lengkap & Peta Keterkaitan" />
              <div class="grid grid-cols-2 gap-2">
                <UButton :to="analyzedUrl" target="_blank" color="gray" variant="soft" icon="i-lucide-external-link" label="Buka Situs" block />
                <UButton color="gray" variant="soft" icon="i-lucide-rotate-cw" label="Scan Ulang" block :loading="analyzing" @click="startAnalysis" />
              </div>
            </div>
            <div v-if="analyzeResult.found_keywords?.length">
              <p class="field-label">Keyword terdeteksi</p>
              <div class="flex flex-wrap gap-1.5">
                <UBadge v-for="kw in analyzeResult.found_keywords.slice(0, 12)" :key="kw" color="rose" variant="subtle">{{ kw }}</UBadge>
              </div>
            </div>
          </div>
        </UCard>

        <!-- Breakdown -->
        <UCard class="xl:col-span-3">
          <template #header>
            <h2 class="section-title">Rincian Skor</h2>
            <p class="section-desc">Kontribusi tiap sinyal</p>
          </template>
          <ScoreBar v-if="analyzeResult?.score_breakdown" :breakdown="analyzeResult.score_breakdown" />
          <p v-else class="muted">Belum tersedia.</p>
        </UCard>

        <!-- Log -->
        <UCard class="xl:col-span-5" :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <UIcon name="i-lucide-terminal" class="w-5 h-5 text-primary-400" />
                <h2 class="section-title">Log Investigasi</h2>
              </div>
              <UButton label="Bersihkan" color="gray" variant="ghost" size="sm" icon="i-lucide-eraser" @click="analyzeLog = []" />
            </div>
          </template>
          <div id="analyze-terminal" class="bg-slate-950/70 h-96 overflow-y-auto px-6 py-4 font-mono text-[13px] space-y-1.5">
            <div v-for="(entry, i) in analyzeLog" :key="i" class="flex items-start gap-4 leading-relaxed">
              <span class="text-slate-600 tabular-nums shrink-0 w-20">{{ entry.time }}</span>
              <span :class="entry.ok ? 'text-emerald-400' : entry.err ? 'text-rose-400' : 'text-slate-300'">{{ entry.text }}</span>
            </div>
            <div v-if="analyzing" class="text-primary-400 animate-pulse pl-24">▌</div>
          </div>
        </UCard>
      </div>

      <UCard v-else>
        <div class="empty-state">
          <div class="empty-icon"><UIcon name="i-lucide-scan-search" class="w-6 h-6" /></div>
          <p class="text-base text-slate-300 font-medium">Masukkan domain target</p>
          <p class="text-sm">Hasil klasifikasi, skor, dan log investigasi akan tampil di sini.</p>
        </div>
      </UCard>
    </template>

    <!-- ── CRAWLER / GOOGLE / CT ── -->
    <div v-else class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <!-- Config -->
      <div class="xl:col-span-4 xl:sticky xl:top-24 space-y-6">
        <UCard>
          <template #header>
            <h2 class="section-title">{{ activeModeLabel }}</h2>
            <p class="section-desc">{{ activeModeDesc }}</p>
          </template>

          <div class="space-y-6">
            <template v-if="activeMode === 'crawler'">
              <div class="grid grid-cols-2 gap-2 p-1 rounded-xl bg-slate-950/60 ring-1 ring-slate-800">
                <UButton block label="Manual" icon="i-lucide-hand" :color="!isAuto ? 'primary' : 'gray'" :variant="!isAuto ? 'solid' : 'ghost'" @click="isAuto = false" />
                <UButton block label="Otomatis" icon="i-lucide-bot" :color="isAuto ? 'primary' : 'gray'" :variant="isAuto ? 'solid' : 'ghost'" @click="isAuto = true" />
              </div>
              <UFormGroup v-if="!isAuto" label="Domain awal" size="lg">
                <UInput v-model="deepRoot" icon="i-lucide-globe" placeholder="contoh-perjudian.com" />
              </UFormGroup>
              <UAlert v-else icon="i-lucide-bot" color="primary" variant="subtle"
                description="Memakai keyword pencarian default dari database sebagai titik awal." />
              <div>
                <div class="flex items-center justify-between mb-2">
                  <span class="field-label !mb-0">Kedalaman crawl</span>
                  <UBadge variant="subtle">Level {{ deepDepth }}</UBadge>
                </div>
                <div class="grid grid-cols-5 gap-2">
                  <UButton v-for="n in 5" :key="n" :label="String(n)" block size="lg"
                    :color="deepDepth === n ? 'primary' : 'gray'" :variant="deepDepth === n ? 'solid' : 'soft'" @click="deepDepth = n" />
                </div>
              </div>
              <UFormGroup label="Batas URL" size="lg">
                <USelect v-model="deepMaxUrls" :options="[{ value: 100, label: '100 — Cepat' }, { value: 250, label: '250 — Sedang' }, { value: 500, label: '500 — Dalam' }, { value: 1000, label: '1.000 — Exhaustive' },
                  { value: 2500, label: '2.500 — Masif' }, { value: 5000, label: '5.000 — Agresif' }, { value: 10000, label: '10.000 — Sangat agresif' }, { value: 25000, label: '25.000 — Maksimum' }]" />
                <p v-if="deepMaxUrls > 1000" class="text-xs text-amber-400 mt-1.5">Batas besar memakai banyak waktu & kuota API — jalankan saat server tidak sibuk.</p>
              </UFormGroup>
              <UAlert v-if="deepDepth >= 4" icon="i-lucide-triangle-alert" color="amber" variant="subtle"
                :description="`Level ${deepDepth} akan menelusuri ribuan domain — proses bisa berlangsung lama.`" />
            </template>

            <UFormGroup v-else-if="activeMode === 'google'" label="Query pencarian" size="lg"
              help="Mendukung operator Google, mis. site:, intitle:, tanda kutip.">
              <UInput v-model="googleQuery" icon="i-lucide-search" placeholder='site:*.xyz "daftar slot"' @keyup.enter="startDiscovery" />
            </UFormGroup>

            <UFormGroup v-else label="Domain / keyword" size="lg" help="Cari subdomain lewat crt.sh Certificate Transparency Logs.">
              <UInput v-model="ctKeyword" icon="i-lucide-badge-check" placeholder="judol.com" @keyup.enter="startDiscovery" />
            </UFormGroup>

            <UButton block size="xl" icon="i-lucide-zap" :loading="discoveryLoading"
              :label="discoveryLoading ? 'Memproses...' : 'Mulai Pencarian'" @click="startDiscovery" />
            <UButton v-if="discoveryLoading" block size="lg" color="gray" variant="soft" icon="i-lucide-eye-off"
              label="Sembunyikan progres" @click="stopDiscovery" />
          </div>
        </UCard>
      </div>

      <!-- Results -->
      <div class="xl:col-span-8 min-w-0 space-y-6">
        <div v-if="discoveryResult && activeMode === 'crawler'" class="grid grid-cols-2 lg:grid-cols-3 2xl:grid-cols-5 gap-4 animate-fade-in">
          <StatCard label="Ditemukan"  :value="discoveryResult.total_found ?? 0"    icon="i-lucide-list-checks"    color="indigo" />
          <StatCard label="Dianalisis" :value="discoveryResult.total_analyzed ?? 0" icon="i-lucide-microscope"     color="emerald" />
          <StatCard label="Judol"      :value="discoveryResult.total_gambling ?? 0" icon="i-lucide-shield-alert"   color="rose" />
          <StatCard label="Sudah Ada"  :value="discoveryResult.total_skipped ?? 0"  icon="i-lucide-copy-check"     color="slate" />
          <StatCard label="Level"      :value="discoveryResult.depth_reached ?? 1"  icon="i-lucide-layers"         color="amber" />
        </div>

        <!-- Google / CT result list -->
        <UCard v-if="discoveryResult?.results?.length && activeMode !== 'crawler'" :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex items-center justify-between gap-4">
              <div>
                <h2 class="section-title">{{ activeMode === 'google' ? 'URL Ditemukan' : 'Subdomain Ditemukan' }}</h2>
                <p class="section-desc">{{ discoveryResult.found_count }} hasil · {{ discoveryResult.query || discoveryResult.keyword }}</p>
              </div>
              <UButton icon="i-lucide-copy" color="gray" variant="soft" label="Salin semua" @click="copyResults" />
            </div>
          </template>
          <div class="max-h-[560px] overflow-y-auto divide-y divide-slate-800/70">
            <div v-for="(r, i) in discoveryResult.results" :key="i" class="flex items-center gap-4 px-6 py-3 hover:bg-slate-800/30">
              <span class="text-sm text-slate-500 tabular-nums w-8">{{ i + 1 }}</span>
              <span class="flex-1 min-w-0 truncate mono text-slate-200">{{ r }}</span>
              <UButton size="sm" color="gray" variant="soft" icon="i-lucide-scan-line" label="Analisis"
                @click="activeMode = 'analyze'; analyzeUrl = r; startAnalysis()" />
            </div>
          </div>
        </UCard>

        <UCard v-if="discoveryLoading || logs.length" :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex flex-wrap items-center justify-between gap-4">
              <div class="flex items-center gap-3">
                <UIcon name="i-lucide-terminal" class="w-5 h-5 text-primary-400" />
                <h2 class="section-title">Log Aktivitas</h2>
                <UBadge :color="discoveryLoading ? 'primary' : 'gray'" variant="subtle">{{ discoveryLoading ? 'Berjalan' : 'Selesai' }}</UBadge>
              </div>
              <div class="flex items-center gap-5">
                <span class="text-sm text-slate-400 tabular-nums"><UIcon name="i-lucide-timer" class="w-4 h-4 align-[-3px] mr-1" />{{ elapsed }}</span>
                <span class="text-sm font-semibold text-emerald-400">{{ foundCount }} ditemukan</span>
                <UButton label="Bersihkan" color="gray" variant="ghost" size="sm" icon="i-lucide-eraser" @click="resetState" />
              </div>
            </div>
          </template>
          <div id="disc-terminal" class="bg-slate-950/70 h-[28rem] overflow-y-auto px-6 py-4 font-mono text-[13px] space-y-1.5">
            <div v-for="(log, i) in logs" :key="i" class="flex items-start gap-4 leading-relaxed">
              <span class="text-slate-600 tabular-nums shrink-0 w-20">{{ log.time }}</span>
              <span :class="{ 'text-emerald-400': log.type === 'FOUND', 'text-slate-600': log.type === 'SKIP', 'text-slate-300': log.type === 'INFO' }">{{ log.text }}</span>
            </div>
            <div v-if="discoveryLoading" class="text-primary-400 animate-pulse pl-24">▌</div>
          </div>
        </UCard>

        <UCard v-if="!discoveryLoading && !logs.length && !discoveryResult">
          <div class="empty-state !py-28">
            <div class="empty-icon"><UIcon :name="activeModeIcon" class="w-6 h-6" /></div>
            <p class="text-base text-slate-300 font-medium">Siap mencari</p>
            <p class="text-sm">Atur parameter di panel kiri, lalu klik <b class="text-slate-200">Mulai Pencarian</b>.</p>
          </div>
        </UCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'

const { get, post, apiBase } = useApi()
const { loading: discoveryLoading, result: discoveryResult, logs, elapsed, startTime, addLog, resetState } = useDiscoveryState()
const route = useRoute()

const activeMode  = ref('crawler')
const isAuto      = ref(true) // Deep Sweep otomatis (keyword bawaan) sebagai bawaan; Manual = mulai dari satu domain
const deepRoot    = ref('')
const deepDepth   = ref(2)
const deepMaxUrls = ref(500)
const googleQuery = ref('')
const ctKeyword   = ref('')

// Analyze state
const analyzeUrl    = ref('')
const analyzing     = ref(false)
const analyzeResult = ref<any>(null)
const analyzeLog    = ref<any[]>([])
const screenshotUrl = ref('')
const analyzedSiteId = ref<number | null>(null)
const analyzedUrl    = ref('')

const modes = [
  { value: 'crawler', label: 'Deep Sweep',   icon: 'i-lucide-satellite-dish', desc: 'Crawl rekursif otomatis dari keyword bawaan, atau dari satu domain' },
  { value: 'google',  label: 'Google',       icon: 'i-lucide-search',      desc: 'Pencarian Google via Serper API' },
  { value: 'ct',      label: 'CT Logs',      icon: 'i-lucide-badge-check', desc: 'Subdomain dari Certificate Transparency' },
  { value: 'analyze', label: 'Analisis',     icon: 'i-lucide-scan-line',   desc: 'Scan mendalam & skor risiko satu domain' },
]

async function copyResults() {
  try {
    await navigator.clipboard.writeText((discoveryResult.value?.results ?? []).join('\n'))
    useToast().add({ title: 'Disalin ke clipboard', icon: 'i-lucide-check', color: 'emerald' })
  } catch {}
}

const activeModeLabel = computed(() => modes.find(m => m.value === activeMode.value)?.label ?? '')
const activeModeIcon  = computed(() => modes.find(m => m.value === activeMode.value)?.icon ?? '')
const activeModeDesc  = computed(() => modes.find(m => m.value === activeMode.value)?.desc ?? '')

const foundCount = computed(() => logs.value.filter((l: any) => l.type === 'FOUND').length)

let timerInterval: any = null
function startTimer() {
  if (timerInterval) clearInterval(timerInterval)
  timerInterval = setInterval(() => {
    if (!startTime.value) return
    const d = Date.now() - startTime.value
    elapsed.value = `${Math.floor(d/60000).toString().padStart(2,'0')}:${Math.floor((d%60000)/1000).toString().padStart(2,'0')}`
  }, 1000)
}
function stopTimer() { clearInterval(timerInterval); timerInterval = null; startTime.value = null }

watch(logs, () => nextTick(() => {
  const el = document.getElementById('disc-terminal')
  if (el) el.scrollTop = el.scrollHeight
}), { deep: true })

onMounted(() => {
  if (discoveryLoading.value && startTime.value) startTimer()
  // Support redirect dari /analyze?url=...
  const q = route.query.url as string
  if (q) { activeMode.value = 'analyze'; analyzeUrl.value = q; startAnalysis() }
  // Pintasan dari halaman detail situs (tidak langsung dijalankan — pengguna menekan Mulai)
  const mode = route.query.mode as string
  if (modes.some(m => m.value === mode)) activeMode.value = mode
  if (mode === 'ct' && route.query.q) { activeMode.value = 'ct'; ctKeyword.value = route.query.q as string }
  if (mode === 'crawler' && route.query.domain) { activeMode.value = 'crawler'; isAuto.value = false; deepRoot.value = route.query.domain as string; deepDepth.value = 1 }
})
onUnmounted(() => clearInterval(timerInterval))
watch(() => route.query.mode, m => { if (modes.some(x => x.value === m)) activeMode.value = m as string })

// ── Discovery ──
async function startDiscovery() {
  discoveryLoading.value = true
  resetState()
  startTime.value = Date.now()
  startTimer()
  try {
    if (activeMode.value === 'crawler') {
      if (!isAuto.value && !deepRoot.value) { discoveryLoading.value = false; stopTimer(); return }
      addLog(isAuto.value ? 'Auto deep sweep (defaults)' : `Crawler: ${deepRoot.value}`)
      const res = isAuto.value
        ? await post<any>('/discovery/deep-sweep', { use_defaults: true, max_depth: deepDepth.value, max_urls: deepMaxUrls.value })
        : await get<any>('/discovery/deep-sweep', { domain: deepRoot.value, depth: deepDepth.value, limit: deepMaxUrls.value })
      discoveryResult.value = res
      addLog(`Selesai — ${res.total_found ?? res.found_count ?? 0} target ditemukan`, 'FOUND')
    } else if (activeMode.value === 'google') {
      if (!googleQuery.value) { discoveryLoading.value = false; stopTimer(); return }
      addLog(`Google: ${googleQuery.value}`)
      const res = await get<any>('/discovery/google', { q: googleQuery.value })
      discoveryResult.value = res
      addLog(`Selesai — ${res.found_count ?? 0} URL`, 'FOUND')
    } else {
      if (!ctKeyword.value) { discoveryLoading.value = false; stopTimer(); return }
      addLog(`CT Logs: ${ctKeyword.value}`)
      const res = await get<any>('/discovery/certs', { q: ctKeyword.value })
      discoveryResult.value = res
      addLog(`Selesai — ${res.found_count ?? 0} domain`, 'FOUND')
    }
  } catch (e: any) {
    addLog(`Error: ${e.message}`)
  } finally { discoveryLoading.value = false; stopTimer() }
}

function stopDiscovery() { discoveryLoading.value = false; stopTimer(); addLog('Dihentikan oleh operator') }

// ── Analyze ──
function aLog(text: string, ok = false, err = false) {
  analyzeLog.value.push({ time: new Date().toLocaleTimeString(), text, ok, err })
  nextTick(() => { const el = document.getElementById('analyze-terminal'); if (el) el.scrollTop = el.scrollHeight })
}

async function startAnalysis() {
  if (!analyzeUrl.value) return
  analyzing.value = true; analyzeResult.value = null; analyzeLog.value = []; screenshotUrl.value = ''; analyzedSiteId.value = null
  aLog(`Scan: ${analyzeUrl.value}`)
  const t0 = performance.now()
  try {
    const raw = await get<any>('/analyze', { url: analyzeUrl.value })
    // Backend membungkus hasil di `result` (crawler) dan `score` (composite) — ratakan untuk template
    const res = {
      ...raw.result,
      is_gambling:     raw.score?.is_gambling ?? false,
      confidence:      raw.score?.final ?? 0,
      priority:        raw.score?.priority,
      score_breakdown: raw.score?.breakdown,
      time_taken:      `${((performance.now() - t0) / 1000).toFixed(1)}s`,
    }
    aLog(`Selesai crawl dalam ${res.time_taken}`)
    if (res.found_keywords?.length) aLog(`Keywords: ${res.found_keywords.slice(0,5).join(', ')}`)
    aLog(res.is_gambling ? `Terdeteksi judol — ${(res.confidence*100).toFixed(1)}%` : `Bersih — ${(res.confidence*100).toFixed(1)}%`, !res.is_gambling, res.is_gambling)
    analyzeResult.value = res
    // Cari id situs yang baru tersimpan agar bisa langsung membuka halaman detail
    analyzedUrl.value = raw.url || analyzeUrl.value
    try { analyzedSiteId.value = (await get<any>('/sites/lookup', { url: analyzedUrl.value }))?.id ?? null } catch {}
    if (analyzedSiteId.value) aLog(`Tersimpan sebagai situs #${analyzedSiteId.value} — buka detail untuk peta keterkaitan`, true)
    if (res.screenshot_path) { screenshotUrl.value = `${apiBase}${res.screenshot_path}`; aLog('Screenshot berhasil', true) }
    aLog('Selesai', true)
  } catch (e: any) {
    aLog(`ERROR: ${e.message}`, false, true)
  } finally { analyzing.value = false }
}
</script>
