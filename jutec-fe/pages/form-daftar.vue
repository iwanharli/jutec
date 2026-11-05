<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Investigasi" title="Form Daftar & Login" icon="i-lucide-clipboard-list"
      subtitle="Form dari situs judol yang sudah valid — dibaca pasif saat scan (tidak pernah diisi). Form yang identik di banyak situs menandakan platform/operator yang sama." />

    <!-- ── Jenis form ── -->
    <div class="flex flex-wrap items-center gap-3">
      <SegmentedControl v-model="kind" :options="kindOptions" />
      <p class="text-sm text-slate-500">{{ kind === 'batch' ? 'Daftar & login otomatis ke banyak situs judol valid sekaligus.' : kind === 'dummy' ? 'Identitas dummy yang dipakai untuk mendaftar di situs judol.' : kind === 'accounts' ? 'Akun dummy hasil Uji Form yang diterima situs judol.' : kind === 'login' ? 'Form masuk akun — tujuan submit menunjukkan server backend yang dipakai.' : 'Form pendaftaran pemain — data yang diminta & rekening penampung.' }}</p>
    </div>

    <ProbeAccountsTab v-if="kind === 'accounts'" />
    <ProbeProfilesTab v-else-if="kind === 'dummy'" />
    <ProbeBatchTab v-else-if="kind === 'batch'" />

    <template v-else>
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

      <div v-if="selFields.length || bank || host || pattern" class="flex flex-wrap items-center gap-2 mt-4 pt-4 border-t border-slate-800">
        <span class="text-sm text-slate-400">Harus memuat:</span>
        <UButton v-for="f in selFields" :key="f" size="sm" color="primary" variant="soft" trailing-icon="i-lucide-x" @click="toggleField(f)">{{ fieldLabel(f) }}</UButton>
        <UButton v-if="pattern" size="sm" color="rose" variant="soft" trailing-icon="i-lucide-x" @click="pattern = ''">Pola form {{ pattern }}</UButton>
        <UButton v-if="bank" size="sm" color="amber" variant="soft" trailing-icon="i-lucide-x" @click="bank = ''">Bank: {{ bank }}</UButton>
        <UButton v-if="host" size="sm" color="sky" variant="soft" trailing-icon="i-lucide-x" @click="host = ''">Submit ke: {{ host }}</UButton>
      </div>
    </UCard>

    <div class="grid grid-cols-1 xl:grid-cols-12 gap-5 items-start">
      <!-- ── Facet: klik untuk memfilter ── -->
      <div class="xl:col-span-3 space-y-5">
        <UCard :ui="{ header: { padding: 'px-4 py-3' }, body: { padding: 'p-3' } }">
          <template #header>
            <div class="flex items-center gap-2.5">
              <UIcon name="i-lucide-copy" class="w-4 h-4 shrink-0 text-rose-400" />
              <h2 class="text-sm font-semibold text-slate-100 flex-1 truncate">Pola Form Identik</h2>
              <span class="text-[11px] text-slate-500 tabular-nums">{{ data?.patterns?.length ?? 0 }}</span>
            </div>
            <p class="text-[11px] text-slate-500 mt-0.5">Field & tujuan submit sama persis di beberapa situs</p>
          </template>
          <p v-if="!data?.patterns?.length" class="text-[12px] text-slate-500 px-2 py-2">Belum ada pola yang dipakai lebih dari satu situs.</p>
          <div v-else class="space-y-2">
            <button v-for="p in (expanded.patterns ? data.patterns : data.patterns.slice(0, 5))" :key="p.id" type="button" @click="pattern = pattern === p.id ? '' : p.id"
              class="w-full text-left rounded-xl p-3 ring-1 transition-colors"
              :class="pattern === p.id ? 'bg-rose-500/10 ring-rose-500/40' : 'bg-slate-950/40 ring-slate-800 hover:ring-slate-600'">
              <div class="flex items-center gap-2">
                <span class="text-sm font-semibold text-rose-300 tabular-nums">{{ p.sites }} situs</span>
                <span class="text-[11px] text-slate-500 truncate">→ {{ p.host || 'tanpa tujuan' }}</span>
              </div>
              <p class="text-[12px] text-slate-300 mt-1 truncate">{{ (p.sample ?? []).join(', ') }}</p>
              <p class="text-[11px] text-slate-500 mt-1 line-clamp-2">{{ (p.fields ?? []).join(' · ') || '—' }}</p>
            </button>
            <button v-if="data.patterns.length > 5" type="button" class="w-full text-[12px] text-slate-400 hover:text-slate-200 inline-flex items-center justify-center gap-1 pt-1"
              @click="expanded.patterns = !expanded.patterns">
              <UIcon :name="expanded.patterns ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'" class="w-3.5 h-3.5" />
              {{ expanded.patterns ? 'Ringkas' : `Tampilkan semua (${data.patterns.length})` }}
            </button>
          </div>
        </UCard>
        <UCard v-for="fc in facetCards" :key="fc.key" :ui="{ header: { padding: 'px-4 py-3' }, body: { padding: 'px-4 py-3' } }">
          <template #header>
            <div class="flex items-center gap-2.5">
              <UIcon :name="fc.icon" class="w-4 h-4 shrink-0" :class="fc.iconColor" />
              <h2 class="text-sm font-semibold text-slate-100 flex-1 truncate">{{ fc.title }}</h2>
              <span class="text-[11px] text-slate-500 tabular-nums">{{ fc.items.length }}</span>
            </div>
            <p class="text-[11px] text-slate-500 mt-0.5">{{ fc.desc }}</p>
          </template>
          <p v-if="!fc.items.length" class="text-[12px] text-slate-500">Belum ada data.</p>
          <template v-else>
            <div class="flex flex-wrap gap-1.5">
              <button v-for="it in (expanded[fc.key] ? fc.items : fc.items.slice(0, 12))" :key="it.value" type="button" @click="fc.click(it.value)"
                class="group inline-flex items-center gap-1.5 rounded-lg pl-2 pr-1 py-1 text-[12px] ring-1 transition-colors max-w-full"
                :class="fc.isOn(it.value) ? fc.on : 'bg-slate-950/40 text-slate-300 ring-slate-800 hover:ring-slate-600 hover:text-slate-100'">
                <span class="truncate">{{ it.label || it.value }}</span>
                <span class="rounded-md px-1.5 py-px text-[10px] tabular-nums"
                  :class="fc.isOn(it.value) ? 'bg-white/10' : 'bg-slate-800 text-slate-400 group-hover:text-slate-200'">{{ it.count }}</span>
              </button>
            </div>
            <button v-if="fc.items.length > 12" type="button" class="mt-2.5 text-[12px] text-slate-400 hover:text-slate-200 inline-flex items-center gap-1"
              @click="expanded[fc.key] = !expanded[fc.key]">
              <UIcon :name="expanded[fc.key] ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'" class="w-3.5 h-3.5" />
              {{ expanded[fc.key] ? 'Ringkas' : `Tampilkan semua (${fc.items.length})` }}
            </button>
          </template>
        </UCard>
      </div>

      <!-- ── Daftar form ── -->
      <UCard class="xl:col-span-9" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
        <template #header>
          <div class="flex items-center gap-3">
            <div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-file-input" class="w-[18px] h-[18px] text-rose-400" /></div>
            <div class="flex-1 min-w-0"><h2 class="card-title">{{ kind === 'login' ? 'Form Login' : 'Form Pendaftaran' }}</h2><p class="card-desc">Klik field, bank atau domain submit untuk memfilter</p></div>
            <UBadge color="gray" variant="subtle">{{ shown.length }}</UBadge>
          </div>
        </template>

        <div v-if="pending && !data" class="p-5 space-y-3"><USkeleton v-for="i in 5" :key="i" class="h-24 w-full" /></div>
        <EmptyState v-else-if="!data?.pool" icon="i-lucide-clipboard-x" :title="kind === 'login' ? 'Belum ada form login' : 'Belum ada form pendaftaran'"
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
              <button v-if="f.pattern_sites > 1" type="button" @click="pattern = f.pattern"
                class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium text-rose-300 ring-1 ring-rose-500/30 hover:bg-rose-500/10">
                <UIcon name="i-lucide-copy" class="w-3 h-3" />Form sama dengan {{ f.pattern_sites - 1 }} situs lain
              </button>
              <span v-if="f.pages > 1" class="text-[11px] text-slate-500">muncul di {{ f.pages }} halaman</span>
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
              <UTooltip v-for="fd in f.fields" :key="fd.key" :text="`Label di situs: ${fd.raw}`">
                <button type="button" @click="!fd.key.startsWith('other:') && toggleField(fd.key)"
                  class="rounded-md px-1.5 py-0.5 text-[12px] ring-1 transition-colors"
                  :class="selFields.includes(fd.key) ? 'bg-primary-500/15 text-primary-300 ring-primary-500/30'
                    : fd.key.startsWith('other:') ? 'text-slate-500 ring-slate-800 mono' : 'bg-slate-800/60 text-slate-300 ring-slate-700/60 hover:ring-slate-500'">{{ fd.key.startsWith('other:') ? fd.raw : fd.label }}</button>
              </UTooltip>
              <span v-if="!f.fields?.length" class="text-[12px] text-slate-600">Field tidak terbaca</span>
            </div>

            <div v-if="f.bank_options?.length || f.referral_codes?.length" class="flex flex-wrap items-center gap-1 mt-2">
              <span v-if="f.bank_options?.length" class="text-[11px] text-slate-500 mr-0.5">Bank:</span>
              <button v-for="b in f.bank_options" :key="b" @click="bank = bank === b.toUpperCase() ? '' : b.toUpperCase()"
                class="rounded px-1.5 py-0.5 text-[11px] ring-1"
                :class="bank === b.toUpperCase() ? 'bg-amber-500/15 text-amber-300 ring-amber-500/30' : 'text-slate-400 ring-slate-800 hover:ring-slate-600'">{{ b }}</button>
              <span v-if="f.referral_codes?.length" class="text-[11px] text-slate-500 ml-2 mr-0.5">Referral:</span>
              <span v-for="r in f.referral_codes" :key="r" class="mono rounded px-1.5 py-0.5 text-[11px] text-violet-300 ring-1 ring-violet-500/25">{{ r }}</span>
            </div>
          </div>
        </div>
      </UCard>
    </div>
    </template>
  </div>
