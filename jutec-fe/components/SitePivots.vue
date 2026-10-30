<template>
  <UCard :ui="{ ring: 'ring-1 ring-amber-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-crosshair" class="w-[18px] h-[18px] text-amber-400" /></div>
        <div class="flex-1 min-w-0">
          <h2 class="card-title">Pivot Terkuat</h2>
          <p class="card-desc">Atribut paling khas yang juga dipakai situs lain — jalur tercepat menuju operator</p>
        </div>
      </div>
    </template>
    <p v-if="!items.length" class="muted">Belum ada atribut khas yang dipakai situs lain. Atribut yang dipakai &gt;40 situs (CDN, nameserver publik) tidak dihitung.</p>
    <ol v-else class="divide-y divide-slate-800/60 -my-2">
      <li v-for="(a, i) in items" :key="a.kind + a.id" class="py-2.5 flex items-start gap-3">
        <span class="w-5 text-[13px] tabular-nums text-slate-600 pt-0.5">{{ i + 1 }}</span>
        <div class="flex-1 min-w-0">
          <p class="mono text-[15px] text-slate-100 break-words [overflow-wrap:anywhere] leading-snug">{{ a.value }}</p>
          <p class="text-[13px] text-slate-500 mt-0.5">{{ a.kindLabel }}<span v-if="a.label && a.kind !== 'link'"> · {{ a.label }}</span> · {{ a.why }}</p>
        </div>
        <div class="flex flex-col items-end gap-1 shrink-0">
          <UBadge :color="a.level === 'kuat' ? 'rose' : a.level === 'sedang' ? 'amber' : 'gray'" variant="subtle" size="xs">{{ a.level }}</UBadge>
          <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-scan-search" label="Situs lain" @click="$emit('lookup', a)" />
        </div>
      </li>
    </ol>
  </UCard>
</template>

<script setup lang="ts">
defineProps<{ items: any[] }>()
defineEmits<{ (e: 'lookup', a: any): void }>()
</script>
