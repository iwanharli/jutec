<template>
  <!-- Identitas dummy tersimpan untuk Uji Form Terkendali -->
  <div class="space-y-5">
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: '' } }">
    <template #header>
      <div class="flex flex-wrap items-center gap-3">
        <div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-contact" class="w-[18px] h-[18px] text-rose-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Data Dummy Tersimpan</h2>
          <p class="card-desc">Identitas untuk didaftarkan saat “Uji daftar & login”. Rekening bank memakai daftar bersama di bawah.</p>
        </div>
        <UButton icon="i-lucide-plus" label="Tambah identitas" @click="edit()" />
      </div>
    </template>

    <div v-if="loading" class="p-5 space-y-2"><USkeleton v-for="i in 3" :key="i" class="h-20 w-full" /></div>
    <EmptyState v-else-if="!rows.length" icon="i-lucide-contact" title="Belum ada data dummy"
      desc="Tambahkan identitas (username, password, HP, rekening uji) untuk menjalankan uji daftar & login." />
    <UTable v-else :rows="rows" :columns="columns"
      :ui="{ wrapper: 'relative overflow-x-auto', td: { padding: 'px-4 py-3' }, th: { padding: 'px-4 py-3' } }">
      <template #label-data="{ row }">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-slate-100">{{ row.label }}</span>
          <UBadge v-if="row.is_default" color="rose" variant="subtle" size="xs">Bawaan</UBadge>
        </div>
      </template>
      <template #username_prefix-data="{ row }">
        <span class="mono text-sm text-slate-200">{{ row.username_prefix }}<span class="text-slate-500">######</span></span>
      </template>
      <template #fullname-data="{ row }"><span class="text-sm text-slate-300">{{ row.fullname || '—' }}</span></template>
      <template #email-data="{ row }"><span class="text-sm text-slate-300">{{ row.email || '—' }}</span></template>
      <template #phone-data="{ row }"><span class="mono text-sm text-slate-300">{{ row.phone || '—' }}</span></template>
      <template #uses-data="{ row }">
        <div class="text-sm tabular-nums"><span class="text-slate-300">{{ row.uses }}</span> uji · <span class="text-emerald-300">{{ row.accepted }}</span> diterima
          <div v-if="row.last_used" class="text-[11px] text-slate-500">terakhir {{ day(row.last_used) }}</div>
        </div>
      </template>
      <template #actions-data="{ row }">
        <div class="flex items-center justify-end gap-1">
          <UTooltip text="Ubah"><UButton size="xs" color="gray" variant="ghost" icon="i-lucide-pencil" @click="edit(row)" /></UTooltip>
          <UTooltip v-if="!row.is_default" text="Jadikan bawaan"><UButton size="xs" color="gray" variant="ghost" icon="i-lucide-star" @click="makeDefault(row)" /></UTooltip>
          <UTooltip text="Hapus"><UButton size="xs" color="gray" variant="ghost" icon="i-lucide-trash-2" @click="remove(row)" /></UTooltip>
        </div>
      </template>
    </UTable>
  </UCard>

  <ProbeBankAccountsCard />
  </div>

  <!-- Form tambah/ubah -->
  <UModal v-model="open" :ui="{ width: 'sm:max-w-3xl' }">
    <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-5' }, footer: { padding: 'px-5 py-3' } }">
      <template #header>
        <div class="flex items-center gap-3">
          <UIcon name="i-lucide-contact" class="w-5 h-5 text-rose-400" />
          <h3 class="card-title flex-1">{{ form.id ? 'Ubah identitas' : 'Tambah identitas' }}</h3>
          <UButton color="gray" variant="ghost" icon="i-lucide-x" @click="open = false" />
        </div>
      </template>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <UFormGroup label="Nama identitas" help="Penanda di daftar, mis. “SIM Unit 1”" class="md:col-span-2">
          <UInput v-model="form.label" placeholder="Identitas 1" icon="i-lucide-tag" />
        </UFormGroup>
        <UFormGroup label="Awalan username" hint="wajib" help="Username tiap uji: awalan + 6 angka acak">
          <UInput v-model="form.username_prefix" placeholder="jt" icon="i-lucide-at-sign" :ui="{ base: 'font-mono' }" />
        </UFormGroup>
        <UFormGroup label="Password" hint="wajib">
          <UInput v-model="form.password" :type="reveal ? 'text' : 'password'" icon="i-lucide-key-round" autocomplete="off">
            <template #trailing><UButton color="gray" variant="link" :padded="false" :icon="reveal ? 'i-lucide-eye-off' : 'i-lucide-eye'" @click="reveal = !reveal" /></template>
          </UInput>
        </UFormGroup>
        <UFormGroup label="Nama lengkap"><UInput v-model="form.fullname" placeholder="Nama dummy" icon="i-lucide-user" /></UFormGroup>
        <UFormGroup label="Email"><UInput v-model="form.email" placeholder="uji@contoh.id" icon="i-lucide-mail" /></UFormGroup>
        <UFormGroup label="No. HP / WA" help="Juga dipakai sebagai nomor akun e-money (DANA, OVO, GoPay, …)" class="md:col-span-2">
          <UInput v-model="form.phone" placeholder="08…" icon="i-lucide-smartphone" :ui="{ base: 'font-mono' }" />
        </UFormGroup>
      </div>

      <template #footer>
        <div class="flex items-center gap-3">
          <p class="flex-1 text-xs text-slate-500 flex items-start gap-1.5">
            <UIcon name="i-lucide-shield-alert" class="w-3.5 h-3.5 mt-0.5 shrink-0 text-amber-400" />Data ini terlihat oleh pengelola situs judol — pakai SIM & rekening khusus investigasi.
          </p>
          <UButton color="gray" variant="ghost" label="Batal" @click="open = false" />
          <UButton icon="i-lucide-save" :loading="saving" :disabled="!form.username_prefix || !form.password" label="Simpan" @click="save" />
        </div>
      </template>
    </UCard>
  </UModal>
