<template>
  <div class="inline-flex items-center gap-1.5">
    <UTooltip :text="tip">
      <UBadge :color="badge.color" variant="subtle" class="cursor-pointer" @click="open = true">
        <UIcon :name="badge.icon" class="w-3.5 h-3.5 mr-1" />{{ badge.label }}
      </UBadge>
    </UTooltip>

    <UModal v-model="open" :ui="{ width: 'sm:max-w-lg' }">
      <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' }, footer: { padding: 'px-5 py-4' } }">
        <template #header>
          <div class="flex items-center gap-3">
            <div class="card-icon bg-emerald-500/10 ring-emerald-500/25"><UIcon name="i-lucide-badge-check" class="w-[18px] h-[18px] text-emerald-400" /></div>
            <div class="flex-1 min-w-0">
              <h2 class="card-title">Verifikasi status situs</h2>
              <p class="card-desc truncate">{{ host }}</p>
            </div>
          </div>
        </template>

        <div class="space-y-4">
          <p class="text-[13px] text-slate-400">
            Keputusan penyidik mengunci status situs — scan ulang & proses otomatis tidak akan mengubahnya. Tercatat atas nama
            <b class="text-slate-200">{{ operator || '—' }}</b> di riwayat & log audit.
          </p>
          <div class="grid grid-cols-2 gap-2">
            <button v-for="o in options" :key="o.value" type="button" @click="choice = o.value"
              class="rounded-lg ring-1 p-3 text-left transition"
              :class="choice === o.value ? o.on : 'ring-slate-800 hover:ring-slate-700'">
              <p class="text-sm font-semibold flex items-center gap-1.5" :class="choice === o.value ? 'text-white' : 'text-slate-300'">
                <UIcon :name="o.icon" class="w-4 h-4" />{{ o.label }}
              </p>
              <p class="text-xs text-slate-500 mt-1">{{ o.hint }}</p>
            </button>
          </div>
          <UFormGroup label="Catatan / alasan" :hint="choice === 'bukan_judol' ? 'disarankan' : 'opsional'">
            <UTextarea v-model="note" :rows="2" placeholder="mis. Portal berita resmi; nomor WA berasal dari kutipan artikel" />
          </UFormGroup>
          <UCheckbox v-if="choice === 'bukan_judol'" v-model="blacklist"
            label="Masukkan domain ke blacklist (tidak diambil lagi oleh discovery)"
            help="Dilewati otomatis untuk domain .go.id / .gov / .ac / .edu — subdomainnya sering dibajak judol." />

          <div v-if="history.length" class="pt-3 border-t border-slate-800">
            <p class="text-xs font-semibold uppercase tracking-wider text-slate-500 mb-1.5">Riwayat keputusan</p>
            <ul class="space-y-1 text-[13px]">
              <li v-for="(h, i) in history" :key="i" class="flex gap-2">
                <span :class="h.status === 'judol' ? 'text-rose-300' : h.status === 'bukan_judol' ? 'text-emerald-300' : 'text-slate-400'">{{ statusLabel[h.status] }}</span>
                <span class="text-slate-500 flex-1 min-w-0 truncate">{{ h.actor }}{{ h.note ? ` — ${h.note}` : '' }}</span>
                <span class="text-slate-600 shrink-0">{{ new Date(h.at).toLocaleDateString('id-ID') }}</span>
              </li>
            </ul>
          </div>
        </div>

        <template #footer>
          <div class="flex flex-wrap gap-2 justify-end">
            <UButton v-if="site.verified_status" color="gray" variant="ghost" label="Kembalikan ke penilaian otomatis" :loading="busy" @click="save('reset')" />
            <UButton color="gray" variant="soft" label="Batal" @click="open = false" />
            <UButton :disabled="!choice" :loading="busy" icon="i-lucide-check" label="Simpan keputusan" @click="save(choice)" />
          </div>
        </template>
      </UCard>
    </UModal>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ site: any }>()
const emit = defineEmits<{ updated: [] }>()
const { get, post } = useApi()
const toast = useToast()
const { name: operator } = useOperator()

const open = ref(false)
const choice = ref<string>('')
const note = ref('')
const blacklist = ref(false)
const busy = ref(false)
const history = ref<any[]>([])

const host = computed(() => (props.site?.url ?? '').replace(/^https?:\/\//, '').replace(/\/$/, ''))
const statusLabel: Record<string, string> = { judol: 'Judol', bukan_judol: 'Bukan judol', reset: 'Dikembalikan ke otomatis' }
const options = [
  { value: 'judol', label: 'Judol', icon: 'i-lucide-shield-alert', hint: 'Bukti cukup: situs/halaman judi online', on: 'ring-rose-500/50 bg-rose-500/10' },
  { value: 'bukan_judol', label: 'Bukan judol', icon: 'i-lucide-shield-check', hint: 'Salah deteksi engine', on: 'ring-emerald-500/50 bg-emerald-500/10' },
]
const badge = computed(() => {
  const v = props.site?.verified_status
  if (v === 'judol') return { label: 'Terverifikasi judol', color: 'rose' as const, icon: 'i-lucide-badge-check' }
  if (v === 'bukan_judol') return { label: 'Terverifikasi bukan judol', color: 'emerald' as const, icon: 'i-lucide-badge-check' }
  return { label: 'Belum diverifikasi', color: 'gray' as const, icon: 'i-lucide-badge-help' }
})
const tip = computed(() => props.site?.verified_status
  ? `Oleh ${props.site.verified_by} · ${new Date(props.site.verified_at).toLocaleString('id-ID')}${props.site.verified_note ? ` — ${props.site.verified_note}` : ''}`
  : 'Status masih dari penilaian otomatis engine — klik untuk memverifikasi')

watch(open, async (v) => {
  if (!v) return
  choice.value = props.site?.verified_status ?? ''
  note.value = ''
  blacklist.value = false
  history.value = (await get<any>(`/sites/${props.site.id}/verifications`).catch(() => null))?.history ?? []
})

async function save(status: string) {
  if (!operator.value) {
    toast.add({ title: 'Isi nama dulu', description: 'Klik tombol nama di kanan atas.', color: 'amber', icon: 'i-lucide-user-round' })
    return
  }
  busy.value = true
  try {
    const r = await post<any>(`/sites/${props.site.id}/verify`, { status, note: note.value, blacklist: blacklist.value })
    toast.add({ title: status === 'reset' ? 'Dikembalikan ke penilaian otomatis' : `Ditandai ${statusLabel[status].toLowerCase()}`,
      description: typeof r.blacklist === 'string' ? `Blacklist: ${r.blacklist}` : undefined, color: 'emerald', icon: 'i-lucide-check' })
    open.value = false
    emit('updated')
  } catch (e: any) {
    toast.add({ title: 'Gagal menyimpan', description: e.message, color: 'rose' })
  } finally { busy.value = false }
}
</script>
