<template>
  <!-- Status pekerjaan otomatis untuk situs ini — analis tak perlu menekan tombol apa pun -->
  <div class="rounded-xl bg-slate-900/50 ring-1 ring-slate-800 px-4 py-3 flex flex-wrap items-center gap-x-4 gap-y-2">
    <span class="inline-flex items-center gap-1.5 text-[12px] font-semibold uppercase tracking-wider text-slate-500">
      <UIcon name="i-lucide-bot" class="w-4 h-4" />Otomatis
    </span>
    <UTooltip v-for="s in steps" :key="s.label" :text="s.tip">
      <span class="inline-flex items-center gap-1.5 text-[13px]" :class="s.state === 'done' ? 'text-slate-200' : s.state === 'na' ? 'text-slate-600' : 'text-slate-400'">
        <UIcon :name="s.state === 'done' ? 'i-lucide-circle-check' : s.state === 'na' ? 'i-lucide-circle-minus' : 'i-lucide-clock'" class="w-4 h-4"
          :class="s.state === 'done' ? 'text-emerald-400' : s.state === 'na' ? 'text-slate-600' : 'text-amber-400'" />
        {{ s.label }}
      </span>
    </UTooltip>
  </div>
</template>

<script setup lang="ts">
type State = 'done' | 'wait' | 'na'
const props = defineProps<{
  site: any; detail: any; evidence: any[]; apks: any[]; tgAccounts: any[]; identities: any[]
  wayback: any[]; revResults: any[]; wallets: any[]
}>()

const steps = computed(() => {
  const s = props.site ?? {}, d = props.detail ?? {}
  const judol = !!s.is_gambling, high = judol && s.priority === 'High'
  const step = (label: string, state: State, tip: string) => ({ label, state, tip })
  const tgPending = props.tgAccounts.filter(a => a.status === 'pending').length
  const contacts = (d.attributes ?? []).filter((a: any) => a.kind === 'artifact' && ['Contact', 'Phone'].includes(a.group)).length
  return [
    step('Reputasi', d.enrichment?.virustotal || d.enrichment?.urlscan ? 'done' : 'wait', 'URLScan & VirusTotal — otomatis setelah setiap scan'),
    step('OCR · QRIS · APK', !judol ? 'na' : props.apks.length || d.image_count ? 'done' : 'wait', 'Analisis lanjutan — otomatis untuk situs judol setelah scan'),
    step('Bukti forensik', !high ? 'na' : props.evidence.some(e => e.status === 'done') ? 'done' : 'wait', 'Diambil otomatis tiap 30 menit untuk situs judol High'),
    step('Telegram', !props.tgAccounts.length ? 'na' : tgPending ? 'wait' : 'done', 'Akun Telegram dicek tiap 2 jam'),
    step('Identitas nomor', !contacts ? 'na' : props.identities.some(i => i.kominfo?.length || i.vfl_checked) ? 'done' : 'wait', 'Nomor HP dicek di data bocoran tiap 1 jam'),
    step('Domain se-ID analytics', !props.revResults.length ? 'na' : props.revResults.every(r => r.fetched_at) ? 'done' : 'wait', 'HackerTarget — tiap jam, menghormati kuota gratis'),
    step('Jejak USDT', !props.wallets.length ? 'na' : props.wallets.every(w => w.profile) ? 'done' : 'wait', 'TronGrid & Etherscan — tiap 3 jam'),
    step(s.tp_listed ? 'Terdaftar TrustPositif' : 'Cek TrustPositif', !judol ? 'na' : s.tp_checked_at ? 'done' : 'wait',
      s.tp_listed ? `Sudah masuk daftar blokir Komdigi${s.tp_listed_since ? ' sejak ' + new Date(s.tp_listed_since).toLocaleDateString('id-ID') : ''} — tidak perlu diajukan lagi`
        : s.tp_checked_at ? 'Belum ada di daftar blokir Komdigi — bisa diajukan' : 'Dicocokkan otomatis tiap 12 jam dengan daftar blokir Komdigi'),
    step('Arsip Wayback', !high ? 'na' : props.wayback.length || s.wayback_checked_at ? 'done' : 'wait', 'Diambil otomatis tiap jam untuk situs judol High'),
  ]
})
</script>
