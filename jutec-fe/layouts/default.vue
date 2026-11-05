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
          <template v-for="item in nav" :key="item.label">
            <NuxtLink v-if="item.to" :to="item.to" class="nav-item" :class="isActive(item.to) ? 'nav-on' : 'nav-off'">
              <UIcon :name="item.icon" class="w-4 h-4" />{{ item.label }}
            </NuxtLink>
            <UDropdown v-else :items="[menuItems(item.children!)]" :popper="{ placement: 'bottom-start' }" :ui="{ width: 'w-72' }">
              <button type="button" class="nav-item" :class="groupActive(item) ? 'nav-on' : 'nav-off'">
                <UIcon :name="item.icon" class="w-4 h-4" />{{ item.label }}
                <UIcon name="i-lucide-chevron-down" class="w-3.5 h-3.5 opacity-60" />
              </button>
              <template #item="{ item: child }">
                <UIcon :name="child.icon" class="w-4 h-4 shrink-0 self-start mt-0.5" :class="isActive(child.path) ? 'text-primary-300' : 'text-slate-400'" />
                <div class="text-left min-w-0">
                  <p class="text-sm font-medium" :class="isActive(child.path) ? 'text-primary-200' : 'text-slate-100'">{{ child.label }}</p>
                  <p class="text-xs text-slate-500 whitespace-normal leading-snug">{{ child.desc }}</p>
                </div>
              </template>
            </UDropdown>
          </template>
        </nav>

        <div class="flex items-center gap-3 shrink-0">
          <UTooltip :text="apiOnline ? 'Backend API terhubung' : 'Backend API tidak merespons'">
            <span class="hidden md:inline-flex items-center gap-1.5 text-xs font-medium" :class="apiOnline ? 'text-emerald-400' : 'text-rose-400'">
              <span class="w-2 h-2 rounded-full" :class="apiOnline ? 'bg-emerald-400 animate-pulse' : 'bg-rose-400'" />
              {{ apiOnline ? 'Online' : 'Offline' }}
            </span>
          </UTooltip>
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
          <!-- Sistem: konfigurasi & log — jarang dipakai, cukup di balik ikon -->
          <UDropdown :items="[menuItems(system)]" :popper="{ placement: 'bottom-end' }" :ui="{ width: 'w-72' }">
            <UButton icon="i-lucide-settings" color="gray" variant="ghost" size="lg" aria-label="Sistem"
              :class="system.some(x => isActive(x.to)) ? 'text-primary-300 bg-primary-500/10' : ''" />
            <template #item="{ item: child }">
              <UIcon :name="child.icon" class="w-4 h-4 shrink-0 self-start mt-0.5" :class="isActive(child.path) ? 'text-primary-300' : 'text-slate-400'" />
              <div class="text-left min-w-0">
                <p class="text-sm font-medium" :class="isActive(child.path) ? 'text-primary-200' : 'text-slate-100'">{{ child.label }}</p>
                <p class="text-xs text-slate-500 whitespace-normal leading-snug">{{ child.desc }}</p>
              </div>
            </template>
          </UDropdown>
        </div>
      </div>
    </header>

    <main class="flex-1">
      <slot />
    </main>

    <UNotifications />
    <ConfirmDialog />
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const { name: operator, set: setOperator } = useOperator()
const draftName = ref(operator.value)
watch(operator, v => (draftName.value = v))
onMounted(() => { const v = operatorName(); if (v && !operator.value) setOperator(v) })
const { apiBase } = useRuntimeConfig().public

type NavChild = { label: string; to: string; icon: string; desc: string }
type NavItem = { label: string; icon: string; to?: string; children?: NavChild[] }

// Dikelompokkan per alur kerja: temukan → verifikasi → telusuri data → investigasi identitas.
const nav: NavItem[] = [
  { label: 'Dashboard', to: '/', icon: 'i-lucide-layout-dashboard' },
  { label: 'Penemuan', icon: 'i-lucide-radar', children: [
    { label: 'Deep Sweep', to: '/discovery?mode=crawler', icon: 'i-lucide-satellite-dish', desc: 'Crawl rekursif otomatis dari keyword bawaan atau satu domain' },
    { label: 'Pencarian Google', to: '/discovery?mode=google', icon: 'i-lucide-search', desc: 'Cari situs judol lewat Google (Serper)' },
    { label: 'CT Logs', to: '/discovery?mode=ct', icon: 'i-lucide-badge-check', desc: 'Subdomain baru dari Certificate Transparency' },
    { label: 'Analisis Situs', to: '/discovery?mode=analyze', icon: 'i-lucide-scan-line', desc: 'Scan mendalam & skor risiko satu domain' },
  ] },
  { label: 'Verifikasi', to: '/verifikasi', icon: 'i-lucide-badge-check' },
  { label: 'Repository', to: '/repository', icon: 'i-lucide-database' },
  { label: 'Network Graph', to: '/network', icon: 'i-lucide-waypoints' },
  { label: 'Investigasi', icon: 'i-lucide-microscope', children: [
    { label: 'Reverse Lookup', to: '/reverse-lookup', icon: 'i-lucide-scan-search', desc: 'Cari situs lain dari IP, rekening, tracking ID, dll.' },
    { label: 'Form Daftar & Login', to: '/form-daftar', icon: 'i-lucide-clipboard-list', desc: 'Field, bank & pola form daftar situs judol valid' },
    { label: 'Leak / Identitas', to: '/leak', icon: 'i-lucide-fingerprint', desc: 'Pemilik nomor HP & NIK dari data bocoran' },
  ] },
]
const system: NavChild[] = [
  { label: 'Keywords', to: '/keywords', icon: 'i-lucide-tags', desc: 'Kata kunci deteksi & pencarian' },
  { label: 'Blacklist', to: '/blacklist', icon: 'i-lucide-ban', desc: 'Domain yang diabaikan (CDN, sosmed, dll.)' },
  { label: 'Audit Log', to: '/audit', icon: 'i-lucide-scroll-text', desc: 'Jejak siapa melakukan apa' },
  { label: 'Pengaturan', to: '/settings', icon: 'i-lucide-settings', desc: 'Konfigurasi sistem & sumber data' },
]

// Tautan boleh berisi ?mode= — cocokkan path, dan mode bila ada
// Item dropdown tanpa `to`: link ke halaman yang sama (beda ?mode=) dianggap aktif semua oleh ULink
// sehingga gayanya berubah & klik tidak bernavigasi. Navigasi & penanda aktif ditangani sendiri.
function menuItems(children: NavChild[]) {
  return children.map(c => ({ ...c, to: undefined, path: c.to, class: isActive(c.to) ? 'bg-primary-500/10' : '', click: () => navigateTo(c.to) }))
}

function isActive(to: string) {
  const [path, qs] = to.split('?')
  if (path === '/') return route.path === '/'
  if (!route.path.startsWith(path)) return false
  const mode = new URLSearchParams(qs ?? '').get('mode')
  return !mode || (route.query.mode ?? 'crawler') === mode
}
const groupActive = (item: NavItem) => !!item.children?.some(c => route.path.startsWith(c.to.split('?')[0]))

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
.nav-item { @apply flex items-center gap-2 px-3.5 py-2 rounded-lg text-sm font-medium whitespace-nowrap transition-colors; }
.nav-on { @apply bg-primary-500/15 text-primary-300 ring-1 ring-primary-500/30; }
.nav-off { @apply text-slate-400 hover:text-white hover:bg-slate-800/70; }
.no-scrollbar { scrollbar-width: none; }
.no-scrollbar::-webkit-scrollbar { display: none; }
</style>
