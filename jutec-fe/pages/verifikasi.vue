<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Kualitas Data" title="Antrean Verifikasi" icon="i-lucide-badge-check"
      subtitle="Periksa situs yang dinilai judol oleh engine. Keputusanmu mengunci status situs, tercatat di log audit, dan menjadi bahan belajar engine." />

    <!-- ── Ringkasan & bahan belajar ── -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-4">
      <div class="xl:col-span-8 grid grid-cols-2 lg:grid-cols-4 gap-3">
        <div v-for="k in kpis" :key="k.label" class="rounded-2xl bg-slate-900/60 ring-1 ring-slate-800 p-4">
          <div class="flex items-center gap-2 text-[13px] text-slate-400">
            <UIcon :name="k.icon" class="w-4 h-4" :class="k.iconColor" />{{ k.label }}
          </div>
          <p class="text-3xl font-semibold tabular-nums mt-2" :class="k.color">{{ fmt(k.value) }}</p>
          <p class="text-[12px] text-slate-500 mt-0.5">{{ k.hint }}</p>
        </div>
      </div>

      <!-- Kemajuan "bahan belajar" — terhubung ke saran bobot kata kunci -->
      <div class="xl:col-span-4 rounded-2xl bg-gradient-to-br from-emerald-500/10 to-slate-900/60 ring-1 ring-emerald-500/20 p-4">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-brain" class="w-4 h-4 text-emerald-400" />
          <p class="text-sm font-semibold text-slate-100">Bahan belajar engine</p>
          <UBadge v-if="learnReady" color="emerald" variant="subtle" size="xs" class="ml-auto">siap</UBadge>
        </div>
        <p class="text-[12px] text-slate-400 mt-1 leading-snug">
          Setiap keputusan dipakai menghitung ketepatan kata kunci. Butuh ±{{ TARGET }} situs per kategori agar saran bobot akurat.
        </p>
        <div class="mt-3 space-y-2.5">
          <div v-for="b in learnBars" :key="b.label">
            <div class="flex justify-between text-[12px] mb-1">
              <span class="text-slate-300">{{ b.label }}</span>
              <span class="tabular-nums" :class="b.n >= TARGET ? 'text-emerald-400' : 'text-slate-400'">{{ b.n }} / {{ TARGET }}</span>
            </div>
            <div class="h-1.5 rounded-full bg-slate-800 overflow-hidden">
              <div class="h-full rounded-full transition-all" :class="b.bar" :style="{ width: `${Math.min(100, (b.n / TARGET) * 100)}%` }" />
            </div>
          </div>
        </div>
        <p v-if="learnHint" class="text-[12px] text-amber-300/90 mt-2.5 leading-snug">{{ learnHint }}</p>
        <UButton to="/keywords" size="2xs" color="emerald" variant="ghost" class="mt-2 -ml-1.5" trailing-icon="i-lucide-arrow-right" label="Lihat saran bobot kata kunci" />
      </div>
    </div>

    <UAlert v-if="!operator" icon="i-lucide-user-round" color="amber" variant="subtle"
      title="Isi nama penyidik dulu" description="Klik tombol nama di kanan atas — setiap keputusan tercatat atas nama seseorang." />

    <!-- ── Antrean ── -->
    <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-0' } }">
      <template #header>
        <div class="flex flex-col lg:flex-row lg:items-center gap-3">
          <div class="flex-1 min-w-0">
            <h2 class="card-title">Belum diverifikasi</h2>
            <p class="card-desc">{{ fmt(total) }} situs · {{ sortHint }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <SegmentedControl v-model="priority" :options="priorities" />
            <USelect v-model="sort" :options="sortOptions" icon="i-lucide-arrow-up-down" class="w-full sm:w-52" />
            <FilterToggle v-model="includeStrong" label="+ bukti kuat" icon="i-lucide-shield-check"
              hint="Situs dengan rekening, provider game, atau klaster operator sudah dianggap judol oleh mesin" />
          </div>
        </div>
        <p class="hidden md:flex items-center gap-3 text-[12px] text-slate-500 mt-3">
          <UIcon name="i-lucide-keyboard" class="w-3.5 h-3.5" />
          <span><kbd>J</kbd> judol</span><span><kbd>B</kbd> bukan judol</span>
          <span><kbd>↓</kbd><kbd>↑</kbd> pindah</span><span><kbd>O</kbd> buka detail</span>
        </p>
      </template>

      <div v-if="loading && !rows.length" class="p-5 space-y-3"><USkeleton v-for="i in 5" :key="i" class="h-28 w-full" /></div>
      <div v-else-if="!rows.length" class="p-5">
        <EmptyState icon="i-lucide-party-popper" tone="good" title="Antrean kosong"
          :desc="priority ? `Semua situs prioritas ${priority} sudah diverifikasi.` : 'Semua situs yang dinilai judol sudah diverifikasi.'"
          :action="priority ? { label: 'Lihat semua prioritas', onClick: () => (priority = '') } : undefined" />
      </div>

      <ul v-else class="divide-y divide-slate-800/70">
        <li v-for="(r, i) in rows" :key="r.id" :ref="el => (itemEls[i] = el as HTMLElement)" @click="cursor = i"
          class="relative px-5 py-4 flex flex-col lg:flex-row gap-4 transition"
          :class="[cursor === i ? 'bg-primary-500/[0.06]' : 'hover:bg-slate-800/20', leaving.has(r.id) && 'opacity-40']">
          <span v-if="cursor === i" class="absolute left-0 top-3 bottom-3 w-0.5 rounded-full bg-primary-500" />

          <!-- Thumbnail bukti -->
          <a :href="r.evidence_id ? shotURL(r.evidence_id) : undefined" target="_blank" rel="noopener"
            class="hidden sm:block w-40 h-24 shrink-0 rounded-lg overflow-hidden ring-1 ring-slate-800 bg-slate-950/70">
            <img v-if="r.evidence_id" :src="shotURL(r.evidence_id)" loading="lazy" alt="" class="w-full h-full object-cover object-top" />
            <div v-else class="w-full h-full flex flex-col items-center justify-center gap-1 text-slate-600">
              <UIcon name="i-lucide-image-off" class="w-5 h-5" /><span class="text-[11px]">belum ada bukti</span>
            </div>
          </a>

          <div class="flex-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <NuxtLink :to="`/site/${r.id}`" target="_blank" class="text-[15px] font-semibold text-slate-100 hover:text-primary-300 break-all">
                {{ host(r.url) }}
              </NuxtLink>
              <PriorityBadge :priority="r.priority" />
              <span class="text-[13px] tabular-nums text-slate-400">{{ Math.round(r.confidence * 100) }}%</span>
              <UBadge v-if="lifeMeta[r.lifecycle]" :color="lifeMeta[r.lifecycle].color" variant="subtle" size="xs">{{ lifeMeta[r.lifecycle].label }}</UBadge>
              <span class="ml-auto text-[12px] text-slate-500">{{ ago(r.created_at) }}</span>
            </div>

            <!-- Kenapa terdeteksi -->
            <p class="text-[13px] text-slate-400 mt-1.5">
              <UIcon name="i-lucide-scale" class="w-3.5 h-3.5 align-[-2px] mr-1 text-slate-500" />{{ ruleText(r.rule) }}
            </p>
            <div v-if="r.keywords?.length" class="flex flex-wrap gap-1 mt-1.5">
              <span v-for="k in r.keywords.slice(0, 8)" :key="k" class="px-1.5 py-0.5 rounded bg-rose-500/10 text-rose-300 text-[12px]">{{ k }}</span>
              <span v-if="r.keywords.length > 8" class="px-1.5 py-0.5 text-[12px] text-slate-500">+{{ r.keywords.length - 8 }}</span>
            </div>

            <!-- Bukti: menyala bila ada -->
            <div class="flex flex-wrap items-center gap-1.5 mt-2.5">
              <span v-for="e in evidenceChips(r)" :key="e.label" class="ev" :class="e.on && (e.strong ? 'ev-strong' : 'ev-on')">
                <UIcon :name="e.icon" class="w-3.5 h-3.5" />{{ e.label }}
              </span>
            </div>
          </div>

          <!-- Kekuatan bukti & keputusan -->
          <div class="lg:w-56 shrink-0 flex flex-col gap-2">
            <div class="flex items-center gap-2 text-[12px]">
              <span class="flex gap-0.5">
                <span v-for="n in 3" :key="n" class="w-4 h-1.5 rounded-full" :class="n <= strength(r).level ? strength(r).bar : 'bg-slate-800'" />
              </span>
              <span :class="strength(r).text">{{ strength(r).label }}</span>
            </div>
            <p class="text-[12px] text-slate-500 leading-snug">{{ strength(r).hint }}</p>
            <div class="flex gap-1.5 mt-auto">
              <UButton class="flex-1 justify-center" size="sm" color="rose" variant="soft" icon="i-lucide-shield-alert" label="Judol"
                :loading="busy === r.id + 'j'" @click.stop="decide(r, 'judol')" />
              <UButton class="flex-1 justify-center" size="sm" color="emerald" variant="soft" icon="i-lucide-shield-check" label="Bukan"
                :loading="busy === r.id + 'b'" @click.stop="decide(r, 'bukan_judol')" />
              <UTooltip text="Buka detail (O)">
                <UButton size="sm" color="gray" variant="ghost" icon="i-lucide-external-link" :to="`/site/${r.id}`" target="_blank" />
              </UTooltip>
            </div>
          </div>
        </li>
      </ul>

      <div v-if="total > limit" class="flex items-center justify-between px-5 py-4 border-t border-slate-800">
        <span class="text-[13px] text-slate-500">Halaman {{ page }} dari {{ Math.ceil(total / limit) }}</span>
        <UPagination v-model="page" :page-count="limit" :total="total" :max="7" />
      </div>
    </UCard>
  </div>
</template>

<script setup lang="ts">
const { get, post, apiBase } = useApi()
const toast = useToast()
const { name: operator } = useOperator()

const rows = ref<any[]>([])
const total = ref(0)
const stats = ref<any>({})
const loading = ref(true)
const busy = ref('')
const leaving = ref(new Set<number>())
const priority = ref('High')
const sort = ref('weak')
const includeStrong = ref(false) // bukti kuat = dikonfirmasi mesin; disembunyikan agar analis fokus ke yang meragukan
const page = ref(1)
const limit = 25
const cursor = ref(0)
const itemEls = ref<HTMLElement[]>([])
const TARGET = 30

const fmt = (n?: number) => (n ?? 0).toLocaleString('id-ID')
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/$/, '')
function ago(d?: string) {
  if (!d) return ''
  const m = Math.round((Date.now() - new Date(d).getTime()) / 60000)
  return m < 60 ? `ditemukan ${Math.max(1, m)} mnt lalu` : m < 1440 ? `ditemukan ${Math.round(m / 60)} jam lalu` : `ditemukan ${Math.round(m / 1440)} hari lalu`
}
const shotURL = (id: number) => `${apiBase}/evidence/${id}/files/screenshot-desktop.png${operator.value ? `?by=${encodeURIComponent(operator.value)}` : ''}`

