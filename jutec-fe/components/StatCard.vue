<template>
  <div class="relative overflow-hidden rounded-2xl bg-slate-900/60 ring-1 ring-slate-800 p-6 flex items-center gap-5 transition hover:ring-slate-700">
    <div class="absolute -right-6 -top-6 w-28 h-28 rounded-full blur-2xl opacity-40" :class="c.glow" />
    <div class="w-12 h-12 rounded-xl flex items-center justify-center shrink-0 ring-1" :class="c.box">
      <UIcon :name="icon" class="w-6 h-6" :class="c.text" />
    </div>
    <div class="min-w-0 relative">
      <p class="text-sm text-slate-400 font-medium truncate">{{ label }}</p>
      <p class="text-3xl font-semibold text-white tabular-nums tracking-tight mt-1">{{ formatted }}</p>
      <p v-if="delta != null" class="text-xs mt-1 flex items-center gap-1 truncate">
        <UIcon :name="delta > 0 ? 'i-lucide-trending-up' : delta < 0 ? 'i-lucide-trending-down' : 'i-lucide-minus'" class="w-3.5 h-3.5 shrink-0" :class="deltaTone" />
        <span :class="deltaTone">{{ delta > 0 ? '+' : '' }}{{ delta.toLocaleString('id-ID') }}</span>
        <span class="text-slate-500">{{ deltaLabel ?? 'vs 7 hari sebelumnya' }}</span>
      </p>
      <p v-if="hint" class="text-xs text-slate-500 mt-1 truncate">{{ hint }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  label: string; value: string | number; hint?: string
  color: 'indigo'|'emerald'|'rose'|'amber'|'blue'|'purple'|'slate'|'cyan'
  icon: string
  delta?: number | null      // perubahan dibanding periode sebelumnya
  deltaLabel?: string
  upIsBad?: boolean          // naik = buruk (mis. situs judol baru), turun = baik
}>()
const deltaTone = computed(() => !props.delta ? 'text-slate-400'
  : (props.delta > 0) === !!props.upIsBad ? 'text-rose-400' : 'text-emerald-400')
const formatted = computed(() => typeof props.value === 'number' ? props.value.toLocaleString('id-ID') : props.value)
const map = {
  indigo:  { box: 'bg-indigo-500/10 ring-indigo-500/25',   text: 'text-indigo-400',  glow: 'bg-indigo-500' },
  emerald: { box: 'bg-emerald-500/10 ring-emerald-500/25', text: 'text-emerald-400', glow: 'bg-emerald-500' },
  rose:    { box: 'bg-rose-500/10 ring-rose-500/25',       text: 'text-rose-400',    glow: 'bg-rose-500' },
  amber:   { box: 'bg-amber-500/10 ring-amber-500/25',     text: 'text-amber-400',   glow: 'bg-amber-500' },
  blue:    { box: 'bg-blue-500/10 ring-blue-500/25',       text: 'text-blue-400',    glow: 'bg-blue-500' },
  purple:  { box: 'bg-purple-500/10 ring-purple-500/25',   text: 'text-purple-400',  glow: 'bg-purple-500' },
  slate:   { box: 'bg-slate-700/40 ring-slate-600/40',     text: 'text-slate-300',   glow: 'bg-slate-500' },
  cyan:    { box: 'bg-cyan-500/10 ring-cyan-500/25',       text: 'text-cyan-400',    glow: 'bg-cyan-500' },
}
const c = computed(() => map[props.color])
</script>
