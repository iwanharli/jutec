<template>
  <!-- Peran situs dalam jaringan judol: landing page (pintu masuk) ↔ situs utama (operator) -->
  <UCard v-if="role === 'landing' || role === 'operator'" :ui="{ ring: role === 'operator' ? 'ring-1 ring-rose-500/40' : 'ring-1 ring-sky-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon" :class="role === 'operator' ? 'bg-rose-500/10 ring-rose-500/25' : 'bg-sky-500/10 ring-sky-500/25'">
          <UIcon :name="role === 'operator' ? 'i-lucide-castle' : 'i-lucide-door-open'" class="w-[18px] h-[18px]" :class="role === 'operator' ? 'text-rose-400' : 'text-sky-400'" />
        </div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">{{ role === 'operator' ? 'Situs utama (operator)' : 'Landing page (pintu masuk)' }}</h2>
          <p class="card-desc">{{ role === 'operator'
            ? 'Punya form login/daftar sendiri, lobi game, atau meminta rekening — di sinilah akun pemain & deposit berada.'
            : 'Halaman promosi: tombol DAFTAR/LOGIN mengarah ke domain lain. Target utamanya adalah situs tujuan.' }}</p>
        </div>
        <UBadge v-if="role === 'operator' && links.entries.length" color="rose" variant="subtle">{{ links.entries.length }} pintu masuk</UBadge>
      </div>
    </template>

    <!-- Landing → situs utama -->
    <template v-if="role === 'landing'">
      <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-2">Mengarah ke</p>
      <p v-if="!links.targets.length" class="muted">Tujuan tidak tercatat.</p>
      <ul class="space-y-2">
        <li v-for="t in links.targets" :key="t.host" class="flex flex-wrap items-center gap-2 rounded-lg bg-slate-950/60 ring-1 ring-slate-800 px-3 py-2.5">
          <UIcon name="i-lucide-arrow-right" class="w-4 h-4 text-slate-500" />
          <NuxtLink v-if="t.site" :to="`/site/${t.site.id}`" class="mono text-[15px] text-slate-100 hover:text-primary-300 break-all">{{ t.host }}</NuxtLink>
          <span v-else class="mono text-[15px] text-slate-300 break-all">{{ t.host }}</span>
          <template v-if="t.site">
            <UBadge :color="t.site.is_gambling ? 'rose' : 'gray'" variant="subtle" size="xs">{{ t.site.is_gambling ? `judol ${t.site.priority}` : 'normal' }}</UBadge>
            <UButton :to="`/site/${t.site.id}`" size="2xs" color="primary" variant="soft" class="ml-auto" label="Buka situs utama" trailing-icon="i-lucide-arrow-right" />
          </template>
          <template v-else>
            <span class="text-[12px] text-amber-400">belum dianalisis</span>
            <UButton :to="`/discovery?mode=analyze&url=${encodeURIComponent('https://' + t.host)}`" size="2xs" color="amber" variant="soft" class="ml-auto" icon="i-lucide-scan-line" label="Analisis" />
          </template>
        </li>
      </ul>
    </template>

    <!-- Situs utama ← landing pages -->
    <template v-else>
      <p v-if="!links.entries.length" class="muted">Belum ada landing page tercatat yang mengarah ke sini.</p>
      <template v-else>
        <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-2">Pintu masuk yang mengarah ke sini</p>
        <ul class="grid sm:grid-cols-2 gap-x-4 gap-y-1">
          <li v-for="e in links.entries.slice(0, showAll ? undefined : 10)" :key="e.id" class="flex items-center gap-2 text-sm min-w-0">
            <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="e.is_gambling ? 'bg-rose-500' : 'bg-slate-500'" />
            <NuxtLink :to="`/site/${e.id}`" class="mono truncate text-slate-300 hover:text-primary-300">{{ e.url.replace(/^https?:\/\//, '') }}</NuxtLink>
          </li>
        </ul>
        <UButton v-if="links.entries.length > 10" size="2xs" color="gray" variant="ghost" class="mt-1"
          :label="showAll ? 'Lebih sedikit' : `+${links.entries.length - 10} lainnya`" @click="showAll = !showAll" />
      </template>
    </template>
  </UCard>
</template>

<script setup lang="ts">
defineProps<{ role?: string | null; links: { targets: any[]; entries: any[] } }>()
const showAll = ref(false)
</script>