</template>

<script setup lang="ts">
const { get } = useApi()

const q = ref('')
const qDebounced = ref('')
let qt: any
watch(q, v => { clearTimeout(qt); qt = setTimeout(() => (qDebounced.value = v), 300) })

const kind = ref<'register' | 'login' | 'accounts' | 'dummy' | 'batch'>((['accounts', 'dummy', 'batch'] as const).find(t => t === useRoute().query.tab) ?? 'register')
const pattern = ref('')
const expanded = reactive<Record<string, boolean>>({})
const kindOptions = computed(() => [
  { value: 'register', label: 'Form Pendaftaran', n: data.value?.counts?.register ?? null },
  { value: 'login', label: 'Form Login', n: data.value?.counts?.login ?? null },
  { value: 'accounts', label: 'Akun Terdaftar', dot: 'bg-emerald-500' },
  { value: 'dummy', label: 'Data Dummy', dot: 'bg-rose-500' },
  { value: 'batch', label: 'Uji Massal', dot: 'bg-rose-500' },
])
const fieldLabel = (k: string) => data.value?.fields?.find((f: any) => f.value === k)?.label ?? k
watch(kind, () => { selFields.value = []; pattern.value = ''; bank.value = ''; host.value = '' })
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
  kind: kind.value === 'login' ? 'login' : 'register', pattern: pattern.value || undefined,
  field: selFields.value.join(',') || undefined, bank: bank.value || undefined,
  ...Object.fromEntries(Object.entries(flags).filter(([, v]) => v)),
}))
// Field bisa berupa teks (backend lama) atau objek baku {key,label,raw} — samakan bentuknya
const asField = (f: any) => typeof f === 'string' ? { key: `other:${f}`, label: f, raw: f } : { key: f?.key ?? `other:${f?.raw ?? ''}`, label: f?.label ?? f?.raw ?? '', raw: f?.raw ?? f?.label ?? '' }
const { data, pending } = useAsyncData('register-forms', async () => {
  const d = await get<any>('/data/register-forms', params.value)
  for (const f of d?.forms ?? []) f.fields = (f.fields ?? []).map(asField)
  return d
}, { watch: [params] })

