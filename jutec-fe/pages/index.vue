<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Overview" title="Dashboard Intelijen"
      subtitle="Apa yang perlu ditindaklanjuti hari ini, perkembangan ancaman, dan jaringan operator yang paling aktif."
      icon="i-lucide-layout-dashboard">
      <template #actions>
        <UButton icon="i-lucide-refresh-cw" color="gray" variant="soft" size="lg" label="Muat Ulang" :loading="loading"
          @click="load(); sitesTable?.reload()" />
        <UButton icon="i-lucide-satellite-dish" size="lg" label="Deep Sweep" to="/discovery?mode=crawler" />
      </template>
    </PageHeader>

    <!-- Deep Sweep yang sedang berjalan di server -->
    <SweepBanner />

    <!-- ── 2. KPI dengan tren ── -->
    <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-4">
      <StatCard label="Judol Beroperasi" :value="k.live ?? '—'" icon="i-lucide-globe" color="indigo"
        :delta="ov ? k.live - k.live_prev : null" up-is-bad :hint="ov ? `${fmt(k.inactive)} tidak aktif · ${fmt(k.total)} tercatat` : ''" />
      <StatCard label="Baru 7 Hari" :value="k.new_7d ?? '—'" icon="i-lucide-sparkles" color="rose"
        :delta="ov ? k.new_7d - k.new_prev_7d : null" up-is-bad />
      <StatCard label="Diblokir 7 Hari" :value="k.blocked_7d ?? '—'" icon="i-lucide-shield-ban" color="emerald"
        :delta="ov ? k.blocked_7d - k.blocked_prev_7d : null" :hint="ov ? `${fmt(k.died_7d)} situs mati minggu ini` : ''" />
      <StatCard label="Operator Aktif" :value="k.active_operators ?? '—'" icon="i-lucide-network" color="purple"
        hint="Klaster dengan ≥1 situs beroperasi" />
      <StatCard label="Rekening Aktif" :value="k.active_money ?? '—'" icon="i-lucide-landmark" color="amber"
        hint="Bank, e-wallet, QRIS & kripto di situs beroperasi" />
    </div>

    <!-- ── 3. Tren & komposisi ── -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <UCard class="xl:col-span-8">
        <template #header>
          <h2 class="section-title">Situs Baru vs Diblokir vs Mati</h2>
          <p class="section-desc">30 hari terakhir — apakah ancaman bertambah dan penindakan berdampak?</p>
        </template>
        <TrendChart v-if="hasTrend" :data="ov.trend" :series="trendSeries" />
        <EmptyState v-else icon="i-lucide-chart-line" title="Belum ada pergerakan dalam 30 hari"
          desc="Grafik terisi setelah situs judol ditemukan, diblokir, atau mati. Jalankan Deep Sweep untuk mulai mengumpulkan data."
          :action="{ label: 'Mulai Deep Sweep', to: '/discovery?mode=crawler', icon: 'i-lucide-satellite-dish' }" />
      </UCard>

      <UCard class="xl:col-span-4">
        <template #header>
          <h2 class="section-title">Status Situs Judol</h2>
          <p class="section-desc">{{ fmt(lifeTotal) }} situs tercatat</p>
        </template>
        <div v-if="lifeTotal">
          <div class="flex h-3 rounded-full overflow-hidden gap-0.5">
            <div v-for="l in lifeRows.filter(r => r.n)" :key="l.key" :class="l.bar" :style="{ width: `${(l.n / lifeTotal) * 100}%` }" />
          </div>
          <ul class="mt-4 space-y-2.5">
            <li v-for="l in lifeRows" :key="l.key" class="flex items-center gap-3 text-sm">
              <span class="w-2.5 h-2.5 rounded-sm shrink-0" :class="l.bar" />
              <span class="text-slate-200">{{ l.label }}</span>
              <span class="text-[12px] text-slate-500 hidden sm:inline">{{ l.hint }}</span>
              <span class="ml-auto tabular-nums font-semibold text-slate-100">{{ fmt(l.n) }}</span>
              <span class="w-10 text-right tabular-nums text-[12px] text-slate-500">{{ Math.round((l.n / lifeTotal) * 100) }}%</span>
            </li>
          </ul>
        </div>
        <EmptyState v-else icon="i-lucide-pie-chart" title="Belum ada situs judol"
          desc="Komposisi online, lolos blokir & tidak aktif muncul setelah situs pertama terdeteksi." />
      </UCard>
    </div>

    <!-- ── 4. Peringkat yang mengarah ke operator ── -->
    <div class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-4 gap-4">
      <div v-for="r in ranks" :key="r.key" class="rounded-2xl bg-slate-900/60 ring-1 ring-slate-800 p-5 flex flex-col min-h-[300px]">
        <!-- Header ringkas -->
        <div class="flex items-start gap-3">
          <div class="w-9 h-9 rounded-xl flex items-center justify-center ring-1 shrink-0" :class="r.box">
            <UIcon :name="r.icon" class="w-[18px] h-[18px]" :class="r.text" />
          </div>
          <div class="flex-1 min-w-0">
            <h3 class="text-sm font-semibold text-slate-100 leading-tight">{{ r.title }}</h3>
            <p class="text-[12px] text-slate-500 mt-0.5 leading-snug">{{ r.desc }}</p>
          </div>
          <NuxtLink v-if="r.more && r.rows.length" :to="r.more" class="text-[12px] text-slate-500 hover:text-primary-300 shrink-0 mt-0.5">Semua</NuxtLink>
        </div>

        <EmptyState v-if="!r.rows.length" compact :icon="r.icon" title="Belum ada data" :desc="r.empty" class="mt-4 flex-1" />
        <template v-else>
          <div class="flex justify-end text-[11px] uppercase tracking-wider text-slate-600 mt-4 mb-1.5">situs aktif</div>
          <ol class="space-y-2.5 flex-1">
            <li v-for="(x, i) in r.rows" :key="i" class="group">
              <div class="flex items-baseline gap-2 min-w-0">
                <span class="text-[11px] tabular-nums w-3 shrink-0" :class="i === 0 ? r.text : 'text-slate-600'">{{ i + 1 }}</span>
                <component :is="x.to ? NuxtLinkCmp : 'span'" :to="x.to" class="text-[13px] truncate flex-1 min-w-0"
                  :class="[x.mono && 'mono', i === 0 ? 'text-white font-medium' : 'text-slate-300', x.to && 'hover:text-primary-300']">{{ x.label }}</component>
                <span class="text-sm font-semibold tabular-nums shrink-0" :class="i === 0 ? 'text-white' : 'text-slate-200'">{{ x.active }}</span>
              </div>
              <!-- Bar proporsional terhadap nilai terbesar di kartu ini -->
              <div class="flex items-center gap-2 mt-1 pl-5">
                <div class="flex-1 h-1 rounded-full bg-slate-800 overflow-hidden">
                  <div class="h-full rounded-full transition-all" :class="r.bar" :style="{ width: `${Math.max(4, (x.active / r.max) * 100)}%`, opacity: i === 0 ? 1 : 0.6 }" />
                </div>
                <span class="text-[11px] text-slate-500 shrink-0 truncate max-w-[55%]">
                  <template v-if="x.sub">{{ x.sub }} · </template>dari {{ x.total }}
                </span>
              </div>
            </li>
          </ol>
        </template>
      </div>
    </div>

    <!-- ── Butuh tindakan: kategori di kiri, isi kategori terpilih di kanan ── -->
    <UCard :ui="{ body: { padding: 'p-0' }, header: { padding: 'px-5 py-4' } }">
      <template #header>
        <div class="flex flex-wrap items-center gap-3">
          <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-list-todo" class="w-[18px] h-[18px] text-amber-400" /></div>
          <div class="flex-1 min-w-0">
            <h2 class="card-title">Butuh Tindakan</h2>
            <p class="card-desc">Situs & temuan yang menunggu keputusan atau langkah lanjutan</p>
          </div>
          <UBadge :color="actionTotal ? 'amber' : 'emerald'" variant="subtle">{{ actionTotal ? `${fmt(actionTotal)} menunggu` : 'semua beres' }}</UBadge>
        </div>
      </template>

      <div class="grid grid-cols-1 lg:grid-cols-12">
        <!-- Daftar kategori -->
        <nav class="lg:col-span-4 xl:col-span-3 p-2 lg:border-r border-b lg:border-b-0 border-slate-800 flex lg:flex-col gap-1 overflow-x-auto no-scrollbar">
          <button v-for="a in actions" :key="a.key" type="button" @click="actionKey = a.key"
            class="shrink-0 lg:shrink text-left rounded-lg px-3 py-2.5 flex items-center gap-3 transition ring-1"
            :class="activeAction?.key === a.key ? 'bg-slate-800/70 ring-slate-700' : 'ring-transparent hover:bg-slate-800/40'">
            <div class="w-8 h-8 rounded-lg flex items-center justify-center ring-1 shrink-0" :class="a.count ? a.box : 'bg-slate-800/60 ring-slate-700'">
              <UIcon :name="a.icon" class="w-4 h-4" :class="a.count ? a.text : 'text-slate-500'" />
            </div>
            <span class="text-sm flex-1 whitespace-nowrap" :class="activeAction?.key === a.key ? 'text-white font-medium' : 'text-slate-300'">{{ a.title }}</span>
            <span class="text-sm font-semibold tabular-nums rounded-md px-2 py-0.5 min-w-[2rem] text-center"
              :class="a.count ? `${a.box} ${a.text} ring-1` : 'text-slate-600'">{{ fmt(a.count) }}</span>
          </button>
        </nav>

        <!-- Isi kategori terpilih -->
        <div v-if="activeAction" class="lg:col-span-8 xl:col-span-9 p-5 flex flex-col min-h-[260px]">
          <div class="flex items-start gap-3 mb-4">
            <div class="flex-1 min-w-0">
              <h3 class="text-base font-semibold text-slate-100">{{ activeAction.title }}</h3>
              <p class="text-[13px] text-slate-400 mt-0.5">{{ activeAction.desc }}</p>
            </div>
            <UButton v-if="activeAction.more && activeAction.count" :to="activeAction.more" size="sm" color="gray" variant="soft"
              :label="activeAction.cta" trailing-icon="i-lucide-arrow-right" />
          </div>

          <EmptyState v-if="!activeAction.items.length" class="flex-1" :tone="noData ? 'neutral' : 'good'"
            :icon="noData ? 'i-lucide-hourglass' : 'i-lucide-check'" :title="noData ? 'Menunggu data' : activeAction.emptyTitle"
            :desc="noData ? 'Muncul setelah situs pertama ditemukan.' : 'Tidak ada yang perlu ditindaklanjuti di kategori ini.'" />

          <ul v-else class="grid sm:grid-cols-2 gap-2">
            <li v-for="it in activeAction.items" :key="it.key">
              <NuxtLink :to="it.to" class="group flex items-center gap-3 rounded-lg bg-slate-950/50 ring-1 ring-slate-800 hover:ring-slate-600 px-3 py-2.5 transition">
                <span class="w-1.5 h-8 rounded-full shrink-0" :class="activeAction.dot" />
                <span class="min-w-0 flex-1 mono text-[13px] text-slate-100 truncate group-hover:text-primary-300">{{ it.label }}</span>
                <span class="text-[12px] tabular-nums text-slate-400 shrink-0">{{ it.sub }}</span>
                <UIcon name="i-lucide-chevron-right" class="w-4 h-4 text-slate-600 group-hover:text-slate-300 shrink-0" />
              </NuxtLink>
            </li>
          </ul>
          <p v-if="activeAction.count > activeAction.items.length" class="text-[12px] text-slate-500 mt-3">
            Menampilkan {{ activeAction.items.length }} dari {{ fmt(activeAction.count) }}.
          </p>
        </div>
      </div>
    </UCard>

    <!-- ── 5. Situs terbaru & kesehatan sistem ── -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <div class="xl:col-span-8 min-w-0">
        <SitesTable ref="sitesTable" title="Domain Judol Terbaru" gambling-only live analyze-path="/analyze" :default-limit="10" />
      </div>

      <SystemHealthCard class="xl:col-span-4" :sys="sys" />
    </div>
  </div>
