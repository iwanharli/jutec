<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Investigasi" title="Form Pendaftaran" icon="i-lucide-clipboard-list"
      subtitle="Form daftar dari situs judol yang sudah valid — dibaca pasif saat scan (tidak pernah diisi). Bandingkan field, pilihan bank & tujuan submit untuk menemukan platform/operator yang sama." />

    <!-- ── Ringkasan hasil filter ── -->
    <div class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3">
      <div v-for="k in kpis" :key="k.label" class="rounded-2xl bg-slate-900/60 ring-1 ring-slate-800 p-4">
        <div class="flex items-center gap-2 text-[13px] text-slate-400"><UIcon :name="k.icon" class="w-4 h-4" :class="k.iconColor" />{{ k.label }}</div>
        <p class="text-2xl font-semibold tabular-nums mt-1.5 text-slate-100">{{ k.value }}</p>
        <p class="text-[12px] text-slate-500 mt-0.5">{{ k.hint }}</p>
      </div>
    </div>

    <!-- ── Filter ── -->
    <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-5' } }">
      <template #header>
        <div class="flex flex-wrap items-center gap-3">
          <div class="card-icon bg-primary-500/10 ring-primary-500/25"><UIcon name="i-lucide-sliders-horizontal" class="w-[18px] h-[18px] text-primary-400" /></div>
          <div class="flex-1 min-w-0"><h2 class="card-title">Filter</h2><p class="card-desc">Persempit form berdasarkan sumber, ciri keamanan & data yang diminta</p></div>
          <UButton v-if="activeFilters" color="gray" variant="soft" icon="i-lucide-rotate-ccw" @click="reset">Reset ({{ activeFilters }})</UButton>
        </div>
      </template>

      <div class="grid grid-cols-1 md:grid-cols-12 gap-3">
        <UInput v-model="q" icon="i-lucide-search" placeholder="Cari domain atau URL tujuan submit…" size="lg" class="md:col-span-6" />
        <USelect v-model="scope" :options="scopeOptions" size="lg" class="md:col-span-4" />
        <FilterToggle v-model="live" label="Hanya situs aktif" icon="i-lucide-activity" class="md:col-span-2" />
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3 mt-4">
        <div v-for="t in toggles" :key="t.key" class="rounded-xl ring-1 p-3 transition-colors"
          :class="flags[t.key] ? 'bg-primary-500/5 ring-primary-500/30' : 'bg-slate-950/40 ring-slate-800'">
          <div class="flex items-center gap-2 text-sm font-medium text-slate-200">
            <UIcon :name="t.icon" class="w-4 h-4" :class="flags[t.key] ? 'text-primary-400' : 'text-slate-400'" />{{ t.label }}
          </div>
          <div class="grid grid-cols-3 gap-1 mt-2.5 rounded-lg bg-slate-900 p-1 ring-1 ring-slate-800">
            <button v-for="o in triOptions" :key="o.value" class="rounded-md py-1.5 text-sm transition-colors"
              :class="flags[t.key] === o.value ? 'bg-primary-500/20 text-primary-200 font-medium' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'"
              @click="flags[t.key] = o.value">{{ o.label }}</button>
          </div>
        </div>
      </div>

      <div v-if="selFields.length || bank || host" class="flex flex-wrap items-center gap-2 mt-4 pt-4 border-t border-slate-800">
        <span class="text-sm text-slate-400">Harus memuat:</span>
        <UButton v-for="f in selFields" :key="f" size="sm" color="primary" variant="soft" trailing-icon="i-lucide-x" @click="toggleField(f)">{{ f }}</UButton>
        <UButton v-if="bank" size="sm" color="amber" variant="soft" trailing-icon="i-lucide-x" @click="bank = ''">Bank: {{ bank }}</UButton>
        <UButton v-if="host" size="sm" color="sky" variant="soft" trailing-icon="i-lucide-x" @click="host = ''">Submit ke: {{ host }}</UButton>
      </div>
    </UCard>

    <div class="grid grid-cols-1 xl:grid-cols-12 gap-5 items-start">
      <!-- ── Facet: klik untuk memfilter ── -->
      <div class="xl:col-span-3 space-y-5">
        <UCard v-for="fc in facetCards" :key="fc.key" :ui="{ header: { padding: 'px-5 py-3.5' }, body: { padding: 'px-3 py-2' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon" :class="fc.iconBg"><UIcon :name="fc.icon" class="w-[18px] h-[18px]" :class="fc.iconColor" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">{{ fc.title }}</h2><p class="card-desc">{{ fc.desc }}</p></div>
            </div>
          </template>
          <p v-if="!fc.items.length" class="text-[12px] text-slate-500 px-2 py-3">Belum ada data.</p>
          <ul v-else class="max-h-72 overflow-y-auto">
            <li v-for="it in fc.items" :key="it.value">
              <button class="w-full flex items-center gap-2 rounded-lg px-2 py-1.5 text-left hover:bg-slate-800/50"
                :class="fc.isOn(it.value) ? 'bg-primary-500/10' : ''" @click="fc.click(it.value)">
                <span class="text-[13px] truncate flex-1" :class="fc.isOn(it.value) ? 'text-primary-300' : 'text-slate-300'">{{ it.value }}</span>
                <span class="relative w-16 h-1.5 rounded-full bg-slate-800 overflow-hidden shrink-0">
                  <span class="absolute inset-y-0 left-0 rounded-full" :class="fc.bar" :style="{ width: `${(it.count / fc.max) * 100}%` }" />
                </span>
                <span class="text-[12px] tabular-nums text-slate-500 w-7 text-right">{{ it.count }}</span>
              </button>
            </li>
          </ul>
        </UCard>
      </div>

      <!-- ── Daftar form ── -->
      <UCard class="xl:col-span-9" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
        <template #header>
          <div class="flex items-center gap-3">
            <div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-file-input" class="w-[18px] h-[18px] text-rose-400" /></div>
            <div class="flex-1 min-w-0"><h2 class="card-title">Daftar Form</h2><p class="card-desc">Klik field, bank atau domain submit untuk memfilter</p></div>
            <UBadge color="gray" variant="subtle">{{ shown.length }}</UBadge>
          </div>
        </template>

        <div v-if="pending && !data" class="p-5 space-y-3"><USkeleton v-for="i in 5" :key="i" class="h-24 w-full" /></div>
        <EmptyState v-else-if="!data?.pool" icon="i-lucide-clipboard-x" title="Belum ada form pendaftaran"
          desc="Form terbaca otomatis saat situs judol di-scan dan akan muncul di sini setelah situsnya valid (diverifikasi analis atau punya bukti kuat)." />
        <EmptyState v-else-if="!shown.length" icon="i-lucide-filter-x" title="Tidak ada form yang cocok" desc="Longgarkan filter atau reset." />
        <div v-else class="divide-y divide-slate-800/60">
          <div v-for="(f, i) in shown" :key="f.site_id + '-' + i" class="px-5 py-4 hover:bg-slate-800/20">
            <div class="flex flex-wrap items-center gap-2">
              <NuxtLink :to="`/site/${f.site_id}`" class="text-sm font-medium text-slate-100 hover:text-primary-300 break-all">{{ stripScheme(f.url) }}</NuxtLink>
              <span v-if="SITE_CATEGORY[f.category]" class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium ring-1" :class="SITE_CATEGORY[f.category].tag">
                <UIcon :name="SITE_CATEGORY[f.category].icon" class="w-3 h-3" />{{ SITE_CATEGORY[f.category].label }}
              </span>
              <span v-if="f.verified === 'judol'" class="inline-flex items-center gap-0.5 text-[11px] text-emerald-400"><UIcon name="i-lucide-badge-check" class="w-3 h-3" />Terverifikasi</span>
              <span v-else class="inline-flex items-center gap-0.5 text-[11px] text-sky-400"><UIcon name="i-lucide-cpu" class="w-3 h-3" />Dikonfirmasi mesin</span>
              <span v-if="f.lifecycle === 'inactive'" class="inline-flex items-center gap-0.5 text-[11px] text-slate-500"><UIcon name="i-lucide-power-off" class="w-3 h-3" />Tidak aktif</span>
              <div class="ml-auto flex items-center gap-1">
                <UTooltip v-for="fl in flagBadges(f)" :key="fl.label" :text="fl.label">
                  <span class="inline-flex items-center justify-center w-6 h-6 rounded-md ring-1" :class="fl.cls"><UIcon :name="fl.icon" class="w-3.5 h-3.5" /></span>
                </UTooltip>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-x-3 gap-y-1 mt-1.5 text-[12px] text-slate-500">
              <a :href="f.page_url" target="_blank" rel="noopener noreferrer" class="hover:text-slate-300 break-all">{{ stripScheme(f.page_url) }}</a>
              <span v-if="f.action" class="inline-flex items-center gap-1">
                <UIcon name="i-lucide-arrow-right" class="w-3 h-3" />
                <span class="mono text-[11px] px-1 rounded bg-slate-800 text-slate-400">{{ f.method || 'GET' }}</span>
                <button class="mono hover:text-sky-300" :class="f.external_action ? 'text-amber-300' : 'text-slate-400'" @click="host = f.action_host">{{ f.action_host || f.action }}</button>
              </span>
            </div>

            <div class="flex flex-wrap gap-1 mt-2.5">
              <button v-for="fd in f.fields" :key="fd" @click="toggleField(norm(fd))"
                class="rounded-md px-1.5 py-0.5 text-[12px] mono ring-1 transition-colors"
                :class="selFields.includes(norm(fd)) ? 'bg-primary-500/15 text-primary-300 ring-primary-500/30' : 'bg-slate-800/60 text-slate-300 ring-slate-700/60 hover:ring-slate-500'">{{ fd }}</button>
              <span v-if="!f.fields.length" class="text-[12px] text-slate-600">Field tidak terbaca</span>
            </div>

            <div v-if="f.bank_options.length || f.referral_codes.length" class="flex flex-wrap items-center gap-1 mt-2">
              <span v-if="f.bank_options.length" class="text-[11px] text-slate-500 mr-0.5">Bank:</span>
              <button v-for="b in f.bank_options" :key="b" @click="bank = bank === b.toUpperCase() ? '' : b.toUpperCase()"
                class="rounded px-1.5 py-0.5 text-[11px] ring-1"
                :class="bank === b.toUpperCase() ? 'bg-amber-500/15 text-amber-300 ring-amber-500/30' : 'text-slate-400 ring-slate-800 hover:ring-slate-600'">{{ b }}</button>
              <span v-if="f.referral_codes.length" class="text-[11px] text-slate-500 ml-2 mr-0.5">Referral:</span>
              <span v-for="r in f.referral_codes" :key="r" class="mono rounded px-1.5 py-0.5 text-[11px] text-violet-300 ring-1 ring-violet-500/25">{{ r }}</span>
            </div>
          </div>
        </div>
      </UCard>
    </div>
  </div>
