<template>
  <div class="page-header">
    <div class="flex items-start gap-4">
      <div v-if="icon" class="hidden sm:flex w-12 h-12 shrink-0 rounded-2xl bg-primary-500/10 ring-1 ring-primary-500/25 items-center justify-center">
        <UIcon :name="icon" class="w-6 h-6 text-primary-400" />
      </div>
      <div>
        <p v-if="eyebrow" class="page-eyebrow">{{ eyebrow }}</p>
        <h1 class="page-title" :class="eyebrow ? '' : '!mt-0'">{{ title }}</h1>
        <p v-if="subtitle" class="page-subtitle">{{ subtitle }}</p>
      </div>
    </div>
    <div v-if="$slots.actions" class="flex flex-wrap items-center gap-3">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; subtitle?: string; eyebrow?: string; icon?: string }>()
</script>
