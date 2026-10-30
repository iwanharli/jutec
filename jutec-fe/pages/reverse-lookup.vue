<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Intelijen" title="Reverse Lookup"
      subtitle="Temukan semua situs yang berbagi atribut yang sama — IP, nameserver, sertifikat SSL, tracking ID, rekening, atau kontak."
      icon="i-lucide-scan-search" />

    <!-- Lookup form -->
    <UCard>
      <div class="flex flex-col lg:flex-row gap-3">
        <USelect v-model="qType" :options="typeOptions" size="xl" class="lg:w-64" />
        <UInput v-model="qValue" size="xl" icon="i-lucide-key-round" class="flex-1" :placeholder="currentType.placeholder"
          :ui="{ base: 'font-mono' }" @keyup.enter="runLookup" />
        <UButton size="xl" icon="i-lucide-search" :loading="loading" :disabled="!qValue.trim()" label="Cari Keterkaitan" @click="runLookup" />
      </div>
      <p class="text-sm text-slate-500 mt-3">{{ currentType.hint }}</p>
    </UCard>

    <div class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <!-- Results -->
      <div class="xl:col-span-8 min-w-0">
        <UCard v-if="result" :ui="{ body: { padding: '' } }" class="animate-fade-in">
          <template #header>
            <div class="flex flex-wrap items-center gap-4">
              <div class="w-12 h-12 rounded-xl bg-primary-500/10 ring-1 ring-primary-500/25 flex items-center justify-center">
                <span class="text-lg font-semibold text-primary-300 tabular-nums">{{ result.sites?.length ?? 0 }}</span>
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="section-title">{{ result.sites?.length ?? 0 }} situs terkait</h2>
                <p class="section-desc truncate">{{ currentType.label }}: <span class="mono text-slate-300">{{ lastQuery }}</span></p>
              </div>
              <UBadge v-if="gamblingCount" color="rose" variant="subtle" size="lg">{{ gamblingCount }} judol</UBadge>
            </div>
          </template>
          <UTable :rows="result.sites ?? []" :columns="columns"
            :empty-state="{ icon: 'i-lucide-search-x', label: 'Tidak ada situs dengan atribut ini.' }">
            <template #url-data="{ row }">
              <NuxtLink v-if="row.id" :to="`/site/${row.id}`" class="text-[15px] font-medium text-slate-200 hover:text-primary-300 truncate block max-w-[440px]">
                {{ row.url.replace(/^https?:\/\//, '') }}
              </NuxtLink>
              <a v-else :href="row.url" target="_blank" rel="noopener" class="text-[15px] font-medium text-slate-200 hover:text-primary-300 truncate block max-w-[440px]">
                {{ row.url.replace(/^https?:\/\//, '') }}
              </a>
            </template>
            <template #priority-data="{ row }"><PriorityBadge :priority="row.priority" /></template>
            <template #confidence-data="{ row }">
              <div class="flex items-center gap-3 w-36">
                <UProgress :value="Math.round(row.confidence * 100)" size="sm" :color="row.confidence >= 0.75 ? 'rose' : 'primary'" class="flex-1" />
                <span class="text-sm font-semibold tabular-nums w-10 text-right">{{ Math.round(row.confidence * 100) }}%</span>
              </div>
            </template>
            <template #is_gambling-data="{ row }">
              <UBadge :color="row.is_gambling ? 'rose' : 'gray'" variant="subtle">{{ row.is_gambling ? 'Judol' : 'Normal' }}</UBadge>
            </template>
            <template #actions-data="{ row }">
              <div class="flex justify-end gap-2"><UButton v-if="row.id" :to="`/site/${row.id}`" label="Detail" size="sm" icon="i-lucide-panel-right-open" />
<UButton :to="`/discovery?url=${encodeURIComponent(row.url)}`" size="sm" color="gray" variant="soft" label="Analisis" trailing-icon="i-lucide-arrow-up-right" /></div>
            </template>
          </UTable>
        </UCard>

        <UCard v-else-if="loading">
          <div class="empty-state">
            <UIcon name="i-lucide-loader-circle" class="w-8 h-8 text-primary-400 animate-spin" />
            <p>Menelusuri keterkaitan...</p>
          </div>
        </UCard>

        <UCard v-else>
          <div class="empty-state !py-28">
            <div class="empty-icon"><UIcon name="i-lucide-scan-search" class="w-6 h-6" /></div>
            <p class="text-base text-slate-300 font-medium">Siap untuk investigasi</p>
            <p class="text-sm max-w-md">Pilih tipe atribut dan masukkan nilainya, atau klik salah satu item populer di panel kanan.</p>
          </div>
        </UCard>
      </div>

      <!-- Popular lists -->
      <div class="xl:col-span-4 space-y-6">
        <UCard :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-emerald-500/10 ring-1 ring-emerald-500/25 flex items-center justify-center">
                <UIcon name="i-lucide-server" class="w-5 h-5 text-emerald-400" />
              </div>
              <div>
                <h2 class="section-title">Nameserver Bersama</h2>
                <p class="section-desc !mt-0">Klik untuk menelusuri</p>
              </div>
            </div>
          </template>
          <div v-if="loadingNet" class="p-5 space-y-3"><USkeleton v-for="i in 5" :key="i" class="h-11 w-full" /></div>
          <div v-else class="max-h-[420px] overflow-y-auto divide-y divide-slate-800/70">
            <button v-for="item in topNS" :key="item.host" @click="queryLookup('NS', item.host)"
              class="w-full flex items-center gap-4 px-6 py-3.5 hover:bg-slate-800/40 transition text-left">
              <div class="flex-1 min-w-0">
                <p class="mono text-slate-200 truncate">{{ item.host }}</p>
                <UProgress :value="Math.min(100, (item.site_count / maxNS) * 100)" color="emerald" size="2xs" class="mt-2" />
              </div>
              <span class="text-sm font-semibold text-slate-300 tabular-nums">{{ item.site_count }}</span>
            </button>
            <p v-if="!topNS.length" class="muted px-6 py-8 text-center">Belum ada data.</p>
          </div>
        </UCard>

        <UCard :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-violet-500/10 ring-1 ring-violet-500/25 flex items-center justify-center">
                <UIcon name="i-lucide-fingerprint" class="w-5 h-5 text-violet-400" />
              </div>
              <div>
                <h2 class="section-title">Artefak Bersama</h2>
                <p class="section-desc !mt-0">Rekening, wallet & kontak terpopuler</p>
              </div>
            </div>
          </template>
          <div v-if="loadingNet" class="p-5 space-y-3"><USkeleton v-for="i in 5" :key="i" class="h-11 w-full" /></div>
          <div v-else class="max-h-[420px] overflow-y-auto divide-y divide-slate-800/70">
            <button v-for="item in topArtifacts" :key="item.type + item.value" @click="queryLookup(item.type, item.value)"
              class="w-full flex items-center gap-4 px-6 py-3.5 hover:bg-slate-800/40 transition text-left">
              <div class="flex-1 min-w-0">
                <p class="mono text-slate-200 truncate">{{ item.value }}</p>
                <p class="text-sm text-slate-400 mt-0.5"><span v-if="item.name" class="text-violet-300">{{ item.name }} · </span>{{ item.type }}</p>
              </div>
              <span class="text-sm font-semibold text-slate-300 tabular-nums">{{ item.site_count }}</span>
            </button>
            <p v-if="!topArtifacts.length" class="muted px-6 py-8 text-center">Belum ada data.</p>
          </div>
        </UCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

const { get } = useApi()

const loading    = ref(false)
const loadingNet = ref(true)
const qType      = ref('IP')
const qValue     = ref('')
const result     = ref<any>(null)
const topNS        = ref<any[]>([])
const topArtifacts = ref<any[]>([])
const lastQuery    = ref('')

const types = [
  { value: 'IP',      label: 'IP Address',         placeholder: '103.21.244.12',              hint: 'Situs yang di-hosting pada IP yang sama (pencocokan sebagian didukung, mis. awalan subnet).' },
  { value: 'NS',      label: 'Nameserver',         placeholder: 'tori.ns.cloudflare.com',     hint: 'Situs yang memakai nameserver yang sama.' },
  { value: 'FP',      label: 'Sidik Jari Infra',   placeholder: 'cloudfront:d33egg70nrp50s / jp-api.namesvr.dev', hint: 'ID distribusi CDN, hash favicon, atau host API backend — penanda kuat platform/operator yang sama.' },
  { value: 'SSL',     label: 'SSL Thumbprint',     placeholder: '40292fc056aa8548…',          hint: 'Situs dengan sertifikat SSL identik — indikasi kuat operator yang sama.' },
  { value: 'GA',      label: 'Google Analytics',   placeholder: 'G-DW5XRK7GYT / UA-123456',   hint: 'Situs yang berbagi ID Google Analytics / GA4.' },
  { value: 'GTM',     label: 'Google Tag Manager', placeholder: 'GTM-P24PF4B',                hint: 'Situs yang berbagi container Tag Manager.' },
  { value: 'FB',      label: 'Facebook Pixel',     placeholder: '1048294223511789',           hint: 'Situs yang berbagi Facebook Pixel ID.' },
  { value: 'Bank',    label: 'Rekening Bank',      placeholder: 'BCA atau 1234567890',        hint: 'Cari berdasarkan nomor rekening atau nama bank.' },
  { value: 'Crypto',  label: 'Crypto Wallet',      placeholder: '0x… / bc1… / T…',            hint: 'Alamat wallet ETH, BTC, atau USDT-TRC20.' },
  { value: 'Contact', label: 'Kontak (WA/TG)',     placeholder: 'username atau 62812…',       hint: 'Kontak WhatsApp, Telegram, Line, atau Discord.' },
  { value: 'Phone',   label: 'Nomor HP',           placeholder: '6281234567890',              hint: 'Nomor HP yang tercantum di situs.' },
  { value: 'QRIS',    label: 'QRIS',               placeholder: 'ID1234…',                    hint: 'Merchant ID QRIS.' },
]
const typeOptions = types.map(t => ({ value: t.value, label: t.label }))
const currentType = computed(() => types.find(t => t.value === qType.value) ?? types[0])
const columns = [
  { key: 'url', label: 'Situs' }, { key: 'priority', label: 'Prioritas' }, { key: 'confidence', label: 'Confidence' },
  { key: 'is_gambling', label: 'Status' }, { key: 'actions', label: '' },
]
const gamblingCount = computed(() => (result.value?.sites ?? []).filter((s: any) => s.is_gambling).length)
const maxNS = computed(() => Math.max(1, ...topNS.value.map((n: any) => n.site_count)))
const route = useRoute()

onMounted(async () => {
  try {
    const data = await get<any>('/intelligence/network-summary')
    if (data) {
      topNS.value        = data.top_ns || []
      topArtifacts.value = data.top_artifacts || []
    }
  } catch {} finally { loadingNet.value = false }
  // Dibuka dari link lain (mis. kartu artefak di Repository): ?type=Bank&v=123
  const t = route.query.type as string, v = route.query.v as string
  if (t && v) queryLookup(t, v)
})

async function runLookup() {
  if (!qValue.value.trim()) return
  loading.value = true
  result.value  = null
  try {
    lastQuery.value = qValue.value.trim()
    result.value = await get<any>('/intelligence/lookup', { type: qType.value, v: lastQuery.value })
  } catch (e: any) {
    useToast().add({ title: 'Lookup gagal', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { loading.value = false }
}

function queryLookup(type: string, val: string) {
  qType.value = type; qValue.value = val; runLookup()
}
</script>