</template>

<script setup lang="ts">
const { get } = useApi()

const q = ref('')
const qDebounced = ref('')
let qt: any
watch(q, v => { clearTimeout(qt); qt = setTimeout(() => (qDebounced.value = v), 300) })

const scope = ref('valid')
const scopeOptions = [
  { value: 'valid', label: 'Terverifikasi + bukti kuat' },
  { value: 'verified', label: 'Hanya terverifikasi analis' },
]
const live = ref(false)
const selFields = ref<string[]>([])
const bank = ref('')
const host = ref('')
const flags = reactive<Record<string, string>>({ captcha: '', otp: '', phone: '', need_bank: '', external: '' })
const toggles = [
  { key: 'captcha', label: 'Captcha', icon: 'i-lucide-shield-check' },
  { key: 'otp', label: 'OTP', icon: 'i-lucide-message-square-lock' },
  { key: 'phone', label: 'Wajib HP/WA', icon: 'i-lucide-smartphone' },
  { key: 'need_bank', label: 'Minta rekening', icon: 'i-lucide-landmark' },
  { key: 'external', label: 'Submit eksternal', icon: 'i-lucide-external-link' },
]
const triOptions = [{ value: '', label: 'Semua' }, { value: 'yes', label: 'Ya' }, { value: 'no', label: 'Tidak' }]

