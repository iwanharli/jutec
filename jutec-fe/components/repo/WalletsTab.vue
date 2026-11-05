<template>
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="space-y-4">
        <div class="flex items-center gap-3">
          <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-coins" class="w-[18px] h-[18px] text-amber-400" /></div>
          <div class="flex-1 min-w-0"><h2 class="card-title">Dompet Kripto</h2><p class="card-desc">Dompet di situs judol beserta aliran dana USDT — diurutkan dari dana masuk terbesar</p></div>
          <UBadge color="gray" variant="subtle">{{ shown.length }}</UBadge>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UInput v-model="q" icon="i-lucide-search" placeholder="Cari alamat dompet…" class="w-full sm:w-72" />
          <SegmentedControl v-model="net" :options="netOpts" />
          <div class="ml-auto flex items-center gap-4 text-sm">
            <span class="text-slate-400">Total masuk <b class="text-emerald-300 tabular-nums">{{ usd(sum.in) }}</b></span>
            <span class="text-slate-400">Keluar <b class="text-rose-300 tabular-nums">{{ usd(sum.out) }}</b></span>
          </div>
        </div>
      </div>
    </template>

    <div v-if="loading" class="p-5 space-y-2"><USkeleton v-for="i in 6" :key="i" class="h-14 w-full" /></div>
    <EmptyState v-else-if="!shown.length" icon="i-lucide-coins" title="Belum ada dompet kripto" desc="Alamat dompet terbaca dari halaman deposit situs judol, lalu diprofilkan otomatis tiap 3 jam." />
    <div v-else class="divide-y divide-slate-800/60">
      <div v-for="w in shown" :key="w.address" class="px-5 py-4 hover:bg-slate-800/20">
        <div class="flex flex-col lg:flex-row lg:items-center gap-3">
          <div class="flex-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <a :href="explorer(w)" target="_blank" rel="noopener noreferrer" class="mono text-sm text-slate-100 hover:text-amber-300 break-all">{{ w.address }}</a>
              <UBadge :color="w.network === 'TRC20' ? 'rose' : w.network === 'ERC20' ? 'sky' : 'gray'" variant="subtle" size="xs">{{ w.network }}</UBadge>
              <UBadge v-if="w.sites > 1" color="amber" variant="subtle" size="xs">dipakai {{ w.sites }} situs</UBadge>
              <span v-if="links(w).length" class="inline-flex items-center gap-1 text-xs text-rose-300"><UIcon name="i-lucide-link-2" class="w-3.5 h-3.5" />bertransaksi dengan {{ links(w).length }} dompet judol lain</span>
            </div>
            <div class="flex flex-wrap gap-1 mt-1.5">
              <NuxtLink v-for="s in w.samples" :key="s.id" :to="`/site/${s.id}`" class="rounded px-1.5 py-0.5 text-[11px] text-rose-300 ring-1 ring-rose-500/25 hover:ring-rose-400">{{ host(s.url) }}</NuxtLink>
            </div>
          </div>
          <div v-if="w.profile" class="grid grid-cols-3 gap-4 shrink-0 text-right">
            <div><p class="text-sm font-semibold tabular-nums text-emerald-300">{{ usd(w.profile.total_in) }}</p><p class="text-[11px] text-slate-500">masuk</p></div>
            <div><p class="text-sm font-semibold tabular-nums text-rose-300">{{ usd(w.profile.total_out) }}</p><p class="text-[11px] text-slate-500">keluar</p></div>
            <div><p class="text-sm font-semibold tabular-nums text-slate-200">{{ w.profile.transfers }}{{ w.profile.complete ? '' : '+' }}</p><p class="text-[11px] text-slate-500">transfer · {{ day(w.profile.last_at) }}</p></div>
          </div>
          <span v-else class="text-xs text-slate-500 shrink-0">{{ ['TRC20', 'ERC20'].includes(w.network) ? 'menunggu diprofilkan' : 'jaringan belum didukung' }}</span>
        </div>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const { get } = useApi()
const q = ref('')
const net = ref('')
const rows = ref<any[]>([])
const loading = ref(true)

const shown = computed(() => rows.value.filter(w => !net.value || w.network === net.value))
const netOpts = computed(() => [{ value: '', label: 'Semua', n: rows.value.length },
  ...['TRC20', 'ERC20', 'BTC', 'Lainnya'].map(v => ({ value: v, label: v, n: rows.value.filter(w => w.network === v).length })).filter(o => o.n)])
const sum = computed(() => shown.value.reduce((a, w) => ({ in: a.in + (w.profile?.total_in ?? 0), out: a.out + (w.profile?.total_out ?? 0) }), { in: 0, out: 0 }))
// Lawan transaksi yang juga dompet situs judol lain di daftar ini
const known = computed(() => new Set(rows.value.map(w => w.address.toLowerCase())))
const links = (w: any) => [...(w.profile?.top_senders ?? []), ...(w.profile?.top_receivers ?? [])]
  .filter((c: any) => c.address.toLowerCase() !== w.address.toLowerCase() && known.value.has(c.address.toLowerCase()))

const usd = (n: number) => `$${Number(n ?? 0).toLocaleString('en-US', { maximumFractionDigits: 0 })}`
const day = (t: number) => t ? new Date(t).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: '2-digit' }) : '—'
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/.*$/, '')
const explorer = (w: any) => w.network === 'ERC20' ? `https://etherscan.io/address/${w.address}` : w.network === 'TRC20' ? `https://tronscan.org/#/address/${w.address}`
  : w.network === 'BTC' ? `https://mempool.space/address/${w.address}` : '#'

let t: any
async function load() {
  loading.value = true
  try { rows.value = ((await get<any>('/data/wallets', { q: q.value || undefined })).results ?? []).map((w: any) => ({ ...w, profile: w.profile ?? null })) }
  catch { rows.value = [] } finally { loading.value = false }
}
watch(q, () => { clearTimeout(t); t = setTimeout(load, 300) })
onMounted(load)
</script>
