<template>
  <div class="min-h-screen flex flex-col bg-slate-950">
    <!-- Topbar -->
    <header class="sticky top-0 z-50 border-b border-slate-800/80 bg-slate-950/85 backdrop-blur-md">
      <div class="w-full max-w-[1920px] mx-auto h-16 px-6 lg:px-10 flex items-center gap-6">
        <NuxtLink to="/" class="flex items-center gap-3 shrink-0">
          <div class="w-9 h-9 rounded-xl bg-gradient-to-br from-primary-500 to-violet-600 flex items-center justify-center shadow-lg shadow-primary-500/20">
            <UIcon name="i-lucide-shield-half" class="w-5 h-5 text-white" />
          </div>
          <div class="leading-tight hidden sm:block">
            <p class="text-sm font-bold text-white tracking-wide">JUTEC</p>
            <p class="text-[11px] text-slate-500">Judol Intelligence</p>
          </div>
        </NuxtLink>

        <nav class="flex-1 min-w-0 flex items-center gap-1 overflow-x-auto no-scrollbar">
          <NuxtLink v-for="item in nav" :key="item.to" :to="item.to"
            class="flex items-center gap-2 px-3.5 py-2 rounded-lg text-sm font-medium whitespace-nowrap transition-colors"
            :class="isActive(item.to)
              ? 'bg-primary-500/15 text-primary-300 ring-1 ring-primary-500/30'
              : 'text-slate-400 hover:text-white hover:bg-slate-800/70'">
            <UIcon :name="item.icon" class="w-4 h-4" />
            {{ item.label }}
          </NuxtLink>
        </nav>

        <div class="flex items-center gap-3 shrink-0">
          <UBadge :color="apiOnline ? 'emerald' : 'rose'" variant="subtle" size="md" class="hidden md:inline-flex gap-1.5">
            <span class="w-1.5 h-1.5 rounded-full" :class="apiOnline ? 'bg-emerald-400 animate-pulse' : 'bg-rose-400'" />
            {{ apiOnline ? 'API Online' : 'API Offline' }}
          </UBadge>
          <UPopover :popper="{ placement: 'bottom-end' }">
            <UButton :color="operator ? 'gray' : 'amber'" variant="soft" icon="i-lucide-user-round"
              :label="operator || 'Isi nama'" class="max-w-[180px]" :ui="{ base: 'truncate' }" />
            <template #panel="{ close }">
              <form class="p-4 w-72 space-y-3" @submit.prevent="setOperator(draftName); close()">
                <div>
                  <p class="text-sm font-semibold text-white">Nama penyidik / analis</p>
                  <p class="text-xs text-slate-500 mt-0.5">Dicatat di log audit untuk setiap tindakan & akses data. Ini penanda, bukan login.</p>
                </div>
                <UInput v-model="draftName" placeholder="mis. Briptu A. Santoso" autofocus />
                <UButton type="submit" block label="Simpan" :disabled="!draftName.trim()" />
              </form>
            </template>
          </UPopover>
          <UTooltip text="Settings">
            <UButton to="/settings" icon="i-lucide-settings" color="gray" variant="ghost" size="lg"
              :class="isActive('/settings') ? 'text-primary-300 bg-primary-500/10' : ''" />
          </UTooltip>
        </div>
      </div>
    </header>

    <main class="flex-1">
      <slot />
    </main>

    <UNotifications />
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const { name: operator, set: setOperator } = useOperator()
const draftName = ref(operator.value)
watch(operator, v => (draftName.value = v))
onMounted(() => { const v = operatorName(); if (v && !operator.value) setOperator(v) })
const { apiBase } = useRuntimeConfig().public

const nav = [
  { label: 'Dashboard',      to: '/',               icon: 'i-lucide-layout-dashboard' },
  { label: 'Repository',     to: '/repository',     icon: 'i-lucide-database' },
  { label: 'Network',        to: '/network',        icon: 'i-lucide-waypoints' },
  { label: 'Discovery',      to: '/discovery',      icon: 'i-lucide-radar' },
  { label: 'Reverse Lookup', to: '/reverse-lookup', icon: 'i-lucide-scan-search' },
  { label: 'Leak',           to: '/leak',           icon: 'i-lucide-fingerprint' },
  { label: 'Keywords',       to: '/keywords',       icon: 'i-lucide-tags' },
  { label: 'Verifikasi',     to: '/verifikasi',     icon: 'i-lucide-badge-check' },
  { label: 'Blacklist',      to: '/blacklist',      icon: 'i-lucide-ban' },
  { label: 'Audit',          to: '/audit',          icon: 'i-lucide-scroll-text' },
]

function isActive(to: string) {
  if (to === '/') return route.path === '/'
  return route.path.startsWith(to)
}

// Status API sungguhan (bukan indikator statis)
const apiOnline = ref(true)
async function ping() {
  try { apiOnline.value = (await fetch(`${apiBase}/health`)).ok } catch { apiOnline.value = false }
}
let pingTimer: any
onMounted(() => { ping(); pingTimer = setInterval(ping, 15000) })
onUnmounted(() => clearInterval(pingTimer))
</script>

<style scoped>
.no-scrollbar { scrollbar-width: none; }
.no-scrollbar::-webkit-scrollbar { display: none; }
</style>
