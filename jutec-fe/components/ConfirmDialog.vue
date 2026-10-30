<template>
  <!-- Dialog konfirmasi global (lihat composables/useConfirm.ts) -->
  <UModal :model-value="state.open" :ui="{ width: 'sm:max-w-md' }" @update:model-value="v => !v && settle(false)">
    <div class="p-6" @keydown.enter.prevent="settle(true)">
      <div class="flex items-start gap-4">
        <div class="w-11 h-11 rounded-xl flex items-center justify-center ring-1 shrink-0" :class="t.box">
          <UIcon :name="state.icon ?? t.icon" class="w-5 h-5" :class="t.text" />
        </div>
        <div class="min-w-0 flex-1 pt-0.5">
          <h3 class="text-base font-semibold text-white leading-snug">{{ state.title }}</h3>
          <p v-if="state.description" class="text-sm text-slate-400 mt-1.5 leading-relaxed">{{ state.description }}</p>
          <ul v-if="state.details?.length" class="mt-3 space-y-1.5">
            <li v-for="d in state.details" :key="d" class="flex items-start gap-2 text-[13px] text-slate-300">
              <UIcon name="i-lucide-dot" class="w-4 h-4 shrink-0 -ml-1" :class="t.text" />{{ d }}
            </li>
          </ul>
        </div>
      </div>
      <div class="flex flex-col-reverse sm:flex-row sm:justify-end gap-2 mt-6">
        <UButton color="gray" variant="soft" :label="state.cancelLabel" @click="settle(false)" />
        <UButton ref="okBtn" :color="t.color" :label="state.confirmLabel" autofocus @click="settle(true)" />
      </div>
    </div>
  </UModal>
</template>

<script setup lang="ts">
const { state, settle } = useConfirm()
const TONES = {
  danger:  { color: 'rose',    icon: 'i-lucide-trash-2',       box: 'bg-rose-500/10 ring-rose-500/25',    text: 'text-rose-400' },
  warning: { color: 'amber',   icon: 'i-lucide-triangle-alert', box: 'bg-amber-500/10 ring-amber-500/25',  text: 'text-amber-400' },
  primary: { color: 'primary', icon: 'i-lucide-circle-help',    box: 'bg-primary-500/10 ring-primary-500/25', text: 'text-primary-400' },
} as const
const t = computed(() => TONES[state.value.tone ?? 'primary'])
</script>
