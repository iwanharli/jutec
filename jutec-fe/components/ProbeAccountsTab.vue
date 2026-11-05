<template>
  <!-- Akun dummy yang berhasil dibuat lewat Uji Form Terkendali -->
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="space-y-4">
        <div class="flex items-center gap-3">
          <div class="card-icon bg-emerald-500/10 ring-emerald-500/25"><UIcon name="i-lucide-user-check" class="w-[18px] h-[18px] text-emerald-400" /></div>
          <div class="flex-1 min-w-0">
            <h2 class="card-title">Akun Terdaftar</h2>
            <p class="card-desc">Akun dummy yang diterima situs judol — pendaftaran berhasil atau bisa login. Password sama dengan data dummy.</p>
          </div>
          <UBadge color="gray" variant="subtle">{{ rows.length }}</UBadge>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UInput v-model="q" icon="i-lucide-search" placeholder="Cari domain atau username…" class="w-full sm:w-72" />
          <SegmentedControl v-model="filter" :options="filterOpts" />
        </div>
      </div>
    </template>

    <div v-if="loading" class="p-5 space-y-2"><USkeleton v-for="i in 4" :key="i" class="h-16 w-full" /></div>
    <EmptyState v-else-if="!shown.length" icon="i-lucide-user-check" title="Belum ada akun terdaftar"
      desc="Jalankan “Uji daftar & login” di halaman detail situs judol. Akun yang diterima situs akan muncul di sini." />
    <div v-else class="divide-y divide-slate-800/60">
      <div v-for="a in shown" :key="a.probe_id" class="px-5 py-4 hover:bg-slate-800/20">
        <div class="flex flex-wrap items-center gap-2">
          <NuxtLink :to="`/site/${a.site_id}`" class="text-sm font-medium text-slate-100 hover:text-primary-300 break-all">{{ host(a.url) }}</NuxtLink>
          <span v-if="SITE_CATEGORY[a.category]" class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium ring-1" :class="SITE_CATEGORY[a.category].tag">
            <UIcon :name="SITE_CATEGORY[a.category].icon" class="w-3 h-3" />{{ SITE_CATEGORY[a.category].label }}
          </span>
          <span v-if="a.lifecycle === 'inactive'" class="text-[11px] text-slate-500">situs tidak aktif</span>
          <span class="ml-auto text-xs text-slate-500">{{ fmt(a.created_at) }}<template v-if="a.requested_by"> · {{ a.requested_by }}</template></span>
        </div>

        <div class="flex flex-wrap items-center gap-2 mt-2">
          <span class="inline-flex items-center gap-1.5 rounded-lg bg-slate-950/50 ring-1 ring-slate-800 pl-2 pr-1 py-1">
            <UIcon name="i-lucide-at-sign" class="w-3.5 h-3.5 text-slate-500" />
            <span class="mono text-sm text-slate-100">{{ a.username }}</span>
            <UTooltip text="Salin username"><UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copy(a.username)" /></UTooltip>
          </span>
          <UBadge :color="a.outcome === 'accepted' ? 'emerald' : 'gray'" variant="subtle" size="xs">{{ a.outcome === 'accepted' ? 'Pendaftaran diterima' : 'Daftar: respons tak jelas' }}</UBadge>
          <UBadge :color="a.login_status === 'logged_in' ? 'emerald' : 'amber'" variant="subtle" size="xs">{{ a.login_status === 'logged_in' ? 'Bisa login' : 'Belum bisa login' }}</UBadge>
          <UBadge v-if="(a.deposit_urls ?? []).length" color="sky" variant="subtle" size="xs">{{ a.deposit_urls.length }} halaman deposit</UBadge>
          <span v-for="p in a.platform ?? []" :key="p" class="text-[11px] text-slate-500">{{ p }}</span>
        </div>
        <p v-if="a.message" class="text-xs text-slate-500 mt-1.5">Pesan situs: “{{ a.message }}”</p>

        <div v-if="money(a).length" class="flex flex-wrap items-center gap-1.5 mt-2.5">
          <span class="text-[11px] text-amber-300/80">Rekening penampung:</span>
          <span v-for="r in money(a)" :key="r.type + r.value" class="rounded-md px-2 py-0.5 text-xs ring-1 ring-amber-500/30 text-amber-100">
            <span class="text-amber-300/80 mr-1">{{ r.name || r.type }}</span><span class="mono">{{ r.value }}</span>
          </span>
        </div>

        <div class="flex flex-wrap items-center gap-1.5 mt-3">
          <UButton size="xs" :color="openId === a.probe_id ? 'primary' : 'gray'" :variant="openId === a.probe_id ? 'soft' : 'soft'"
            :icon="openId === a.probe_id ? 'i-lucide-chevron-up' : 'i-lucide-list'" :label="openId === a.probe_id ? 'Tutup detail' : 'Detail uji'" @click="toggle(a.probe_id)" />
          <UButton size="xs" color="gray" variant="soft" icon="i-lucide-log-in" :to="a.page_url" target="_blank" label="Buka situs" />
          <UButton size="xs" color="gray" variant="ghost" icon="i-lucide-panel-right-open" :to="`/site/${a.site_id}`" label="Halaman situs" />
        </div>

        <!-- Detail lengkap uji: berkas bukti & respons server -->
        <div v-if="openId === a.probe_id" class="mt-3 rounded-xl bg-slate-950/50 ring-1 ring-slate-800 p-4 space-y-3">
          <div v-if="!detail" class="space-y-2"><USkeleton v-for="i in 3" :key="i" class="h-6 w-full" /></div>
          <template v-else>
            <div v-if="Object.keys(detail.result?.filled ?? {}).length" class="text-xs text-slate-500">
              Field diisi: <span class="text-slate-300">{{ Object.keys(detail.result.filled).join(', ') }}</span>
              <template v-if="detail.result?.missing?.length"> · <span class="text-amber-400">tanpa data: {{ detail.result.missing.join(', ') }}</span></template>
            </div>
            <p v-if="detail.result?.login?.message" class="text-xs"><span class="text-slate-500">Pesan login:</span> <span class="text-slate-200">“{{ detail.result.login.message }}”</span></p>
            <div v-if="files.length">
              <p class="text-[11px] uppercase tracking-wider text-slate-500 mb-1.5">Berkas bukti</p>
              <div class="flex flex-wrap gap-1.5">
                <UButton v-for="f in files" :key="f.name" size="2xs" color="gray" variant="soft" :icon="f.name.endsWith('.png') ? 'i-lucide-image' : 'i-lucide-file-code'"
                  :to="`${apiBase}/form-probes/${a.probe_id}/files/${f.name}`" target="_blank" :label="f.name" />
              </div>
            </div>
            <details v-if="exchanges.length" open>
              <summary class="cursor-pointer text-[11px] uppercase tracking-wider text-slate-500 select-none">Respons server ({{ exchanges.length }} permintaan)</summary>
              <div class="mt-2 space-y-2">
                <div v-for="(x, i) in exchanges" :key="i" class="rounded-lg bg-slate-900/60 ring-1 ring-slate-800 p-2.5">
                  <div class="flex items-center gap-2 text-xs">
                    <span class="mono px-1 rounded bg-slate-800 text-slate-300">{{ x.method }}</span>
                    <span class="mono tabular-nums" :class="x.status >= 400 ? 'text-rose-400' : 'text-emerald-400'">{{ x.status || '—' }}</span>
                    <span class="mono text-slate-400 truncate">{{ x.url }}</span>
                  </div>
                  <p v-if="x.request" class="mono text-[11px] text-slate-500 mt-1.5 break-all"><span class="text-slate-400">dikirim:</span> {{ x.request }}</p>
                  <p v-if="x.headers && Object.keys(x.headers).length" class="mono text-[11px] text-slate-500 mt-1 break-all">
                    <span v-for="(v, k) in x.headers" :key="k" class="mr-3"><span class="text-slate-400">{{ k }}:</span> {{ v }}</span>
                  </p>
                  <pre v-if="x.body" class="mono text-[11px] text-slate-400 mt-1.5 max-h-40 overflow-auto whitespace-pre-wrap break-all">{{ x.body }}</pre>
                </div>
              </div>
            </details>
            <p v-else class="text-xs text-slate-500">Tidak ada respons server yang terekam.</p>
          </template>
        </div>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const { get, apiBase } = useApi()
