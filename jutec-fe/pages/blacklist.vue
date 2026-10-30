<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Konfigurasi" title="Domain Blacklist"
      subtitle="Domain yang dilewati saat crawling dan Deep Sweep — CDN, media sosial, analytics, dan layanan umum lain yang bukan target."
      icon="i-lucide-ban">
      <template #actions>
        <UButton icon="i-lucide-refresh-cw" size="lg" color="gray" variant="soft" label="Muat Ulang" :loading="loading" @click="load" />
      </template>
    </PageHeader>

    <div class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <!-- Add panel -->
      <div class="xl:col-span-4 xl:sticky xl:top-24 space-y-6">
        <UCard>
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-rose-500/10 ring-1 ring-rose-500/25 flex items-center justify-center">
                <UIcon name="i-lucide-shield-minus" class="w-5 h-5 text-rose-400" />
              </div>
              <div>
                <h2 class="section-title">Tambah Domain</h2>
                <p class="section-desc !mt-0">Akan diabaikan oleh crawler</p>
              </div>
            </div>
          </template>
          <div class="space-y-5">
            <UFormGroup label="Domain" size="lg" help="Tanpa http://, mis. facebook.com">
              <UInput v-model="newDomain" icon="i-lucide-globe" placeholder="contoh.com" :ui="{ base: 'font-mono' }" @keyup.enter="add" />
            </UFormGroup>
            <UFormGroup label="Kategori" size="lg">
              <USelectMenu v-model="newCategory" :options="categoryOptions" creatable searchable
                placeholder="Pilih atau ketik kategori" />
            </UFormGroup>
            <UButton block size="xl" icon="i-lucide-plus" :loading="adding" :disabled="!newDomain.trim()" label="Tambahkan ke Blacklist" @click="add" />
          </div>
        </UCard>

        <UCard>
          <template #header><h2 class="section-title">Per Kategori</h2></template>
          <div class="space-y-1">
            <button v-for="c in categories" :key="c.name" @click="filterCat = filterCat === c.name ? '' : c.name"
              class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg transition text-left"
              :class="filterCat === c.name ? 'bg-primary-500/15 ring-1 ring-primary-500/30' : 'hover:bg-slate-800/60'">
              <UIcon :name="catIcon(c.name)" class="w-[18px] h-[18px] text-slate-400" />
              <span class="flex-1 text-sm text-slate-200 capitalize">{{ catLabel(c.name) }}</span>
              <span class="text-sm tabular-nums text-slate-400">{{ c.count }}</span>
            </button>
          </div>
        </UCard>
      </div>

      <!-- List -->
      <div class="xl:col-span-8 min-w-0">
        <UCard>
          <template #header>
            <div class="flex flex-col sm:flex-row sm:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">Daftar Domain</h2>
                <p class="section-desc">
                  {{ filtered.length }} dari {{ list.length }} domain
                  <template v-if="filterCat"> · kategori <b class="text-slate-200">{{ catLabel(filterCat) }}</b>
                    <UButton color="gray" variant="link" size="xs" label="reset" :padded="false" class="ml-1" @click="filterCat = ''" />
                  </template>
                </p>
              </div>
              <UInput v-model="search" icon="i-lucide-search" placeholder="Cari domain…" class="w-full sm:w-72" />
            </div>
          </template>

          <div v-if="loading && !list.length" class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
            <USkeleton v-for="i in 9" :key="i" class="h-16 w-full" />
          </div>
          <div v-else-if="!filtered.length" class="empty-state">
            <div class="empty-icon"><UIcon name="i-lucide-shield-off" class="w-6 h-6" /></div>
            <p>{{ search || filterCat ? 'Tidak ada domain yang cocok.' : 'Blacklist masih kosong.' }}</p>
          </div>
          <div v-else class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
            <div v-for="item in filtered" :key="item.id"
              class="group rounded-xl bg-slate-950/60 ring-1 ring-slate-800 hover:ring-slate-700 px-4 py-3.5 flex items-center gap-3 transition">
              <div class="w-9 h-9 rounded-lg bg-slate-800/70 flex items-center justify-center shrink-0">
                <UIcon :name="catIcon(item.category)" class="w-[18px] h-[18px] text-slate-400" />
              </div>
              <div class="flex-1 min-w-0">
                <p class="mono text-slate-100 truncate" :title="item.domain">{{ item.domain }}</p>
                <p class="text-xs text-slate-500 mt-0.5 capitalize">{{ catLabel(item.category) }}</p>
              </div>
              <UTooltip text="Hapus">
                <UButton icon="i-lucide-trash-2" color="rose" variant="ghost" size="sm" class="opacity-60 group-hover:opacity-100"
                  @click="toDelete = item" />
              </UTooltip>
            </div>
          </div>
        </UCard>
      </div>
    </div>

    <!-- Delete confirmation -->
    <UModal :model-value="!!toDelete" @update:model-value="v => !v && (toDelete = null)">
      <UCard>
        <template #header>
          <div class="flex items-center gap-3">
            <UIcon name="i-lucide-triangle-alert" class="w-6 h-6 text-rose-400" />
            <h3 class="section-title">Hapus dari blacklist?</h3>
          </div>
        </template>
        <p class="text-slate-300">
          Domain <span class="mono text-white">{{ toDelete?.domain }}</span> akan kembali di-crawl pada sweep berikutnya.
        </p>
        <template #footer>
          <div class="flex justify-end gap-3">
            <UButton color="gray" variant="soft" label="Batal" @click="toDelete = null" />
            <UButton color="rose" icon="i-lucide-trash-2" label="Hapus" :loading="deleting" @click="confirmRemove" />
          </div>
        </template>
      </UCard>
    </UModal>
  </div>
