<template>
  <!-- Domain bekas situs sah yang kini dipakai judol (riwayat Wayback Machine) -->
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-5' } }">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-history" class="w-[18px] h-[18px] text-amber-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Domain Bekas Situs Sah</h2>
          <p class="card-desc">Domain ini dulu dipakai situs lain, lalu diambil alih untuk judol — biasanya domain kedaluwarsa yang didaftarkan ulang operator untuk menumpang reputasinya di Google</p>
        </div>
      </div>
    </template>
    <ol class="relative border-l border-slate-700 ml-2 space-y-5">
      <li class="pl-5">
        <span class="absolute -left-1.5 w-3 h-3 rounded-full bg-emerald-500 ring-4 ring-slate-900" />
        <p class="text-xs text-slate-500">{{ ym(t.first_seen) }} – {{ ym(t.legit_until) }} · situs sah</p>
        <p class="text-sm text-slate-100 mt-0.5">{{ t.legit_title || 'Judul tidak terbaca' }}</p>
        <a v-if="t.legit_url" :href="t.legit_url" target="_blank" rel="noopener noreferrer" class="text-xs text-primary-400 hover:underline">Lihat arsip ↗</a>
      </li>
      <li class="pl-5">
        <span class="absolute -left-1.5 w-3 h-3 rounded-full bg-rose-500 ring-4 ring-slate-900" />
        <p class="text-xs text-slate-500">sejak {{ ym(t.judol_since) }} · dipakai judol</p>
        <p class="text-sm text-rose-200 mt-0.5 mono">{{ t.domain }}</p>
        <a v-if="t.judol_url" :href="t.judol_url" target="_blank" rel="noopener noreferrer" class="text-xs text-primary-400 hover:underline">Lihat arsip ↗</a>
      </li>
    </ol>
    <p class="text-xs text-slate-500 mt-4 flex items-start gap-1.5">
      <UIcon name="i-lucide-info" class="w-3.5 h-3.5 mt-0.5 shrink-0" />
      Pemilik sah lama bukan pelaku. Data pendaftar baru dapat diminta ke registrar; tanggal pengambilalihan membantu mencocokkan waktu pendaftaran ulang.
    </p>
  </UCard>
</template>

<script setup lang="ts">
defineProps<{ t: any }>()
const ym = (ts?: string) => ts ? new Date(`${ts.slice(0, 4)}-${ts.slice(4, 6)}-01`).toLocaleDateString('id-ID', { month: 'short', year: 'numeric' }) : '—'
</script>
