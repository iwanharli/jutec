<template>
  <div class="flex items-start gap-2.5 rounded-lg bg-primary-500/5 ring-1 ring-primary-500/20 px-3 py-2.5 text-[13px]">
    <UIcon name="i-lucide-loader-circle" class="w-4 h-4 text-primary-400 animate-spin shrink-0 mt-0.5" />
    <div class="min-w-0">
      <p class="text-slate-200 font-medium">{{ text }}</p>
      <p v-if="sub" class="text-slate-500 mt-0.5">{{ sub }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ text: string; sub?: string }>()
</script>
