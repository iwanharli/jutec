<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Konfigurasi" title="Keywords"
      subtitle="Kelola kata kunci untuk deteksi heuristik konten dan query dorking yang dipakai saat Deep Sweep."
      icon="i-lucide-tags">
      <template #actions>
        <div class="flex p-1 rounded-xl bg-slate-900/70 ring-1 ring-slate-800 gap-1">
          <UButton v-for="t in types" :key="t.value" :icon="t.icon" :label="t.label" size="lg"
            :color="activeType === t.value ? 'primary' : 'gray'" :variant="activeType === t.value ? 'solid' : 'ghost'"
            @click="activeType = t.value" />
        </div>
      </template>
    </PageHeader>

    <div class="grid grid-cols-1 xl:grid-cols-12 gap-6 items-start">
      <!-- Add panel -->
      <div class="xl:col-span-4 xl:sticky xl:top-24">
        <UCard>
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-amber-500/10 ring-1 ring-amber-500/25 flex items-center justify-center">
                <UIcon name="i-lucide-plus" class="w-5 h-5 text-amber-400" />
              </div>
              <div>
                <h2 class="section-title">Tambah Keyword</h2>
                <p class="section-desc !mt-0">{{ currentType.desc }}</p>
              </div>
            </div>
          </template>

          <div class="space-y-6">
            <UFormGroup label="Frasa" size="lg">
              <UInput v-model="form.keyword" :placeholder="activeType === 'detection' ? 'contoh: slot gacor maxwin' : 'contoh: situs slot gacor hari ini'"
                @keyup.enter="save" />
            </UFormGroup>

            <div v-if="activeType === 'detection'">
              <div class="flex items-center justify-between mb-3">
                <span class="field-label !mb-0">Bobot</span>
                <UBadge :color="weightColor(Number(form.weight))" variant="subtle" size="lg" class="tabular-nums">{{ Number(form.weight).toFixed(1) }}</UBadge>
              </div>
              <URange v-model="form.weight" :min="0.1" :max="1" :step="0.1" color="amber" size="md" />
              <div class="flex justify-between text-xs text-slate-500 mt-2">
                <span>0.1 · Lemah</span><span>1.0 · Sangat kuat</span>
              </div>
            </div>

            <UButton block size="xl" icon="i-lucide-save" :loading="saving" :disabled="!form.keyword.trim()" label="Simpan Keyword" @click="save" />

            <UAlert v-if="activeType === 'detection'" icon="i-lucide-lightbulb" color="amber" variant="subtle"
              description="Bobot tinggi (0.8+) sebaiknya hanya untuk kata yang eksklusif judol agar tidak memicu false positive." />
          </div>
        </UCard>

        <!-- Belajar dari verifikasi: ketepatan kata kunci dihitung dari situs yang sudah diverifikasi analis -->
        <UCard v-if="activeType === 'detection' && stats" class="mt-6">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-emerald-500/10 ring-1 ring-emerald-500/25 flex items-center justify-center">
                <UIcon name="i-lucide-brain" class="w-5 h-5 text-emerald-400" />
              </div>
              <div>
                <h2 class="section-title">Belajar dari Verifikasi</h2>
                <p class="section-desc !mt-0">Saran bobot dari situs yang sudah diverifikasi analis</p>
              </div>
            </div>
          </template>
          <div class="space-y-3">
            <div v-for="b in readiness" :key="b.label">
              <div class="flex justify-between text-[13px] mb-1">
                <span class="text-slate-300">{{ b.label }}</span>
                <span class="tabular-nums" :class="b.n >= TARGET ? 'text-emerald-400' : 'text-slate-400'">{{ b.n }} / {{ TARGET }}</span>
              </div>
              <UProgress :value="Math.min(100, (b.n / TARGET) * 100)" :color="b.n >= TARGET ? 'emerald' : 'primary'" size="xs" />
            </div>
            <p class="text-[12px] text-slate-500 leading-snug">
              Saran muncul untuk kata kunci yang ada di ≥{{ stats.min_sample }} situs terverifikasi. Makin banyak situs diverifikasi
              (judol <i>dan</i> bukan judol), makin akurat sarannya.
              <NuxtLink to="/verifikasi" class="text-primary-300 hover:underline">Buka antrean verifikasi →</NuxtLink>
            </p>
            <UButton v-if="pendingSuggestions.length" block color="emerald" variant="soft" icon="i-lucide-wand-sparkles"
              :label="`Terapkan semua saran (${pendingSuggestions.length})`" :loading="applying" @click="applyAll" />
          </div>
        </UCard>
      </div>

      <!-- List -->
      <div class="xl:col-span-8 min-w-0">
        <UCard>
          <template #header>
            <div class="flex flex-col sm:flex-row sm:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">{{ currentType.label }}</h2>
                <p class="section-desc">{{ filteredKeywords.length }} dari {{ keywords.length }} keyword</p>
              </div>
              <USelect v-if="activeType === 'detection' && stats" v-model="sortBy" :options="sortOptions" class="sm:w-52" />
              <UInput v-model="search" icon="i-lucide-search" placeholder="Cari keyword..." class="sm:w-72" />
            </div>
          </template>

          <div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
            <USkeleton v-for="i in 9" :key="i" class="h-16 w-full" />
          </div>

          <div v-else-if="!filteredKeywords.length" class="empty-state">
            <div class="empty-icon"><UIcon name="i-lucide-tags" class="w-6 h-6" /></div>
            <p>{{ search ? 'Tidak ada keyword yang cocok.' : 'Belum ada keyword.' }}</p>
          </div>

          <div v-else class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
            <div v-for="kw in filteredKeywords" :key="kw.id"
              class="group rounded-xl bg-slate-950/60 ring-1 ring-slate-800 hover:ring-slate-700 px-4 py-3.5 flex items-center gap-3 transition">
              <div class="flex-1 min-w-0">
                <p class="text-[15px] text-slate-100 truncate" :title="kw.keyword">{{ kw.keyword }}</p>
                <div v-if="activeType === 'detection'" class="flex items-center gap-2 mt-2">
                  <UProgress :value="Math.round((kw.weight ?? 0) * 100)" :color="weightColor(kw.weight)" size="2xs" class="flex-1" />
                  <span class="text-xs font-semibold tabular-nums text-slate-400 w-7 text-right">{{ (kw.weight ?? 0).toFixed(1) }}</span>
                </div>
                <!-- Ketepatan menurut verifikasi -->
                <div v-if="activeType === 'detection' && statOf(kw)" class="flex flex-wrap items-center gap-x-2 gap-y-1 mt-1.5 text-[12px]">
                  <template v-if="statOf(kw).precision != null">
                    <span :class="precisionColor(statOf(kw).precision)">ketepatan {{ Math.round(statOf(kw).precision * 100) }}%</span>
                    <span class="text-slate-600">· {{ statOf(kw).verified_judol }}/{{ statOf(kw).verified_judol + statOf(kw).verified_bukan }} terverifikasi</span>
                  </template>
                  <span v-else class="text-slate-600">
                    {{ statOf(kw).verified_judol + statOf(kw).verified_bukan }}/{{ stats.min_sample }} terverifikasi · belum cukup data
                  </span>
                  <span class="text-slate-600">· muncul di {{ seenIn(kw) }} situs</span>
                  <UButton v-if="hasSuggestion(kw)" size="2xs" color="emerald" variant="soft" class="ml-auto"
                    :label="`Saran ${statOf(kw).suggested_weight.toFixed(1)} · Terapkan`" @click="applyWeight(kw, statOf(kw).suggested_weight)" />
                </div>
              </div>
              <UTooltip text="Hapus">
                <UButton icon="i-lucide-trash-2" color="rose" variant="ghost" size="sm"
                  class="opacity-60 group-hover:opacity-100" @click="remove(kw.id)" />
              </UTooltip>
            </div>
          </div>
        </UCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, watch } from 'vue'