const sortOptions = [
  { label: 'Bukti terlemah dulu', value: 'weak' },
  { label: 'Confidence tertinggi', value: 'confidence' },
  { label: 'Scan terbaru', value: 'recent' },
]
const sortHint = computed(() => ({
  weak: 'bukti terlemah ditampilkan duluan — paling mungkin salah deteksi',
  confidence: 'confidence tertinggi duluan — paling cepat dikonfirmasi',
  recent: 'scan terbaru duluan',
} as Record<string, string>)[sort.value])
const priorities = computed(() => [
  { label: 'High', value: 'High', n: stats.value.pending_high, color: 'rose', dot: 'bg-rose-500' },
  { label: 'Medium', value: 'Medium', n: stats.value.pending_medium, color: 'amber', dot: 'bg-amber-500' },
  { label: 'Low', value: 'Low', n: stats.value.pending_low, dot: 'bg-slate-500' },
  { label: 'Semua', value: '', n: stats.value.pending },
])

const kpis = computed(() => [
  { label: 'Perlu analis', value: stats.value.pending, hint: `bukti lemah/sedang · ${fmt(stats.value.machine_confirmed)} dikonfirmasi mesin`, icon: 'i-lucide-hourglass', iconColor: 'text-amber-400', color: 'text-white' },
  { label: 'Hari ini', value: stats.value.today, hint: 'keputusan dibuat', icon: 'i-lucide-calendar-check', iconColor: 'text-primary-400', color: 'text-white' },
  { label: 'Judol', value: stats.value.verified_judol, hint: 'terverifikasi', icon: 'i-lucide-shield-alert', iconColor: 'text-rose-400', color: 'text-rose-300' },
  { label: 'Bukan judol', value: stats.value.verified_not, hint: 'false positive dikoreksi', icon: 'i-lucide-shield-check', iconColor: 'text-emerald-400', color: 'text-emerald-300' },
])
const learnBars = computed(() => [
  { label: 'Judol', n: stats.value.verified_judol ?? 0, bar: 'bg-rose-500' },
  { label: 'Bukan judol', n: stats.value.verified_not ?? 0, bar: 'bg-emerald-500' },
])
const learnReady = computed(() => learnBars.value.every(b => b.n >= TARGET))
// Engine belajar dari perbedaan: hanya memverifikasi satu sisi tidak cukup
const learnHint = computed(() => {
  const [j, b] = learnBars.value.map(x => x.n)
  if (j >= 10 && b < j / 4) return 'Contoh "bukan judol" masih sedikit — urutkan "Bukti terlemah dulu" untuk menemukan false positive.'
  if (b >= 10 && j < b / 4) return 'Contoh "judol" masih sedikit — urutkan "Confidence tertinggi" untuk konfirmasi cepat.'
  return ''
})