</template>

<script setup lang="ts">
const { get, post, put, del } = useApi()
const { confirm } = useConfirm()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)
const open = ref(false)
const saving = ref(false)
const reveal = ref(false)
const columns = [
  { key: 'label', label: 'Identitas' },
  { key: 'username_prefix', label: 'Username' },
  { key: 'fullname', label: 'Nama' },
  { key: 'email', label: 'Email' },
  { key: 'phone', label: 'HP / e-money' },
  { key: 'uses', label: 'Pemakaian' },
  { key: 'actions', label: '', class: 'w-28' },
]
const blank = () => ({ id: 0, label: '', username_prefix: '', password: '', fullname: '', email: '', phone: '' })
const form = reactive(blank())

const day = (d: string) => new Date(d).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })

async function load() {
  loading.value = true
  try { rows.value = (await get<any>('/probe-profiles')).results ?? [] } catch { rows.value = [] } finally { loading.value = false }
}
function edit(p?: any) {
  const { accounts, ...rest } = p ?? {}
  Object.assign(form, blank(), p ? rest : {})
  reveal.value = false
  open.value = true
}
async function save() {
  saving.value = true
  const body = { ...form }
  try {
    if (form.id) await put(`/probe-profiles/${form.id}`, body)
    else await post('/probe-profiles', body)
    open.value = false
    toast.add({ title: 'Identitas disimpan', color: 'emerald', icon: 'i-lucide-check' })
    await load()
  } catch (e: any) {
    toast.add({ title: e?.data?.error || 'Gagal menyimpan', color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { saving.value = false }
}
async function makeDefault(p: any) {
  await post(`/probe-profiles/${p.id}/default`, {})
  await load()
}
async function remove(p: any) {
  const ok = await confirm({ title: `Hapus identitas “${p.label}”?`, tone: 'danger', confirmLabel: 'Hapus',
    description: 'Riwayat uji & akun terdaftar yang memakai identitas ini tetap tersimpan.' })
  if (!ok) return
  await del(`/probe-profiles/${p.id}`)
  await load()
}
onMounted(load)
</script>