</template>

<script setup lang="ts">
const { get } = useApi()

const sitesTable = ref<any>(null)
const ov = ref<any>(null)
const sys = ref<any>({})
const loading = ref(false)
const k = computed(() => ov.value?.kpi ?? {})

const fmt = (n?: number) => (n ?? 0).toLocaleString('id-ID')
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/$/, '')
const day = (d?: string) => d ? new Date(d).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' }) : ''
function ago(d?: string) {
  if (!d) return '—'
  const m = Math.round((Date.now() - new Date(d).getTime()) / 60000)
  return m < 1 ? 'baru saja' : m < 60 ? `${m} mnt lalu` : m < 1440 ? `${Math.round(m / 60)} jam lalu` : `${Math.round(m / 1440)} hari lalu`
}
function inTime(d?: string) {
  if (!d) return '—'
  const m = Math.max(0, Math.round((new Date(d).getTime() - Date.now()) / 60000))
  return m < 1 ? 'sebentar lagi' : m < 60 ? `${m} mnt lagi` : `${Math.round(m / 60)} jam lagi`
}

// Warna tren tervalidasi (dataviz validator, mode gelap, surface #0f172a): lolos band, chroma, CVD & kontras
const trendSeries = [
  { key: 'new', label: 'Baru', color: '#e11d48' },
  { key: 'blocked', label: 'Diblokir', color: '#d97706' },
  { key: 'died', label: 'Mati', color: '#0891b2' },
]