const RULES: Record<string, string> = {
  'game_provider+artifact': 'Provider game + artefak transaksi',
  'content+artifact': 'Konten judol kuat + artefak/kontak',
  'template+artifact': 'Template mirip situs judol + artefak',
  'enrichment+artifact': 'Reputasi buruk + artefak',
  'hostname+artifact': 'Nama domain berbau judol + artefak',
  strong_signal: 'Satu sinyal kuat (konten/provider/reputasi)',
  medium_signal: 'Sinyal sedang (kata kunci, TLD, atau URL)',
  not_gambling_cap: 'Sinyal kuat tapi dibatasi (situs institusi)',
}
const ruleText = (r: string) => (r ? `Terdeteksi karena: ${RULES[r] ?? r}` : 'Alasan deteksi belum tercatat (scan sebelum fitur ini)')

const lifeMeta: Record<string, { label: string; color: any }> = {
  blocked_alive: { label: 'lolos blokir', color: 'orange' },
  uncertain: { label: 'belum pasti', color: 'amber' },
  inactive: { label: 'tidak aktif', color: 'gray' },
}

const evidenceChips = (r: any) => [
  { label: `${r.money} rekening`, icon: 'i-lucide-landmark', on: r.money > 0, strong: true },
  { label: `${r.providers} provider game`, icon: 'i-lucide-dice-5', on: r.providers > 0, strong: true },
  { label: r.clustered ? 'dalam klaster' : 'tanpa klaster', icon: 'i-lucide-network', on: r.clustered, strong: true },
  { label: `${r.contacts ?? 0} kontak`, icon: 'i-lucide-message-circle', on: (r.contacts ?? 0) > 0, strong: false },
  { label: `${r.apps ?? 0} aplikasi`, icon: 'i-lucide-smartphone', on: (r.apps ?? 0) > 0, strong: false },
]

