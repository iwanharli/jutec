<template>
  <!-- Bukti situs sah yang disusupi: cloaking & link judol tersembunyi -->
  <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'p-5' } }" class="ring-violet-500/30">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon bg-violet-500/10 ring-violet-500/25"><UIcon name="i-lucide-syringe" class="w-[18px] h-[18px] text-violet-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Situs Disusupi</h2>
          <p class="card-desc">Situs ini tampak sah, tapi menyimpan konten judol yang disembunyikan dari pengunjung biasa</p>
        </div>
        <div class="flex gap-1.5">
          <UBadge v-if="inj.cloaking" color="violet" variant="subtle">Cloaking</UBadge>
          <UBadge v-if="inj.signals?.includes('hidden_links')" color="violet" variant="subtle">Link tersembunyi</UBadge>
        </div>
      </div>
    </template>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <div v-if="inj.cloaking" class="rounded-xl bg-slate-950/40 ring-1 ring-slate-800 p-4">
        <p class="flex items-center gap-2 text-sm font-medium text-slate-100"><UIcon name="i-lucide-venetian-mask" class="w-4 h-4 text-violet-400" />Cloaking</p>
        <p class="text-sm text-slate-400 mt-1.5 leading-relaxed">
          Konten judol hanya disajikan ke
          <b class="text-slate-200">{{ inj.cloak_via === 'googlebot' ? 'Googlebot (mesin pencari)' : 'pengunjung yang datang dari Google' }}</b>.
          Pengunjung yang membuka langsung melihat halaman normal, sehingga pemilik situs sering tidak sadar sudah dibobol.
        </p>
        <p v-if="inj.cloak_redirect" class="text-sm mt-3">
          <span class="text-slate-500">Dialihkan ke:</span>
          <a :href="inj.cloak_redirect" target="_blank" rel="noopener noreferrer" class="mono text-violet-300 hover:underline break-all ml-1">{{ inj.cloak_redirect }}</a>
        </p>
        <div v-if="inj.cloak_keywords?.length" class="mt-3">
          <p class="text-xs uppercase tracking-wider text-slate-500 mb-1.5">Kata kunci di versi tersembunyi</p>
          <div class="flex flex-wrap gap-1">
            <span v-for="k in inj.cloak_keywords" :key="k" class="rounded px-1.5 py-0.5 text-xs text-rose-300 ring-1 ring-rose-500/25">{{ k }}</span>
          </div>
        </div>
      </div>

      <div v-if="inj.hidden_links" class="rounded-xl bg-slate-950/40 ring-1 ring-slate-800 p-4">
        <p class="flex items-center gap-2 text-sm font-medium text-slate-100"><UIcon name="i-lucide-eye-off" class="w-4 h-4 text-violet-400" />Link judol tersembunyi</p>
        <p class="text-sm text-slate-400 mt-1.5">
          <b class="text-slate-200 tabular-nums">{{ inj.hidden_links }}</b> dari {{ inj.judol_links }} link judol ke domain lain disembunyikan
          (di luar layar, ukuran 0, atau tidak ditampilkan) — pola SEO spam penyusup.
        </p>
        <ul class="mt-3 space-y-1 max-h-48 overflow-y-auto">
          <li v-for="s in inj.hidden_samples" :key="s" class="mono text-xs text-slate-400 break-all">{{ s }}</li>
        </ul>
        <div v-if="inj.hidden_hosts?.length" class="flex flex-wrap gap-1 mt-3">
          <UButton v-for="h in inj.hidden_hosts" :key="h" :to="`/discovery?mode=analyze&url=${encodeURIComponent('https://' + h)}`"
            size="2xs" color="gray" variant="soft" icon="i-lucide-scan-line" :label="h" />
        </div>
      </div>
    </div>
    <p class="text-xs text-slate-500 mt-4 flex items-start gap-1.5">
      <UIcon name="i-lucide-info" class="w-3.5 h-3.5 mt-0.5 shrink-0" />
      Tindak lanjut: laporkan ke pengelola situs / CSIRT instansi agar celahnya ditutup, dan analisis domain tujuan link sebagai situs judol.
    </p>
  </UCard>
</template>

<script setup lang="ts">
defineProps<{ inj: any }>()
</script>