const params = computed(() => ({
  scope: scope.value, q: qDebounced.value || undefined, live: live.value ? 1 : undefined,
  field: selFields.value.join(',') || undefined, bank: bank.value || undefined,
  ...Object.fromEntries(Object.entries(flags).filter(([, v]) => v)),
}))
const { data, pending } = useAsyncData('register-forms', () => get<any>('/data/register-forms', params.value), { watch: [params] })

// Filter domain submit di sisi klien (facet host dari hasil server)
const shown = computed(() => (data.value?.forms ?? []).filter((f: any) => !host.value || f.action_host === host.value))

const norm = (f: string) => f.toLowerCase().trim().replace(/[_\-\[\]]/g, ' ').replace(/\s+/g, ' ')
function toggleField(f: string) {
  selFields.value = selFields.value.includes(f) ? selFields.value.filter(x => x !== f) : [...selFields.value, f]
}
const activeFilters = computed(() => selFields.value.length + (bank.value ? 1 : 0) + (host.value ? 1 : 0) + (live.value ? 1 : 0) + (q.value ? 1 : 0)
  + Object.values(flags).filter(Boolean).length)
function reset() {
  q.value = ''; live.value = false; selFields.value = []; bank.value = ''; host.value = ''
  for (const k in flags) flags[k] = ''
}

