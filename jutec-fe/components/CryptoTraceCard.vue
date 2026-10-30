<template>
  <UCard :ui="{ ring: 'ring-1 ring-orange-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon bg-orange-500/10 ring-orange-500/25"><UIcon name="i-lucide-bitcoin" class="w-[18px] h-[18px] text-orange-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Jejak Dana USDT</h2>
          <p class="card-desc">Riwayat transfer USDT — TRC20 (TronGrid) & ERC20 (Etherscan)</p>
        </div>
        <UButton size="sm" color="gray" variant="soft" icon="i-lucide-refresh-cw" label="Perbarui" :loading="busy" @click="$emit('refresh')" />
      </div>
    </template>
    <div class="space-y-5">
      <div v-for="w in wallets" :key="w.address">
        <div class="flex flex-wrap items-center gap-2">
          <a :href="explorer(w.address)" target="_blank" rel="noopener" class="mono text-[15px] text-slate-100 hover:text-orange-300 break-all">{{ w.address }}</a>
          <UBadge v-if="linkedCount(w)" color="rose" variant="solid" size="xs"><UIcon name="i-lucide-link" class="w-3 h-3 mr-1" />terhubung {{ linkedCount(w) }} dompet situs lain</UBadge>
        </div>
        <p v-if="w.error && !w.profile" class="text-[13px] text-rose-300 mt-1">{{ w.error }}</p>
        <template v-if="w.profile">
          <p v-if="!w.profile.transfers" class="muted mt-1">Belum ada transfer USDT di alamat ini.</p>
          <template v-else>
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 mt-2">
              <div v-for="s in stats(w.profile)" :key="s.k" class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 px-3 py-2">
                <p class="text-[12px] uppercase tracking-wider text-slate-500">{{ s.k }}</p>
                <p class="font-semibold tabular-nums" :class="s.c">{{ s.v }}</p>
              </div>
            </div>
            <p v-if="!w.profile.complete" class="text-[12px] text-slate-500 mt-1.5">Ringkasan dari {{ w.profile.transfers }} transfer terbaru — riwayat lengkapnya lebih panjang.</p>
            <div class="grid lg:grid-cols-2 gap-4 mt-3">
              <div v-for="side in sides" :key="side.key">
                <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-1.5">{{ side.label }}</p>
                <p v-if="!w.profile[side.key]?.length" class="muted">—</p>
                <ul class="space-y-1">
                  <li v-for="cp in w.profile[side.key]" :key="cp.address" class="text-sm">
                    <div class="flex items-center gap-2">
                      <a :href="explorer(cp.address)" target="_blank" rel="noopener" class="mono truncate text-slate-300 hover:text-orange-300">{{ cp.address }}</a>
                      <span class="ml-auto tabular-nums text-slate-100 shrink-0">{{ usd(cp.amount) }}</span>
                      <span class="text-[12px] text-slate-500 shrink-0 w-10 text-right">{{ cp.count }}×</span>
                    </div>
                    <p v-if="w.linked_sites[cp.address]" class="text-[12px] text-rose-300 pl-1">
                      ↳ dompet di
                      <template v-for="(s, i) in w.linked_sites[cp.address]" :key="s.id"><NuxtLink :to="`/site/${s.id}`" class="mono hover:underline">{{ s.url.replace(/^https?:\/\//, '') }}</NuxtLink><span v-if="i < w.linked_sites[cp.address].length - 1">, </span></template>
                    </p>
                  </li>
                </ul>
              </div>
            </div>
          </template>
        </template>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const explorer = (a: string) => a?.toLowerCase().startsWith('0x') ? `https://etherscan.io/address/${a}` : `https://tronscan.org/#/address/${a}`
defineProps<{ wallets: any[]; busy: boolean }>()
defineEmits<{ (e: 'refresh'): void }>()
const sides = [{ key: 'top_senders', label: 'Pengirim terbesar (dana masuk)' }, { key: 'top_receivers', label: 'Penerima terbesar (dana keluar)' }]
const usd = (n: number) => `$${n.toLocaleString('en-US', { maximumFractionDigits: n >= 1000 ? 0 : 2 })}`
const day = (t: number) => t ? new Date(t).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—'
const linkedCount = (w: any) => Object.keys(w.linked_sites ?? {}).length
const stats = (p: any) => [
  { k: 'Masuk', v: usd(p.total_in), c: 'text-emerald-400' },
  { k: 'Keluar', v: usd(p.total_out), c: 'text-rose-400' },
  { k: 'Transfer', v: p.transfers.toLocaleString('id-ID'), c: 'text-white' },
  { k: 'Aktif', v: `${day(p.first_at)} – ${day(p.last_at)}`, c: 'text-slate-200 text-sm' },
]
</script>
