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

    <!-- Sweep yang sedang berjalan (state dibagi dengan halaman Discovery) -->
    <UAlert v-if="sweepRunning" icon="i-lucide-loader-circle" color="primary" variant="subtle"
      title="Deep Sweep sedang berjalan" :description="`${elapsed} · ${foundCount} target ditemukan sejauh ini`"
      :actions="[{ label: 'Lihat progres', to: '/discovery?mode=crawler', color: 'primary', variant: 'solid' }]" />

    <!-- ── 1. Butuh tindakan ── -->
    <section>
      <div class="flex items-end justify-between mb-3">
        <div>
          <h2 class="section-title">Butuh Tindakan</h2>
          <p class="section-desc">Situs & temuan yang menunggu keputusan atau langkah lanjutan</p>
        </div>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-5 gap-4">
        <div v-for="a in actions" :key="a.key" class="rounded-2xl bg-slate-900/60 ring-1 p-4 flex flex-col"
          :class="a.count ? a.ring : 'ring-slate-800'">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg flex items-center justify-center ring-1 shrink-0" :class="a.count ? a.box : 'bg-slate-800/60 ring-slate-700'">
              <UIcon :name="a.icon" class="w-4 h-4" :class="a.count ? a.text : 'text-slate-500'" />
            </div>
            <p class="text-sm font-medium text-slate-200 leading-tight flex-1">{{ a.title }}</p>
            <span class="text-2xl font-semibold tabular-nums" :class="a.count ? 'text-white' : 'text-slate-600'">{{ a.count }}</span>
          </div>
          <p class="text-[12px] text-slate-500 mt-1.5 leading-snug">{{ a.desc }}</p>
          <ul v-if="a.items.length" class="mt-3 space-y-1 flex-1">
            <li v-for="it in a.items" :key="it.key" class="flex items-center gap-2 text-[13px] min-w-0">
              <NuxtLink :to="it.to" class="mono text-slate-300 hover:text-primary-300 truncate">{{ it.label }}</NuxtLink>
              <span class="ml-auto text-slate-500 shrink-0">{{ it.sub }}</span>
            </li>
          </ul>
          <p v-else class="mt-3 text-[13px] text-slate-600 flex-1">Tidak ada — aman.</p>
          <UButton v-if="a.more && a.count" :to="a.more" size="2xs" color="gray" variant="ghost" class="mt-2 self-start"
            :label="a.count > a.items.length ? `Lihat semua (${a.count})` : 'Buka'" trailing-icon="i-lucide-arrow-right" />
        </div>
      </div>
    </section>

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
        <TrendChart v-if="ov?.trend?.length" :data="ov.trend" :series="trendSeries" />
        <div v-else class="empty-state !py-16"><p>Belum ada data tren.</p></div>
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
        <div v-else class="empty-state !py-10"><p>Belum ada situs judol.</p></div>
      </UCard>
    </div>

    <!-- ── 4. Peringkat yang mengarah ke operator ── -->
    <div class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-4 gap-4">
      <UCard v-for="r in ranks" :key="r.key" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-3' } }">
        <template #header>
          <div class="flex items-center gap-2.5">
            <UIcon :name="r.icon" class="w-4 h-4 text-slate-400" />
            <h3 class="text-sm font-semibold text-slate-100 flex-1">{{ r.title }}</h3>
            <NuxtLink v-if="r.more" :to="r.more" class="text-[12px] text-slate-500 hover:text-primary-300">Semua →</NuxtLink>
          </div>
        </template>
        <p v-if="!r.rows.length" class="text-[13px] text-slate-600 py-3">Belum ada data.</p>
        <ol v-else class="divide-y divide-slate-800/60 -my-1">
          <li v-for="(x, i) in r.rows" :key="i" class="py-2 flex items-center gap-2.5 min-w-0">
            <span class="text-[12px] text-slate-600 tabular-nums w-3">{{ i + 1 }}</span>
            <div class="min-w-0 flex-1">
              <component :is="x.to ? resolveComponent('NuxtLink') : 'span'" :to="x.to"
                class="block text-[13px] truncate" :class="[x.mono && 'mono', x.to ? 'text-slate-200 hover:text-primary-300' : 'text-slate-200']">{{ x.label }}</component>
              <p v-if="x.sub" class="text-[11px] text-slate-500 truncate">{{ x.sub }}</p>
            </div>
            <span class="text-[13px] tabular-nums text-slate-100 font-semibold shrink-0">{{ x.active }}</span>
            <span class="text-[11px] tabular-nums text-slate-500 shrink-0 w-8">/{{ x.total }}</span>
          </li>
        </ol>
        <p v-if="r.rows.length" class="text-[11px] text-slate-600 mt-1">situs aktif / total</p>
      </UCard>
    </div>

    <!-- ── 5. Situs terbaru & kesehatan sistem ── -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <div class="xl:col-span-8 min-w-0">
        <SitesTable ref="sitesTable" title="Domain Judol Terbaru" gambling-only live analyze-path="/analyze" :default-limit="10" />
      </div>

      <UCard class="xl:col-span-4" :ui="{ body: { padding: 'px-5 py-4' } }">
        <template #header>
          <div class="flex items-center gap-2.5">
            <span class="w-2 h-2 rounded-full" :class="sys.redis && sys.servers ? 'bg-emerald-500' : 'bg-rose-500'" />
            <h2 class="section-title flex-1">Kesehatan Sistem</h2>
            <span class="text-[12px] text-slate-500">{{ sys.redis ? `${sys.servers} worker aktif` : 'Redis tidak terhubung' }}</span>
          </div>
        </template>

        <p class="text-[12px] uppercase tracking-wider text-slate-500 mb-2">Antrean tugas</p>
        <p v-if="!sys.queues?.length" class="text-[13px] text-slate-500">{{ sys.redis ? 'Belum ada antrean.' : 'Worker background nonaktif — scan otomatis & pengayaan tidak berjalan.' }}</p>
        <table v-else class="w-full text-[13px] tabular-nums">
          <thead class="text-[11px] text-slate-500 uppercase"><tr><th class="text-left font-medium pb-1">Antrean</th><th class="text-right font-medium">Antre</th><th class="text-right font-medium">Jalan</th><th class="text-right font-medium">Selesai*</th><th class="text-right font-medium">Gagal*</th></tr></thead>
          <tbody>
            <tr v-for="q in sys.queues" :key="q.name" class="text-slate-300">
              <td class="py-1">{{ q.name }}<UBadge v-if="q.paused" color="amber" variant="subtle" size="xs" class="ml-1">jeda</UBadge></td>
              <td class="text-right" :class="q.pending > 100 && 'text-amber-400'">{{ q.pending + q.scheduled + q.retry }}</td>
              <td class="text-right">{{ q.active }}</td>
              <td class="text-right">{{ q.processed_today }}</td>
              <td class="text-right" :class="q.failed_today && 'text-rose-400'">{{ q.failed_today }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="sys.queues?.length" class="text-[11px] text-slate-600 mt-1">* hari ini</p>
        <p v-if="sys.scheduler" class="text-[12px] text-slate-500 mt-2">
          {{ sys.scheduler.jobs }} jadwal otomatis · terakhir {{ ago(sys.scheduler.last_at) }} · berikutnya {{ inTime(sys.scheduler.next_at) }}
        </p>

        <p class="text-[12px] uppercase tracking-wider text-slate-500 mt-5 mb-2">Kunci API</p>
        <ul class="space-y-1.5">
          <li v-for="key in sys.api_keys ?? []" :key="key.name" class="flex items-center gap-2 text-[13px]">
            <UIcon :name="key.set ? 'i-lucide-circle-check' : key.free_tier ? 'i-lucide-circle-dashed' : 'i-lucide-circle-x'" class="w-4 h-4 shrink-0"
              :class="key.set ? 'text-emerald-400' : key.free_tier ? 'text-slate-500' : key.required ? 'text-rose-400' : 'text-amber-400'" />
            <span class="text-slate-300">{{ key.name }}</span>
            <span class="ml-auto text-[12px] text-slate-500">{{ key.set ? 'terpasang' : key.free_tier ? 'kuota gratis' : 'tidak ada — nonaktif' }}</span>
          </li>
        </ul>
      </UCard>
    </div>
  </div>
</template>

<script setup lang="ts">
const { get } = useApi()
const { loading: sweepRunning, logs, elapsed } = useDiscoveryState()
const foundCount = computed(() => logs.value.filter((l: any) => l.type === 'FOUND').length)

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
    { key: 'unverified', title: 'Belum diverifikasi', icon: 'i-lucide-badge-help', count: a.unverified?.count ?? 0,
      desc: 'Prioritas High yang masih beroperasi, belum dikonfirmasi analis',
      ring: 'ring-amber-500/40', box: 'bg-amber-500/10 ring-amber-500/25', text: 'text-amber-400',
      items: siteItems(a.unverified, x => `${Math.round(x.confidence * 100)}%`), more: '/verifikasi' },
    { key: 'revived', title: 'Hidup lagi', icon: 'i-lucide-rotate-ccw', count: a.revived?.count ?? 0,
      desc: 'Situs mati yang aktif kembali ≤30 hari — operator memakai ulang domain',
      ring: 'ring-rose-500/40', box: 'bg-rose-500/10 ring-rose-500/25', text: 'text-rose-400',
      items: siteItems(a.revived, x => day(x.at)), more: '' },
    { key: 'reappeared', title: 'Rekening muncul lagi', icon: 'i-lucide-repeat', count: a.reappeared?.length ?? 0,
      desc: 'Rekening/kontak situs mati kini dipakai situs baru',
      ring: 'ring-rose-500/40', box: 'bg-rose-500/10 ring-rose-500/25', text: 'text-rose-400',
      items: (a.reappeared ?? []).map((x: any) => ({ key: x.value, label: x.value, sub: x.name || x.type,
        to: `/reverse-lookup?type=${encodeURIComponent(x.type)}&v=${encodeURIComponent(x.value)}` })), more: '/repository?tab=artifacts' },
    { key: 'blocked_alive', title: 'Lolos blokir', icon: 'i-lucide-shield-off', count: a.blocked_alive?.count ?? 0,
      desc: 'Diblokir ISP tapi server masih melayani pemain lewat VPN/DNS lain',
      ring: 'ring-orange-500/40', box: 'bg-orange-500/10 ring-orange-500/25', text: 'text-orange-400',
      items: siteItems(a.blocked_alive, x => x.at ? `sejak ${day(x.at)}` : ''), more: '/repository?tab=sites' },
    { key: 'no_evidence', title: 'Bukti belum diambil', icon: 'i-lucide-camera-off', count: a.no_evidence?.count ?? 0,
      desc: 'High beroperasi tanpa rekaman bukti forensik',
      ring: 'ring-sky-500/40', box: 'bg-sky-500/10 ring-sky-500/25', text: 'text-sky-400',
      items: siteItems(a.no_evidence, x => `${Math.round(x.confidence * 100)}%`), more: '' },
  ]
})

