<template>
  <div class="page animate-fade-in">
    <PageHeader eyebrow="Data" title="Repository"
      subtitle="Telusuri seluruh hasil pemindaian: domain, artefak transaksi, aplikasi, infrastruktur tracking, operator, dan data leak."
      icon="i-lucide-database">
      <template #actions>
        <UButton size="lg" icon="i-lucide-refresh-ccw-dot" :loading="resolveAllRunning"
          :disabled="pending.total === 0" :color="pending.total > 0 ? 'amber' : 'gray'" variant="soft"
          :label="resolveAllRunning ? 'Memproses...' : pending.total > 0 ? `Resolve Pending (${pending.total})` : 'Tidak ada pending'"
          @click="resolveAll" />
      </template>
    </PageHeader>

    <div class="grid grid-cols-1 xl:grid-cols-[260px_minmax(0,1fr)] gap-6 items-start">
      <!-- Sub-navigation -->
      <nav class="flex xl:flex-col gap-1.5 overflow-x-auto xl:overflow-visible xl:sticky xl:top-24 rounded-2xl xl:bg-slate-900/60 xl:ring-1 xl:ring-slate-800 xl:p-3">
        <p class="hidden xl:block px-3 pt-1 pb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">Kategori</p>
        <button v-for="t in tabs" :key="t.value" @click="activeTab = t.value"
          class="flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium whitespace-nowrap transition-colors text-left"
          :class="activeTab === t.value
            ? 'bg-primary-500/15 text-primary-200 ring-1 ring-primary-500/30'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/70'">
          <UIcon :name="t.icon" class="w-[18px] h-[18px] shrink-0" />
          <span class="flex-1">{{ t.label }}</span>
          <UBadge v-if="t.value === 'telegram' && pending.telegram" color="amber" variant="subtle" size="xs">{{ pending.telegram }}</UBadge>
          <UBadge v-else-if="(t.value === 'kominfo' || t.value === 'dukcapil') && pending.leak" color="amber" variant="subtle" size="xs">{{ pending.leak }}</UBadge>
        </button>
      </nav>

      <div class="min-w-0">
        <!-- ── SITUS ── -->
        <SitesTable v-if="activeTab === 'sites'" title="Semua Situs" />

        <!-- ── ARTEFAK ── -->
        <UCard v-else-if="activeTab === 'artifacts'">
          <template #header>
            <div class="flex flex-col lg:flex-row lg:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">Artefak Transaksi</h2>
                <p class="section-desc">Rekening, wallet, kontak, dan QRIS yang dipakai bersama oleh banyak situs</p>
              </div>
              <div class="flex flex-wrap gap-3">
                <USelect v-model="artType" :options="[{ label: 'Semua Tipe', value: '' }, ...artTypeOptions.map(t => ({ label: t, value: t }))]"
                  class="w-48" @change="artName = ''; loadArtifacts()" />
                <USelect v-if="artType && artNameOptions.length" v-model="artName"
                  :options="[{ label: 'Semua Nama', value: '' }, ...artNameOptions.map(n => ({ label: n, value: n }))]"
                  class="w-48" @change="loadArtifacts()" />
                <div class="flex items-center"><UTooltip text="Bawaannya disembunyikan: item yang semua situs pemakainya sudah tidak aktif">
                  <UToggle v-model="showInactive" /><span class="ml-2 text-sm text-slate-400">Tampilkan dari situs tidak aktif</span>
                </UTooltip></div>
                <UBadge color="gray" variant="subtle" size="lg">{{ artifacts.length }} artefak</UBadge>
              </div>
            </div>
          </template>
          <div class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-4">
            <NuxtLink v-for="art in artifacts" :key="art.id ?? art.value"
              :to="`/reverse-lookup?type=${encodeURIComponent(art.type)}&v=${encodeURIComponent(art.value)}`"
              class="group rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-5 flex items-center gap-4 hover:ring-primary-500/40 transition">
              <div class="w-11 h-11 rounded-xl bg-violet-500/10 ring-1 ring-violet-500/25 flex items-center justify-center shrink-0">
                <UIcon :name="artifactIcon(art.type)" class="w-5 h-5 text-violet-400" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-[15px] font-semibold text-slate-100 truncate mono">{{ art.value }}</p>
                <p class="text-sm text-slate-400 mt-0.5">
                  <span v-if="art.name" class="text-violet-300">{{ art.name }}</span>
                  <span v-if="art.name" class="text-slate-600"> · </span>{{ art.type }}
                  <UBadge v-if="art.reappeared" color="rose" variant="solid" size="xs" class="ml-1.5"><UIcon name="i-lucide-rotate-ccw" class="w-3 h-3 mr-1" />muncul lagi</UBadge>
                </p>
              </div>
              <div class="text-right shrink-0">
                <p class="text-xl font-semibold tabular-nums" :class="art.active_site_count ? 'text-white' : 'text-slate-500'">{{ art.active_site_count ?? art.site_count }}<span class="text-sm text-slate-500 font-normal">/{{ art.site_count }}</span></p>
                <p class="text-xs text-slate-500">situs aktif</p>
              </div>
            </NuxtLink>
          </div>
          <div v-if="!artifacts.length" class="empty-state">
            <div class="empty-icon"><UIcon name="i-lucide-fingerprint" class="w-6 h-6" /></div>
            <p>Tidak ada artefak untuk filter ini.</p>
          </div>
        </UCard>

        <!-- ── APPS ── -->
        <UCard v-else-if="activeTab === 'apps'">
          <template #header>
            <h2 class="section-title">Aplikasi Mobile</h2>
            <p class="section-desc">APK dan link aplikasi yang ditemukan di situs judol · {{ apps.length }} aplikasi</p>
          </template>
          <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
            <div v-for="app in apps" :key="app.id ?? app.url" class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-5 flex flex-col gap-4">
              <div class="flex items-center gap-3">
                <div class="w-11 h-11 rounded-xl flex items-center justify-center shrink-0 ring-1"
                  :class="app.platform === 'ios' ? 'bg-slate-700/40 ring-slate-600' : 'bg-emerald-500/10 ring-emerald-500/25'">
                  <UIcon :name="app.platform === 'ios' ? 'i-lucide-apple' : 'i-lucide-smartphone'" class="w-5 h-5"
                    :class="app.platform === 'ios' ? 'text-slate-200' : 'text-emerald-400'" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-[15px] font-semibold text-slate-100 truncate">{{ app.filename || 'Mobile App' }}</p>
                  <UBadge :color="app.platform === 'android' ? 'emerald' : 'gray'" variant="subtle" size="xs" class="mt-1">{{ app.platform ?? 'unknown' }}</UBadge>
                </div>
              </div>
              <p class="text-xs text-slate-500 mono truncate" :title="app.url">{{ app.url }}</p>
              <div class="flex items-center justify-between text-sm border-t border-slate-800 pt-3 mt-auto">
                <span class="text-slate-400">Ditemukan di <b class="text-slate-100">{{ app.site_count }}</b> situs</span>
                <UButton v-if="app.url" :to="app.url" target="_blank" size="sm" color="gray" variant="soft" icon="i-lucide-external-link" label="Buka" />
              </div>
            </div>
          </div>
          <div v-if="!apps.length" class="empty-state">
            <div class="empty-icon"><UIcon name="i-lucide-smartphone" class="w-6 h-6" /></div>
            <p>Belum ada aplikasi.</p>
          </div>
        </UCard>

        <!-- ── TRACKING IDs ── -->
        <UCard v-else-if="activeTab === 'tracking'" :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex flex-col lg:flex-row lg:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">Tracking ID Bersama</h2>
                <p class="section-desc">Analytics, pixel, dan widget yang dipakai lebih dari satu situs · {{ filteredTracking.length }} ID</p>
              </div>
              <div class="flex flex-col sm:flex-row gap-3">
                <div class="flex items-center"><UTooltip text="Bawaannya disembunyikan: item yang semua situs pemakainya sudah tidak aktif">
                  <UToggle v-model="showInactive" /><span class="ml-2 text-sm text-slate-400">Tampilkan dari situs tidak aktif</span>
                </UTooltip></div>
                <UInput v-model="trackSearch" icon="i-lucide-search" placeholder="Cari ID atau platform..." class="sm:w-72" />
                <USelect v-model="trackPlatform" class="sm:w-56"
                  :options="[{ label: 'Semua Platform', value: '' }, ...trackPlatforms.map(p => ({ label: p, value: p }))]"
                  @change="trackPage = 1; loadTracking()" />
              </div>
            </div>
          </template>
          <UTable :rows="pagedTracking" :columns="trackColumns"
            :empty-state="{ icon: 'i-lucide-radio-tower', label: 'Tidak ada tracking ID.' }">
            <template #value-data="{ row }">
              <div class="flex items-center gap-3">
                <div class="w-9 h-9 rounded-lg flex items-center justify-center shrink-0 ring-1" :class="trackColor(row.platform).bg">
                  <UIcon :name="trackColor(row.platform).icon" class="w-4 h-4" :class="trackColor(row.platform).text" />
                </div>
                <span class="mono text-slate-100">{{ row.value }}</span>
              </div>
            </template>
            <template #platform-data="{ row }"><UBadge color="gray" variant="subtle">{{ row.platform }}</UBadge></template>
            <template #site_count-data="{ row }">
              <span class="font-semibold tabular-nums" :class="row.active_site_count ? 'text-white' : 'text-slate-500'">{{ row.active_site_count ?? row.site_count }}</span><span class="text-slate-500">/{{ row.site_count }} aktif</span>
              <UBadge v-if="row.reappeared" color="rose" variant="solid" size="xs" class="ml-1.5"><UIcon name="i-lucide-rotate-ccw" class="w-3 h-3 mr-1" />muncul lagi</UBadge>
            </template>
            <template #actions-data="{ row }">
              <UButton size="sm" color="gray" variant="soft" icon="i-lucide-scan-search" label="Lookup"
                :to="`/reverse-lookup?type=${lookupTypeFor(row.platform)}&v=${encodeURIComponent(row.value)}`" />
            </template>
          </UTable>
          <template #footer>
            <div class="flex items-center justify-between gap-4">
              <span class="muted">Halaman {{ trackPage }} dari {{ Math.max(1, Math.ceil(filteredTracking.length / trackLimit)) }}</span>
              <UPagination v-model="trackPage" :page-count="trackLimit" :total="filteredTracking.length" :max="7" />
            </div>
          </template>
        </UCard>

        <!-- ── OPERATORS ── -->
        <UCard v-else-if="activeTab === 'operators'">
          <template #header>
            <div class="flex flex-col sm:flex-row sm:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">Operator Terdeteksi</h2>
                <p class="section-desc">Klaster situs berdasarkan UUID platform, brand code, dan nameserver · {{ operators.length }} operator</p>
              </div>
              <UButton icon="i-lucide-git-fork" :loading="discovering" label="Jalankan Clustering" @click="runDiscover" />
            </div>
          </template>
          <div class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-4">
            <div v-for="op in operators" :key="op.id" class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-5 space-y-4">
              <div class="flex items-start gap-3">
                <div class="w-11 h-11 rounded-xl bg-rose-500/10 ring-1 ring-rose-500/25 flex items-center justify-center shrink-0">
                  <UIcon name="i-lucide-user-round-search" class="w-5 h-5 text-rose-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-[15px] font-semibold text-slate-100 truncate mono">{{ op.name || op.operator_key }}</p>
                  <p class="text-sm text-slate-400 mt-0.5">{{ op.platform }}</p>
                </div>
                <UBadge color="rose" variant="subtle" size="md">{{ op.site_count }} situs</UBadge>
              </div>
              <div class="flex items-center gap-3 pt-3 border-t border-slate-800">
                <span class="text-sm text-slate-400 shrink-0">Rata-rata confidence</span>
                <UProgress :value="Math.round(op.avg_confidence * 100)" color="rose" size="sm" class="flex-1" />
                <span class="text-sm font-semibold text-rose-300 tabular-nums">{{ Math.round(op.avg_confidence * 100) }}%</span>
              </div>
            </div>
          </div>
          <div v-if="!operators.length" class="empty-state">
            <div class="empty-icon"><UIcon name="i-lucide-users" class="w-6 h-6" /></div>
            <p>Belum ada operator. Jalankan clustering untuk mengelompokkan situs.</p>
          </div>
        </UCard>

        <!-- ── KOMINFO ── -->
        <UCard v-else-if="activeTab === 'kominfo'" :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex flex-col lg:flex-row lg:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">Leak KomInfo</h2>
                <p class="section-desc">Data registrasi nomor HP · {{ komTotal.toLocaleString('id-ID') }} data</p>
              </div>
              <UInput v-model="komSearch" @update:model-value="onKomSearch" icon="i-lucide-search" placeholder="Cari nomor / NIK..." class="lg:w-80" />
            </div>
          </template>
          <UTable :rows="kominfo" :columns="komColumns" :loading="komLoading"
            :empty-state="{ icon: 'i-lucide-card-sim', label: 'Tidak ada data KomInfo.' }">
            <template #phone-data="{ row }"><span class="mono text-slate-100">{{ row.phone }}</span></template>
            <template #nik-data="{ row }"><span class="mono">{{ row.nik }}</span></template>
            <template #provider-data="{ row }"><UBadge color="gray" variant="subtle">{{ row.provider || '—' }}</UBadge></template>
          </UTable>
          <template #footer>
            <div class="flex items-center justify-between gap-4">
              <span class="muted">Halaman {{ komPage }} dari {{ Math.max(1, Math.ceil(komTotal / komLimit)) }}</span>
              <UPagination v-model="komPage" :page-count="komLimit" :total="komTotal" :max="7" @update:model-value="loadKomInfo" />
            </div>
          </template>
        </UCard>

        <!-- ── DUKCAPIL ── -->
        <UCard v-else-if="activeTab === 'dukcapil'" :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex flex-col lg:flex-row lg:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">Leak Dukcapil</h2>
                <p class="section-desc">Data kependudukan terkait nomor HP · {{ dukTotal.toLocaleString('id-ID') }} data</p>
              </div>
              <UInput v-model="dukSearch" @update:model-value="onDukSearch" icon="i-lucide-search" placeholder="Cari nama / nomor / NIK..." class="lg:w-80" />
            </div>
          </template>
          <UTable :rows="dukcapil" :columns="dukColumns" :loading="dukLoading"
            :empty-state="{ icon: 'i-lucide-id-card', label: 'Tidak ada data Dukcapil.' }">
            <template #full_name-data="{ row }"><span class="font-semibold text-slate-100 capitalize">{{ row.full_name || '—' }}</span></template>
            <template #phone-data="{ row }"><span class="mono">{{ row.phone }}</span></template>
            <template #nik-data="{ row }"><span class="mono">{{ row.nik }}</span></template>
            <template #gender-data="{ row }">
              <UBadge :color="isMale(row.gender) ? 'sky' : isFemale(row.gender) ? 'rose' : 'gray'" variant="subtle">
                {{ isMale(row.gender) ? 'Laki-laki' : isFemale(row.gender) ? 'Perempuan' : '—' }}
              </UBadge>
            </template>
            <template #address-data="{ row }">
              <span class="block max-w-[420px] truncate text-slate-400" :title="addressOf(row)">{{ addressOf(row) }}</span>
            </template>
          </UTable>
          <template #footer>
            <div class="flex items-center justify-between gap-4">
              <span class="muted">Halaman {{ dukPage }} dari {{ Math.max(1, Math.ceil(dukTotal / dukLimit)) }}</span>
              <UPagination v-model="dukPage" :page-count="dukLimit" :total="dukTotal" :max="7" @update:model-value="loadDukcapil" />
            </div>
          </template>
        </UCard>

        <!-- ── LINKS ── -->
        <UCard v-else-if="activeTab === 'links'" :ui="{ body: { padding: '' } }">
          <template #header>
            <div class="flex flex-col lg:flex-row lg:items-center gap-4">
              <div class="flex-1">
                <h2 class="section-title">Link Eksternal Bersama</h2>
                <p class="section-desc">Domain yang di-link dari banyak situs judol · {{ linkTotal.toLocaleString('id-ID') }} domain</p>
              </div>
              <div class="flex flex-col sm:flex-row gap-3">
                <div class="flex items-center"><UTooltip text="Bawaannya disembunyikan: item yang semua situs pemakainya sudah tidak aktif">
                  <UToggle v-model="showInactive" /><span class="ml-2 text-sm text-slate-400">Tampilkan dari situs tidak aktif</span>
                </UTooltip></div>
                <UInput v-model="linkSearch" icon="i-lucide-search" placeholder="Cari domain / URL..." class="sm:w-72" />
                <div class="flex gap-1.5">
                  <UButton v-for="lt in linkTypeOpts" :key="lt.value" :label="lt.label"
                    :color="linkTypeFilter === lt.value ? lt.color : 'gray'" :variant="linkTypeFilter === lt.value ? 'soft' : 'ghost'"
                    @click="linkTypeFilter = lt.value; linkPage = 1" />
                </div>
              </div>
            </div>
          </template>
          <UTable :rows="links" :columns="linkColumns" :loading="linkLoading"
            :empty-state="{ icon: 'i-lucide-link', label: 'Tidak ada link eksternal.' }">
            <template #domain-data="{ row }"><span class="font-semibold text-slate-100">{{ row.domain }}</span></template>
            <template #url-data="{ row }">
              <a :href="row.url" target="_blank" rel="noopener" class="mono text-slate-400 hover:text-cyan-300 truncate block max-w-[380px]" :title="row.url">{{ row.url }}</a>
            </template>
            <template #link_type-data="{ row }">
              <UBadge :color="row.link_type === 'iframe' ? 'amber' : row.link_type === 'register' ? 'emerald' : 'cyan'" variant="subtle">{{ row.link_type }}</UBadge>
            </template>
            <template #site_count-data="{ row }">
              <span class="font-semibold tabular-nums" :class="!row.active_site_count ? 'text-slate-500' : row.site_count >= 5 ? 'text-rose-400' : row.site_count >= 3 ? 'text-amber-400' : 'text-slate-200'">{{ row.active_site_count ?? row.site_count }}</span><span class="text-slate-500">/{{ row.site_count }}</span>
              <UBadge v-if="row.reappeared" color="rose" variant="solid" size="xs" class="ml-1.5"><UIcon name="i-lucide-rotate-ccw" class="w-3 h-3 mr-1" />muncul lagi</UBadge>
            </template>
            <template #sites-data="{ row }">
              <div class="flex flex-wrap gap-1.5">
                <UBadge v-for="s in (row.sites || []).slice(0, 2)" :key="s" color="gray" variant="subtle" class="mono max-w-[200px] truncate">{{ stripScheme(s) }}</UBadge>
                <UBadge v-if="(row.sites || []).length > 2" color="gray" variant="outline">+{{ row.sites.length - 2 }}</UBadge>
              </div>
            </template>
          </UTable>
          <template #footer>
            <div class="flex items-center justify-between gap-4">
              <span class="muted">Halaman {{ linkPage }} dari {{ Math.max(1, Math.ceil(linkTotal / linkLimit)).toLocaleString('id-ID') }}</span>
              <UPagination v-model="linkPage" :page-count="linkLimit" :total="linkTotal" :max="7" @update:model-value="loadLinks" />
            </div>
          </template>
        </UCard>

        <!-- ── TELEGRAM ── -->
        <UCard v-else-if="activeTab === 'telegram'">
          <template #header>
            <h2 class="section-title">Channel Telegram</h2>
            <p class="section-desc">{{ telegram.length }} channel · {{ pendingTg }} username belum di-resolve</p>
          </template>
          <div class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-4">
            <div v-for="ch in telegram" :key="ch.id" class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 p-5 space-y-3">
              <div class="flex items-start gap-3">
                <div class="w-11 h-11 rounded-xl bg-sky-500/10 ring-1 ring-sky-500/25 flex items-center justify-center shrink-0">
                  <UIcon name="i-lucide-send" class="w-5 h-5 text-sky-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-[15px] font-semibold text-slate-100 truncate">{{ ch.display_name || ch.username }}</p>
                  <p class="text-sm text-slate-400">@{{ ch.username }}</p>
                </div>
                <UBadge :color="ch.is_active ? 'emerald' : 'gray'" variant="subtle">{{ ch.is_active ? 'Aktif' : 'Inaktif' }}</UBadge>
              </div>
              <p v-if="ch.bio" class="text-sm text-slate-400 leading-relaxed line-clamp-3">{{ ch.bio }}</p>
              <div class="pt-3 border-t border-slate-800 flex items-center justify-between text-sm">
                <span class="text-slate-400"><UIcon name="i-lucide-users" class="w-4 h-4 align-[-3px] mr-1" />{{ ch.member_count?.toLocaleString('id-ID') ?? '—' }} member</span>
                <span v-if="ch.channel_type" class="text-slate-500">{{ ch.channel_type }}</span>
              </div>
            </div>
          </div>
          <div v-if="!telegram.length" class="empty-state">
            <div class="empty-icon"><UIcon name="i-lucide-send" class="w-6 h-6" /></div>
            <p>Belum ada channel Telegram yang di-resolve.</p>
            <p v-if="pendingTg" class="text-sm">Klik <b class="text-amber-300">Resolve Pending</b> untuk memproses {{ pendingTg }} username.</p>
          </div>
        </UCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'

