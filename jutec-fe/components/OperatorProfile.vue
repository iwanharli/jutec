<template>
  <div class="space-y-4">
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
      <div v-for="s in stats" :key="s.label" class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 px-3 py-2">
        <p class="text-[12px] uppercase tracking-wider text-slate-500">{{ s.label }}</p>
        <p class="text-lg font-semibold tabular-nums" :class="s.color">{{ s.value }}</p>
        <p v-if="s.hint" class="text-[12px] text-slate-500 leading-tight">{{ s.hint }}</p>
      </div>
    </div>

    <!-- Komposisi status anggota -->
    <div v-if="p.total">
      <div class="flex h-2 rounded-full overflow-hidden bg-slate-800">
        <div v-for="b in bars" :key="b.label" :class="b.color" :style="{ width: `${(b.n / p.total) * 100}%` }" />
      </div>
      <div class="flex flex-wrap gap-x-3 gap-y-1 mt-1.5 text-[12px] text-slate-400">
        <span v-for="b in bars" :key="b.label" class="inline-flex items-center gap-1"><span class="w-2 h-2 rounded-full" :class="b.color" />{{ b.label }} {{ b.n }}</span>
      </div>
    </div>

    <div v-if="p.sharedArtifacts.length">
      <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-2">Rekening & kontak dipakai bergantian</p>
      <ul class="space-y-1.5">
        <li v-for="a in p.sharedArtifacts.slice(0, 8)" :key="a.type + a.value" class="flex items-start gap-2 text-sm">
          <UBadge color="gray" variant="subtle" size="xs" class="shrink-0 tabular-nums mt-0.5">{{ a.sites }} situs</UBadge>
          <span class="text-slate-500 shrink-0">{{ a.name || a.type }}</span>
          <span class="mono text-slate-200 break-words [overflow-wrap:anywhere]">{{ a.value }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ op: any }>()
const p = computed(() => operatorProfile(props.op))
const day = (t: number | null) => t ? new Date(t).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—'
const stats = computed(() => [
  { label: 'Situs', value: p.value.total, hint: `${p.value.judol} judol`, color: 'text-white' },
  { label: 'Masih hidup', value: p.value.online + p.value.blockedAlive, hint: p.value.blockedAlive ? `${p.value.blockedAlive} lolos blokir` : 'online', color: 'text-rose-400' },
  { label: 'Respawn', value: p.value.respawnDays == null ? '—' : `${p.value.respawnDays < 1 ? '<1' : Math.round(p.value.respawnDays)} hari`,
    hint: p.value.respawnSamples ? `median dari ${p.value.respawnSamples} blokir` : 'blokir → situs baru', color: 'text-amber-400' },
  { label: 'Aktif', value: day(p.value.firstSeen), hint: `s.d. ${day(p.value.lastSeen)} · ${p.value.recent} baru (30 hr)`, color: 'text-slate-200 text-sm' },
])
const bars = computed(() => [
  { label: 'Online', n: p.value.online, color: 'bg-emerald-500' },
  { label: 'Diblokir · hidup', n: p.value.blockedAlive, color: 'bg-orange-500' },
  { label: 'Diblokir', n: p.value.blocked - p.value.blockedAlive, color: 'bg-slate-500' },
  { label: 'Mati', n: p.value.dead, color: 'bg-slate-700' },
  { label: 'Belum dicek', n: p.value.unchecked, color: 'bg-slate-800 ring-1 ring-inset ring-slate-600' },
].filter(b => b.n))
</script>