const lifeRows = computed(() => {
  const l = ov.value?.lifecycle ?? {}
  return [
    { key: 'active', label: 'Online', hint: '', n: l.active ?? 0, bar: 'bg-emerald-500' },
    { key: 'blocked_alive', label: 'Lolos blokir', hint: 'diblokir, masih hidup', n: l.blocked_alive ?? 0, bar: 'bg-orange-500' },
    { key: 'uncertain', label: 'Belum pasti', hint: 'gagal diakses <14 hari', n: l.uncertain ?? 0, bar: 'bg-amber-300' },
    { key: 'inactive', label: 'Tidak aktif', hint: 'mati / domain hilang', n: l.inactive ?? 0, bar: 'bg-slate-600' },
  ]
})
const lifeTotal = computed(() => lifeRows.value.reduce((s, r) => s + r.n, 0))

const ranks = computed(() => {
  const o = ov.value ?? {}
  return [
    { key: 'ops', title: 'Operator Teraktif', icon: 'i-lucide-network', more: '/repository?tab=operators',
      rows: (o.top_operators ?? []).map((x: any) => ({ label: x.label, sub: x.sub, active: x.active_sites, total: x.total_sites, mono: true })) },
    { key: 'money', title: 'Rekening Terbanyak', icon: 'i-lucide-landmark', more: '/repository?tab=artifacts',
      rows: (o.top_money ?? []).map((x: any) => ({ label: x.value, sub: [x.name, x.type].filter(Boolean).join(' · '), active: x.active_sites, total: x.total_sites, mono: true,
        to: `/reverse-lookup?type=${encodeURIComponent(x.type)}&v=${encodeURIComponent(x.value)}` })) },
    { key: 'prov', title: 'Provider Game', icon: 'i-lucide-gamepad-2', more: '',
      rows: (o.top_providers ?? []).map((x: any) => ({ label: x.label, active: x.active_sites, total: x.total_sites })) },
    { key: 'hosts', title: 'Hosting (non-CDN)', icon: 'i-lucide-server', more: '',
      rows: (o.top_hosts ?? []).map((x: any) => ({ label: x.label, sub: x.sub, active: x.active_sites, total: x.total_sites, mono: true, to: `/ip/${x.label}` })) },
  ]
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