// Kekuatan bukti: rekening/provider/klaster = bukti kuat; kontak/aplikasi + banyak kata kunci = sedang
function strength(r: any) {
  if (r.money > 0 || r.providers > 0 || r.clustered)
    return { level: 3, label: 'Bukti kuat', bar: 'bg-rose-500', text: 'text-rose-300', hint: 'Ada rekening, provider game, atau klaster operator — biasanya cukup dikonfirmasi.' }
  if (((r.contacts ?? 0) > 0 || (r.apps ?? 0) > 0) && (r.keywords?.length ?? 0) >= 3)
    return { level: 2, label: 'Bukti sedang', bar: 'bg-amber-500', text: 'text-amber-300', hint: 'Kontak/aplikasi + kata kunci judi. Lihat thumbnail atau detail sebelum memutuskan.' }
  return { level: 1, label: 'Bukti lemah', bar: 'bg-slate-500', text: 'text-slate-300', hint: 'Hanya kata kunci — rawan salah deteksi (berita, ulasan, exchange). Buka detailnya.' }
}

async function load() {
  loading.value = true
  try {
    const d = await get<any>('/verification/queue', { priority: priority.value || undefined, sort: sort.value, limit, offset: (page.value - 1) * limit, include_strong: includeStrong.value ? 1 : undefined })
    rows.value = d?.results ?? []
    total.value = d?.total ?? 0
    stats.value = d?.stats ?? {}
    cursor.value = Math.min(cursor.value, Math.max(0, rows.value.length - 1))
  } catch {} finally { loading.value = false }
}