const siteItems = (list: any, sub: (x: any) => string) => (list?.items ?? []).map((x: any) => ({ key: x.id, label: host(x.url), to: `/site/${x.id}`, sub: sub(x) }))
const actions = computed(() => {
  const a = ov.value?.actions ?? {}
  return [
    { key: 'unverified', cta: 'Buka antrean verifikasi', dot: 'bg-amber-500', title: 'Belum diverifikasi', icon: 'i-lucide-badge-help', count: a.unverified?.count ?? 0,
      desc: 'Bukti lemah/sedang — perlu keputusan analis. Situs dengan bukti kuat sudah dikonfirmasi mesin.',
      ring: 'ring-amber-500/40', box: 'bg-amber-500/10 ring-amber-500/25', text: 'text-amber-400',
      items: siteItems(a.unverified, x => `${Math.round(x.confidence * 100)}%`), more: '/verifikasi', emptyTitle: 'Semua sudah diverifikasi' },
    { key: 'ready_block', cta: '', dot: 'bg-emerald-500', title: 'Siap diajukan blokir', icon: 'i-lucide-gavel', count: a.ready_block?.count ?? 0,
      desc: 'Judol pasti (terverifikasi/bukti kuat), bukti forensik lengkap, dan masih bisa diakses — siap dilaporkan.',
      ring: 'ring-emerald-500/40', box: 'bg-emerald-500/10 ring-emerald-500/25', text: 'text-emerald-400',
      items: siteItems(a.ready_block, x => `${Math.round(x.confidence * 100)}%`), more: '', emptyTitle: 'Belum ada yang siap' },
    { key: 'revived', cta: '', dot: 'bg-rose-500', title: 'Hidup lagi', icon: 'i-lucide-rotate-ccw', count: a.revived?.count ?? 0,
      desc: 'Situs mati yang aktif kembali ≤30 hari — operator memakai ulang domain',
      ring: 'ring-rose-500/40', box: 'bg-rose-500/10 ring-rose-500/25', text: 'text-rose-400',
      items: siteItems(a.revived, x => day(x.at)), more: '', emptyTitle: 'Tidak ada yang hidup lagi' },
    { key: 'reappeared', cta: 'Buka repository', dot: 'bg-rose-500', title: 'Rekening muncul lagi', icon: 'i-lucide-repeat', count: a.reappeared?.length ?? 0,
      desc: 'Rekening/kontak situs mati kini dipakai situs baru',
      ring: 'ring-rose-500/40', box: 'bg-rose-500/10 ring-rose-500/25', text: 'text-rose-400',
      items: (a.reappeared ?? []).map((x: any) => ({ key: x.value, label: x.value, sub: x.name || x.type,
        to: `/reverse-lookup?type=${encodeURIComponent(x.type)}&v=${encodeURIComponent(x.value)}` })), more: '/repository?tab=artifacts', emptyTitle: 'Tidak ada yang berulang' },
    { key: 'blocked_alive', cta: 'Lihat semua situs', dot: 'bg-orange-500', title: 'Lolos blokir', icon: 'i-lucide-shield-off', count: a.blocked_alive?.count ?? 0,
      desc: 'Diblokir ISP tapi server masih melayani pemain lewat VPN/DNS lain',
      ring: 'ring-orange-500/40', box: 'bg-orange-500/10 ring-orange-500/25', text: 'text-orange-400',
      items: siteItems(a.blocked_alive, x => x.at ? `sejak ${day(x.at)}` : ''), more: '/repository?tab=sites', emptyTitle: 'Tidak ada yang lolos' },
    { key: 'no_evidence', cta: '', dot: 'bg-sky-500', title: 'Bukti belum diambil', icon: 'i-lucide-camera-off', count: a.no_evidence?.count ?? 0,
      desc: 'High beroperasi tanpa rekaman bukti forensik',
      ring: 'ring-sky-500/40', box: 'bg-sky-500/10 ring-sky-500/25', text: 'text-sky-400',
      items: siteItems(a.no_evidence, x => `${Math.round(x.confidence * 100)}%`), more: '', emptyTitle: 'Semua sudah ada bukti' },
  ]
})

