<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Pelacakan IP" :title="ip" icon="i-lucide-server-cog"
      subtitle="Pemilik blok IP, ASN, lokasi, kontak abuse, port yang pernah teramati, dan situs judol lain di IP yang sama.">
      <template #actions>
        <form class="flex gap-2" @submit.prevent="go">
          <UInput v-model="query" icon="i-lucide-search" placeholder="Lacak IP lain…" class="w-56" />
          <UButton type="submit" color="gray" variant="soft" label="Lacak" />
        </form>
        <UButton color="gray" variant="soft" icon="i-lucide-refresh-cw" label="Perbarui" :loading="loading" @click="load(true)" />
      </template>
    </PageHeader>

    <UAlert v-if="error" icon="i-lucide-circle-alert" color="rose" variant="subtle" :title="error" />

    <div v-else-if="loading && !intel" class="space-y-3"><USkeleton v-for="i in 3" :key="i" class="h-28 w-full" /></div>

    <template v-else-if="intel">
      <UAlert v-if="intel.cdn" icon="i-lucide-cloud" color="amber" variant="subtle" :title="`IP ini milik ${intel.cdn} (CDN)`"
        description="Ribuan situs memakai IP ini bersama-sama, jadi kesamaan IP bukan bukti keterkaitan. Server asli operator tersembunyi di belakangnya — gunakan 'Cari IP Asli' di halaman detail situs, atau minta datanya resmi ke penyedia CDN." />

      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <div v-for="k in kpis" :key="k.label" class="rounded-xl bg-slate-900/60 ring-1 ring-slate-800 px-4 py-3">
          <p class="text-xs uppercase tracking-wider text-slate-500">{{ k.label }}</p>
          <p class="text-lg font-semibold text-white mt-0.5 break-words">{{ k.value || '—' }}</p>
          <p v-if="k.hint" class="text-xs text-slate-500 mt-0.5 break-words">{{ k.hint }}</p>
        </div>
      </div>

      <div class="grid grid-cols-1 xl:grid-cols-2 gap-5 items-start">
        <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-0' } }">
          <template #header><h2 class="section-title">Pemilik Jaringan (RDAP)</h2></template>
          <dl class="divide-y divide-slate-800/70 text-sm">
            <div v-for="r in ownerRows" :key="r.k" class="flex gap-3 px-5 py-2.5">
              <dt class="w-40 shrink-0 text-slate-500">{{ r.k }}</dt>
              <dd class="flex-1 min-w-0 text-slate-100 break-words" :class="r.mono && 'mono'">{{ r.v || '—' }}</dd>
            </div>
            <div class="flex gap-3 px-5 py-2.5">
              <dt class="w-40 shrink-0 text-slate-500">Kontak abuse</dt>
              <dd class="flex-1 min-w-0 space-y-1">
                <p v-if="!intel.abuse_emails?.length" class="text-slate-500">—</p>
                <div v-for="m in intel.abuse_emails" :key="m" class="flex items-center gap-1.5">
                  <span class="mono text-amber-200 break-all">{{ m }}</span>
                  <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copy(m)" />
                </div>
              </dd>
            </div>
          </dl>
          <p class="px-5 py-3 text-xs text-slate-500 border-t border-slate-800">
            Pemegang blok IP & kontak abuse adalah pihak tujuan surat permintaan data pelanggan / log akses server.
          </p>
        </UCard>

        <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header><h2 class="section-title">Layanan Teramati (Shodan InternetDB)</h2></template>
          <div class="space-y-4 text-sm">
            <div>
              <p class="text-xs text-slate-500 mb-1.5">Port terbuka</p>
              <p v-if="!intel.ports?.length" class="text-slate-500">Belum pernah teramati.</p>
              <div class="flex flex-wrap gap-1.5">
                <UBadge v-for="p in intel.ports" :key="p" color="gray" variant="subtle" class="mono">{{ p }}</UBadge>
              </div>
            </div>
            <div v-if="intel.hostnames?.length">
              <p class="text-xs text-slate-500 mb-1.5">Hostname{{ intel.cdn ? ' (pelanggan CDN lain — tidak terkait)' : '' }}</p>
              <p class="mono text-slate-300 break-all">{{ intel.hostnames.join(', ') }}</p>
            </div>
            <div v-if="intel.tags?.length">
              <p class="text-xs text-slate-500 mb-1.5">Tag</p>
              <div class="flex flex-wrap gap-1.5"><UBadge v-for="t in intel.tags" :key="t" color="gray" variant="outline">{{ t }}</UBadge></div>
            </div>
            <div v-if="intel.vulns?.length">
              <p class="text-xs text-slate-500 mb-1.5">Kerentanan (CVE) teramati</p>
              <div class="flex flex-wrap gap-1.5"><UBadge v-for="v in intel.vulns" :key="v" color="rose" variant="subtle" class="mono">{{ v }}</UBadge></div>
            </div>
            <p class="text-xs text-slate-500">Data diambil dari pengamatan pasif Shodan — tidak ada pemindaian langsung dari aplikasi ini.</p>
          </div>
        </UCard>
      </div>

      <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
        <template #header>
          <div class="flex items-center gap-2">
            <h2 class="section-title flex-1">Situs di Database pada IP Ini</h2>
            <UBadge color="gray" variant="subtle">{{ sitesTotal }}</UBadge>
          </div>
        </template>
        <p v-if="!sites.length" class="px-5 py-4 text-sm text-slate-500">Tidak ada situs di database yang memakai IP ini.</p>
        <div v-else class="divide-y divide-slate-800/70">
          <NuxtLink v-for="s in sites" :key="s.id" :to="`/site/${s.id}`" class="flex items-center gap-3 px-5 py-2.5 hover:bg-slate-800/40">
            <span class="w-1 h-6 rounded-full" :class="s.priority === 'High' ? 'bg-rose-500' : s.priority === 'Medium' ? 'bg-amber-500' : 'bg-slate-700'" />
            <span class="flex-1 min-w-0 truncate text-slate-200">{{ s.url.replace(/^https?:\/\//, '').replace(/\/$/, '') }}</span>
            <UBadge :color="s.is_gambling ? 'rose' : 'gray'" variant="subtle" size="xs">{{ s.is_gambling ? 'Judol' : 'Normal' }}</UBadge>
            <span class="text-xs tabular-nums text-slate-400 w-10 text-right">{{ Math.round(s.confidence * 100) }}%</span>
          </NuxtLink>
        </div>
      </UCard>

      <UCard v-if="history.length" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
        <template #header>
          <div class="flex items-center gap-2">
            <h2 class="section-title flex-1">Riwayat Perubahan IP</h2>
            <UBadge color="gray" variant="subtle">{{ history.length }} versi</UBadge>
          </div>
          <p class="text-[13px] text-slate-500 mt-1">Setiap versi yang berbeda disimpan permanen — perubahan pemilik blok, ASN, port, atau hostname terlihat di sini.</p>
        </template>
        <div class="divide-y divide-slate-800/70">
          <div v-for="(h, idx) in history" :key="h.fetched_at" class="px-5 py-3 text-sm grid grid-cols-1 md:grid-cols-[170px_1fr] gap-x-4 gap-y-1">
            <p class="text-slate-300">{{ fmtDate(h.fetched_at) }}<UBadge v-if="idx === 0" color="primary" variant="subtle" size="xs" class="ml-2">terkini</UBadge></p>
            <div class="space-y-0.5 min-w-0">
              <p class="text-slate-200"><span class="mono text-slate-400">{{ h.asn }}</span> {{ h.as_name }} · {{ h.isp }}</p>
              <p class="text-slate-400 text-[13px]">{{ [h.city, h.country].filter(Boolean).join(', ') }} · <span class="mono">{{ h.net_range || '—' }}</span> · {{ h.reg_org || '—' }}</p>
              <p class="text-slate-500 text-[13px]">Port: <span class="mono">{{ (h.ports ?? []).join(', ') || '—' }}</span>
                <template v-if="h.abuse_emails?.length"> · abuse: <span class="mono">{{ h.abuse_emails.join(', ') }}</span></template></p>
              <p v-for="d in diffFrom(idx)" :key="d" class="text-amber-300 text-[13px]">↳ {{ d }}</p>
            </div>
          </div>
        </div>
      </UCard>

      <p class="text-xs text-slate-500">Data diperbarui {{ fmtDate(intel.fetched_at) }} · dianggap segar 7 hari (setelah itu diambil ulang saat dibuka); semua versi lama tetap tersimpan di riwayat.
        <span v-if="Object.keys(intel.errors ?? {}).length"> Sumber gagal: {{ Object.keys(intel.errors).join(', ') }}.</span>
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const router = useRouter()
const { get } = useApi()
const toast = useToast()

const ip = computed(() => decodeURIComponent(route.params.ip as string))
const query = ref('')
const intel = ref<any>(null)
const sites = ref<any[]>([])
const sitesTotal = ref(0)
const history = ref<any[]>([])
const loading = ref(true)
const error = ref('')

const kpis = computed(() => {
  const i = intel.value ?? {}
  return [
    { label: 'Lokasi', value: [i.city, i.country].filter(Boolean).join(', '), hint: i.region },
    { label: 'ASN', value: i.asn, hint: i.as_name },
    { label: 'Penyedia', value: i.isp, hint: i.hosting ? 'Pusat data / hosting' : i.proxy ? 'Proxy / VPN' : '' },
    { label: 'Situs di IP ini', value: String(sitesTotal.value), hint: i.cdn ? 'IP CDN — tidak bermakna' : '' },
  ]
})
const ownerRows = computed(() => {
  const i = intel.value ?? {}
  return [
    { k: 'Registri (RIR)', v: i.registry }, { k: 'Nama jaringan', v: i.net_name }, { k: 'Blok IP', v: i.net_range, mono: true },
    { k: 'Pemegang blok', v: i.reg_org }, { k: 'Organisasi', v: i.org }, { k: 'ISP', v: i.isp },
    { k: 'Reverse DNS', v: (i.reverse_dns ?? []).join(', '), mono: true },
  ]
})

function fmtDate(d?: string) { return d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—' }
async function copy(t: string) {
  try { await navigator.clipboard.writeText(t); toast.add({ title: 'Disalin', description: t, color: 'emerald', timeout: 2000 }) } catch {}
}
// Ringkas apa yang berubah dibanding versi sebelumnya (lebih lama)
function diffFrom(idx: number) {
  const a = history.value[idx], b = history.value[idx + 1]
  if (!b) return []
  const out: string[] = []
  if (a.asn !== b.asn || a.isp !== b.isp) out.push(`Penyedia berubah dari ${b.asn} ${b.isp}`)
  if (a.reg_org !== b.reg_org) out.push(`Pemegang blok berubah dari ${b.reg_org || '—'}`)
  const pa = new Set(a.ports ?? []), pb = new Set(b.ports ?? [])
  const opened = [...pa].filter(p => !pb.has(p)), closed = [...pb].filter(p => !pa.has(p))
  if (opened.length) out.push(`Port baru terbuka: ${opened.join(', ')}`)
  if (closed.length) out.push(`Port tertutup: ${closed.join(', ')}`)
  if ((a.hostnames ?? []).join() !== (b.hostnames ?? []).join()) out.push('Daftar hostname berubah')
  return out
}
function go() { const v = query.value.trim(); if (v) router.push(`/ip/${encodeURIComponent(v)}`) }

async function load(refresh = false) {
  loading.value = true
  error.value = ''
  try {
    const d = await get<any>(`/ips/${encodeURIComponent(ip.value)}`, { refresh: refresh ? 1 : undefined })
    intel.value = d.intel
    sites.value = d.sites ?? []
    sitesTotal.value = d.sites_total ?? 0
    history.value = d.history ?? []
  } catch (e: any) {
    error.value = e.message
  } finally { loading.value = false }
}

watch(ip, () => { intel.value = null; load() })
onMounted(() => load())
useHead({ title: () => `${ip.value} — Pelacakan IP` })
</script>