async function decide(r: any, status: 'judol' | 'bukan_judol') {
  if (!operator.value) {
    toast.add({ title: 'Isi nama dulu', description: 'Klik tombol nama di kanan atas.', color: 'amber', icon: 'i-lucide-user-round' })
    return
  }
  busy.value = r.id + (status === 'judol' ? 'j' : 'b')
  try {
    await post(`/sites/${r.id}/verify`, { status, note: '' })
    leaving.value = new Set(leaving.value).add(r.id)
    await new Promise(res => setTimeout(res, 180))
    const idx = rows.value.findIndex(x => x.id === r.id)
    rows.value = rows.value.filter(x => x.id !== r.id)
    cursor.value = Math.min(idx, rows.value.length - 1)
    total.value--
    stats.value.pending--
    stats.value.today = (stats.value.today ?? 0) + 1
    const key = r.priority === 'High' ? 'pending_high' : r.priority === 'Medium' ? 'pending_medium' : 'pending_low'
    stats.value[key]--
    status === 'judol' ? stats.value.verified_judol++ : stats.value.verified_not++
    toast.add({
      title: `${host(r.url)} → ${status === 'judol' ? 'judol' : 'bukan judol'}`,
      icon: 'i-lucide-check', color: status === 'judol' ? 'rose' : 'emerald', timeout: 6000,
      actions: [{ label: 'Batalkan', click: async () => { await post(`/sites/${r.id}/verify`, { status: 'reset' }).catch(() => {}); load() } }],
    })
    if (rows.value.length < 5 && total.value > rows.value.length) load()
  } catch (e: any) {
    toast.add({ title: 'Gagal menyimpan', description: e.message, color: 'rose' })
  } finally { busy.value = '' }
}

// Pintasan keyboard — verifikasi cepat tanpa mouse
function onKey(e: KeyboardEvent) {
  const t = e.target as HTMLElement
  if (e.metaKey || e.ctrlKey || e.altKey || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t?.tagName) || t?.isContentEditable) return
  const r = rows.value[cursor.value]
  const move = (d: number) => {
    cursor.value = Math.max(0, Math.min(rows.value.length - 1, cursor.value + d))
    itemEls.value[cursor.value]?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }
  switch (e.key.toLowerCase()) {
    case 'j': if (r && !busy.value) decide(r, 'judol'); break
    case 'b': if (r && !busy.value) decide(r, 'bukan_judol'); break
    case 'arrowdown': case 'k': e.preventDefault(); move(1); break
    case 'arrowup': e.preventDefault(); move(-1); break
    case 'o': if (r) window.open(`/site/${r.id}`, '_blank'); break
    default: return
  }
}
onMounted(() => { load(); window.addEventListener('keydown', onKey) })
onUnmounted(() => window.removeEventListener('keydown', onKey))

watch([priority, sort, includeStrong], () => { cursor.value = 0; page.value === 1 ? load() : (page.value = 1) })
watch(page, () => { cursor.value = 0; load() })
useHead({ title: 'Antrean Verifikasi — Jutec' })
</script>

<style scoped>
.ev { @apply inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[12px] bg-slate-800/40 text-slate-600 ring-1 ring-slate-800; }
.ev-on { @apply text-slate-200 bg-slate-800 ring-slate-700; }
.ev-strong { @apply text-rose-200 bg-rose-500/10 ring-rose-500/30; }
kbd { @apply inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 mr-1 rounded bg-slate-800 ring-1 ring-slate-700 text-[10px] font-mono text-slate-300; }
</style>