const NuxtLinkCmp = resolveComponent('NuxtLink')
const actionKey = ref('')
const actionTotal = computed(() => actions.value.reduce((n, a) => n + (a.count ?? 0), 0))
// Kategori aktif: pilihan pengguna, atau kategori pertama yang berisi
const activeAction = computed(() => actions.value.find(a => a.key === actionKey.value) ?? actions.value.find(a => a.count) ?? actions.value[0])

const lifeRows = computed(() => {
  const l = ov.value?.lifecycle ?? {}
  return [
    { key: 'active', label: 'Online', hint: '', n: l.active ?? 0, bar: 'bg-emerald-500' },
    { key: 'blocked_alive', label: 'Lolos blokir', hint: 'diblokir, masih hidup', n: l.blocked_alive ?? 0, bar: 'bg-orange-500' },
    { key: 'uncertain', label: 'Belum pasti', hint: 'gagal diakses <14 hari', n: l.uncertain ?? 0, bar: 'bg-amber-300' },
    { key: 'inactive', label: 'Tidak aktif', hint: 'mati / domain hilang', n: l.inactive ?? 0, bar: 'bg-slate-600' },
  ]
})
// Database masih kosong → kartu "aman" menyesatkan; tampilkan "menunggu data" sampai situs pertama tercatat
const noData = computed(() => !!ov.value && !k.value.total)
const hasTrend = computed(() => (ov.value?.trend ?? []).some((d: any) => d.new || d.died || d.blocked))
const lifeTotal = computed(() => lifeRows.value.reduce((s, r) => s + r.n, 0))

