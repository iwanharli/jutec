<template>
  <UCard :ui="{ ring: 'ring-1 ring-orange-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-orange-500/10 ring-orange-500/25">
          <UIcon name="i-lucide-server-cog" class="w-[18px] h-[18px] text-orange-400" />
        </div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Pelacakan IP & Hosting</h2>
          <p class="card-desc">Pemilik blok IP, ASN, lokasi, kontak abuse, port teramati — dari sumber pasif (RDAP, ip-api, Shodan InternetDB, DNS)</p>
        </div>
        <UBadge v-if="behindCDN" color="amber" variant="subtle"><UIcon name="i-lucide-cloud" class="w-3.5 h-3.5 mr-1" />Di balik Cloudflare</UBadge>
        <UButton color="gray" variant="soft" icon="i-lucide-refresh-cw" :loading="loading" @click="load(true)" />
        <UButton color="orange" icon="i-lucide-crosshair" :loading="originBusy"
          :label="originBusy ? 'Mencari IP asli… (±1 menit)' : origin ? 'Cari Ulang IP Asli' : 'Cari IP Asli'" @click="findOrigin" />
      </div>
    </template>

    <div v-if="loading && !ips.length" class="space-y-2"><USkeleton v-for="i in 2" :key="i" class="h-20 w-full" /></div>
    <p v-else-if="!ips.length" class="text-sm text-slate-400">Belum ada IP tercatat untuk situs ini.</p>

    <!-- IP yang terlihat -->
    <div v-else>
      <p class="pb-1 text-xs font-semibold uppercase tracking-wider text-slate-500">IP yang terlihat publik · {{ ips.length }}</p>
      <div class="divide-y divide-slate-800/70">
        <IpIntelRow v-for="r in ips" :key="r.ip" :row="r" />
      </div>
      <UAlert v-if="behindCDN" class="mt-3" icon="i-lucide-info" color="amber" variant="subtle"
        title="IP di atas milik Cloudflare, bukan server operator"
        description="Cloudflare menyembunyikan server asli. Gunakan 'Cari IP Asli' untuk mencari IP yang bocor lewat subdomain, server email, atau riwayat scan. Data pelanggan & IP asli juga bisa dimintakan resmi ke Cloudflare melalui jalur permintaan penegak hukum." />
    </div>

    <!-- Hasil pencarian IP asli -->
    <div v-if="origin" class="mt-4 pt-4 border-t border-slate-800">
      <div class="pb-2 flex flex-wrap items-center gap-2">
        <p class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ origin.behind_cdn ? 'Kandidat IP asli (origin)' : 'IP server langsung (DNS saat ini)' }}</p>
        <UBadge :color="origin.candidates.length ? 'rose' : 'gray'" variant="subtle" size="xs">{{ origin.candidates.length }} {{ origin.behind_cdn ? 'kandidat' : 'IP' }}</UBadge>
        <span class="text-xs text-slate-500 ml-auto">diperiksa {{ fmtDate(origin.checked_at) }} · {{ origin.subdomains.length }} host ber-DNS</span>
      </div>
      <p class="pb-1 text-[13px] text-slate-400">
        <template v-if="origin.behind_cdn">IP di luar rentang CDN yang terhubung dengan domain ini. Kandidat dengan bukti terbanyak paling mungkin server asli — tetap perlu dikonfirmasi (mis. permintaan data ke penyedia hosting).</template>
        <template v-else>Situs ini tidak memakai CDN, jadi IP dari DNS saat ini adalah server langsungnya. Bila berbeda dari IP saat scan, operator kemungkinan sudah pindah server.</template>
      </p>
      <div v-if="originIps.length" class="divide-y divide-slate-800/70">
        <IpIntelRow v-for="r in originIps" :key="r.ip" :row="r" :evidence="candidateOf(r.ip)" />
      </div>
      <div v-for="(n, i) in origin.notes" :key="i" class="py-2 text-[13px] text-slate-500 flex gap-2">
        <UIcon name="i-lucide-info" class="w-4 h-4 shrink-0 mt-0.5" />{{ n }}
      </div>
      <details v-if="origin.subdomains.length" class="pt-2 text-[13px]">
        <summary class="cursor-pointer text-slate-400 hover:text-slate-200">Host yang diperiksa ({{ origin.subdomains.length }})</summary>
        <div class="flex flex-wrap gap-1.5 mt-2">
          <span v-for="s in origin.subdomains" :key="s" class="mono text-xs bg-slate-950/60 ring-1 ring-slate-800 rounded px-2 py-0.5 text-slate-300">{{ s }}</span>
        </div>
      </details>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const props = defineProps<{ siteId: number | string }>()
const { get, post } = useApi()
const toast = useToast()

const ips = ref<any[]>([])
const origin = ref<any>(null)
const originIps = ref<any[]>([])
const loading = ref(true)
const originBusy = ref(false)

const behindCDN = computed(() => ips.value.some(r => r.intel?.cdn) || origin.value?.behind_cdn)
const candidateOf = (ip: string) => origin.value?.candidates.find((c: any) => c.ip === ip)
function fmtDate(d?: string) { return d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—' }

async function load(refresh = false) {
  loading.value = true
  try {
    const d = await get<any>(`/sites/${props.siteId}/infra`, { refresh: refresh ? 1 : undefined })
    ips.value = d?.ips ?? []
    origin.value = d?.origin ?? null
    originIps.value = d?.origin_ips ?? []
  } catch (e: any) {
    toast.add({ title: 'Gagal memuat data IP', description: e.message, color: 'rose' })
  } finally { loading.value = false }
}

async function findOrigin() {
  originBusy.value = true
  try {
    const d = await post<any>(`/sites/${props.siteId}/origin`, {})
    origin.value = d.origin
    originIps.value = d.origin_ips ?? []
    const n = d.origin?.candidates?.length ?? 0
    toast.add(n
      ? { title: `${n} kandidat IP asli ditemukan`, description: 'Periksa bukti tiap kandidat di bawah.', color: 'rose', icon: 'i-lucide-crosshair' }
      : { title: 'Belum ada IP asli yang bocor', description: d.origin?.notes?.[0] ?? '', color: 'gray', icon: 'i-lucide-crosshair' })
  } catch (e: any) {
    toast.add({ title: 'Pencarian IP asli gagal', description: e.message, color: 'rose' })
  } finally { originBusy.value = false }
}

onMounted(() => load())
</script>