const pct = (n: number) => data.value?.total ? `${Math.round((n / data.value.total) * 100)}%` : '–'
const kpis = computed(() => {
  const d = data.value, s = d?.stats ?? {}
  return [
    { label: 'Situs', icon: 'i-lucide-globe', iconColor: 'text-rose-400', value: d?.sites ?? 0, hint: `dari ${d?.pool ?? 0} situs valid berform` },
    { label: 'Form', icon: 'i-lucide-file-input', iconColor: 'text-slate-400', value: d?.total ?? 0, hint: 'form daftar sesuai filter' },
    { label: 'Captcha', icon: 'i-lucide-shield-check', iconColor: 'text-sky-400', value: pct(s.captcha ?? 0), hint: `${s.captcha ?? 0} form` },
    { label: 'Wajib HP/WA', icon: 'i-lucide-smartphone', iconColor: 'text-emerald-400', value: pct(s.phone ?? 0), hint: `${s.phone ?? 0} form · OTP ${s.otp ?? 0}` },
    { label: 'Minta rekening', icon: 'i-lucide-landmark', iconColor: 'text-amber-400', value: pct(s.bank ?? 0), hint: `${s.bank ?? 0} form` },
    { label: 'Submit eksternal', icon: 'i-lucide-external-link', iconColor: 'text-violet-400', value: pct(s.external ?? 0), hint: 'kirim data ke domain lain' },
  ]
})

const facetCards = computed(() => {
  const d = data.value
  const mk = (items: any[]) => Math.max(1, ...items.map(i => i.count))
  return [
    { key: 'fields', title: 'Field paling umum', desc: 'Pilih beberapa: form harus memuat semuanya', icon: 'i-lucide-text-cursor-input', iconBg: 'bg-primary-500/10 ring-primary-500/25', iconColor: 'text-primary-400', bar: 'bg-primary-500/70',
      items: d?.fields ?? [], max: mk(d?.fields ?? []), isOn: (v: string) => selFields.value.includes(v), click: toggleField },
    { key: 'banks', title: 'Pilihan bank / e-wallet', desc: 'Dari dropdown form daftar', icon: 'i-lucide-landmark', iconBg: 'bg-amber-500/10 ring-amber-500/25', iconColor: 'text-amber-400', bar: 'bg-amber-500/70',
      items: d?.banks ?? [], max: mk(d?.banks ?? []), isOn: (v: string) => bank.value === v, click: (v: string) => (bank.value = bank.value === v ? '' : v) },
    { key: 'hosts', title: 'Tujuan submit', desc: 'Domain yang menerima data — sama = backend sama', icon: 'i-lucide-server', iconBg: 'bg-sky-500/10 ring-sky-500/25', iconColor: 'text-sky-400', bar: 'bg-sky-500/70',
      items: d?.hosts ?? [], max: mk(d?.hosts ?? []), isOn: (v: string) => host.value === v, click: (v: string) => (host.value = host.value === v ? '' : v) },
  ]
})

function flagBadges(f: any) {
  const on = 'text-slate-200 ring-slate-600 bg-slate-800', off = 'text-slate-700 ring-slate-800'
  return [
    { label: f.has_captcha ? 'Pakai captcha' : 'Tanpa captcha', icon: 'i-lucide-shield-check', cls: f.has_captcha ? on : off },
    { label: f.has_otp ? 'Verifikasi OTP' : 'Tanpa OTP', icon: 'i-lucide-message-square-lock', cls: f.has_otp ? on : off },
    { label: f.needs_phone ? 'Wajib nomor HP/WA' : 'Tanpa nomor HP', icon: 'i-lucide-smartphone', cls: f.needs_phone ? on : off },
    { label: f.needs_bank ? 'Minta nomor rekening' : 'Tanpa rekening', icon: 'i-lucide-landmark', cls: f.needs_bank ? 'text-amber-300 ring-amber-500/30 bg-amber-500/10' : off },
    { label: f.external_action ? 'Submit ke domain lain' : 'Submit ke domain sendiri', icon: 'i-lucide-external-link', cls: f.external_action ? 'text-violet-300 ring-violet-500/30 bg-violet-500/10' : off },
  ]
}

function stripScheme(u: string) { return (u ?? '').replace(/^https?:\/\//, '').replace(/\/$/, '') }

useHead({ title: 'Form Pendaftaran' })
</script>