</template>

<script setup lang="ts">
const { get, post, del } = useApi()
const toast = useToast()

const loading     = ref(false)
const adding      = ref(false)
const deleting    = ref(false)
const list        = ref<any[]>([])
const newDomain   = ref('')
const newCategory = ref('other')
const search      = ref('')
const filterCat   = ref('')
const toDelete    = ref<any>(null)

const catMeta: Record<string, { label: string; icon: string }> = {
  cdn:           { label: 'CDN',               icon: 'i-lucide-cloud' },
  platform:      { label: 'Platform',          icon: 'i-lucide-layout-grid' },
  aggregator:    { label: 'Aggregator',        icon: 'i-lucide-list' },
  analytics:     { label: 'Analytics',         icon: 'i-lucide-chart-no-axes-column' },
  social:        { label: 'Media Sosial',      icon: 'i-lucide-share-2' },
  shortener:     { label: 'URL Shortener',     icon: 'i-lucide-link-2' },
  chat:          { label: 'Chat',              icon: 'i-lucide-message-circle' },
  email:         { label: 'Email',             icon: 'i-lucide-mail' },
  standard:      { label: 'Standar',           icon: 'i-lucide-globe' },
  payment:       { label: 'Pembayaran',        icon: 'i-lucide-credit-card' },
  gambling_intl: { label: 'Judi Internasional', icon: 'i-lucide-dices' },
  news:          { label: 'Media Berita',      icon: 'i-lucide-newspaper' },
  regulator:     { label: 'Regulator Judi',    icon: 'i-lucide-scale' },
  other:         { label: 'Lainnya',           icon: 'i-lucide-circle-dot' },
}
const catLabel = (c?: string) => catMeta[c ?? '']?.label ?? (c || 'Lainnya')
const catIcon  = (c?: string) => catMeta[c ?? '']?.icon ?? 'i-lucide-circle-dot'

const categories = computed(() => {
  const counts: Record<string, number> = {}
  for (const d of list.value) counts[d.category || 'other'] = (counts[d.category || 'other'] ?? 0) + 1
  return Object.entries(counts).map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count)
})
const categoryOptions = computed(() => [...new Set(['other', ...Object.keys(catMeta), ...categories.value.map(c => c.name)])])

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return list.value.filter(d =>
    (!filterCat.value || (d.category || 'other') === filterCat.value) &&
    (!q || d.domain.toLowerCase().includes(q)))
})

async function load() {
  loading.value = true
  try {
    const res = await get<any>('/blacklist')
    list.value = (res?.domains ?? res ?? []).sort((a: any, b: any) => a.domain.localeCompare(b.domain))
  }
  catch { list.value = [] } finally { loading.value = false }
}

async function add() {
  const domain = newDomain.value.trim().replace(/^https?:\/\//, '').replace(/\/.*$/, '')
  if (!domain) return
  adding.value = true
  try {
    await post('/blacklist', { domain, category: newCategory.value === 'other' ? '' : newCategory.value })
    toast.add({ title: 'Domain ditambahkan', description: domain, color: 'emerald', icon: 'i-lucide-check' })
    newDomain.value = ''
    await load()
  } catch (e: any) {
    toast.add({ title: 'Gagal menambah domain', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { adding.value = false }
}

async function confirmRemove() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await del(`/blacklist/${toDelete.value.id}`)
    toast.add({ title: 'Domain dihapus', description: toDelete.value.domain, color: 'gray', icon: 'i-lucide-trash-2' })
    toDelete.value = null
    await load()
  } catch (e: any) {
    toast.add({ title: 'Gagal menghapus', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { deleting.value = false }
}

onMounted(load)
</script>
