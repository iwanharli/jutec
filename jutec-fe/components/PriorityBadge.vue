<template>
  <UBadge :color="color" variant="subtle" size="md" class="font-semibold">
    <UIcon :name="icon" class="w-3.5 h-3.5 mr-1" />
    {{ priority || 'Low' }}
  </UBadge>
</template>

<script setup lang="ts">
const props = defineProps<{ priority: string }>()
const color = computed(() => props.priority === 'High' ? 'rose' : props.priority === 'Medium' ? 'amber' : 'gray')
const icon  = computed(() => props.priority === 'High' ? 'i-lucide-flame' : props.priority === 'Medium' ? 'i-lucide-alert-triangle' : 'i-lucide-minus')
</script>
