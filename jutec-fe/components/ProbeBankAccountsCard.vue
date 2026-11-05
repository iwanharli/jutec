<template>
  <!-- Rekening uji bank bersama — dipakai semua identitas dummy (settings probe_accounts) -->
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-5' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-landmark" class="w-[18px] h-[18px] text-amber-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Rekening Uji Bank</h2>
          <p class="card-desc">Dipakai bersama oleh semua data dummy — engine memilih bank yang tersedia di dropdown form situs</p>
        </div>
        <UBadge color="gray" variant="subtle">{{ clean().length }} rekening</UBadge>
      </div>
    </template>

    <div class="space-y-2">
      <div v-for="(a, i) in rows" :key="i" class="grid grid-cols-12 gap-2 items-center">
        <USelectMenu v-model="a.bank" :options="BANKS" searchable creatable placeholder="Pilih bank" class="col-span-12 sm:col-span-3" />
        <UInput v-model="a.number" placeholder="Nomor rekening" icon="i-lucide-hash" class="col-span-6 sm:col-span-4" :ui="{ base: 'font-mono' }" />
        <UInput v-model="a.name" placeholder="Nama pemilik (opsional)" icon="i-lucide-id-card" class="col-span-5 sm:col-span-4" />
        <UButton color="gray" variant="ghost" icon="i-lucide-trash-2" class="col-span-1 justify-self-end" @click="rows.splice(i, 1)" />
      </div>
    </div>
    <div class="flex flex-wrap items-center gap-3 mt-4">
      <UButton size="sm" color="gray" variant="soft" icon="i-lucide-plus" label="Tambah rekening" @click="rows.push({ bank: '', number: '', name: '' })" />
      <p class="flex-1 min-w-0 text-xs text-slate-500">E-money (DANA, OVO, GoPay, …) tidak perlu diisi — memakai No. HP / WA tiap identitas. Nama pemilik kosong → nama lengkap identitas.</p>
      <UButton :disabled="!dirty" :loading="saving" icon="i-lucide-save" label="Simpan" @click="save" />
    </div>
  </UCard>
</template>

<script setup lang="ts">
const { get, patch } = useApi()
const toast = useToast()
const BANKS = ['BCA', 'BRI', 'BNI', 'MANDIRI', 'BSI', 'CIMB NIAGA', 'DANAMON', 'PERMATA', 'PANIN', 'BTN', 'SEABANK', 'JAGO', 'NEO', 'MAYBANK', 'OCBC']
type Acct = { bank: string; number: string; name: string }
const rows = ref<Acct[]>([])
const saved = ref('[]')
const saving = ref(false)
const clean = () => rows.value.filter(a => a.bank && a.number).map(a => ({ bank: a.bank.trim(), number: a.number.trim(), name: (a.name || '').trim() }))
const dirty = computed(() => JSON.stringify(clean()) !== saved.value)

async function load() {
  try {
    const all = await get<any[]>('/settings')
    const v = (all ?? []).find(s => s.key === 'probe_accounts')?.value || '[]'
    let list: Acct[] = []
    try { list = JSON.parse(v) } catch {}
    saved.value = JSON.stringify(list)
    rows.value = list.length ? list.map(a => ({ ...a })) : [{ bank: '', number: '', name: '' }]
  } catch {}
}
async function save() {
  saving.value = true
  try {
    const v = JSON.stringify(clean())
    await patch('/settings/probe_accounts', { value: v })
    saved.value = v
    toast.add({ title: 'Rekening uji disimpan', color: 'emerald', icon: 'i-lucide-check' })
  } catch { toast.add({ title: 'Gagal menyimpan', color: 'rose', icon: 'i-lucide-circle-alert' }) }
  finally { saving.value = false }
}
onMounted(load)
</script>
