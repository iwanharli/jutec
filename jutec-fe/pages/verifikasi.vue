<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Kualitas Data" title="Antrean Verifikasi" icon="i-lucide-badge-check"
      subtitle="Situs bertanda judol yang belum diperiksa manusia. Keputusan penyidik mengunci status situs dan tercatat di log audit." />

    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <div v-for="k in kpis" :key="k.label" class="rounded-xl bg-slate-900/60 ring-1 ring-slate-800 px-4 py-3">
        <p class="text-xs uppercase tracking-wider text-slate-500">{{ k.label }}</p>
        <p class="text-2xl font-semibold tabular-nums mt-0.5" :class="k.color">{{ (k.value ?? 0).toLocaleString('id-ID') }}</p>
      </div>
    </div>

    <UAlert v-if="!operator" icon="i-lucide-user-round" color="amber" variant="subtle"
      title="Isi nama penyidik dulu" description="Klik tombol nama di kanan atas — setiap keputusan tercatat atas nama seseorang." />

    <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
      <template #header>
        <div class="flex flex-wrap items-center gap-3">
          <div class="card-icon bg-primary-500/10 ring-primary-500/25"><UIcon name="i-lucide-list-checks" class="w-[18px] h-[18px] text-primary-400" /></div>
          <div class="flex-1 min-w-0">
            <h2 class="card-title">Belum diverifikasi</h2>
            <p class="card-desc">{{ total.toLocaleString('id-ID') }} situs · bukti terlemah ditampilkan duluan karena paling mungkin salah deteksi</p>
          </div>
          <USelect v-model="priority" :options="[{ label: 'Semua prioritas', value: '' }, { label: 'High', value: 'High' }, { label: 'Medium', value: 'Medium' }, { label: 'Low', value: 'Low' }]" class="w-40" />
          <USelect v-model="sort" :options="[{ label: 'Bukti terlemah dulu', value: 'weak' }, { label: 'Confidence tertinggi', value: 'confidence' }, { label: 'Scan terbaru', value: 'recent' }]" class="w-48" />
        </div>
      </template>

      <div v-if="loading && !rows.length" class="space-y-2"><USkeleton v-for="i in 6" :key="i" class="h-12 w-full" /></div>
      <p v-else-if="!rows.length" class="text-sm text-slate-400 py-6 text-center">Antrean kosong — semua situs pada filter ini sudah diverifikasi.</p>
      <ul v-else class="divide-y divide-slate-800/70 -my-2">
        <li v-for="r in rows" :key="r.id" class="py-2.5 flex flex-wrap items-center gap-x-4 gap-y-2">
          <div class="flex-1 min-w-[260px]">
            <NuxtLink :to="`/site/${r.id}`" target="_blank" class="text-[15px] font-medium text-slate-100 hover:text-primary-300 break-all">
              {{ r.url.replace(/^https?:\/\//, '') }}
            </NuxtLink>
            <div class="flex flex-wrap items-center gap-1.5 mt-1 text-xs">
              <PriorityBadge :priority="r.priority" />
              <span class="text-slate-400 tabular-nums">{{ Math.round(r.confidence * 100) }}%</span>
              <span class="ev" :class="r.money ? 'ev-on' : ''"><UIcon name="i-lucide-landmark" class="w-3 h-3" />{{ r.money }} rekening</span>
              <span class="ev" :class="r.artifacts ? 'ev-on' : ''"><UIcon name="i-lucide-wallet" class="w-3 h-3" />{{ r.artifacts }} artefak</span>
              <span class="ev" :class="r.providers ? 'ev-on' : ''"><UIcon name="i-lucide-dice-5" class="w-3 h-3" />{{ r.providers }} provider</span>
              <span class="ev" :class="r.clustered ? 'ev-on' : ''"><UIcon name="i-lucide-network" class="w-3 h-3" />{{ r.clustered ? 'dalam klaster' : 'tanpa klaster' }}</span>
              <span v-if="r.status !== 'active'" class="ev text-amber-300">{{ r.status === 'unreachable' ? 'tak terjangkau' : 'offline' }}</span>
            </div>
          </div>
          <div class="flex gap-1.5">
            <UButton size="sm" color="rose" variant="soft" icon="i-lucide-shield-alert" label="Judol" :loading="busy === r.id + 'j'" @click="decide(r, 'judol')" />
            <UButton size="sm" color="emerald" variant="soft" icon="i-lucide-shield-check" label="Bukan" :loading="busy === r.id + 'b'" @click="decide(r, 'bukan_judol')" />
            <UTooltip text="Buka detail di tab baru"><UButton size="sm" color="gray" variant="ghost" icon="i-lucide-external-link" :to="`/site/${r.id}`" target="_blank" /></UTooltip>
          </div>
        </li>
      </ul>

      <div v-if="total > limit" class="flex justify-end pt-4 mt-4 border-t border-slate-800">
        <UPagination v-model="page" :page-count="limit" :total="total" :max="7" />
      </div>
    </UCard>
  </div>
</template>

<script setup lang="ts">
const { get, post } = useApi()
const toast = useToast()
const { name: operator } = useOperator()

const rows = ref<any[]>([])
const total = ref(0)
const stats = ref<any>({})
const loading = ref(true)
const busy = ref('')
const priority = ref('High')
const sort = ref('weak')
const page = ref(1)
const limit = 25

const kpis = computed(() => [
  { label: 'Belum diverifikasi', value: stats.value.pending, color: 'text-white' },
  { label: 'Belum diverifikasi · High', value: stats.value.pending_high, color: 'text-amber-300' },
  { label: 'Terverifikasi judol', value: stats.value.verified_judol, color: 'text-rose-300' },
  { label: 'Terverifikasi bukan judol', value: stats.value.verified_not, color: 'text-emerald-300' },
])

async function load() {
  loading.value = true
  try {
    const d = await get<any>('/verification/queue', { priority: priority.value || undefined, sort: sort.value, limit, offset: (page.value - 1) * limit })
    rows.value = d?.results ?? []
    total.value = d?.total ?? 0
    stats.value = d?.stats ?? {}
  } catch {} finally { loading.value = false }
}

async function decide(r: any, status: string) {
  if (!operator.value) {
    toast.add({ title: 'Isi nama dulu', description: 'Klik tombol nama di kanan atas.', color: 'amber', icon: 'i-lucide-user-round' })
    return
  }
  busy.value = r.id + (status === 'judol' ? 'j' : 'b')
  try {
    await post(`/sites/${r.id}/verify`, { status, note: '' })
    rows.value = rows.value.filter(x => x.id !== r.id)
    total.value--
    stats.value.pending--
    if (r.priority === 'High') stats.value.pending_high--
    status === 'judol' ? stats.value.verified_judol++ : stats.value.verified_not++
    toast.add({
      title: `${r.url.replace(/^https?:\/\//, '')} → ${status === 'judol' ? 'judol' : 'bukan judol'}`,
      icon: 'i-lucide-check', color: status === 'judol' ? 'rose' : 'emerald', timeout: 6000,
      actions: [{ label: 'Batalkan', click: async () => { await post(`/sites/${r.id}/verify`, { status: 'reset' }).catch(() => {}); load() } }],
    })
    if (rows.value.length < 5 && total.value > rows.value.length) load()
  } catch (e: any) {
    toast.add({ title: 'Gagal menyimpan', description: e.message, color: 'rose' })
  } finally { busy.value = '' }
}

watch([priority, sort], () => { page.value === 1 ? load() : (page.value = 1) })
watch(page, load)
onMounted(load)
useHead({ title: 'Antrean Verifikasi — Jutec' })
</script>

<style scoped>
.ev { @apply inline-flex items-center gap-1 rounded px-1.5 py-0.5 bg-slate-800/60 text-slate-500; }
.ev-on { @apply text-slate-200 bg-slate-800; }
</style>
