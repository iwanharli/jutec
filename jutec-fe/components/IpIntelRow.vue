<template>
  <div class="py-3 grid grid-cols-1 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)_minmax(0,1fr)] gap-x-6 gap-y-3 text-sm">
    <!-- Identitas IP -->
    <div class="min-w-0">
      <div class="flex flex-wrap items-center gap-2">
        <NuxtLink :to="`/ip/${encodeURIComponent(row.ip)}`" class="mono text-[15px] font-semibold text-slate-100 hover:text-primary-300 break-all">{{ row.ip }}</NuxtLink>
        <UBadge v-if="i?.cdn" color="amber" variant="subtle" size="xs">CDN · {{ i.cdn }}</UBadge>
        <UBadge v-else-if="evidence" color="rose" variant="subtle" size="xs">{{ evidence.sources?.includes('apex') && evidence.sources.length === 1 ? 'IP DNS saat ini' : 'Kandidat origin' }}</UBadge>
        <UBadge v-if="i?.proxy" color="violet" variant="subtle" size="xs">Proxy/VPN</UBadge>
        <UBadge v-if="i && !i.cdn && i.hosting" color="gray" variant="subtle" size="xs">Hosting</UBadge>
      </div>
      <p v-if="row.error" class="text-rose-400 text-[13px] mt-1">{{ row.error }}</p>
      <template v-else-if="i">
        <p class="text-slate-300 mt-1 flex items-center gap-1.5">
          <UIcon name="i-lucide-map-pin" class="w-3.5 h-3.5 text-slate-500" />
          {{ [i.city, i.region, i.country].filter(Boolean).join(', ') || 'Lokasi tidak diketahui' }}
          <span v-if="i.country_code" class="mono text-xs text-slate-500">({{ i.country_code }})</span>
        </p>
        <p v-if="i.reverse_dns?.length" class="text-slate-400 text-[13px] mt-0.5 mono break-all">rDNS: {{ i.reverse_dns.join(', ') }}</p>
      </template>
      <div v-if="evidence" class="mt-2 space-y-1">
        <div class="flex flex-wrap gap-1">
          <UBadge v-for="s in evidence.sources" :key="s" color="gray" variant="outline" size="xs">{{ sourceLabel[s] ?? s }}</UBadge>
        </div>
        <p v-for="e in evidence.evidence" :key="e" class="mono text-xs text-slate-400 break-all">• {{ e }}</p>
      </div>
    </div>

    <!-- Pemilik jaringan -->
    <div v-if="i" class="min-w-0 space-y-1">
      <p class="text-slate-100"><span class="mono text-slate-400">{{ i.asn || 'AS?' }}</span> {{ i.as_name || i.isp }}</p>
      <p class="text-slate-400 text-[13px]">{{ i.isp }}<template v-if="i.org && i.org !== i.isp"> · {{ i.org }}</template></p>
      <p class="text-slate-500 text-[13px]">
        <span v-if="i.registry">{{ i.registry }}</span>
        <span v-if="i.net_name"> · {{ i.net_name }}</span>
        <span v-if="i.net_range" class="mono"> · {{ i.net_range }}</span>
      </p>
      <p v-if="i.reg_org && i.reg_org !== i.org" class="text-slate-500 text-[13px]">Pemegang blok: {{ i.reg_org }}</p>
      <div v-for="m in i.abuse_emails" :key="m" class="flex items-center gap-1.5 text-[13px]">
        <UIcon name="i-lucide-mail-warning" class="w-3.5 h-3.5 text-amber-400" />
        <span class="mono text-amber-200 break-all">{{ m }}</span>
        <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copy(m)" />
      </div>
    </div>

    <!-- Port & situs lain -->
    <div v-if="i" class="min-w-0 space-y-2">
      <div v-if="i.ports?.length">
        <p class="text-xs text-slate-500 mb-1">Port teramati (Shodan){{ i.cdn ? ' — milik CDN' : '' }}</p>
        <div class="flex flex-wrap gap-1">
          <span v-for="p in i.ports" :key="p" class="mono text-xs rounded px-1.5 py-0.5"
            :class="i.cdn ? 'bg-slate-800/60 text-slate-500' : riskyPort(p) ? 'bg-rose-500/15 text-rose-300' : 'bg-slate-800 text-slate-300'">{{ p }}</span>
        </div>
      </div>
      <div v-if="i.vulns?.length" class="text-xs text-rose-300">{{ i.vulns.length }} CVE teramati</div>
      <div v-if="!i.cdn && i.hostnames?.length" class="text-xs text-slate-400 break-all">Host lain: <span class="mono">{{ i.hostnames.slice(0, 6).join(', ') }}</span></div>
      <div class="text-[13px]">
        <span v-if="i.cdn" class="text-slate-500">IP CDN dipakai bersama banyak situs — kesamaan IP ini bukan bukti keterkaitan.</span>
        <template v-else-if="row.shared_sites">
          <span class="text-rose-300 font-medium">{{ row.shared_sites }} situs lain di IP ini</span>
          <div class="flex flex-wrap gap-1 mt-1">
            <NuxtLink v-for="s in row.sites.slice(0, 5)" :key="s.id" :to="`/site/${s.id}`"
              class="text-xs mono rounded px-1.5 py-0.5 hover:text-primary-300" :class="s.is_gambling ? 'bg-rose-500/10 text-rose-200' : 'bg-slate-800 text-slate-300'">
              {{ s.url.replace(/^https?:\/\//, '').replace(/\/$/, '') }}
            </NuxtLink>
          </div>
        </template>
        <span v-else class="text-slate-500">Tidak ada situs lain di database pada IP ini.</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ row: any; evidence?: any }>()
const i = computed(() => props.row.intel)
const toast = useToast()
const sourceLabel: Record<string, string> = { subdomain: 'Subdomain', mx: 'Server email (MX)', urlscan: 'Riwayat URLScan', apex: 'Domain utama' }
// Port panel admin / database / remote yang sering terbuka di server operator
const riskyPort = (p: number) => [21, 22, 23, 2082, 2083, 2086, 2087, 3306, 3389, 5432, 6379, 8888, 10000, 27017].includes(p)
async function copy(t: string) {
  try { await navigator.clipboard.writeText(t); toast.add({ title: 'Disalin', description: t, color: 'emerald', timeout: 2000 }) } catch {}
}
</script>