const openId = ref<number | null>(null)
const detail = ref<any>(null)
async function toggle(id: number) {
  if (openId.value === id) { openId.value = null; return }
  openId.value = id; detail.value = null
  try { detail.value = await get<any>(`/form-probes/${id}`) } catch {}
}
const files = computed(() => Array.isArray(detail.value?.files) ? detail.value.files : [])
const exchanges = computed(() => [...(detail.value?.result?.exchanges ?? []), ...(detail.value?.result?.login?.exchanges ?? [])])
const toast = useToast()
const q = ref('')
const filter = ref('')
const rows = ref<any[]>([])
const loading = ref(true)

const filterOpts = computed(() => [
  { value: '', label: 'Semua', n: rows.value.length },
  { value: 'login', label: 'Bisa login', n: rows.value.filter(a => a.login_status === 'logged_in').length },
  { value: 'money', label: 'Ada rekening', n: rows.value.filter(a => money(a).length).length },
])
const shown = computed(() => rows.value.filter(a =>
  !filter.value || (filter.value === 'login' ? a.login_status === 'logged_in' : money(a).length > 0)))

const money = (a: any) => {
  const all = (a.artifacts ?? []).filter((x: any) => x && x.value)
  return all.filter((x: any, i: number) => all.findIndex((y: any) => y.type === x.type && y.value === x.value) === i)
}
const host = (u: string) => u.replace(/^https?:\/\//, '').replace(/\/$/, '')
const fmt = (d: string) => new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
async function copy(v: string) {
  try { await navigator.clipboard.writeText(v); toast.add({ title: 'Username disalin', color: 'emerald', icon: 'i-lucide-check' }) } catch {}
}

let t: any
async function load() {
  loading.value = true
  try { rows.value = (await get<any>('/data/probe-accounts', { q: q.value || undefined })).results ?? [] } catch { rows.value = [] }
  finally { loading.value = false }
}
watch(q, () => { clearTimeout(t); t = setTimeout(load, 300) })
onMounted(load)
</script>