// Filter domain submit di sisi klien (facet host dari hasil server)
const shown = computed(() => (data.value?.forms ?? []).filter((f: any) => !host.value || f.action_host === host.value))

function toggleField(f: string) {
  selFields.value = selFields.value.includes(f) ? selFields.value.filter(x => x !== f) : [...selFields.value, f]
}
const activeFilters = computed(() => selFields.value.length + (bank.value ? 1 : 0) + (host.value ? 1 : 0) + (live.value ? 1 : 0) + (q.value ? 1 : 0) + (pattern.value ? 1 : 0)
  + Object.values(flags).filter(Boolean).length)
function reset() {
  q.value = ''; live.value = false; selFields.value = []; bank.value = ''; host.value = ''; pattern.value = ''
  for (const k in flags) flags[k] = ''
}

const pct = (n: number) => data.value?.total ? `${Math.round((n / data.value.total) * 100)}%` : '–'
const kpis = computed(() => {
  const d = data.value, s = d?.stats ?? {}
  return [
    { label: 'Situs', icon: 'i-lucide-globe', iconColor: 'text-rose-400', value: d?.sites ?? 0, hint: `dari ${d?.pool ?? 0} situs valid berform` },
    { label: 'Form', icon: 'i-lucide-file-input', iconColor: 'text-slate-400', value: d?.total ?? 0, hint: kind.value === 'login' ? 'form login sesuai filter' : 'form daftar sesuai filter' },
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
    { key: 'fields', title: 'Field paling umum', desc: 'Pilih beberapa: form harus memuat semuanya', icon: 'i-lucide-text-cursor-input', iconBg: 'bg-primary-500/10 ring-primary-500/25', iconColor: 'text-primary-400', bar: 'bg-primary-500/70', on: 'bg-primary-500/15 text-primary-200 ring-primary-500/40',
      items: d?.fields ?? [], max: mk(d?.fields ?? []), isOn: (v: string) => selFields.value.includes(v), click: toggleField },
    { key: 'banks', title: 'Pilihan bank / e-wallet', desc: 'Dari dropdown form daftar', icon: 'i-lucide-landmark', iconBg: 'bg-amber-500/10 ring-amber-500/25', iconColor: 'text-amber-400', bar: 'bg-amber-500/70', on: 'bg-amber-500/15 text-amber-200 ring-amber-500/40',
      items: d?.banks ?? [], max: mk(d?.banks ?? []), isOn: (v: string) => bank.value === v, click: (v: string) => (bank.value = bank.value === v ? '' : v) },
    { key: 'hosts', title: 'Tujuan submit', desc: 'Domain yang menerima data — sama = backend sama', icon: 'i-lucide-server', iconBg: 'bg-sky-500/10 ring-sky-500/25', iconColor: 'text-sky-400', bar: 'bg-sky-500/70', on: 'bg-sky-500/15 text-sky-200 ring-sky-500/40',
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