const { get, post, put, del } = useApi()

const types = [
  { value: 'detection', label: 'Deteksi Heuristik', icon: 'i-lucide-crosshair', desc: 'Kata yang dicari di konten halaman' },
  { value: 'search',    label: 'Query Dorking',     icon: 'i-lucide-search',    desc: 'Query Google untuk menemukan situs baru' },
]
const currentType = computed(() => types.find(t => t.value === activeType.value) ?? types[0])
const toast  = useToast()
const saving = ref(false)
function weightColor(w: number) { return w >= 0.8 ? 'rose' : w >= 0.5 ? 'amber' : 'primary' }

const activeType = ref('detection')
const loading    = ref(false)
const keywords   = ref<any[]>([])
const search     = ref('')
const form       = reactive({ keyword: '', weight: 0.5 })

const filteredKeywords = computed(() => {
  const q = search.value.toLowerCase()
  const list = q ? keywords.value.filter((k: any) => k.keyword.toLowerCase().includes(q)) : [...keywords.value]
  if (activeType.value !== 'detection' || !stats.value) return list
  const prec = (k: any) => statOf(k)?.precision ?? 2 // tanpa data → paling bawah
  if (sortBy.value === 'precision') return list.sort((a, b) => prec(a) - prec(b))
  if (sortBy.value === 'seen') return list.sort((a, b) => seenIn(b) - seenIn(a))
  return list.sort((a, b) => a.keyword.localeCompare(b.keyword))
})

