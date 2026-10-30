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

const { get, post, del } = useApi()

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
  if (!search.value) return keywords.value
  const q = search.value.toLowerCase()
  return keywords.value.filter((k: any) => k.keyword.toLowerCase().includes(q))
})

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

watch(activeType, load, { immediate: true })
</script>