const ranks = computed(() => {
  const o = ov.value ?? {}
  const list = [
    { key: 'ops', title: 'Operator Teraktif', desc: 'Klaster dengan situs terbanyak', icon: 'i-lucide-network', more: '/repository?tab=operators',
      box: 'bg-rose-500/10 ring-rose-500/25', text: 'text-rose-400', bar: 'bg-rose-500', empty: 'Klaster terbentuk saat ≥2 situs berbagi atribut kuat',
      rows: (o.top_operators ?? []).map((x: any) => ({ label: x.label, sub: x.sub, active: x.active_sites, total: x.total_sites, mono: true })) },
    { key: 'money', title: 'Rekening Terbanyak', desc: 'Rekening & e-wallet lintas situs', icon: 'i-lucide-landmark', more: '/repository?tab=artifacts',
      box: 'bg-violet-500/10 ring-violet-500/25', text: 'text-violet-400', bar: 'bg-violet-500', empty: 'Rekening & e-wallet dari situs yang masih beroperasi',
      rows: (o.top_money ?? []).map((x: any) => ({ label: x.value, sub: x.name || x.type, active: x.active_sites, total: x.total_sites, mono: true,
        to: `/reverse-lookup?type=${encodeURIComponent(x.type)}&v=${encodeURIComponent(x.value)}` })) },
    { key: 'prov', title: 'Provider Game', desc: 'Penyedia game di situs judol', icon: 'i-lucide-gamepad-2', more: '',
      box: 'bg-amber-500/10 ring-amber-500/25', text: 'text-amber-400', bar: 'bg-amber-500', empty: 'Terdeteksi dari panggilan API game saat crawl',
      rows: (o.top_providers ?? []).map((x: any) => ({ label: x.label, active: x.active_sites, total: x.total_sites })) },
    { key: 'hosts', title: 'Hosting (non-CDN)', desc: 'IP server asli yang dipakai bersama', icon: 'i-lucide-server', more: '',
      box: 'bg-cyan-500/10 ring-cyan-500/25', text: 'text-cyan-400', bar: 'bg-cyan-500', empty: 'IP server asli yang dipakai ≥2 situs aktif',
      rows: (o.top_hosts ?? []).map((x: any) => ({ label: x.label, sub: x.sub, active: x.active_sites, total: x.total_sites, mono: true, to: `/ip/${x.label}` })) },
  ]
  return list.map(r => ({ ...r, max: Math.max(1, ...r.rows.map((x: any) => x.active)) }))
})

async function load() {
  loading.value = true
  try {
    const d = await get<any>('/data/overview')
    ov.value = d?.overview ?? null
    sys.value = d?.system ?? {}
  } catch {} finally { loading.value = false }
}

let poll: any = null
onMounted(() => { load(); poll = setInterval(load, 30_000) })
onUnmounted(() => clearInterval(poll))
</script>

<style scoped>
.no-scrollbar { scrollbar-width: none; }
.no-scrollbar::-webkit-scrollbar { display: none; }
</style>
