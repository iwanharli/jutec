<template>
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="space-y-4">
        <div class="flex items-center gap-3">
          <div class="card-icon bg-violet-500/10 ring-violet-500/25"><UIcon name="i-lucide-syringe" class="w-[18px] h-[18px] text-violet-400" /></div>
          <div class="flex-1 min-w-0"><h2 class="card-title">Situs Susupan</h2><p class="card-desc">Situs institusi & organisasi yang dibobol untuk menampung konten judol — dikelompokkan per instansi</p></div>
          <UBadge color="gray" variant="subtle">{{ groups.length }} instansi</UBadge>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UInput v-model="q" icon="i-lucide-search" placeholder="Cari domain instansi…" class="w-full sm:w-72" />
          <SegmentedControl v-model="type" :options="typeOpts" />
          <UButton class="ml-auto" color="gray" icon="i-lucide-file-down" label="CSV untuk CSIRT" :disabled="!groups.length" @click="exportCsv" />
        </div>
      </div>
    </template>

    <div v-if="loading" class="p-5 space-y-2"><USkeleton v-for="i in 5" :key="i" class="h-16 w-full" /></div>
    <EmptyState v-else-if="!groups.length" icon="i-lucide-syringe" title="Belum ada situs susupan"
      desc="Ditemukan lewat pencarian dork (site:go.id …), arsip Common Crawl, dan deteksi cloaking/link tersembunyi saat scan." />
    <div v-else class="divide-y divide-slate-800/60">
      <div v-for="g in groups" :key="g.host" class="px-5 py-4">
        <div class="flex flex-wrap items-center gap-2">
          <UIcon :name="g.gov ? 'i-lucide-landmark' : 'i-lucide-building-2'" class="w-4 h-4 text-violet-400" />
          <span class="mono text-sm font-medium text-slate-100">{{ g.host }}</span>
          <UBadge color="gray" variant="subtle" size="xs">{{ g.kind }}</UBadge>
          <UBadge v-for="sig in g.signals" :key="sig" color="violet" variant="subtle" size="xs">{{ SIG[sig] ?? sig }}</UBadge>
          <span class="ml-auto text-xs text-slate-500">{{ g.pages.length }} halaman · ditemukan {{ day(g.first) }}</span>
        </div>
        <ul class="mt-2 space-y-1">
          <li v-for="p in g.pages.slice(0, 5)" :key="p.id" class="flex items-center gap-2 text-sm">
            <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="p.lifecycle === 'inactive' ? 'bg-slate-600' : 'bg-rose-500'" />
            <NuxtLink :to="`/site/${p.id}`" class="text-slate-300 hover:text-primary-300 break-all">{{ path(p.url) }}</NuxtLink>
            <span v-if="p.lifecycle === 'inactive'" class="text-[11px] text-slate-500 shrink-0">sudah dibersihkan / mati</span>
          </li>
          <li v-if="g.pages.length > 5" class="text-xs text-slate-500 pl-3.5">+{{ g.pages.length - 5 }} halaman lain</li>
        </ul>
        <p v-if="g.targets.length" class="text-xs text-slate-500 mt-2">
          Mengarah ke: <span v-for="(t, i) in g.targets.slice(0, 6)" :key="t" class="mono text-slate-400">{{ t }}<span v-if="i < Math.min(6, g.targets.length) - 1">, </span></span>
          <span v-if="g.targets.length > 6"> +{{ g.targets.length - 6 }}</span>
        </p>
      </div>
    </div>
  </UCard>
</template>

<script setup lang="ts">
const { get } = useApi()
const q = ref('')
const type = ref('')
const rows = ref<any[]>([])
const loading = ref(true)
const SIG: Record<string, string> = { cloaking: 'Cloaking', hidden_links: 'Link tersembunyi', parasite: 'Parasite SEO', subdomain_abuse: 'Subdomain disalahgunakan', path: 'Halaman sisipan' }

const isGov = (h: string) => /\.(go|mil|desa)\.id$|\.gov(\.[a-z]{2})?$/.test(h)
const kindOf = (h: string) => /\.(go|desa|mil)\.id$|\.gov/.test(h) ? 'Pemerintah' : /\.ac\.id$|\.edu/.test(h) ? 'Kampus'
  : /\.sch\.id$/.test(h) ? 'Sekolah' : /\.(or|ponpes)\.id$/.test(h) ? 'Organisasi' : 'Lainnya'

const groups = computed(() => {
  const m = new Map<string, any>()
  for (const r of rows.value) {
    const g = m.get(r.host) ?? { host: r.host, gov: isGov(r.host), kind: kindOf(r.host), pages: [], signals: new Set<string>(), targets: new Set<string>(), first: r.found_at }
    g.pages.push(r)
    const inj = r.injection
    for (const s of inj?.signals ?? []) g.signals.add(s)
    if (!inj?.detected) g.signals.add('path')
    for (const h of inj?.hidden_hosts ?? []) g.targets.add(h)
    for (const t of inj?.cloak_targets ?? []) g.targets.add(t.replace(/^https?:\/\//, ''))
    if (inj?.cloak_redirect) g.targets.add(inj.cloak_redirect.replace(/^https?:\/\//, '').replace(/\/.*$/, ''))
    if (r.found_at < g.first) g.first = r.found_at
    m.set(r.host, g)
  }
  return [...m.values()].map(g => ({ ...g, signals: [...g.signals], targets: [...g.targets] }))
    .filter(g => !type.value || g.kind === type.value)
    .sort((a, b) => b.pages.length - a.pages.length)
})
const typeOpts = computed(() => [{ value: '', label: 'Semua' }, ...['Pemerintah', 'Kampus', 'Sekolah', 'Organisasi', 'Lainnya']
  .map(v => ({ value: v, label: v, n: new Set(rows.value.filter(r => kindOf(r.host) === v).map(r => r.host)).size })).filter(o => o.n)])

const day = (d: string) => d ? new Date(d).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—'
const path = (u: string) => { try { const x = new URL(u); return (x.pathname + x.search) || '/' } catch { return u } }

function exportCsv() {
  const lines = [['instansi', 'jenis', 'sinyal', 'url_halaman', 'status', 'mengarah_ke', 'ditemukan'].join(',')]
  const esc = (v: string) => `"${String(v ?? '').replace(/"/g, '""')}"`
  for (const g of groups.value) for (const p of g.pages)
    lines.push([g.host, g.kind, g.signals.map((s: string) => SIG[s] ?? s).join('; '), p.url, p.lifecycle === 'inactive' ? 'tidak aktif' : 'aktif', g.targets.join('; '), p.found_at].map(esc).join(','))
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob(['﻿' + lines.join('\n')], { type: 'text/csv;charset=utf-8' }))
  a.download = `jutec-situs-susupan-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
}

let t: any
async function load() {
  loading.value = true
  try { rows.value = (await get<any>('/data/injected', { q: q.value || undefined })).results ?? [] }
  catch { rows.value = [] } finally { loading.value = false }
}
watch(q, () => { clearTimeout(t); t = setTimeout(load, 300) })
onMounted(load)
</script>
