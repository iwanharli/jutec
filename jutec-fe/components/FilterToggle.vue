<template>
  <!-- Sakelar on/off berbentuk chip setinggi input, dengan tooltip penjelasan -->
  <UTooltip :text="hint" :prevent="!hint">
    <button type="button" class="h-9 inline-flex items-center gap-2 rounded-lg px-3 text-sm ring-1 whitespace-nowrap transition-colors"
      :class="modelValue ? 'bg-primary-500/10 text-primary-200 ring-primary-500/30' : 'bg-slate-950/40 text-slate-400 ring-slate-800 hover:text-slate-200 hover:ring-slate-700'"
      @click="emit('update:modelValue', !modelValue)">
      <span class="relative w-7 h-4 rounded-full transition-colors" :class="modelValue ? 'bg-primary-500' : 'bg-slate-700'">
        <span class="absolute top-0.5 w-3 h-3 rounded-full bg-white transition-all" :class="modelValue ? 'left-3.5' : 'left-0.5'" />
      </span>
      <UIcon v-if="icon" :name="icon" class="w-4 h-4" />{{ label }}
    </button>
  </UTooltip>
</template>

<script setup lang="ts">
defineProps<{ modelValue: boolean; label: string; hint?: string; icon?: string }>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()
</script>
