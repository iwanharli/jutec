<template>
  <!-- Pilihan berjejer (satu aktif) — tinggi sama dengan UInput/USelect agar sejajar di bar filter -->
  <div class="inline-flex items-center gap-0.5 h-9 p-0.5 rounded-lg bg-slate-950/60 ring-1 ring-slate-800 max-w-full overflow-x-auto">
    <UTooltip v-for="o in options" :key="String(o.value)" :text="o.hint" :prevent="!o.hint">
      <button type="button" class="h-8 inline-flex items-center gap-1.5 rounded-md px-3 text-sm whitespace-nowrap transition-colors"
        :class="modelValue === o.value ? activeCls(o) : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/70'"
        @click="emit('update:modelValue', clearable && modelValue === o.value ? '' : o.value)">
        <span v-if="o.dot" class="w-1.5 h-1.5 rounded-full" :class="o.dot" />
        {{ o.label }}
        <span v-if="o.n != null" class="tabular-nums text-xs opacity-70">{{ o.n.toLocaleString('id-ID') }}</span>
      </button>
    </UTooltip>
  </div>
</template>

<script setup lang="ts">
type Opt = { value: any; label: string; n?: number | null; hint?: string; dot?: string; color?: string }
defineProps<{ modelValue: any; options: Opt[]; clearable?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [any] }>()
const COLORS: Record<string, string> = {
  rose: 'bg-rose-500/15 text-rose-200 ring-1 ring-rose-500/30',
  amber: 'bg-amber-500/15 text-amber-200 ring-1 ring-amber-500/30',
  emerald: 'bg-emerald-500/15 text-emerald-200 ring-1 ring-emerald-500/30',
  sky: 'bg-sky-500/15 text-sky-200 ring-1 ring-sky-500/30',
}
const activeCls = (o: Opt) => `font-medium ${COLORS[o.color ?? ''] ?? 'bg-primary-500/15 text-primary-200 ring-1 ring-primary-500/30'}`
</script>