// ── Belajar dari verifikasi ──
const TARGET = 30
const stats = ref<any>(null)
const applying = ref(false)
const sortBy = ref('az')
const sortOptions = [
  { label: 'Urut A–Z', value: 'az' },
  { label: 'Ketepatan terendah', value: 'precision' },
  { label: 'Paling sering muncul', value: 'seen' },
]
const statMap = computed(() => new Map((stats.value?.keywords ?? []).map((k: any) => [k.id, k])))
const statOf = (kw: any): any => statMap.value.get(kw.id)
const seenIn = (kw: any) => { const s = statOf(kw); return s ? s.verified_judol + s.verified_bukan + s.auto_judol + s.auto_bukan : 0 }
const hasSuggestion = (kw: any) => { const s = statOf(kw); return s?.suggested_weight != null && Math.abs(s.suggested_weight - (kw.weight ?? 0)) >= 0.1 }
const pendingSuggestions = computed(() => keywords.value.filter(hasSuggestion))
const precisionColor = (p: number) => (p >= 0.8 ? 'text-emerald-400' : p >= 0.5 ? 'text-amber-400' : 'text-rose-400')
const readiness = computed(() => [
  { label: 'Terverifikasi judol', n: stats.value?.verified_judol ?? 0 },
  { label: 'Terverifikasi bukan judol', n: stats.value?.verified_bukan ?? 0 },
])

async function loadStats() {
  stats.value = activeType.value === 'detection' ? await get<any>('/keywords/detection/stats').catch(() => null) : null
}
async function applyWeight(kw: any, weight: number, quiet = false) {
  try {
    await put(`/keywords/detection/${kw.id}`, { weight })
    kw.weight = weight
    if (!quiet) toast.add({ title: `Bobot "${kw.keyword}" → ${weight.toFixed(1)}`, color: 'emerald', icon: 'i-lucide-check' })
  } catch (e: any) {
    toast.add({ title: 'Gagal mengubah bobot', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  }
}
async function applyAll() {
  const list = pendingSuggestions.value
  if (!confirm(`Terapkan saran bobot untuk ${list.length} kata kunci? Berlaku untuk scan berikutnya.`)) return
  applying.value = true
  try { for (const kw of list) await applyWeight(kw, statOf(kw).suggested_weight, true) }
  finally { applying.value = false }
  toast.add({ title: `${list.length} bobot diperbarui`, description: 'Berlaku untuk scan berikutnya.', color: 'emerald', icon: 'i-lucide-wand-sparkles' })
}

async function load() {
  loading.value = true
  try {
    const res = await get<any>(`/keywords/${activeType.value}`)
    keywords.value = res?.keywords ?? res ?? []
  } catch { keywords.value = [] }
  finally { loading.value = false }
}

async function save() {
  if (!form.keyword.trim()) return
  saving.value = true
  try {
    await post(`/keywords/${activeType.value}`, {
      keyword: form.keyword.trim(), weight: Number(form.weight), type: activeType.value,
    })
    toast.add({ title: 'Keyword ditambahkan', description: form.keyword.trim(), color: 'emerald', icon: 'i-lucide-check' })
  } catch (e: any) {
    toast.add({ title: 'Gagal menambah keyword', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
    return
  } finally { saving.value = false }
  form.keyword = ''
  await load()
}

async function remove(id: number) {
  if (!confirm('Hapus keyword ini?')) return
  try { await del(`/keywords/${activeType.value}/${id}`) }
  catch (e: any) { toast.add({ title: 'Gagal menghapus', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' }) }
  await load()
}

watch(activeType, () => { load(); loadStats() }, { immediate: true })
</script>