const { get, post } = useApi()

const tabs = [
  { value: 'sites',     label: 'Situs',        icon: 'i-lucide-globe' },
  { value: 'artifacts', label: 'Artefak',      icon: 'i-lucide-fingerprint' },
  { value: 'apps',      label: 'Aplikasi',     icon: 'i-lucide-smartphone' },
  { value: 'tracking',  label: 'Tracking ID',  icon: 'i-lucide-radio-tower' },
  { value: 'operators', label: 'Operator',     icon: 'i-lucide-users' },
  { value: 'links',     label: 'Link Eksternal', icon: 'i-lucide-link' },
  { value: 'telegram',  label: 'Telegram',     icon: 'i-lucide-send' },
  { value: 'kominfo',   label: 'KomInfo',      icon: 'i-lucide-card-sim' },
  { value: 'dukcapil',  label: 'Dukcapil',     icon: 'i-lucide-id-card' },
]

const trackColumns = [{ key: 'value', label: 'Tracking ID' }, { key: 'platform', label: 'Platform' }, { key: 'site_count', label: 'Dipakai' }, { key: 'actions', label: '' }]
const komColumns   = [{ key: 'phone', label: 'Nomor HP' }, { key: 'nik', label: 'NIK' }, { key: 'provider', label: 'Provider' }, { key: 'reg_date', label: 'Tgl Registrasi' }]
const dukColumns   = [{ key: 'full_name', label: 'Nama' }, { key: 'phone', label: 'Nomor HP' }, { key: 'nik', label: 'NIK' }, { key: 'gender', label: 'Gender' }, { key: 'address', label: 'Alamat' }]
const linkColumns  = [{ key: 'domain', label: 'Domain' }, { key: 'url', label: 'URL' }, { key: 'link_type', label: 'Tipe' }, { key: 'site_count', label: 'Situs' }, { key: 'sites', label: 'Ditemukan di' }]

