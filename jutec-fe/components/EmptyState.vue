<template>
  <!-- Keadaan kosong: menjelaskan KENAPA kosong & apa langkah berikutnya, bukan sekadar "tidak ada data" -->
  <div class="flex flex-col items-center justify-center text-center rounded-xl border border-dashed"
    :class="[tone === 'good' ? 'border-emerald-500/20 bg-emerald-500/[0.03]' : 'border-slate-700/70 bg-slate-950/30',
      compact ? 'px-3 py-4 gap-1.5' : 'px-6 py-10 gap-2.5']">
    <div class="relative flex items-center justify-center rounded-full ring-1"
      :class="[tone === 'good' ? 'bg-emerald-500/10 ring-emerald-500/25' : 'bg-slate-800/60 ring-slate-700',
        compact ? 'w-8 h-8' : 'w-12 h-12']">
      <UIcon :name="icon" :class="[tone === 'good' ? 'text-emerald-400' : 'text-slate-400', compact ? 'w-4 h-4' : 'w-6 h-6']" />
      <span v-if="!compact" class="absolute inset-0 rounded-full blur-xl opacity-30 -z-10"
        :class="tone === 'good' ? 'bg-emerald-500' : 'bg-primary-500'" />
    </div>
    <p class="font-medium" :class="[compact ? 'text-[13px]' : 'text-sm', tone === 'good' ? 'text-emerald-300' : 'text-slate-200']">{{ title }}</p>
    <p v-if="desc" class="text-slate-500 leading-snug max-w-xs" :class="compact ? 'text-[12px]' : 'text-[13px]'">{{ desc }}</p>
    <UButton v-if="action" :to="action.to" :icon="action.icon" :label="action.label" size="xs" color="gray" variant="soft"
      class="mt-1" @click="action.onClick?.()" />
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  icon?: string
  title: string
  desc?: string
  tone?: 'neutral' | 'good' // good = kosong itu kabar baik (mis. tidak ada antrean tindakan)
  compact?: boolean
  action?: { label: string; to?: string; icon?: string; onClick?: () => void }
}>(), { icon: 'i-lucide-inbox', tone: 'neutral', compact: false })
</script>