function stripScheme(u: string) { return u.replace(/^https?:\/\//, '') }
function isMale(g?: string)   { return /^(l|m|male|laki)/i.test(g ?? '') }
function isFemale(g?: string) { return /^(p|f|female|perempuan|wanita)/i.test(g ?? '') }
function addressOf(r: any) { return [r.district, r.location, r.region].filter(Boolean).join(', ') || r.address || '—' }
function artifactIcon(type: string) {
  return ({ Bank: 'i-lucide-landmark', Crypto: 'i-lucide-bitcoin', Contact: 'i-lucide-message-circle', Phone: 'i-lucide-phone', QRIS: 'i-lucide-qr-code', 'E-Wallet': 'i-lucide-wallet' } as Record<string, string>)[type] ?? 'i-lucide-fingerprint'
}
// Platform tracking → tipe di halaman Reverse Lookup
function lookupTypeFor(platform: string) {
  return ({ GoogleAnalytics: 'GA', GoogleTag: 'GA', GoogleTagManager: 'GTM', FacebookPixel: 'FB' } as Record<string, string>)[platform] ?? 'GA'
}

const linkTypeOpts = [
  { value: '',         label: 'Semua',    color: 'primary' },
  { value: 'external', label: 'External', color: 'cyan' },
  { value: 'iframe',   label: 'Iframe',   color: 'amber' },
  { value: 'register', label: 'Register', color: 'emerald' },
] as const

const trackPlatforms = computed(() =>
  [...new Set(tracking.value.map((t: any) => t.platform).filter(Boolean))].sort()
)

const route  = useRoute()
const router = useRouter()
// Tab tersinkron ke URL (?tab=...) agar bisa dibagikan & bertahan saat reload
const activeTab = ref((route.query.tab as string) || 'sites')
watch(activeTab, (tab) => router.replace({ query: { ...route.query, tab } }))
const artifacts      = ref<any[]>([])
const apps           = ref<any[]>([])
const tracking       = ref<any[]>([])
const operators      = ref<any[]>([])
const links          = ref<any[]>([])
const telegram       = ref<any[]>([])
const discovering    = ref(false)
const resolveAllRunning = ref(false)
const pendingTg      = ref(0)
const enriching      = ref(false)
const pendingLeak    = ref(0)
const kominfo        = ref<any[]>([])
const dukcapil       = ref<any[]>([])
const komSearch      = ref('')
const dukSearch      = ref('')
const komPage        = ref(1)
const dukPage        = ref(1)
const komLimit       = 15
const dukLimit       = 15
const komTotal       = ref(0)
const dukTotal       = ref(0)
const komLoading     = ref(false)
const dukLoading     = ref(false)
let   komTimer: any  = null
let   dukTimer: any  = null
const artType        = ref('')
const artName        = ref('')
const artFilters     = ref<Record<string, string[]>>({})
const trackPlatform  = ref('')
const trackSearch    = ref('')
const trackPage      = ref(1)
const trackLimit     = 15
const linkSearch     = ref('')
const linkTypeFilter = ref('')
const linkPage       = ref(1)
const linkLimit      = 15
const linkTotal      = ref(0)
const linkLoading    = ref(false)
let   linkTimer: any = null

const artTypeOptions    = computed(() => Object.keys(artFilters.value).sort())
const artNameOptions    = computed(() => artFilters.value[artType.value] ?? [])
const pending           = computed(() => ({
  telegram: pendingTg.value,
  leak: pendingLeak.value,
  total: pendingTg.value + pendingLeak.value,
}))
const filteredTracking  = computed(() => {
  const q = trackSearch.value.toLowerCase()
  if (!q) return tracking.value
  return tracking.value.filter((t: any) =>
    t.value?.toLowerCase().includes(q) || t.platform?.toLowerCase().includes(q)
  )
})
const pagedTracking = computed(() => {
  const start = (trackPage.value - 1) * trackLimit
  return filteredTracking.value.slice(start, start + trackLimit)
})

function trackColor(platform: string) {
  const map: Record<string, { bg: string; icon: string; text: string }> = {
    GoogleAnalytics:  { bg: 'bg-amber-500/10 ring-amber-500/25',     icon: 'i-lucide-chart-no-axes-column', text: 'text-amber-400' },
    GoogleTag:        { bg: 'bg-amber-500/10 ring-amber-500/25',     icon: 'i-lucide-tag',            text: 'text-amber-400' },
    GoogleTagManager: { bg: 'bg-blue-500/10 ring-blue-500/25',       icon: 'i-lucide-tags',           text: 'text-blue-400' },
    FacebookPixel:    { bg: 'bg-indigo-500/10 ring-indigo-500/25',   icon: 'i-lucide-facebook',       text: 'text-indigo-400' },
    LiveChat:         { bg: 'bg-emerald-500/10 ring-emerald-500/25', icon: 'i-lucide-message-square', text: 'text-emerald-400' },
    TawkTo:           { bg: 'bg-emerald-500/10 ring-emerald-500/25', icon: 'i-lucide-message-square', text: 'text-emerald-400' },
    Crisp:            { bg: 'bg-emerald-500/10 ring-emerald-500/25', icon: 'i-lucide-message-square', text: 'text-emerald-400' },
    Hotjar:           { bg: 'bg-orange-500/10 ring-orange-500/25',   icon: 'i-lucide-flame',          text: 'text-orange-400' },
    AsiaSigma:        { bg: 'bg-rose-500/10 ring-rose-500/25',       icon: 'i-lucide-dices',          text: 'text-rose-400' },
    GigaGaming:       { bg: 'bg-violet-500/10 ring-violet-500/25',   icon: 'i-lucide-gamepad-2',      text: 'text-violet-400' },
  }
  return map[platform] ?? { bg: 'bg-slate-800 ring-slate-700', icon: 'i-lucide-tag', text: 'text-slate-400' }
}

watch(trackSearch,    () => { trackPage.value = 1 })
watch(linkSearch,     () => { clearTimeout(linkTimer); linkTimer = setTimeout(() => { linkPage.value = 1; loadLinks() }, 350) })
watch(linkTypeFilter, () => { linkPage.value = 1; loadLinks() })
// Situs tidak aktif disembunyikan bawaannya; satu sakelar berlaku untuk artefak, tracking & link
const showInactive = ref(false)
watch(showInactive, () => {
  loadArtifacts()
  if (tracking.value.length || activeTab.value === 'tracking') { trackPage.value = 1; loadTracking() }
  if (links.value.length || activeTab.value === 'links') { linkPage.value = 1; loadLinks() }
})

function onKomSearch() {
  clearTimeout(komTimer)
  komTimer = setTimeout(() => { komPage.value = 1; loadKomInfo() }, 350)
}
function onDukSearch() {
  clearTimeout(dukTimer)
  dukTimer = setTimeout(() => { dukPage.value = 1; loadDukcapil() }, 350)
}

function loadTab(tab: string) {
  if (tab === 'tracking'  && !tracking.value.length)  loadTracking()
  if (tab === 'operators' && !operators.value.length) loadOperators()
  if (tab === 'links'     && !links.value.length)     loadLinks()
  if (tab === 'telegram'  && !telegram.value.length)  loadTelegram()
  if (tab === 'kominfo'   && !kominfo.value.length)   { loadKomInfo();   loadLeakPending() }
  if (tab === 'dukcapil'  && !dukcapil.value.length)  { loadDukcapil();  loadLeakPending() }
}
watch(activeTab, loadTab)

onMounted(() => {
  if (activeTab.value !== 'sites') loadTab(activeTab.value)
  loadArtFilters()
  loadArtifacts()
  loadApps()
  loadPendingCounts()
})

async function loadArtFilters() {
  try {
    const d = await get<any>('/data/artifacts/filters')
    artFilters.value = d?.filters ?? {}
  } catch {}
}

async function loadArtifacts() {
  try {
    const d = await get<any>('/data/artifacts', {
      type: artType.value || undefined,
      name: artName.value || undefined,
      include_inactive: showInactive.value ? 1 : undefined,
    })
    const r = d?.results ?? d ?? []
    artifacts.value = Array.isArray(r) ? r.filter((a: any) => a?.value) : []
  } catch {}
}

async function loadApps() {
  try {
    const d = await get<any>('/data/apps')
    apps.value = d?.results ?? d ?? []
  } catch {}
}

async function loadTracking() {
  try {
    const d = await get<any>('/data/tracking', { platform: trackPlatform.value || undefined, include_inactive: showInactive.value ? 1 : undefined })
    tracking.value = d?.results ?? []
  } catch {}
}

async function loadOperators() {
  try {
    const d = await get<any>('/data/operators')
    operators.value = d?.operators ?? []
  } catch {}
}

async function runDiscover() {
  discovering.value = true
  try {
    await post('/operators/discover', {})
    await loadOperators()
  } catch {} finally { discovering.value = false }
}

async function loadLinks() {
  linkLoading.value = true
  try {
    const d = await get<any>('/data/links', {
      q:      linkSearch.value || undefined,
      type:   linkTypeFilter.value || undefined,
      include_inactive: showInactive.value ? 1 : undefined,
      limit:  linkLimit,
      offset: (linkPage.value - 1) * linkLimit,
    })
    links.value = d?.results ?? []
    linkTotal.value = d?.total ?? 0
  } catch {} finally { linkLoading.value = false }
}

async function loadTelegram() {
  try {
    const [d, p] = await Promise.all([
      get<any>('/data/telegram'),
      get<any>('/telegram/pending-count').catch(() => null),
    ])
    telegram.value = d?.results ?? []
    pendingTg.value = p?.count ?? 0
  } catch {}
}

async function loadPendingCounts() {
  const [tg, leak] = await Promise.all([
    get<any>('/telegram/pending-count').catch(() => null),
    get<any>('/leak/pending-count').catch(() => null),
  ])
  pendingTg.value = tg?.count ?? 0
  pendingLeak.value = leak?.count ?? 0
}

async function resolveAll() {
  if (resolveAllRunning.value || pending.value.total === 0) return
  resolveAllRunning.value = true
  try {
    const tasks: Promise<unknown>[] = []
    if (pending.value.telegram > 0) tasks.push(post('/telegram/resolve-pending', {}))
    if (pending.value.leak > 0) tasks.push(post('/leak/enrich', {}))
    await Promise.all(tasks)
    await Promise.all([
      activeTab.value === 'telegram' ? loadTelegram() : loadPendingCounts(),
      activeTab.value === 'kominfo' ? loadKomInfo() : Promise.resolve(),
      activeTab.value === 'dukcapil' ? loadDukcapil() : Promise.resolve(),
    ])
    await loadPendingCounts()
  } catch {} finally { resolveAllRunning.value = false }
}

async function loadKomInfo() {
  komLoading.value = true
  try {
    const d = await get<any>('/data/kominfo', {
      q:      komSearch.value || undefined,
      limit:  komLimit,
      offset: (komPage.value - 1) * komLimit,
    })
    kominfo.value = d?.results ?? []
    komTotal.value = d?.total ?? 0
  } catch {} finally { komLoading.value = false }
}

async function loadLeakPending() {
  try {
    const d = await get<any>('/leak/pending-count')
    pendingLeak.value = d?.count ?? 0
  } catch {}
}

async function enrichLeak() {
  enriching.value = true
  try {
    await post('/leak/enrich', {})
    await Promise.all([loadKomInfo(), loadDukcapil(), loadLeakPending()])
  } catch {} finally { enriching.value = false }
}

async function loadDukcapil() {
  dukLoading.value = true
  try {
    const d = await get<any>('/data/dukcapil', {
      q:      dukSearch.value || undefined,
      limit:  dukLimit,
      offset: (dukPage.value - 1) * dukLimit,
    })
    dukcapil.value = d?.results ?? []
    dukTotal.value = d?.total ?? 0
  } catch {} finally { dukLoading.value = false }
}
</script>
