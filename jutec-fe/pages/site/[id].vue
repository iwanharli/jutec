<template>
  <div class="page !space-y-5 animate-fade-in">
    <UCard v-if="loading && !detail">
      <div class="empty-state !py-32">
        <UIcon name="i-lucide-loader-circle" class="w-8 h-8 text-primary-400 animate-spin" />
        <p>Memuat detail situs...</p>
      </div>
    </UCard>
    <UCard v-else-if="error">
      <div class="empty-state !py-32">
        <div class="empty-icon"><UIcon name="i-lucide-search-x" class="w-6 h-6" /></div>
        <p class="text-base text-slate-300 font-medium">{{ error }}</p>
        <UButton to="/repository" color="gray" variant="soft" icon="i-lucide-arrow-left" label="Kembali ke Repository" />
      </div>
    </UCard>

    <template v-else-if="detail">
      <!-- ── Header ── -->
      <UButton color="gray" variant="link" icon="i-lucide-arrow-left" label="Kembali" :padded="false" class="-mb-2" @click="$router.back()" />
      <div class="flex flex-col xl:flex-row xl:items-center gap-4">
        <div class="flex items-center gap-4 min-w-0 flex-1">
          <div class="w-12 h-12 shrink-0 rounded-xl flex items-center justify-center ring-1"
            :class="site.is_gambling ? 'bg-rose-500/10 ring-rose-500/30' : 'bg-emerald-500/10 ring-emerald-500/30'">
            <UIcon :name="site.is_gambling ? 'i-lucide-shield-alert' : 'i-lucide-shield-check'" class="w-6 h-6"
              :class="site.is_gambling ? 'text-rose-400' : 'text-emerald-400'" />
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-2xl lg:text-3xl font-semibold text-white break-words">{{ host }}</h1>
              <UBadge :color="site.is_gambling ? 'rose' : 'emerald'" variant="subtle">{{ site.is_gambling ? 'Situs Judol' : 'Normal' }}</UBadge>
              <PriorityBadge :priority="site.priority" />
              <VerifyControl :site="site" @updated="load(String(route.params.id))" />
              <UTooltip v-if="reachMeta" :text="reachTip">
                <UBadge :color="reachMeta.color" variant="subtle"><UIcon :name="reachMeta.icon" class="w-3.5 h-3.5 mr-1" />{{ reachMeta.label }}</UBadge>
              </UTooltip>
              <UBadge v-if="site.role === 'operator'" color="rose" variant="solid"><UIcon name="i-lucide-castle" class="w-3.5 h-3.5 mr-1" />Situs utama</UBadge>
              <UBadge v-else-if="site.role === 'landing'" color="sky" variant="subtle"><UIcon name="i-lucide-door-open" class="w-3.5 h-3.5 mr-1" />Landing page</UBadge>
              <UBadge v-if="site.app_promo" color="amber" variant="subtle"><UIcon name="i-lucide-smartphone" class="w-3.5 h-3.5 mr-1" />Promosi APK</UBadge>
              <UBadge v-if="site.status && site.status !== 'active'" color="amber" variant="subtle">{{ site.status }}</UBadge>
            </div>
            <p class="text-sm text-slate-500 mt-1">
              Situs #{{ site.id }} · <a :href="site.url" target="_blank" rel="noopener" class="mono hover:text-primary-300 break-words">{{ site.url }}</a>
            </p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2 shrink-0">
          <!-- Utilitas -->
          <UButtonGroup size="md" orientation="horizontal">
            <UTooltip text="Buka situs di tab baru">
              <UButton color="gray" variant="soft" icon="i-lucide-external-link" :to="site.url" target="_blank" />
            </UTooltip>
            <UTooltip text="Salin URL">
              <UButton color="gray" variant="soft" icon="i-lucide-copy" @click="copyText(site.url)" />
            </UTooltip>
            <UTooltip v-if="exportWaiting" text="Menunggu data selesai — klik untuk batal">
              <UButton color="gray" variant="soft" icon="i-lucide-file-down" loading @click="requestExport()" />
            </UTooltip>
            <UDropdown v-else :items="exportItems" :popper="{ placement: 'bottom-end' }">
              <UTooltip text="Ekspor laporan">
                <UButton color="gray" variant="soft" icon="i-lucide-file-down" trailing-icon="i-lucide-chevron-down" />
              </UTooltip>
            </UDropdown>
          </UButtonGroup>

          <!-- Investigasi lanjutan -->
          <UDropdown :items="investigateItems" :popper="{ placement: 'bottom-end' }" :ui="{ width: 'w-80' }">
            <UButton color="gray" variant="soft" icon="i-lucide-microscope" label="Investigasi" trailing-icon="i-lucide-chevron-down"
              :loading="deepLoading || mirrorLoading" />
            <template #item="{ item }">
              <UIcon :name="item.icon" class="w-5 h-5 shrink-0 text-slate-400 self-start mt-0.5" />
              <div class="text-left min-w-0">
                <p class="text-sm font-medium text-slate-100">{{ item.label }}</p>
                <p class="text-xs text-slate-500 whitespace-normal leading-snug">{{ item.desc }}</p>
              </div>
            </template>
          </UDropdown>

          <!-- Aksi utama -->
          <UButton icon="i-lucide-scan-line" label="Scan Ulang" :to="`/discovery?url=${encodeURIComponent(site.url)}`" />
        </div>
      </div>

      <UAlert v-if="site.status === 'unreachable'" icon="i-lucide-wifi-off" color="amber" variant="subtle"
        title="Situs tidak bisa diakses saat scan terakhir"
        description="Domain mati, diblokir, atau menolak koneksi — belum ada konten, artefak, maupun infrastruktur yang tercatat." />

      <UAlert v-if="site.lifecycle === 'inactive'" icon="i-lucide-archive" color="gray" variant="subtle"
        :title="`Situs tidak aktif${inactiveSince ? ` sejak ${fmtDate(inactiveSince)}` : ''}`"
        description="Disembunyikan dari dashboard & repository secara bawaan. Seluruh data tetap tersimpan sebagai bukti, dan situs tetap dicek mingguan — bila hidup lagi akan otomatis kembali aktif." />
      <UAlert v-else-if="revivedRecently" icon="i-lucide-rotate-ccw" color="rose" variant="subtle"
        :title="`Situs hidup lagi ${fmtDate(site.revived_at)}`"
        description="Sebelumnya sudah tidak aktif — operator kemungkinan memakai ulang domain lama. Pertimbangkan scan ulang & ambil bukti baru." />

      <!-- ── KPI strip ── -->
      <div class="grid grid-cols-2 md:grid-cols-3 2xl:grid-cols-6 gap-3">
        <div v-for="k in kpis" :key="k.label" class="rounded-xl bg-slate-900/60 ring-1 ring-slate-800 px-4 py-3 flex items-center gap-3">
          <div class="w-9 h-9 rounded-lg flex items-center justify-center shrink-0" :class="k.box">
            <UIcon :name="k.icon" class="w-[18px] h-[18px]" :class="k.text" />
          </div>
          <div class="min-w-0">
            <p class="text-[13px] uppercase tracking-wider text-slate-500">{{ k.label }}</p>
            <p class="text-lg font-semibold text-white leading-tight">{{ k.value }}</p>
            <p v-if="k.hint" class="text-[13px] text-slate-500 leading-tight">{{ k.hint }}</p>
          </div>
        </div>
      </div>

      <!-- ── Submenu per informasi ── -->
      <nav class="sticky top-16 z-30 -mx-1 px-1 py-2 bg-slate-950/85 backdrop-blur-md">
        <div class="flex gap-1 overflow-x-auto no-scrollbar rounded-xl bg-slate-900/70 ring-1 ring-slate-800 p-1">
          <button v-for="t in TABS" :key="t.key" type="button" @click="tab = t.key"
            class="flex items-center gap-2 px-3.5 py-2 rounded-lg text-sm font-medium whitespace-nowrap transition"
            :class="tab === t.key ? 'bg-primary-500/15 text-primary-200 ring-1 ring-primary-500/30'
              : t.count ? 'text-slate-300 hover:text-white hover:bg-slate-800/70' : 'text-slate-600 hover:text-slate-400'">
            <UIcon :name="t.icon" class="w-4 h-4" />{{ t.label }}
            <UIcon v-if="t.loading" name="i-lucide-loader-circle" class="w-3.5 h-3.5 animate-spin text-primary-400" />
            <span v-else-if="t.count" class="text-[11px] tabular-nums rounded-full px-1.5 py-px"
              :class="tab === t.key ? 'bg-primary-500/25 text-primary-100' : 'bg-slate-800 text-slate-400'">{{ t.count }}</span>
          </button>
        </div>
      </nav>

      <!-- ── Peta keterkaitan (lebar penuh, bisa layar penuh) ── -->
      <div v-if="tab === 'jaringan'" ref="mapEl" class="rounded-2xl ring-1 ring-slate-800 bg-slate-900/60 overflow-hidden flex flex-col"
        :class="isFullscreen ? 'bg-slate-950 !rounded-none' : ''">
        <div class="px-5 py-4 border-b border-slate-800 space-y-3">
          <div class="flex flex-wrap items-center gap-3">
            <div class="card-icon bg-primary-500/10 ring-primary-500/25"><UIcon name="i-lucide-share-2" class="w-[18px] h-[18px] text-primary-400" /></div>
            <div class="flex-1 min-w-0">
              <h2 class="card-title">Peta Keterkaitan</h2>
              <p class="card-desc">{{ filteredStats.attrs }} atribut · {{ filteredStats.sites }} situs terkait</p>
            </div>
            <UInput v-model="mapSearch" icon="i-lucide-search" placeholder="Cari domain / atribut di peta..." size="sm" class="w-64" />
            <div class="flex p-0.5 rounded-lg bg-slate-950/60 ring-1 ring-slate-800 gap-0.5">
              <UButton v-for="v in views" :key="v.value" size="sm" :icon="v.icon" :label="v.label"
                :color="view === v.value ? 'primary' : 'gray'" :variant="view === v.value ? 'solid' : 'ghost'" @click="view = v.value" />
            </div>
            <UDropdown :items="graphExportItems" :popper="{ placement: 'bottom-end' }">
              <UButton size="sm" color="gray" variant="soft" icon="i-lucide-image-down" label="Unduh" :disabled="!filteredStats.attrs" />
            </UDropdown>
            <UTooltip :text="isFullscreen ? 'Keluar layar penuh (Esc)' : 'Layar penuh'">
              <UButton size="sm" color="gray" variant="soft" :icon="isFullscreen ? 'i-lucide-minimize' : 'i-lucide-maximize'"
                :label="isFullscreen ? 'Keluar' : 'Layar Penuh'" @click="toggleFullscreen" />
            </UTooltip>
          </div>
          <!-- Filter -->
          <div class="flex flex-col 2xl:flex-row 2xl:items-start gap-3">
            <!-- Jenis atribut -->
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-2">
                <p class="text-xs font-semibold uppercase tracking-wider text-slate-500">Jenis atribut</p>
                <span class="text-xs text-slate-600">{{ activeKinds.size }}/{{ availableKinds.length }} aktif</span>
                <UButton size="2xs" color="gray" variant="ghost" class="ml-auto 2xl:ml-1"
                  :icon="activeKinds.size === availableKinds.length ? 'i-lucide-square-dashed' : 'i-lucide-check-check'"
                  :label="activeKinds.size === availableKinds.length ? 'Kosongkan' : 'Pilih semua'"
                  @click="activeKinds = new Set(activeKinds.size === availableKinds.length ? [] : availableKinds.map(k => k.kind))" />
              </div>
              <div class="flex flex-wrap gap-1.5">
                <button v-for="k in availableKinds" :key="k.kind" type="button" @click="toggleKind(k.kind)"
                  class="group/kind inline-flex items-center gap-2 rounded-full pl-2.5 pr-1 py-1 text-[13px] font-medium ring-1 transition"
                  :class="activeKinds.has(k.kind) ? 'text-slate-100' : 'text-slate-500 ring-slate-800 hover:text-slate-300 hover:ring-slate-700'"
                  :style="activeKinds.has(k.kind) ? { background: k.color + '1f', '--tw-ring-color': k.color + '66' } : {}">
                  <span class="w-2 h-2 rounded-full transition"
                    :style="activeKinds.has(k.kind) ? { background: k.color } : { boxShadow: `inset 0 0 0 1.5px ${k.color}` }" />
                  {{ k.label }}
                  <span class="tabular-nums text-[11px] min-w-[22px] text-center rounded-full px-1.5 py-px"
                    :class="activeKinds.has(k.kind) ? 'bg-slate-950/50 text-slate-200' : 'bg-slate-800/60 text-slate-500'">{{ k.n }}</span>
                </button>
              </div>
            </div>

            <!-- Opsi tampilan -->
            <div class="2xl:w-auto shrink-0">
              <p class="text-xs font-semibold uppercase tracking-wider text-slate-500 mb-2">Tampilan</p>
              <div class="flex flex-wrap gap-1.5">
                <UTooltip v-for="o in mapOptions" :key="o.label" :text="o.hint">
                  <button type="button" @click="o.toggle()"
                    class="inline-flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-[13px] ring-1 transition"
                    :class="o.on ? 'bg-primary-500/15 ring-primary-500/40 text-primary-100' : 'ring-slate-800 text-slate-400 hover:text-slate-200 hover:ring-slate-700'">
                    <UIcon :name="o.icon" class="w-4 h-4" :class="o.on ? 'text-primary-300' : 'text-slate-500'" />
                    {{ o.label }}
                    <span class="w-7 h-4 rounded-full relative transition" :class="o.on ? 'bg-primary-500' : 'bg-slate-700'">
                      <span class="absolute top-0.5 w-3 h-3 rounded-full bg-white transition-all" :class="o.on ? 'left-3.5' : 'left-0.5'" />
                    </span>
                  </button>
                </UTooltip>
              </div>
            </div>
          </div>
        </div>

        <div v-if="!relations" class="empty-state"><UIcon name="i-lucide-loader-circle" class="w-7 h-7 animate-spin text-primary-400" /><p>Membangun relasi...</p></div>
        <div v-else-if="view === 'graph'" class="relative bg-[#030a17]" :class="isFullscreen ? 'flex-1' : 'h-[640px]'">
          <div class="absolute inset-0 pointer-events-none opacity-30" style="background-image: radial-gradient(circle, #1e293b 1px, transparent 1px); background-size: 26px 26px;" />
          <SiteRelationGraph :relations="filteredRelations" :kind-meta="kindMeta" :show-isolated="showIsolated" :highlight="mapSearch"
            @open-site="id => navigateTo(`/site/${id}`)" @open-attr="openLookup" />
          <p v-if="!filteredStats.attrs" class="absolute inset-0 flex items-center justify-center text-slate-500 pointer-events-none">Tidak ada data untuk filter ini.</p>
        </div>
          <!-- Tree -->
          <div v-else class="overflow-y-auto" :class="isFullscreen ? 'flex-1' : 'h-[640px]'"><div class=" px-5 py-4 space-y-2">
            <div class="flex items-center gap-3 pb-3 border-b border-slate-800">
              <UIcon name="i-lucide-globe" class="w-5 h-5 text-rose-400" />
              <span class="mono text-white font-semibold">{{ host }}</span>
              <UButton size="xs" color="gray" variant="ghost" :label="allOpen ? 'Tutup semua' : 'Buka semua'" class="ml-auto" @click="toggleAll" />
            </div>
            <details v-for="grp in treeGroups" :key="grp.kind" class="tree-node group/kind" :open="openKinds.has(grp.kind)"
              @toggle="(e: any) => setOpen(openKinds, grp.kind, e.target.open)">
              <summary class="tree-summary">
                <UIcon name="i-lucide-chevron-right" class="w-4 h-4 text-slate-500 transition-transform group-open/kind:rotate-90" />
                <span class="w-2.5 h-2.5 rounded-full" :style="{ background: grp.color }" />
                <span class="font-semibold text-slate-100">{{ grp.label }}</span>
                <UBadge color="gray" variant="subtle" size="xs">{{ grp.attrs.length }}</UBadge>
                <span v-if="grp.linked" class="text-[13px] text-primary-300">{{ grp.linked }} terhubung ke situs lain</span>
              </summary>
              <div class="tree-children">
                <details v-for="a in grp.attrs" :key="a.kind + a.id" class="tree-node group/attr" :open="openAttrs.has(a.kind + a.id)"
                  @toggle="(e: any) => setOpen(openAttrs, a.kind + a.id, e.target.open)">
                  <summary class="tree-summary" :class="!a.sites.length && 'cursor-default'">
                    <UIcon name="i-lucide-chevron-right" class="w-4 h-4 transition-transform group-open/attr:rotate-90"
                      :class="a.sites.length ? 'text-slate-500' : 'text-transparent'" />
                    <span class="mono text-slate-200 break-words">{{ a.value }}</span>
                    <span v-if="a.label" class="text-sm text-slate-400">{{ a.kind === 'link' ? `${a.label} URL` : a.label }}</span>
                    <UBadge v-if="a.group" color="gray" variant="outline" size="xs">{{ a.group }}</UBadge>
                    <UBadge v-if="a.common" color="amber" variant="subtle" size="xs">umum · {{ a.shared_sites }} situs</UBadge>
                    <UBadge v-else-if="a.shared_sites" color="primary" variant="subtle" size="xs">{{ a.shared_sites }} situs lain</UBadge>
                    <span v-else class="text-[13px] text-slate-600">unik</span>
                    <UButton v-if="lookupType(a)" size="2xs" color="gray" variant="ghost" icon="i-lucide-scan-search" class="ml-auto"
                      @click.prevent="openLookup(a)" />
                  </summary>
                  <div v-if="a.sites.length" class="tree-children">
                    <NuxtLink v-for="s in a.sites" :key="s.id" :to="`/site/${s.id}`"
                      class="flex items-center gap-3 rounded-lg px-3 py-2 hover:bg-slate-800/60 transition">
                      <span class="w-2 h-2 rounded-full shrink-0" :class="s.is_gambling ? 'bg-rose-500' : 'bg-slate-500'" />
                      <span class="text-sm text-slate-200 break-words">{{ stripScheme(s.url) }}</span>
                      <span class="ml-auto text-[13px] tabular-nums text-slate-500">{{ Math.round(s.confidence * 100) }}%</span>
                    </NuxtLink>
                    <p v-if="a.shared_sites > a.sites.length" class="text-[13px] text-slate-500 px-3 py-1">
                      + {{ a.shared_sites - a.sites.length }} situs lain —
                      <a class="text-primary-300 hover:underline cursor-pointer" @click="openLookup(a)">lihat semua</a>
                    </p>
                  </div>
                </details>
              </div>
            </details>
            <p v-if="!treeGroups.length" class="muted py-8 text-center">Tidak ada atribut{{ showIsolated ? '' : ' yang terhubung ke situs lain' }}.</p>
          </div>
        </div>
      </div>

      <!-- ── Peringatan: artefak dipakai situs judol terverifikasi ── -->
      <UAlert v-if="tab === 'ringkasan' && flaggedArtifacts.length" icon="i-lucide-triangle-alert" color="rose" variant="subtle"
        :title="`${flaggedArtifacts.length} rekening/kontak juga dipakai situs yang sudah diverifikasi judol`">
        <template #description>
          <ul class="mt-1 space-y-1">
            <li v-for="a in flaggedArtifacts" :key="a.id" class="text-sm">
              <span class="text-slate-400">{{ a.label || a.group }}</span> <span class="mono text-slate-100">{{ a.value }}</span>
              <span class="text-slate-400"> → </span>
              <template v-for="(v, i) in a.verified_sites.slice(0, 3)" :key="v.id">
                <NuxtLink :to="`/site/${v.id}`" class="mono text-rose-300 hover:underline">{{ stripScheme(v.url) }}</NuxtLink><span v-if="i < Math.min(3, a.verified_sites.length) - 1">, </span>
              </template>
              <span v-if="a.verified_sites.length > 3" class="text-slate-400"> +{{ a.verified_sites.length - 3 }} lainnya</span>
            </li>
          </ul>
        </template>
      </UAlert>

      <!-- ── Ringkasan · situs terkait ── -->
      <div v-if="tab === 'ringkasan' || tab === 'jaringan'" class="grid grid-cols-1 gap-5 items-stretch" :class="tab === 'ringkasan' && 'xl:grid-cols-2'">
        <!-- Ringkasan intelijen -->
        <UCard v-if="tab === 'ringkasan'" :ui="{ body: { padding: 'px-5 py-4' }, header: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-primary-500/10 ring-primary-500/25">
                <UIcon name="i-lucide-clipboard-list" class="w-[18px] h-[18px] text-primary-400" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Ringkasan Intelijen</h2>
                <p class="card-desc">Temuan utama dari seluruh scan situs ini</p>
              </div>
              <UBadge color="gray" variant="subtle">{{ facts.length }} temuan</UBadge>
            </div>
          </template>

          <ul class="divide-y divide-slate-800/70 -my-3">
            <li v-for="(f, i) in facts" :key="i" class="flex items-start gap-3 py-3">
              <div class="w-8 h-8 rounded-lg bg-slate-800/70 ring-1 ring-slate-700/60 flex items-center justify-center shrink-0">
                <UIcon :name="f.icon" class="w-4 h-4" :class="f.color" />
              </div>
              <div class="min-w-0 flex-1 pt-0.5">
                <p class="text-sm font-medium text-slate-100 leading-snug">{{ f.title }}</p>
                <p v-if="f.detail" class="text-[13px] text-slate-400 mono mt-1 leading-relaxed [overflow-wrap:anywhere]">{{ f.detail }}</p>
              </div>
            </li>
          </ul>

          <div v-if="composition.length" class="mt-3 pt-4 border-t border-slate-800">
            <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-2.5">Komposisi atribut</p>
            <div class="space-y-2">
              <div v-for="c in composition" :key="c.kind" class="flex items-center gap-3 text-[13px]">
                <span class="w-32 shrink-0 text-slate-400 truncate">{{ c.label }}</span>
                <div class="flex-1 h-2 rounded-full bg-slate-800 overflow-hidden">
                  <div class="h-full rounded-full transition-all" :style="{ width: c.pct + '%', background: c.color }" />
                </div>
                <span class="w-6 text-right tabular-nums text-slate-200 font-medium">{{ c.n }}</span>
                <UTooltip :text="`${c.linked} dari ${c.n} terhubung ke situs lain`">
                  <span class="w-10 inline-flex items-center justify-end gap-0.5 tabular-nums" :class="c.linked ? 'text-primary-300' : 'text-slate-600'">
                    <UIcon name="i-lucide-link-2" class="w-3 h-3" />{{ c.linked }}
                  </span>
                </UTooltip>
              </div>
            </div>
          </div>

          <div v-if="missing.length" class="mt-4 pt-4 border-t border-slate-800">
            <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-2">Tidak ditemukan</p>
            <div class="flex flex-wrap gap-1.5">
              <UBadge v-for="m in missing" :key="m" color="gray" variant="outline" size="xs">{{ m }}</UBadge>
            </div>
          </div>
        </UCard>

        <!-- Situs paling terkait -->
        <UCard class="flex flex-col" :ui="{ body: { padding: '', base: 'flex-1 relative min-h-[360px]' }, header: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-indigo-500/10 ring-indigo-500/25"><UIcon name="i-lucide-waypoints" class="w-[18px] h-[18px] text-indigo-400" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Situs Terkait</h2><p class="card-desc">Urut kekuatan bukti: operator & artefak &gt; tracking & SSL &gt; link, IP & NS</p></div>
              <UBadge color="gray" variant="subtle">{{ relations?.related.length ?? 0 }}</UBadge>
            </div>
          </template>
          <div v-if="!relations" class="px-5 py-4 space-y-2"><USkeleton v-for="i in 8" :key="i" class="h-10 w-full" /></div>
          <div v-else-if="!relations.related.length" class="empty-state !py-14">
            <div class="empty-icon"><UIcon name="i-lucide-unlink" class="w-6 h-6" /></div>
            <p class="text-sm">Belum ada situs lain yang berbagi atribut.</p>
          </div>
          <div v-else class="absolute inset-0 overflow-y-auto divide-y divide-slate-800/60">
            <div v-for="(r, i) in relations.related" :key="r.id" class="group/rel flex items-start gap-3 px-5 py-3 hover:bg-slate-800/40">
              <span class="w-5 text-[13px] tabular-nums text-slate-600 pt-1">{{ Number(i) + 1 }}</span>
              <div class="flex-1 min-w-0">
                <NuxtLink :to="`/site/${r.id}`" class="text-sm text-slate-100 hover:text-primary-300 break-words leading-snug">{{ stripScheme(r.url) }}</NuxtLink>
                <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 mt-1">
                  <span v-for="k in r.shared_kinds" :key="k" class="inline-flex items-center gap-1 text-[13px] text-slate-400">
                    <span class="w-1.5 h-1.5 rounded-full" :style="{ background: kindMeta[k]?.color }" />{{ kindMeta[k]?.label ?? k }}
                  </span>
                  <span class="text-[13px]" :class="r.is_gambling ? 'text-rose-400' : 'text-slate-500'">· {{ r.is_gambling ? 'Judol' : 'Normal' }}</span>
                </div>
              </div>
              <div class="flex flex-col items-end gap-1 shrink-0">
                <UBadge :color="r.strength >= 4 ? 'rose' : r.strength >= 2 ? 'amber' : 'gray'" variant="subtle" size="xs">{{ r.shared_count }} sama</UBadge>
                <div class="flex gap-0.5 opacity-60 group-hover/rel:opacity-100">
                  <UTooltip text="Detail"><UButton :to="`/site/${r.id}`" size="2xs" color="gray" variant="ghost" icon="i-lucide-panel-right-open" /></UTooltip>
                  <UTooltip text="Analisis ulang"><UButton :to="`/discovery?url=${encodeURIComponent(r.url)}`" size="2xs" color="gray" variant="ghost" icon="i-lucide-scan-line" /></UTooltip>
                </div>
              </div>
            </div>
          </div>
        </UCard>
      </div>

      <SiteRoleCard v-if="tab === 'ringkasan' && detail.role_links" :role="site.role" :links="detail.role_links" />

      <!-- ── Dasar skor & pivot terkuat ── -->
      <div v-if="tab === 'ringkasan'" class="grid grid-cols-1 xl:grid-cols-2 gap-5 items-start">
        <SiteScoreExplain :data="scoreData" />
        <SitePivots :items="pivots" @lookup="openLookup" />
      </div>

      <!-- ── Bukti forensik ── -->
      <UCard v-if="tab === 'bukti'" :ui="{ ring: 'ring-1 ring-sky-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
        <template #header>
          <div class="flex flex-wrap items-center gap-3">
            <div class="card-icon bg-sky-500/10 ring-sky-500/25">
              <UIcon name="i-lucide-file-lock-2" class="w-[18px] h-[18px] text-sky-400" />
            </div>
            <div class="flex-1 min-w-0">
              <h2 class="card-title">Bukti Forensik</h2>
              <p class="card-desc">HTML asli, screenshot desktop & HP, dan log jaringan — di-hash SHA-256, tulis-sekali, tercatat di log audit</p>
            </div>
            <UBadge v-if="evidence.length" color="gray" variant="subtle">{{ evidence.length }} pengambilan</UBadge>
            <UButton color="sky" icon="i-lucide-camera" :loading="evidenceBusy" :label="evidenceBusy ? 'Mengambil bukti…' : 'Ambil Bukti Sekarang'" @click="captureEvidence" />
          </div>
        </template>

        <p v-if="!evidence.length" class="text-sm text-slate-400">
          Belum ada bukti tersimpan. Ambil bukti sebelum situs berganti konten atau diblokir — hasilnya bisa diverifikasi ulang kapan pun dan diunduh sebagai paket ZIP.
        </p>
        <div v-else class="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-4">
          <div v-for="ev in evidence" :key="ev.id" class="rounded-xl bg-slate-950/60 ring-1 ring-slate-800 overflow-hidden flex flex-col">
            <a v-if="ev.status === 'done'" :href="evFileURL(ev, 'screenshot-desktop.png')" target="_blank" rel="noopener"
              class="block h-40 overflow-hidden bg-slate-900 border-b border-slate-800 group/shot">
              <img :src="evFileURL(ev, 'screenshot-desktop.png')" :alt="`Screenshot bukti #${ev.id}`" loading="lazy"
                class="w-full object-cover object-top opacity-90 group-hover/shot:opacity-100 transition" />
            </a>
            <div v-else class="h-40 flex flex-col items-center justify-center gap-2 bg-slate-900 border-b border-slate-800 text-sm"
              :class="ev.status === 'error' ? 'text-rose-400' : 'text-slate-400'">
              <UIcon :name="ev.status === 'error' ? 'i-lucide-circle-x' : 'i-lucide-loader-circle'" class="w-6 h-6" :class="ev.status === 'pending' && 'animate-spin'" />
              {{ ev.status === 'error' ? 'Gagal mengambil bukti' : 'Sedang mengambil…' }}
            </div>

            <div class="p-4 space-y-2.5 flex-1 text-sm">
              <div class="flex items-center gap-2">
                <span class="font-semibold text-white">#{{ ev.id }}</span>
                <span class="text-slate-400">{{ fmtDate(ev.captured_at || ev.created_at) }}</span>
                <UBadge v-if="evVerify[ev.id]" :color="evVerify[ev.id].intact ? 'emerald' : 'rose'" variant="subtle" size="xs" class="ml-auto">
                  <UIcon :name="evVerify[ev.id].intact ? 'i-lucide-shield-check' : 'i-lucide-shield-x'" class="w-3.5 h-3.5 mr-1" />
                  {{ evVerify[ev.id].intact ? 'Utuh' : 'Berubah!' }}
                </UBadge>
              </div>
              <p v-if="ev.status === 'error'" class="text-rose-300 text-[13px] break-words">{{ ev.error }}</p>
              <template v-else-if="ev.status === 'done'">
                <dl class="grid grid-cols-[92px_1fr] gap-x-2 gap-y-1 text-[13px]">
                  <dt class="text-slate-500">Status HTTP</dt><dd class="text-slate-200">{{ ev.http_status || '—' }}</dd>
                  <dt class="text-slate-500">IP server</dt><dd class="mono text-slate-200">{{ ev.server_ip || '—' }}</dd>
                  <dt class="text-slate-500">URL akhir</dt><dd class="mono text-slate-300 break-all">{{ ev.final_url }}</dd>
                  <dt v-if="ev.redirects?.length" class="text-slate-500">Redirect</dt>
                  <dd v-if="ev.redirects?.length" class="mono text-slate-400 break-all">{{ ev.redirects.join(' → ') }}</dd>
                  <dt class="text-slate-500">Diambil oleh</dt><dd class="text-slate-200">{{ ev.requested_by }}</dd>
                  <dt class="text-slate-500">Hash manifest</dt>
                  <dd class="flex items-start gap-1 min-w-0">
                    <span class="mono text-slate-400 break-all text-xs">{{ ev.manifest_sha256 }}</span>
                    <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copyText(ev.manifest_sha256)" />
                  </dd>
                </dl>
                <details class="text-[13px]">
                  <summary class="cursor-pointer text-slate-400 hover:text-slate-200">{{ ev.files.length }} file</summary>
                  <ul class="mt-1.5 space-y-1">
                    <li v-for="f in ev.files" :key="f.name" class="flex items-center gap-2">
                      <UIcon :name="evFileIcon(f.kind)" class="w-3.5 h-3.5 text-slate-500 shrink-0" />
                      <a :href="evFileURL(ev, f.name)" target="_blank" rel="noopener" class="text-slate-200 hover:text-primary-300">{{ f.name }}</a>
                      <span class="text-slate-500 tabular-nums">{{ fmtSize(f.size_bytes) }}</span>
                      <span v-if="evVerify[ev.id]" class="ml-auto">
                        <UIcon :name="evVerify[ev.id].files.find((x: any) => x.name === f.name)?.ok ? 'i-lucide-check' : 'i-lucide-x'"
                          class="w-3.5 h-3.5" :class="evVerify[ev.id].files.find((x: any) => x.name === f.name)?.ok ? 'text-emerald-400' : 'text-rose-400'" />
                      </span>
                    </li>
                  </ul>
                </details>
              </template>
            </div>

            <div v-if="ev.status === 'done'" class="flex gap-2 px-4 pb-4">
              <UButton size="sm" color="gray" variant="soft" icon="i-lucide-shield-check" label="Verifikasi" :loading="evVerifying === ev.id" @click="verifyEvidence(ev)" />
              <UButton size="sm" color="gray" variant="soft" icon="i-lucide-smartphone" :to="evFileURL(ev, 'screenshot-mobile.png')" target="_blank" />
              <UButton size="sm" color="sky" variant="soft" icon="i-lucide-file-archive" label="Unduh ZIP" class="ml-auto" @click="downloadEvidence(ev)" />
            </div>
          </div>
        </div>
      </UCard>

      <!-- ── Pelacakan IP & hosting ── -->
      <SiteInfraCard v-if="tab === 'infra'" :site-id="site.id" />

      <SiteTimeline v-if="tab === 'riwayat'" :events="timeline" />

      <!-- ── Kartu data (masonry: mengisi ruang tanpa celah) ── -->
      <div class="columns-1 lg:columns-2 2xl:columns-3 gap-5 [&>*]:mb-5 [&>*]:break-inside-avoid">
        <!-- Status pengumpulan data (muncul selama ada proses berjalan) -->
        <UCard v-if="collecting.length" :ui="{ ring: 'ring-1 ring-primary-500/40', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-primary-500/10 ring-primary-500/25"><UIcon name="i-lucide-loader-circle" class="w-[18px] h-[18px] text-primary-400 animate-spin" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Data Sedang Dikumpulkan</h2><p class="card-desc">Card terkait akan muncul atau diperbarui otomatis begitu datanya masuk.</p></div>
              <UBadge color="primary" variant="subtle">{{ collecting.length }} proses</UBadge>
            </div>
          </template>
          <div class="space-y-2">
            <CollectingNote v-for="c in collecting" :key="c.key" :text="c.text" :sub="c.sub" />
          </div>
        </UCard>

        <UCard v-for="sec in sections.filter(x => x.tab === tab)" :key="sec.key" :ui="{ ring: sec.accent ?? 'ring-1 ring-slate-800', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-slate-800/70 ring-slate-700/60"><UIcon :name="sec.icon" class="w-[18px] h-[18px]" :class="sec.color" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">{{ sec.title }}</h2><p v-if="sec.desc" class="card-desc">{{ sec.desc }}</p></div>
              <UBadge color="gray" variant="subtle">{{ sec.items.length }}</UBadge>
            </div>
          </template>
          <AttrList :items="sec.items" empty="" :max="sec.max" :confidence="confidenceOf" @lookup="openLookup" />
        </UCard>

        <!-- Pesan sambutan widget chat -->
        <UCard v-if="inTab('kontak') && (site.widget_links?.length)" :ui="{ ring: 'ring-1 ring-emerald-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-emerald-500/10 ring-emerald-500/25"><UIcon name="i-lucide-message-square-text" class="w-[18px] h-[18px] text-emerald-400" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Link dari Widget Live Chat</h2><p class="card-desc">Tombol di pesan sambutan CS (LiveChat dll) — sering berisi link alternatif, APK & Telegram resmi</p></div>
              <UBadge color="emerald" variant="subtle">{{ site.widget_links.length }}</UBadge>
            </div>
          </template>
          <ul class="divide-y divide-slate-800/60 -my-1.5">
            <li v-for="u in site.widget_links" :key="u" class="flex items-start gap-2 py-2">
              <UIcon :name="widgetIcon(u)" class="w-4 h-4 mt-0.5 shrink-0 text-slate-400" />
              <a :href="u" target="_blank" rel="noopener" class="flex-1 mono text-[15px] text-slate-100 hover:text-primary-300 break-words [overflow-wrap:anywhere]">{{ u }}</a>
              <UTooltip text="Salin"><UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copyText(u)" /></UTooltip>
              <UTooltip v-if="!/\.apk|t\.me|wa\.me/i.test(u)" text="Analisis"><UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-scan-line" :to="`/discovery?url=${encodeURIComponent(u)}`" /></UTooltip>
            </li>
          </ul>
        </UCard>

        <!-- Metode pembayaran (dari dropdown form daftar) -->
        <CryptoTraceCard v-if="inTab('keuangan') && wallets.length" :wallets="wallets" :busy="cryptoBusy" @refresh="loadCrypto(String(route.params.id), true)" />

        <UCard v-if="inTab('keuangan') && (payments.total)" :ui="{ ring: 'ring-1 ring-violet-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-violet-500/10 ring-violet-500/25"><UIcon name="i-lucide-landmark" class="w-[18px] h-[18px] text-violet-400" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Metode Pembayaran Diterima</h2><p class="card-desc">Dari pilihan bank/e-wallet di form pendaftaran</p></div>
              <UBadge color="violet" variant="subtle">{{ payments.total }}</UBadge>
            </div>
          </template>
          <div class="space-y-3">
            <div v-for="g in payments.groups" :key="g.cat">
              <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-1.5 flex items-center gap-1.5">
                <UIcon :name="g.icon" class="w-3.5 h-3.5" />{{ g.cat }} <span class="text-slate-600">· {{ g.items.length }}</span>
              </p>
              <div class="flex flex-wrap gap-1.5">
                <UTooltip v-for="it in g.items" :key="it.name" :text="`Cari situs lain yang mencantumkan rekening ${it.name}`" :prevent="g.cat !== 'Bank' && g.cat !== 'E-Wallet'">
                  <UButton size="xs" :color="(g.color as any)" variant="soft" :label="it.name"
                    :to="g.cat === 'Bank' || g.cat === 'E-Wallet' ? `/reverse-lookup?type=${g.cat === 'Bank' ? 'Bank' : 'Contact'}&v=${encodeURIComponent(it.name)}` : undefined" />
                </UTooltip>
              </div>
            </div>
          </div>
        </UCard>

        <!-- Identitas terkait (data bocoran) -->
        <UCard v-if="inTab('kontak') && (identities.length)" :ui="{ ring: 'ring-1 ring-cyan-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <CollectingNote v-if="Object.values(vflRunning).some(Boolean)" class="mb-3"
            text="Pengecekan identitas berantai sedang berjalan…" sub="Nomor → NIK → nomor lain, jeda 30 detik antar kueri (±1–3 menit)." />
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-cyan-500/10 ring-cyan-500/25">
                <UIcon name="i-lucide-user-search" class="w-[18px] h-[18px] text-cyan-400" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Identitas Terkait</h2>
                <p class="card-desc">Pemilik nomor kontak dari data registrasi seluler & kependudukan</p>
              </div>
              <UBadge color="cyan" variant="subtle">{{ identities.length }} nomor</UBadge>
            </div>
          </template>
          <div class="divide-y divide-slate-800/70 -my-3">
            <div v-for="it in identities" :key="it.artifact_id" class="py-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="flex items-center gap-2 min-w-0">
                  <UIcon name="i-lucide-phone" class="w-4 h-4 text-slate-500 shrink-0" />
                  <span class="mono text-sm text-slate-100">{{ it.phone }}</span>
                  <UBadge v-if="it.label" color="gray" variant="subtle" size="xs">{{ it.label }}</UBadge>
                </div>
                <UButton size="2xs" :color="it.kominfo?.length ? 'gray' : 'cyan'" variant="soft" icon="i-lucide-link"
                  :loading="vflRunning[it.artifact_id]" :label="it.kominfo?.length ? 'Cek ulang' : 'Cek Identitas'" @click="checkIdentity(it)" />
              </div>
              <div v-if="!it.kominfo?.length && !it.dukcapil?.length" class="text-[13px] text-slate-500 mt-1.5 pl-6">
                {{ it.checked ? 'Sudah dicek — tidak ada data cocok.' : 'Belum dicek. Klik "Cek Identitas".' }}
              </div>
              <div v-for="(k, ki) in it.kominfo" :key="'k' + ki" class="mt-2 pl-6 text-sm grid grid-cols-2 gap-x-4 gap-y-1">
                <div class="col-span-2 flex items-center gap-2">
                  <UIcon name="i-lucide-id-card" class="w-3.5 h-3.5 text-slate-500" />
                  <span class="mono text-slate-200">{{ k.nik }}</span>
                  <UBadge color="gray" variant="subtle" size="xs">{{ k.source }}</UBadge>
                </div>
                <span class="text-slate-400">{{ k.provider }}</span>
                <span class="mono text-slate-500">reg {{ k.reg_date }}</span>
              </div>
              <div v-for="(d, di) in it.dukcapil" :key="'d' + di" class="mt-2 pl-6 text-sm rounded-lg bg-slate-950/50 ring-1 ring-slate-800 p-3">
                <p class="font-semibold text-white capitalize">{{ d.full_name }}</p>
                <p class="text-slate-400 mono text-[13px] mt-0.5">NIK {{ d.nik }} · {{ d.gender }} · {{ d.birth_date }}</p>
                <p class="text-slate-300 text-[13px] mt-1 capitalize leading-relaxed">{{ [d.address, d.district, d.location, d.region].filter(Boolean).join(', ') }}</p>
              </div>
              <div v-if="it.related?.length" class="mt-2 pl-6">
                <p class="text-[13px] text-slate-500 mb-1">Nomor lain dengan NIK sama · {{ it.related.length }}</p>
                <div class="flex flex-wrap gap-1.5">
                  <NuxtLink v-for="(r, ri) in it.related" :key="'r' + ri" :to="`/leak?q=${r.phone}`"
                    class="mono text-[13px] text-cyan-400 hover:underline bg-slate-950/50 ring-1 ring-slate-800 rounded px-2 py-0.5">{{ r.phone }}</NuxtLink>
                </div>
              </div>
            </div>
          </div>
        </UCard>

        <!-- Merchant QRIS -->
        <UCard v-if="inTab('keuangan') && (qrisMerchants.length)" :ui="{ ring: 'ring-1 ring-violet-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-violet-500/10 ring-violet-500/25">
                <UIcon name="i-lucide-qr-code" class="w-[18px] h-[18px] text-violet-400" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Merchant QRIS</h2>
                <p class="card-desc">Penampung dana dari QR pembayaran — nama & PJSP penerbit</p>
              </div>
              <UBadge color="violet" variant="subtle">{{ qrisMerchants.length }}</UBadge>
            </div>
          </template>
          <div class="space-y-2.5">
            <div v-for="q in qrisMerchants" :key="q.id" class="rounded-lg bg-slate-950/50 ring-1 ring-slate-800 p-3 flex items-start gap-3">
              <UIcon name="i-lucide-store" class="w-4 h-4 text-violet-400 shrink-0 mt-0.5" />
              <div class="min-w-0 flex-1">
                <p v-if="q.label" class="text-sm font-medium text-white">{{ q.label }}</p>
                <p class="mono text-[13px] text-slate-400 break-words [overflow-wrap:anywhere]">{{ q.value }}</p>
              </div>
              <UTooltip text="Cari situs lain dengan merchant QRIS yang sama">
                <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-scan-search"
                  :to="`/reverse-lookup?type=QRIS&v=${encodeURIComponent(q.value)}`" />
              </UTooltip>
            </div>
          </div>
        </UCard>

        <!-- Provider game -->
        <UCard v-if="inTab('konten') && (gameProviders.length)" :ui="{ ring: 'ring-1 ring-fuchsia-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-fuchsia-500/10 ring-fuchsia-500/25">
                <UIcon name="i-lucide-dice-5" class="w-[18px] h-[18px] text-fuchsia-400" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Provider Game</h2>
                <p class="card-desc">Penyedia game yang dimuat situs — penanda jaringan aktor</p>
              </div>
              <UBadge color="fuchsia" variant="subtle">{{ gameProviders.length }}</UBadge>
            </div>
          </template>
          <div class="flex flex-wrap gap-1.5">
            <UBadge v-for="g in gameProviders" :key="g" color="fuchsia" variant="subtle" size="sm">{{ g }}</UBadge>
          </div>
        </UCard>

        <!-- Login & pendaftaran -->
        <UCard v-if="inTab('konten') && (auth)" :ui="{ ring: 'ring-1 ring-amber-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-key-round" class="w-[18px] h-[18px] text-amber-400" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Login & Pendaftaran</h2><p class="card-desc">Dipetakan pasif dari HTML — form tidak pernah diisi atau dikirim</p></div>
              <UBadge color="gray" variant="subtle">{{ (auth.forms ?? []).length }} form</UBadge>
            </div>
          </template>

          <div class="space-y-3">
            <div v-for="grp in authLinks" :key="grp.label">
              <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-1">{{ grp.label }}</p>
              <div v-for="u in grp.urls" :key="u" class="flex items-start gap-2 py-0.5">
                <a :href="u" target="_blank" rel="noopener" class="flex-1 mono text-[15px] text-slate-200 hover:text-primary-300 break-words [overflow-wrap:anywhere]">{{ u }}</a>
                <UTooltip text="Salin"><UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copyText(u)" /></UTooltip>
                <UTooltip text="Analisis"><UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-scan-line" :to="`/discovery?url=${encodeURIComponent(u)}`" /></UTooltip>
              </div>
            </div>

            <div v-if="auth.referral_codes?.length">
              <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-1">Kode referral / afiliasi</p>
              <div class="flex flex-wrap gap-1.5">
                <UButton v-for="r in auth.referral_codes" :key="r" size="xs" color="amber" variant="soft" icon="i-lucide-scan-search" :label="r"
                  :to="`/reverse-lookup?type=FP&v=${encodeURIComponent(r.toLowerCase())}`" />
              </div>
              <p class="text-[13px] text-slate-500 mt-1">Klik untuk mencari situs lain dengan kode afiliasi yang sama</p>
            </div>

            <div v-for="(f, i) in auth.forms ?? []" :key="i" class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 p-3 space-y-2">
              <div class="flex flex-wrap items-center gap-1.5">
                <UBadge :color="f.type === 'register' ? 'rose' : f.type === 'login' ? 'primary' : 'gray'" variant="subtle" size="xs">
                  {{ ({ register: 'Form Daftar', login: 'Form Login', change_password: 'Ganti Password' } as any)[f.type] ?? f.type }}
                </UBadge>
                <UTooltip v-if="f.seen_on?.length" :text="f.seen_on.join('\n')">
                  <UBadge color="gray" variant="outline" size="xs">juga di {{ f.seen_on.length }} halaman lain</UBadge>
                </UTooltip>
                <UBadge color="gray" variant="outline" size="xs">{{ f.method }}</UBadge>
                <UBadge v-if="f.external_action" color="amber" variant="subtle" size="xs">submit ke domain lain</UBadge>
                <UBadge v-if="f.has_captcha" color="gray" variant="subtle" size="xs">captcha</UBadge>
                <UBadge v-if="f.has_otp" color="violet" variant="subtle" size="xs">OTP</UBadge>
                <UBadge v-if="f.needs_phone" color="emerald" variant="subtle" size="xs">wajib no. HP/WA</UBadge>
                <UBadge v-if="f.needs_bank" color="cyan" variant="subtle" size="xs">minta rekening</UBadge>
              </div>
              <p class="text-sm text-slate-400">Endpoint: <span class="mono text-slate-200 break-words [overflow-wrap:anywhere]">{{ f.action }}</span></p>
              <p v-if="f.page_url && !f.page_url.includes('#')" class="text-sm text-slate-500">Halaman: <span class="mono break-words [overflow-wrap:anywhere]">{{ f.page_url }}</span></p>
              <div class="flex flex-wrap gap-1">
                <span v-for="fl in f.fields" :key="fl" class="px-1.5 py-0.5 rounded bg-slate-800 text-[13px] text-slate-300">{{ fl }}</span>
              </div>
              <div v-if="f.bank_options?.length" class="flex flex-wrap items-center gap-1">
                <span class="text-[13px] text-slate-500 mr-1">Bank/e-wallet:</span>
                <UBadge v-for="b in f.bank_options" :key="b" color="violet" variant="subtle" size="xs">{{ b }}</UBadge>
              </div>
            </div>
            <p v-if="!(auth.forms ?? []).length" class="muted">Form tidak terbaca — kemungkinan dimuat lewat JavaScript atau berada di domain lain.</p>
          </div>
        </UCard>

        <!-- Telegram -->
        <UCard v-if="inTab('kontak') && (tgAccounts.length)" :ui="{ ring: 'ring-1 ring-sky-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-sky-500/10 ring-sky-500/25"><UIcon name="i-lucide-send" class="w-[18px] h-[18px] text-sky-400" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Akun Telegram</h2><p class="card-desc">Dicek lewat Telegram (hanya info publik — tanpa mengirim pesan atau bergabung)</p></div>
              <UBadge color="emerald" variant="subtle">{{ tgAccounts.filter(a => a.status === 'valid').length }} valid</UBadge>
              <UBadge v-if="tgAccounts.some(a => a.status === 'not_found')" color="rose" variant="subtle" size="xs">{{ tgAccounts.filter(a => a.status === 'not_found').length }} tidak ditemukan</UBadge>
            </div>
          </template>
          <div class="space-y-3">
            <div v-for="a in tgAccounts" :key="a.username" class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 p-3">
              <div class="flex flex-wrap items-center gap-2">
                <a :href="`https://t.me/${a.username}`" target="_blank" rel="noopener" class="mono text-[15px] text-slate-100 hover:text-sky-300">@{{ a.username }}</a>
                <UBadge :color="(tgStatus[a.status].color as any)" variant="subtle" size="xs"><UIcon :name="tgStatus[a.status].icon" class="w-3.5 h-3.5 mr-1" />{{ tgStatus[a.status].label }}</UBadge>
                <UBadge v-if="a.status === 'valid'" color="sky" variant="outline" size="xs">{{ tgTypes[a.type] ?? a.type }}</UBadge>
                <UBadge v-if="a.site_count > 1" color="rose" variant="subtle" size="xs">dipakai {{ a.site_count }} situs</UBadge>
              </div>
              <template v-if="a.status === 'valid'">
                <p class="text-[15px] text-white mt-1.5">{{ a.name }}
                  <span v-if="a.members" class="text-sm text-slate-400"> · {{ a.members.toLocaleString('id-ID') }} anggota</span></p>
                <p v-if="a.bio" class="text-sm text-slate-400 mt-1 break-words">{{ a.bio }}</p>
                <div v-if="a.type === 'channel' || a.type === 'group'" class="flex flex-wrap items-center gap-x-3 gap-y-1 mt-2 text-[13px] text-slate-500">
                  <span>{{ a.message_count }} pesan terbaca</span>
                  <span v-if="a.last_message_at">pesan terakhir {{ fmtDate(a.last_message_at) }}</span>
                  <span v-if="a.artifact_count">{{ a.artifact_count }} artefak dari pesan</span>
                  <span v-if="a.last_scraped_at">dibaca {{ fmtDate(a.last_scraped_at) }}</span>
                </div>
                <div v-if="a.type === 'channel' || a.type === 'group'" class="flex gap-1.5 mt-2">
                  <UButton size="xs" color="gray" variant="soft" icon="i-lucide-messages-square" :label="tgOpen === a.username ? 'Tutup pesan' : 'Lihat pesan'"
                    :disabled="!a.message_count" @click="toggleTgMessages(a.username)" />
                  <UButton size="xs" color="sky" variant="soft" icon="i-lucide-refresh-cw" label="Baca sekarang" :loading="tgScraping === a.username" @click="scrapeTg(a.username)" />
                </div>
                <div v-if="tgOpen === a.username" class="mt-2 max-h-80 overflow-y-auto space-y-2">
                  <div v-for="m in tgMessages" :key="m.message_id" class="rounded bg-slate-900 p-2">
                    <p class="text-[13px] text-slate-500">{{ fmtDate(m.date) }}</p>
                    <p class="text-sm text-slate-200 whitespace-pre-line break-words">{{ m.text }}</p>
                  </div>
                </div>
              </template>
              <p v-else-if="a.status === 'not_found'" class="text-sm text-slate-400 mt-1">Username tidak terdaftar di Telegram — kemungkinan sudah dihapus/diganti operator.</p>
              <p v-else class="text-sm text-slate-400 mt-1">Belum dicek — diproses otomatis setiap 2 jam{{ a.last_error ? ` (terakhir: ${a.last_error})` : '' }}.</p>
            </div>
          </div>
        </UCard>

        <!-- Klaster operator -->
        <UCard v-if="inTab('jaringan')" v-for="op in clusters" :key="op.id" :ui="{ ring: 'ring-1 ring-rose-500/40', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-rose-500/10 ring-rose-500/25">
                <UIcon name="i-lucide-network" class="w-[18px] h-[18px] text-rose-400" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Klaster Operator</h2>
                <p class="card-desc">Berbagi atribut kuat — kemungkinan satu operator</p>
              </div>
              <UBadge color="rose" variant="subtle">{{ op.site_count }} situs</UBadge>
            </div>
          </template>
          <div class="rounded-lg bg-rose-500/5 ring-1 ring-rose-500/15 p-4 mb-4">
            <p class="text-[13px] text-slate-500">Nama klaster</p>
            <p class="text-[15px] text-white font-semibold mt-0.5 break-words [overflow-wrap:anywhere]">{{ op.name }}</p>
          </div>
          <OperatorProfile :op="op" class="mb-4" />
          <p class="text-[13px] uppercase tracking-wider text-slate-500 mb-2">Bukti penghubung</p>
          <ul class="space-y-2 mb-4">
            <li v-for="e in op.evidence ?? []" :key="e.attr" class="flex items-start gap-2.5 text-sm">
              <UBadge color="gray" variant="subtle" size="xs" class="shrink-0 tabular-nums mt-0.5">{{ e.sites }} situs</UBadge>
              <span class="mono text-slate-200 break-words [overflow-wrap:anywhere]">{{ e.attr }}</span>
            </li>
          </ul>
          <details class="group/op rounded-lg bg-slate-950/50 ring-1 ring-slate-800">
            <summary class="cursor-pointer select-none text-sm text-primary-300 hover:text-primary-200 list-none flex items-center gap-2 px-4 py-2.5.5">
              <UIcon name="i-lucide-chevron-right" class="w-4 h-4 transition-transform group-open/op:rotate-90" />
              Lihat {{ op.sites?.length ?? 0 }} anggota klaster
            </summary>
            <div class="max-h-64 overflow-y-auto divide-y divide-slate-800/60 border-t border-slate-800">
              <a v-for="u in op.sites ?? []" :key="u" :href="`/discovery?url=${encodeURIComponent(u)}`"
                class="block text-sm text-slate-300 hover:text-primary-300 hover:bg-slate-800/40 break-words px-4 py-2.5 [overflow-wrap:anywhere]">{{ stripScheme(u) }}</a>
            </div>
          </details>
        </UCard>

        <ReverseAnalyticsCard v-if="inTab('jaringan') && revResults.length" :results="revResults" :busy="revBusy" @refresh="refreshReverse" />

        <!-- Analisis APK -->
        <UCard v-if="inTab('konten')" v-for="ap in apks" :key="ap.url" :ui="{ ring: 'ring-1 ring-emerald-500/30', header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-emerald-500/10 ring-emerald-500/25"><UIcon name="i-lucide-package-search" class="w-[18px] h-[18px] text-emerald-400" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Analisis APK</h2>
                <p class="card-desc">Bedah statis aplikasi — sertifikat developer, izin & domain di dalam kode</p></div>
              <UBadge v-if="ap.size_bytes" color="gray" variant="subtle">{{ (ap.size_bytes / 1048576).toFixed(1) }} MB</UBadge>
            </div>
          </template>
          <div class="mb-3 pb-3 border-b border-slate-800 text-sm">
            <p class="text-[13px] text-slate-500">File APK</p>
            <div class="flex items-start gap-1.5">
              <a :href="ap.url" target="_blank" rel="noopener" class="flex-1 min-w-0 mono text-[13px] text-primary-300 hover:underline break-all">{{ ap.url }}</a>
              <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copyText(ap.url)" />
            </div>
          </div>
          <p v-if="ap.error && !ap.package_name" class="text-sm text-rose-400">Gagal dianalisis: {{ ap.error }}</p>
          <dl v-else class="grid grid-cols-2 gap-x-4 gap-y-2.5 text-sm">
            <div class="col-span-2"><dt class="text-[13px] text-slate-500">Nama paket</dt><dd class="mono text-slate-100 break-words">{{ ap.package_name || '—' }}</dd></div>
            <div><dt class="text-[13px] text-slate-500">Label</dt><dd class="text-slate-100">{{ ap.app_label || '—' }}</dd></div>
            <div><dt class="text-[13px] text-slate-500">Versi</dt><dd class="text-slate-100">{{ ap.version_name || '—' }}</dd></div>
            <div class="col-span-2">
              <dt class="text-[13px] text-slate-500">Penanda tangan (developer)</dt>
              <dd class="mono text-slate-100 break-words [overflow-wrap:anywhere]">{{ ap.cert_subject || '—' }}</dd>
              <dd v-if="ap.cert_not_before" class="text-[13px] text-slate-500">sertifikat dibuat {{ fmtDate(ap.cert_not_before) }}</dd>
            </div>
            <div class="col-span-2">
              <dt class="text-[13px] text-slate-500">SHA-256 sertifikat</dt>
              <dd class="flex items-start gap-2">
                <span class="mono text-[13px] text-slate-300 break-all flex-1">{{ ap.cert_sha256 || '—' }}</span>
                <UButton v-if="ap.cert_sha256" size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" @click="copyText(ap.cert_sha256)" />
              </dd>
            </div>
          </dl>
          <UButton v-if="ap.cert_sha256" class="mt-3" block size="sm" :color="ap.same_cert_sites ? 'rose' : 'gray'" variant="soft" icon="i-lucide-scan-search"
            :label="ap.same_cert_sites ? `${ap.same_cert_sites} situs lain memakai APK dari developer yang sama` : 'Cari situs lain dengan sertifikat yang sama'"
            :to="`/reverse-lookup?type=FP&v=${ap.cert_sha256.slice(0, 32)}`" />
          <div v-if="riskyPerms(ap.permissions).length" class="mt-3">
            <p class="text-[13px] text-slate-500 mb-1">Izin berisiko ({{ ap.permissions.length }} izin total)</p>
            <div class="flex flex-wrap gap-1"><UBadge v-for="p in riskyPerms(ap.permissions)" :key="p" color="amber" variant="subtle" size="xs">{{ p }}</UBadge></div>
          </div>
          <div v-if="ap.domains?.length" class="mt-3">
            <p class="text-[13px] text-slate-500 mb-1">Domain di dalam kode</p>
            <div class="flex flex-wrap gap-1"><UBadge v-for="d in ap.domains" :key="d" color="cyan" variant="subtle" size="xs" class="mono">{{ d }}</UBadge></div>
          </div>
        </UCard>

        <!-- Perubahan antar scan -->
        <UCard v-if="inTab('riwayat') && (changes.length)" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-amber-500/10 ring-amber-500/25">
                <UIcon name="i-lucide-git-compare" class="w-[18px] h-[18px] text-amber-400" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Perubahan Antar Scan</h2>
                <p class="card-desc">Rotasi rekening, kontak, domain daftar & aplikasi</p>
              </div>
              <UBadge color="gray" variant="subtle">{{ changes.length }}</UBadge>
            </div>
          </template>
          <div class="max-h-[420px] overflow-y-auto divide-y divide-slate-800/70 -my-3">
            <div v-for="ch in changes" :key="ch.scan_id" class="py-3">
              <p class="text-[13px] font-medium text-slate-400 mb-2">{{ fmtDate(ch.at) }}</p>
              <div class="space-y-1.5">
                <div v-for="(vals, cat) in ch.added" :key="'a' + cat" class="flex items-start gap-2 text-sm">
                  <UBadge color="emerald" variant="subtle" size="xs" class="shrink-0 mt-0.5">+ {{ cat }}</UBadge>
                  <span class="mono text-slate-300 break-words [overflow-wrap:anywhere]">{{ vals.join(' · ') }}</span>
                </div>
                <div v-for="(vals, cat) in ch.removed" :key="'r' + cat" class="flex items-start gap-2 text-sm">
                  <UBadge color="rose" variant="subtle" size="xs" class="shrink-0 mt-0.5">− {{ cat }}</UBadge>
                  <span class="mono text-slate-500 line-through break-words [overflow-wrap:anywhere]">{{ vals.join(' · ') }}</span>
                </div>
              </div>
            </div>
          </div>
        </UCard>

        <!-- Arsip Wayback -->
        <UCard v-if="inTab('riwayat')" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-slate-700/40 ring-slate-600/40"><UIcon name="i-lucide-archive" class="w-[18px] h-[18px] text-slate-300" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Arsip Wayback Machine</h2><p class="card-desc">Snapshot lama situs — rekening & kontak yang sudah dihapus masih bisa terlihat di sini</p></div>
              <UButton size="xs" color="primary" variant="soft" icon="i-lucide-history" :loading="waybackLoading"
                :label="wayback.length ? 'Perbarui' : 'Ambil Arsip'" @click="fetchWayback" />
            </div>
          </template>
          <p v-if="!wayback.length" class="muted">{{ waybackLoading ? 'Mengambil snapshot dari archive.org (±30–60 detik)...' : 'Belum diambil. Klik "Ambil Arsip".' }}</p>
          <div v-else class="space-y-2.5 max-h-[420px] overflow-y-auto">
            <div v-for="w in wayback" :key="w.timestamp" class="rounded-lg bg-slate-950/60 ring-1 ring-slate-800 p-3">
              <div class="flex items-center justify-between gap-2">
                <a :href="w.url" target="_blank" rel="noopener" class="text-sm font-medium text-primary-300 hover:underline">{{ fmtWayback(w.timestamp) }}</a>
                <span class="text-[13px] text-slate-500">{{ (w.artifacts ?? []).length }} artefak · {{ (w.tracking ?? []).length }} tracking</span>
              </div>
              <div v-if="(w.artifacts ?? []).length" class="flex flex-wrap gap-1 mt-2">
                <UBadge v-for="a in w.artifacts" :key="a.type + a.value" color="violet" variant="subtle" size="xs" class="mono">{{ a.name || a.type }}: {{ a.value }}</UBadge>
              </div>
              <p v-if="(w.register ?? []).length" class="text-[13px] text-slate-400 mt-1.5 break-words [overflow-wrap:anywhere]">Daftar → {{ w.register.join(' · ') }}</p>
            </div>
          </div>
        </UCard>

        <!-- WHOIS -->
        <UCard v-if="inTab('infra') && (detail.whois)" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-amber-500/10 ring-amber-500/25">
                <UIcon name="i-lucide-file-search" class="w-[18px] h-[18px] text-amber-400" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">WHOIS</h2>
                <p class="card-desc truncate">{{ whois.registrar || 'Registrar tidak diketahui' }}</p>
              </div>
              <UBadge :color="whois.is_private ? 'amber' : 'emerald'" variant="subtle">
                <UIcon :name="whois.is_private ? 'i-lucide-eye-off' : 'i-lucide-eye'" class="w-3.5 h-3.5 mr-1" />{{ whois.is_private ? 'Privat' : 'Publik' }}
              </UBadge>
            </div>
          </template>

          <!-- Ringkasan: umur & risiko -->
          <div class="grid grid-cols-2 divide-x divide-slate-800 pb-4 border-b border-slate-800">
            <div class="pr-4">
              <p class="text-[13px] text-slate-500">Umur domain</p>
              <p class="text-2xl font-semibold tabular-nums mt-0.5" :class="whoisAge.color">{{ whoisAge.value }}</p>
              <p class="text-xs mt-0.5" :class="whoisAge.color">{{ whoisAge.note }}</p>
            </div>
            <div class="pl-4">
              <p class="text-[13px] text-slate-500">Skor risiko domain</p>
              <p class="text-2xl font-semibold tabular-nums mt-0.5" :class="whoisRisk.color">{{ whoisRisk.value }}</p>
              <UProgress :value="whoisRisk.pct" size="xs" :color="(whoisRisk.bar as any)" class="mt-2" />
            </div>
          </div>

          <!-- Linimasa masa aktif -->
          <div v-if="whois.registered_at || whois.expires_at" class="py-4 border-b border-slate-800">
            <div class="flex justify-between text-[13px]">
              <div><p class="text-slate-500">Terdaftar</p><p class="text-slate-200 font-medium">{{ fmtDay(whois.registered_at) }}</p></div>
              <div class="text-right"><p class="text-slate-500">Kedaluwarsa</p><p class="font-medium" :class="whoisLife.soon ? 'text-amber-300' : 'text-slate-200'">{{ fmtDay(whois.expires_at) }}</p></div>
            </div>
            <div v-if="whoisLife.pct != null" class="relative h-2 rounded-full bg-slate-800 mt-3">
              <div class="absolute inset-y-0 left-0 rounded-full bg-gradient-to-r from-amber-500/70 to-amber-400" :style="{ width: whoisLife.pct + '%' }" />
              <div class="absolute -top-1 w-0.5 h-4 bg-white rounded" :style="{ left: `calc(${whoisLife.pct}% - 1px)` }" />
            </div>
            <p v-if="whoisLife.left != null" class="text-xs mt-2" :class="whoisLife.soon ? 'text-amber-300' : 'text-slate-500'">
              {{ whoisLife.left >= 0 ? `Berakhir ${whoisLife.left} hari lagi` : `Sudah lewat ${-whoisLife.left} hari` }}{{ whoisLife.soon ? ' — operator mungkin akan pindah domain' : '' }}
            </p>
          </div>

          <!-- Detail pendaftar -->
          <dl class="divide-y divide-slate-800/70 -mb-2">
            <div v-for="r in whoisDetails" :key="r.k" class="group/w flex items-start gap-3 py-2">
              <UIcon :name="r.icon" class="w-4 h-4 text-slate-500 shrink-0 mt-0.5" />
              <dt class="w-24 shrink-0 text-[13px] text-slate-500">{{ r.k }}</dt>
              <dd class="flex-1 min-w-0 text-sm text-slate-100 break-words [overflow-wrap:anywhere]" :class="r.mono && 'mono'">{{ r.v }}</dd>
              <UButton size="2xs" color="gray" variant="ghost" icon="i-lucide-copy" class="opacity-0 group-hover/w:opacity-100" @click="copyText(r.v)" />
            </div>
          </dl>
          <p v-if="whois.is_private" class="mt-2 pt-3 text-xs text-slate-500 border-t border-slate-800">
            Data pendaftar disembunyikan layanan privasi WHOIS — umum dipakai operator judol untuk menutupi identitas.
          </p>
        </UCard>

        <!-- Reputasi & promosi -->
        <UCard v-if="inTab('konten')" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3"><div class="card-icon bg-rose-500/10 ring-rose-500/25"><UIcon name="i-lucide-shield-question" class="w-[18px] h-[18px] text-rose-400" /></div><div class="flex-1 min-w-0"><h2 class="card-title">Reputasi</h2><p class="card-desc">Penilaian keamanan dari VirusTotal & URLScan</p></div>
              <UButton size="xs" color="primary" variant="soft" icon="i-lucide-sparkles" :loading="enriching"
                :label="detail.enrichment.virustotal || detail.enrichment.urlscan ? 'Perbarui' : 'Perkaya Sekarang'" @click="enrichNow" /></div>
          </template>
          <CollectingNote v-if="enriching" class="mb-3" text="Mengambil data URLScan & VirusTotal…" sub="Biasanya 30–60 detik; card diperbarui otomatis." />
          <div class="space-y-2.5 text-sm">
            <div class="flex items-center justify-between gap-3">
              <span class="text-slate-400">VirusTotal</span>
              <span v-if="detail.enrichment.virustotal" class="text-right">
                <b :class="detail.enrichment.virustotal.malicious ? 'text-rose-400' : 'text-emerald-400'">{{ detail.enrichment.virustotal.malicious }}</b>
                <span class="text-slate-400"> / {{ detail.enrichment.virustotal.total }} engine berbahaya</span>
              </span>
              <span v-else class="text-slate-500">belum ada data</span>
            </div>
            <div class="flex items-center justify-between gap-3">
              <span class="text-slate-400">URLScan</span>
              <span v-if="detail.enrichment.urlscan" class="text-slate-200">{{ detail.enrichment.urlscan.verdict || '—' }} · skor {{ detail.enrichment.urlscan.score }}</span>
              <span v-else class="text-slate-500">belum ada data</span>
            </div>
            <a v-if="detail.enrichment.urlscan?.screenshot_url" :href="detail.enrichment.urlscan.screenshot_url" target="_blank" rel="noopener"
              class="block rounded-lg overflow-hidden ring-1 ring-slate-800 hover:ring-primary-500/50">
              <img :src="detail.enrichment.urlscan.screenshot_url" alt="Screenshot URLScan" loading="lazy" class="w-full h-auto" />
              <p class="text-[13px] text-slate-500 px-2 py-1 bg-slate-950/60">Screenshot dari URLScan.io — tanpa membuka situs dari perangkatmu</p>
            </a>
            <div class="flex items-center justify-between gap-3">
              <span class="text-slate-400">Gambar tercatat</span><span class="text-slate-200">{{ detail.image_count }}</span>
            </div>
            <UAlert v-if="site.app_promo && !byKind('app').length" icon="i-lucide-smartphone" color="amber" variant="subtle" class="mt-1"
              title="Promosi aplikasi tanpa link publik"
              :description="`Bukti: ${site.app_promo_evidence.join('; ')}. Kemungkinan dibagikan setelah login atau via CS.`" />
          </div>
        </UCard>

        <!-- Halaman internal -->
        <UCard v-if="inTab('konten') && (site.internal_pages?.length)" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-slate-700/40 ring-slate-600/40">
                <UIcon name="i-lucide-files" class="w-[18px] h-[18px] text-slate-300" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Halaman Internal Dianalisis</h2>
                <p class="card-desc">Path yang dijelajahi crawler di situs ini</p>
              </div>
              <UBadge color="gray" variant="subtle">{{ site.internal_pages.length }}</UBadge>
            </div>
          </template>
          <div class="max-h-[420px] overflow-y-auto divide-y divide-slate-800/60 -my-2">
            <a v-for="p in site.internal_pages" :key="p" :href="p" target="_blank" rel="noopener"
              class="group/ip flex items-center gap-2.5 mono text-sm text-slate-300 hover:text-primary-300 hover:bg-slate-800/40 -mx-2 px-2 rounded py-2 break-words [overflow-wrap:anywhere]">
              <UIcon name="i-lucide-file-text" class="w-4 h-4 shrink-0 text-slate-500 group-hover/ip:text-primary-400" />
              <span class="flex-1 min-w-0">{{ p.replace(/^https?:\/\/[^/]+/, '') || '/' }}</span>
              <UIcon name="i-lucide-external-link" class="w-3.5 h-3.5 shrink-0 text-slate-600 opacity-0 group-hover/ip:opacity-100" />
            </a>
          </div>
        </UCard>

        <!-- Riwayat scan -->
        <UCard v-if="inTab('riwayat')" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3"><div class="card-icon bg-slate-700/40 ring-slate-600/40"><UIcon name="i-lucide-history" class="w-[18px] h-[18px] text-slate-300" /></div><div class="flex-1 min-w-0"><h2 class="card-title">Riwayat Scan</h2><p class="card-desc">Hasil tiap pemindaian & perubahan skor situs</p></div>
              <span class="text-[13px] text-slate-500">rescan otomatis {{ site.rescan_count }}×</span></div>
          </template>
          <ul class="divide-y divide-slate-800/60 -my-1.5">
            <li v-for="sc in detail.scans" :key="sc.id" class="flex items-center gap-3 py-2 text-sm">
              <span class="w-2 h-2 rounded-full shrink-0" :class="sc.status === 'ok' ? 'bg-emerald-500' : 'bg-rose-500'" />
              <span class="text-slate-300 flex-1">{{ fmtDate(sc.created_at) }}</span>
              <span v-if="sc.score != null" class="font-semibold tabular-nums text-slate-100">{{ Math.round(sc.score * 100) }}%</span>
              <span v-else class="text-[13px] text-rose-400">tidak terjangkau</span>
              <PriorityBadge v-if="sc.priority" :priority="sc.priority" />
            </li>
          </ul>
        </UCard>

        <!-- Riwayat keterjangkauan & blokir -->
        <UCard v-if="inTab('riwayat')" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-orange-500/10 ring-orange-500/25"><UIcon name="i-lucide-radio" class="w-[18px] h-[18px] text-orange-400" /></div>
              <div class="flex-1 min-w-0"><h2 class="card-title">Keterjangkauan & Blokir</h2>
                <p class="card-desc">Dicek otomatis tiap ±12 jam: DNS publik, blokir DNS ISP, dan respons server</p></div>
              <UBadge v-if="reachMeta" :color="reachMeta.color" variant="subtle">{{ reachMeta.label }}</UBadge>
            </div>
          </template>
          <p v-if="!reachHistory.length" class="text-sm text-slate-500">Belum pernah dicek — pengecekan berjalan otomatis setelah backend diperbarui.</p>
          <ul v-else class="divide-y divide-slate-800/60 -my-2">
            <li v-for="(e, i) in reachHistory" :key="i" class="py-2 flex items-center gap-3 text-sm">
              <span class="w-2 h-2 rounded-full shrink-0" :class="REACH[e.status]?.dot" />
              <span class="flex-1 text-slate-200">{{ REACH[e.status]?.label ?? e.status }}<span v-if="e.http_status" class="text-slate-500"> · HTTP {{ e.http_status }}</span></span>
              <span class="text-xs text-slate-500">{{ fmtDate(e.at) }}</span>
            </li>
          </ul>
          <p v-if="site.blocked_since" class="text-xs text-slate-500 mt-3 pt-3 border-t border-slate-800">Pertama terdeteksi diblokir {{ fmtDate(site.blocked_since) }}</p>
        </UCard>

        <!-- Log audit situs -->
        <UCard v-if="inTab('riwayat')" :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3"><div class="card-icon bg-slate-700/40 ring-slate-600/40"><UIcon name="i-lucide-scroll-text" class="w-[18px] h-[18px] text-slate-300" /></div><div class="flex-1 min-w-0"><h2 class="card-title">Log Audit Situs</h2><p class="card-desc">Siapa melakukan apa terhadap situs ini — tercatat permanen</p></div>
              <UButton size="2xs" color="gray" variant="ghost" label="Semua log" trailing-icon="i-lucide-arrow-up-right" to="/audit" /></div>
          </template>
          <p v-if="!siteAudit.length" class="text-sm text-slate-500">Belum ada aktivitas tercatat.</p>
          <ul v-else class="divide-y divide-slate-800/60 max-h-[360px] overflow-y-auto -my-2">
            <li v-for="a in siteAudit" :key="a.id" class="py-2 text-sm">
              <div class="flex items-center gap-2">
                <span class="text-slate-200 flex-1 truncate">{{ a.action }}</span>
                <UBadge :color="a.status >= 400 ? 'rose' : 'emerald'" variant="subtle" size="xs">{{ a.status }}</UBadge>
              </div>
              <p class="text-xs text-slate-500">{{ a.actor === 'anonymous' ? 'Tanpa nama' : a.actor }} · {{ fmtDate(a.at) }}</p>
            </li>
          </ul>
        </UCard>
      </div>

      <!-- Konfirmasi ekspor saat data masih dikumpulkan -->
      <UModal v-model="exportAsk" :ui="{ width: 'sm:max-w-md' }">
        <UCard :ui="{ header: { padding: 'px-5 py-4' }, body: { padding: 'px-5 py-4' }, footer: { padding: 'px-5 py-4' } }">
          <template #header>
            <div class="flex items-center gap-3">
              <div class="card-icon bg-amber-500/10 ring-amber-500/25"><UIcon name="i-lucide-hourglass" class="w-[18px] h-[18px] text-amber-400" /></div>
              <div class="flex-1 min-w-0">
                <h2 class="card-title">Analisis belum selesai</h2>
                <p class="card-desc">{{ collecting.length }} proses masih mengumpulkan data untuk situs ini</p>
              </div>
            </div>
          </template>
          <div class="space-y-2">
            <CollectingNote v-for="c in collecting" :key="c.key" :text="c.text" :sub="c.sub" />
          </div>
          <p class="text-[13px] text-slate-400 mt-3">Laporan yang diekspor sekarang tidak memuat hasil proses di atas.</p>
          <template #footer>
            <div class="flex flex-col-reverse sm:flex-row gap-2 sm:justify-end">
              <UButton color="gray" variant="soft" icon="i-lucide-file-down" label="Ekspor yang sudah siap" @click="exportAsk = false; runExport()" />
              <UButton icon="i-lucide-hourglass" label="Tunggu selesai" @click="waitForExport" />
            </div>
          </template>
        </UCard>
      </UModal>

      <!-- Tab tanpa data -->
      <div v-if="!currentTab.count && !currentTab.loading && tab !== 'ringkasan' && tab !== 'bukti' && tab !== 'infra'"
        class="rounded-2xl ring-1 ring-slate-800 bg-slate-900/40 empty-state !py-16">
        <div class="empty-icon"><UIcon :name="currentTab.icon" class="w-6 h-6" /></div>
        <p class="text-base text-slate-300 font-medium">Belum ada data {{ currentTab.label.toLowerCase() }}</p>
        <p class="text-sm max-w-md">{{ currentTab.empty }}</p>
        <div class="flex gap-2 mt-2">
          <UButton size="sm" color="gray" variant="soft" icon="i-lucide-microscope" label="Analisis Lanjutan" :loading="deepLoading" @click="runDeep" />
          <UButton size="sm" icon="i-lucide-scan-line" label="Scan Ulang" :to="`/discovery?url=${encodeURIComponent(site.url)}`" />
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const { get } = useApi()

const detail    = ref<any>(null)
const relations = ref<any>(null)
const loading   = ref(true)
const error     = ref('')
const router    = useRouter()
// Tampilan tersimpan di URL (?view=tree) agar bisa dibagikan
const view      = ref<'graph' | 'tree'>(route.query.view === 'tree' ? 'tree' : 'graph')
watch(view, v => router.replace({ query: { ...route.query, view: v } }))
const showIsolated = ref(false)


const views = [
  { value: 'graph' as const, label: 'Graf', icon: 'i-lucide-waypoints' },
  { value: 'tree' as const,  label: 'Tree', icon: 'i-lucide-list-tree' },
]

const kindMeta: Record<string, { label: string; color: string }> = {
  operator: { label: 'Operator',    color: '#f43f5e' },
  artifact: { label: 'Artefak',     color: '#a855f7' },
  fingerprint: { label: 'Sidik Jari Infra', color: '#ec4899' },
  tracking: { label: 'Tracking ID', color: '#3b82f6' },
  ssl:      { label: 'Sertifikat SSL', color: '#14b8a6' },
  app:      { label: 'Aplikasi',    color: '#22c55e' },
  link:     { label: 'Domain Tertaut', color: '#06b6d4' },
  ip:       { label: 'IP Address',  color: '#f59e0b' },
  ns:       { label: 'Nameserver',  color: '#10b981' },
  txt:      { label: 'TXT Record',  color: '#94a3b8' },
}

// ── Peta: layar penuh & filter ──
const mapEl = ref<HTMLElement | null>(null)
const { isFullscreen, toggle: toggleFullscreen } = useFullscreen(mapEl)
const mapSearch    = ref('')
const onlyGambling = ref(false)
const hideCommon   = ref(false)
const activeKinds  = ref(new Set<string>())
const mapOptions = computed(() => [
  { label: 'Hanya judol', icon: 'i-lucide-shield-alert', on: onlyGambling.value, toggle: () => (onlyGambling.value = !onlyGambling.value),
    hint: 'Tampilkan hanya situs terkait yang terdeteksi judol' },
  { label: 'Sembunyikan umum', icon: 'i-lucide-eye-off', on: hideCommon.value, toggle: () => (hideCommon.value = !hideCommon.value),
    hint: 'Sembunyikan atribut yang dipakai sangat banyak situs (CDN, nameserver publik, dll)' },
  { label: 'Tanpa relasi', icon: 'i-lucide-circle-dashed', on: showIsolated.value, toggle: () => (showIsolated.value = !showIsolated.value),
    hint: 'Ikut tampilkan atribut yang tidak dipakai situs lain' },
])
const kindsInited  = ref(false)

const availableKinds = computed(() => Object.entries(kindMeta)
  .map(([kind, m]) => ({ kind, ...m, n: (relations.value?.attributes ?? []).filter((a: any) => a.kind === kind).length }))
  .filter(k => k.n))
watch(availableKinds, ks => {
  if (!kindsInited.value && ks.length) { activeKinds.value = new Set(ks.map(k => k.kind)); kindsInited.value = true }
})
function toggleKind(k: string) {
  const s = new Set(activeKinds.value)
  s.has(k) ? s.delete(k) : s.add(k)
  activeKinds.value = s
}

// Relasi setelah filter — dipakai graf & tree
const filteredRelations = computed(() => {
  const rel = relations.value
  if (!rel) return null
  const attributes = rel.attributes
    .filter((a: any) => activeKinds.value.has(a.kind) && !(hideCommon.value && a.common))
    .map((a: any) => ({ ...a, sites: onlyGambling.value ? a.sites.filter((x: any) => x.is_gambling) : a.sites }))
  return { ...rel, attributes }
})
const filteredStats = computed(() => {
  const attrs = (filteredRelations.value?.attributes ?? []).filter((a: any) => showIsolated.value || a.sites.length || a.common)
  return { attrs: attrs.length, sites: new Set(attrs.flatMap((a: any) => a.sites.map((x: any) => x.id))).size }
})

const site = computed(() => detail.value?.site ?? {})
const toast = useToast()
async function copyText(t: string) {
  try { await navigator.clipboard.writeText(t); toast.add({ title: 'Disalin', description: t, icon: 'i-lucide-check', color: 'emerald', timeout: 2000 }) } catch {}
}
// Kata kunci brand untuk mencari domain mirror: "indo303a.site" → "indo303a"
const brandKeyword = computed(() => host.value.split('/')[0].replace(/^www\./, '').split('.')[0])
const linkRank = (a: any) => (String(a.group).includes('register') ? 0 : String(a.group).includes('alternate') ? 1 : 2)
const sortedLinks = computed(() => [...byKind('link')].sort((a: any, b: any) => linkRank(a) - linkRank(b)))
// Link pendaftaran ke domain lain + domain alternatif (tanpa domain situs itu sendiri)
const keyLinks = computed(() => sortedLinks.value.filter((a: any) =>
  linkRank(a) < 2 && a.label && !host.value.startsWith(a.label.replace(/^www\./, ''))))
const host = computed(() => (site.value.url ?? '').replace(/^https?:\/\//, '').replace(/\/$/, ''))
const byKind = (k: string) => (detail.value?.attributes ?? []).filter((a: any) => a.kind === k)
const kindSummary = computed(() => {
  const c: Record<string, number> = {}
  for (const a of detail.value?.attributes ?? []) c[a.kind] = (c[a.kind] ?? 0) + 1
  return Object.entries(c).map(([k, n]) => `${n} ${kindMeta[k]?.label.toLowerCase() ?? k}`).slice(0, 3).join(', ') || 'Belum ada'
})
const strongRelated = computed(() => (relations.value?.related ?? []).filter((r: any) => r.strength >= 4).length)
const usedKinds = computed(() => {
  const out: Record<string, any> = {}
  for (const a of relations.value?.attributes ?? []) if (showIsolated.value || a.sites.length || a.common) out[a.kind] = kindMeta[a.kind]
  return out
})

const treeGroups = computed(() => {
  const groups: any[] = []
  for (const [kind, meta] of Object.entries(kindMeta)) {
    const attrs = (filteredRelations.value?.attributes ?? []).filter((a: any) => a.kind === kind && (showIsolated.value || a.sites.length || a.common))
      .sort((a: any, b: any) => b.sites.length - a.sites.length)
    if (attrs.length) groups.push({ kind, ...meta, attrs, linked: attrs.filter((a: any) => a.sites.length).length })
  }
  return groups
})

// Tree expand state
const openKinds = ref(new Set<string>())
const openAttrs = ref(new Set<string>())
const allOpen = computed(() => treeGroups.value.length > 0 && treeGroups.value.every(g => openKinds.value.has(g.kind)))
function setOpen(set: Set<string>, key: string, open: boolean) { open ? set.add(key) : set.delete(key) }
function toggleAll() {
  const open = !allOpen.value
  openKinds.value = new Set(open ? treeGroups.value.map(g => g.kind) : [])
  openAttrs.value = new Set(open ? treeGroups.value.flatMap(g => g.attrs.filter((a: any) => a.sites.length).map((a: any) => a.kind + a.id)) : [])
}

const whoisRows = computed(() => {
  const w = detail.value?.whois ?? {}
  return [
    { k: 'Domain', v: w.domain, mono: true, full: true }, { k: 'TLD', v: w.tld },
    { k: 'Registrar', v: w.registrar, full: true },
    { k: 'Umur domain', v: w.domain_age_days >= 0 ? `${w.domain_age_days} hari` : '' }, { k: 'WHOIS privat', v: w.is_private ? 'Ya' : 'Tidak' },
    { k: 'Terdaftar', v: fmtDate(w.registered_at) }, { k: 'Kedaluwarsa', v: fmtDate(w.expires_at) },
    { k: 'Registrant', v: [w.registrant, w.org].filter(Boolean).join(' · '), full: true },
    { k: 'Email', v: w.email, mono: true, full: true },
    { k: 'Skor risiko domain', v: w.risk_score != null ? `${Math.round(w.risk_score * 100)}%` : '' },
  ]
})


// ── Kartu WHOIS ──
const whois = computed<any>(() => detail.value?.whois ?? {})
const whoisAge = computed(() => {
  const d = whois.value.domain_age_days
  if (d == null || d < 0) return { value: '—', note: 'tidak diketahui', color: 'text-slate-400' }
  const value = d >= 365 ? `${(d / 365).toFixed(1)} thn` : `${d} hari`
  if (d < 90) return { value, note: 'Domain sangat baru — pola khas domain judol sekali pakai', color: 'text-rose-400' }
  if (d < 365) return { value, note: 'Domain kurang dari setahun', color: 'text-amber-400' }
  return { value, note: 'Domain sudah lama terdaftar', color: 'text-emerald-400' }
})
const whoisRisk = computed(() => {
  const r = whois.value.risk_score
  if (r == null) return { value: '—', pct: 0, color: 'text-slate-400', bar: 'gray' }
  const pct = Math.round(r * 100)
  return { value: `${pct}%`, pct, color: pct >= 70 ? 'text-rose-400' : pct >= 40 ? 'text-amber-400' : 'text-emerald-400',
    bar: pct >= 70 ? 'rose' : pct >= 40 ? 'amber' : 'emerald' }
})
const whoisLife = computed(() => {
  const reg = whois.value.registered_at ? new Date(whois.value.registered_at).getTime() : null
  const exp = whois.value.expires_at ? new Date(whois.value.expires_at).getTime() : null
  const now = Date.now()
  const left = exp ? Math.round((exp - now) / 86400000) : null
  const pct = reg && exp && exp > reg ? Math.min(100, Math.max(0, ((now - reg) / (exp - reg)) * 100)) : null
  return { pct, left, soon: left != null && left <= 60 }
})
const whoisDetails = computed(() => [
  { k: 'Domain', v: whois.value.domain, icon: 'i-lucide-globe', mono: true },
  { k: 'TLD', v: whois.value.tld, icon: 'i-lucide-tag' },
  { k: 'Registrar', v: whois.value.registrar, icon: 'i-lucide-building-2' },
  { k: 'Registrant', v: whois.value.registrant, icon: 'i-lucide-user' },
  { k: 'Organisasi', v: whois.value.org, icon: 'i-lucide-briefcase' },
  { k: 'Email', v: whois.value.email, icon: 'i-lucide-mail', mono: true },
].filter(r => r.v))
function fmtDay(d?: string) { return d ? new Date(d).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—' }

// ── Kartu data: hanya kategori yang ada isinya ──
const OWNER_PLATFORMS: Record<string, { label: string; to: string }> = {
  ReCaptcha: { label: 'reCAPTCHA', to: 'Google' }, GoogleAPIKey: { label: 'Kunci API Google', to: 'Google' },
  FirebaseProject: { label: 'Project Firebase', to: 'Google' }, AdSense: { label: 'AdSense', to: 'Google' },
  TelegramBot: { label: 'Bot Telegram', to: 'Telegram' }, TelegramChat: { label: 'Chat admin Telegram', to: 'Telegram' },
  CloudflareAnalytics: { label: 'Cloudflare Analytics', to: 'Cloudflare' }, Clarity: { label: 'Microsoft Clarity', to: 'Microsoft' },
  TikTokPixel: { label: 'TikTok Pixel', to: 'TikTok' }, YandexMetrica: { label: 'Yandex Metrica', to: 'Yandex' },
  OneSignal: { label: 'OneSignal', to: 'OneSignal' }, Sentry: { label: 'Sentry', to: 'Sentry' },
  Jivo: { label: 'JivoChat', to: 'Jivo' }, Tidio: { label: 'Tidio', to: 'Tidio' }, Zendesk: { label: 'Zendesk', to: 'Zendesk' },
  Smartsupp: { label: 'Smartsupp', to: 'Smartsupp' },
}
const CONTACT_TYPES = new Set(['Contact', 'Phone'])

// ── Keterjangkauan & blokir ISP ──
const REACH: Record<string, { label: string; color: any; icon: string; dot: string }> = {
  online: { label: 'Online', color: 'emerald', icon: 'i-lucide-wifi', dot: 'bg-emerald-500' },
  blocked_alive: { label: 'Diblokir · masih hidup', color: 'orange', icon: 'i-lucide-shield-off', dot: 'bg-orange-500' },
  blocked: { label: 'Diblokir', color: 'gray', icon: 'i-lucide-shield-ban', dot: 'bg-slate-500' },
  offline: { label: 'Offline', color: 'amber', icon: 'i-lucide-wifi-off', dot: 'bg-amber-500' },
  nxdomain: { label: 'Domain hilang', color: 'gray', icon: 'i-lucide-unlink', dot: 'bg-slate-600' },
}
// Siklus hidup (lihat site_lifecycle di backend): tidak aktif = domain hilang / tak terjangkau ≥14 hari
const inactiveSince = computed(() => site.value.reach_status === 'nxdomain' ? (site.value.offline_since ?? site.value.reach_checked_at) : site.value.offline_since)
const revivedRecently = computed(() => site.value.revived_at && Date.now() - new Date(site.value.revived_at).getTime() < 30 * 86_400_000)
const reachMeta = computed(() => REACH[site.value.reach_status ?? ''] ?? null)
const reachTip = computed(() => {
  const s = site.value
  const parts = [`Dicek ${s.reach_checked_at ? fmtDate(s.reach_checked_at) : '—'}`]
  if (s.blocked_since) parts.push(`diblokir sejak ${fmtDate(s.blocked_since)}`)
  if (s.reach_status === 'blocked_alive') parts.push('server masih merespons — bisa diakses lewat VPN/DNS lain')
  return parts.join(' · ')
})
const reachHistory = ref<any[]>([])
async function loadReach() {
  reachHistory.value = (await get<any>(`/sites/${route.params.id}/reachability`).catch(() => null))?.history ?? []
}

// ── Submenu ──
const TAB_KEYS = ['ringkasan', 'jaringan', 'keuangan', 'kontak', 'infra', 'konten', 'riwayat', 'bukti'] as const
const tab = ref<string>(TAB_KEYS.includes(route.query.tab as any) ? (route.query.tab as string) : 'ringkasan')
watch(tab, t => {
  router.replace({ query: { ...route.query, tab: t === 'ringkasan' ? undefined : t } })
  if (t === 'riwayat') loadSiteAudit()
})
const inTab = (t: string) => tab.value === t
const siteAudit = ref<any[]>([])
async function loadSiteAudit() {
  const r = await get<any>('/audit', { target: `site:${route.params.id}`, limit: 30 }).catch(() => null)
  siteAudit.value = r?.results ?? []
}

const TABS = computed(() => {
  const n = (k: string) => sections.value.find(x => x.key === k)?.items.length ?? 0
  const identityHits = identities.value.filter((i: any) => i.kominfo?.length || i.dukcapil?.length).length
  return [
    { key: 'ringkasan', label: 'Ringkasan', icon: 'i-lucide-clipboard-list', count: facts.value.length, loading: false, empty: '' },
    { key: 'jaringan', label: 'Jaringan', icon: 'i-lucide-waypoints', count: relations.value?.related.length ?? 0, loading: !relations.value,
      empty: 'Belum ada situs lain yang berbagi atribut dengan situs ini.' },
    { key: 'keuangan', label: 'Keuangan', icon: 'i-lucide-landmark', count: n('money') + payments.value.total + wallets.value.length, loading: !!deepSince.value,
      empty: 'Belum ada rekening, e-wallet, QRIS, atau pilihan pembayaran yang tertangkap. QRIS di gambar terbaca lewat Analisis Lanjutan.' },
    { key: 'kontak', label: 'Kontak & Identitas', icon: 'i-lucide-user-search',
      count: n('contact') + n('owner') + tgAccounts.value.length + (site.value.widget_links?.length ?? 0) + identityHits, loading: extrasLoading.value,
      empty: 'Belum ada kontak operator, akun Telegram, atau identifier pengelola yang tertangkap.' },
    { key: 'infra', label: 'Infrastruktur', icon: 'i-lucide-server', count: n('infra') + n('fingerprint') + n('tracking'), loading: false, empty: '' },
    { key: 'konten', label: 'Konten & Aplikasi', icon: 'i-lucide-layout-template',
      count: n('app') + apks.value.length + (auth.value?.forms?.length ?? 0) + gameProviders.value.length + (site.value.internal_pages?.length ?? 0),
      loading: !!deepSince.value || enriching.value, empty: 'Belum ada form, aplikasi, atau provider game yang tertangkap.' },
    { key: 'riwayat', label: 'Riwayat', icon: 'i-lucide-history', count: changes.value.length + wayback.value.length + (detail.value?.scans?.length ?? 0),
      loading: extrasLoading.value || waybackLoading.value, empty: '' },
    { key: 'bukti', label: 'Bukti', icon: 'i-lucide-file-lock-2', count: evidence.value.filter((e: any) => e.status === 'done').length,
      loading: evidenceBusy.value, empty: '' },
  ]
})
const currentTab = computed(() => TABS.value.find(t => t.key === tab.value) ?? TABS.value[0])

const sections = computed(() => {
  const out: any[] = []
  const push = (key: string, title: string, icon: string, color: string, items: any[], extra: any = {}) => {
    if (items.length) out.push({ key, title, icon, color, items, ...extra })
  }
  push('register', 'Alur Pendaftaran & Domain Cadangan', 'i-lucide-route', 'text-rose-400', keyLinks.value,
    { tab: 'jaringan', desc: 'Tujuan tombol DAFTAR/LOGIN (termasuk hasil shortener) dan domain alternatif', accent: 'ring-1 ring-rose-500/40' })
  push('money', 'Rekening & Wallet', 'i-lucide-wallet', 'text-violet-400', byKind('artifact').filter((a: any) => !CONTACT_TYPES.has(a.group)),
    { tab: 'keuangan', desc: 'Rekening bank, e-wallet, virtual account, QRIS & kripto yang tercantum di situs' })
  push('contact', 'Kontak Operator', 'i-lucide-message-circle', 'text-emerald-400', byKind('artifact').filter((a: any) => CONTACT_TYPES.has(a.group)),
    { tab: 'kontak', desc: 'Nomor WhatsApp, Telegram & kontak CS yang dipakai operator' })
  push('app', 'Aplikasi', 'i-lucide-smartphone', 'text-emerald-400', byKind('app'), { tab: 'konten', desc: 'Link unduhan APK, Play Store & App Store yang dipromosikan situs' })
  push('fingerprint', 'Sidik Jari Infrastruktur', 'i-lucide-scan-face', 'text-pink-400', byKind('fingerprint'),
    { tab: 'infra', desc: 'CDN, favicon & API backend — penanda platform yang sama' })
  // Identifier pengelola: kunci layanan yang terdaftar atas akun pemilik → tujuan permintaan data
  const owner = byKind('tracking').filter((a: any) => OWNER_PLATFORMS[a.group])
  push('owner', 'Identifier Pengelola', 'i-lucide-key-round', 'text-amber-400',
    owner.map((a: any) => ({ ...a, label: `${OWNER_PLATFORMS[a.group].label} → ${OWNER_PLATFORMS[a.group].to}` })),
    { tab: 'kontak', desc: 'Kunci layanan yang terdaftar atas akun pemilik situs. Kunci sama di situs lain = akun pengelola sama; pemilik akunnya dapat dimintakan resmi ke penyedia layanan.', accent: 'ring-1 ring-amber-500/40' })
  push('tracking', 'Tracking & Operator', 'i-lucide-radio-tower', 'text-blue-400', [...byKind('operator'), ...byKind('tracking').filter((a: any) => !OWNER_PLATFORMS[a.group])], { tab: 'infra', desc: 'Analytics, pixel & widget chat — ID yang sama menghubungkan situs satu pengelola' })
  push('infra', 'Hosting & DNS', 'i-lucide-server', 'text-emerald-400', [...byKind('ip'), ...byKind('ns'), ...byKind('ssl'), ...byKind('txt')], { tab: 'infra', desc: 'IP server, nameserver, sertifikat SSL & token verifikasi DNS' })
  push('link', 'Domain Tertaut', 'i-lucide-link', 'text-cyan-400', sortedLinks.value.filter((a: any) => linkRank(a) === 2), { tab: 'jaringan', max: 8, desc: 'Domain lain yang ditautkan dari halaman situs ini' })
  return out
})

// Jumlah atribut per jenis + berapa yang terhubung ke situs lain
// Antrekan URLScan + VirusTotal, lalu muat ulang data setelah worker sempat memproses
const enriching = ref(false)
const { post, download, apiBase } = useApi()

// ── Bukti forensik ──
const evidence     = ref<any[]>([])
const evidenceBusy = ref(false)
const evVerify     = ref<Record<number, any>>({})
const evVerifying  = ref<number | null>(null)
const { name: operator } = useOperator()

async function loadEvidence() {
  const r = await get<any>(`/sites/${route.params.id}/evidence`).catch(() => null)
  evidence.value = r?.captures ?? []
}
function evFileURL(ev: any, name: string) {
  return `${apiBase}/evidence/${ev.id}/files/${encodeURIComponent(name)}${operator.value ? `?by=${encodeURIComponent(operator.value)}` : ''}`
}
function evFileIcon(kind: string) {
  return ({ html: 'i-lucide-file-code', screenshot: 'i-lucide-image', network: 'i-lucide-network', manifest: 'i-lucide-file-check' } as Record<string, string>)[kind] ?? 'i-lucide-file'
}
function fmtSize(b: number) { return b >= 1048576 ? `${(b / 1048576).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB` }

async function captureEvidence() {
  if (!operator.value) {
    toast.add({ title: 'Isi nama dulu', description: 'Klik tombol nama di kanan atas — nama dicatat sebagai pengambil bukti.', color: 'amber', icon: 'i-lucide-user-round' })
    return
  }
  evidenceBusy.value = true
  try {
    const r = await post<any>(`/sites/${route.params.id}/evidence`, {})
    toast.add({ title: 'Pengambilan bukti diantrekan', description: r.message, color: 'sky', icon: 'i-lucide-camera' })
    await loadEvidence()
    // pantau hingga selesai (maks ±3 menit)
    for (let i = 0; i < 36; i++) {
      await new Promise(res => setTimeout(res, 5000))
      await loadEvidence()
      const ev = evidence.value.find(e => e.id === r.capture_id)
      if (ev && ev.status !== 'pending') {
        toast.add(ev.status === 'done'
          ? { title: `Bukti #${ev.id} tersimpan`, description: `${ev.files.length} file · manifest ${ev.manifest_sha256.slice(0, 16)}…`, color: 'emerald', icon: 'i-lucide-shield-check' }
          : { title: `Bukti #${ev.id} gagal`, description: ev.error, color: 'rose', icon: 'i-lucide-circle-x' })
        break
      }
    }
  } catch (e: any) {
    toast.add({ title: 'Gagal mengambil bukti', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { evidenceBusy.value = false }
}

async function verifyEvidence(ev: any) {
  evVerifying.value = ev.id
  try {
    const r = await get<any>(`/evidence/${ev.id}/verify`)
    evVerify.value = { ...evVerify.value, [ev.id]: r }
    toast.add(r.intact
      ? { title: `Bukti #${ev.id} utuh`, description: 'Semua hash cocok dengan catatan saat diambil.', color: 'emerald', icon: 'i-lucide-shield-check' }
      : { title: `Bukti #${ev.id} BERUBAH`, description: 'Ada file yang hash-nya tidak cocok atau hilang.', color: 'rose', icon: 'i-lucide-shield-x' })
  } catch (e: any) {
    toast.add({ title: 'Verifikasi gagal', description: e.message, color: 'rose' })
  } finally { evVerifying.value = null }
}

async function downloadEvidence(ev: any) {
  try { await download(`/evidence/${ev.id}/zip`, `bukti-${ev.id}.zip`) }
  catch (e: any) { toast.add({ title: 'Unduh gagal', description: e.message, color: 'rose' }) }
}

// ── Sumber eksternal: reverse analytics (HackerTarget) & jejak USDT (TronGrid) ──
const revResults = ref<any[]>([])
const revBusy = ref(false)
const wallets = ref<any[]>([])
const cryptoBusy = ref(false)
async function loadReverse(id: string) {
  revResults.value = (await get<any>(`/sites/${id}/reverse-analytics`).catch(() => null))?.results ?? []
}
async function refreshReverse(queue: boolean) {
  revBusy.value = true
  try {
    const r = await post<any>(`/sites/${route.params.id}/reverse-analytics${queue ? '?queue=1' : ''}`, {})
    revResults.value = r.results ?? []
    const total = revResults.value.reduce((s, x) => s + x.domains.length, 0)
    toast.add(r.quota_exceeded
      ? { title: 'Kuota HackerTarget habis', description: 'Sebagian ID belum dicek — coba lagi besok atau isi HACKERTARGET_API_KEY.', color: 'amber', icon: 'i-lucide-gauge' }
      : { title: `${total} domain ditemukan`, description: r.queued?.length ? `${r.queued.length} domain baru diantrekan untuk dianalisis` : `${r.asked} ID dicek`, color: 'emerald', icon: 'i-lucide-radar' })
  } catch (e: any) {
    toast.add({ title: 'Reverse lookup gagal', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { revBusy.value = false }
}
async function loadCrypto(id: string, refresh = false) {
  cryptoBusy.value = true
  try { wallets.value = (await get<any>(`/sites/${id}/crypto`, refresh ? { refresh: 1 } : undefined).catch(() => null))?.wallets ?? [] }
  finally { cryptoBusy.value = false }
}

// ── Analisis lanjutan: klaster operator, APK, perubahan, Wayback, mirror ──
const clusters   = ref<any[]>([])
const apks       = ref<any[]>([])
const changes    = ref<any[]>([])
const wayback    = ref<any[]>([])
const identities = ref<any[]>([])
const vflRunning = ref<Record<number, boolean>>({})

// QRIS merchant (artefak type QRIS) & provider game untuk kartu khusus
const qrisMerchants = computed(() => (detail.value?.attributes ?? []).filter((a: any) => a.kind === 'artifact' && a.group === 'QRIS'))
const gameProviders = computed<string[]>(() => detail.value?.game_providers ?? [])

// Jalankan pengecekan berantai VflOSINT untuk satu nomor, lalu muat ulang identitas
async function checkIdentity(it: any) {
  vflRunning.value = { ...vflRunning.value, [it.artifact_id]: true }
  try {
    const r = await post<any>('/leak/vfl', { phone: it.phone })
    toast.add({ title: 'Pengecekan diantrekan', description: r.message, color: 'cyan', icon: 'i-lucide-bot' })
    let last = -1, idle = 0
    for (let i = 0; i < 40 && idle < 6; i++) {
      await new Promise(res => setTimeout(res, 8000))
      const idn = await get<any>(`/sites/${route.params.id}/identities`).catch(() => null)
      const fresh = (idn?.identities ?? []).find((x: any) => x.artifact_id === it.artifact_id)
      const n = (fresh?.kominfo?.length ?? 0) + (fresh?.related?.length ?? 0)
      idle = n === last ? idle + 1 : 0
      last = n
      if (idn) identities.value = idn.identities
    }
  } catch (e: any) {
    toast.add({ title: 'Tidak bisa dicek', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally {
    vflRunning.value = { ...vflRunning.value, [it.artifact_id]: false }
  }
}
const deepLoading = ref(false), mirrorLoading = ref(false), waybackLoading = ref(false)

const investigateItems = computed(() => [[
  { label: 'Analisis Lanjutan', icon: 'i-lucide-microscope', desc: 'Dekode QRIS, OCR banner & analisis APK di background (1–5 menit)',
    disabled: deepLoading.value, click: () => runDeep() },
  { label: 'Cari Mirror', icon: 'i-lucide-copy-plus', desc: `Cari domain baru berisi "${brandKeyword.value}" di CT Logs & antrekan analisisnya`,
    disabled: mirrorLoading.value, click: () => findMirrors() },
  { label: 'Deep Crawl', icon: 'i-lucide-bug', desc: 'Jelajahi seluruh halaman situs di halaman Discovery',
    to: `/discovery?mode=crawler&domain=${encodeURIComponent(site.value.url ?? '')}` },
]])

async function loadExtras(id: string) {
  extrasLoading.value = true
  try { await loadExtrasInner(id) } finally { extrasLoading.value = false }
}
async function loadExtrasInner(id: string) {
  const [a, c, w, idn] = await Promise.all([
    get<any>(`/sites/${id}/apk`).catch(() => null),
    get<any>(`/sites/${id}/changes`).catch(() => null),
    get<any>(`/sites/${id}/wayback`).catch(() => null),
    get<any>(`/sites/${id}/identities`).catch(() => null),
  ])
  apks.value = a?.apks ?? []
  await Promise.all([loadTelegram(id), loadEvidence(), loadReach(), loadSiteAudit(), loadReverse(id), loadCrypto(id)])
  changes.value = c?.changes ?? []
  wayback.value = w?.snapshots ?? []
  identities.value = idn?.identities ?? []
  const ops = (detail.value?.attributes ?? []).filter((x: any) => x.kind === 'operator' && x.group === 'Klaster')
  clusters.value = (await Promise.all(ops.map((o: any) => get<any>(`/operators/${o.id}`).catch(() => null)))).filter(Boolean)
}

// ── Status pengumpulan data ──
const extrasLoading = ref(true)
const deepSince = ref<number | null>(null)
const deepTick  = ref(Date.now())

// Analisis lanjutan berjalan di worker tanpa status; pantau dengan memuat ulang data tiap 15 detik
// hingga hasilnya stabil (2× tanpa perubahan setelah ada perubahan) atau 6 menit berlalu.
async function watchDeep(id: string, since: number) {
  if (deepSince.value) return
  deepSince.value = since
  deepTick.value = Date.now()
  const sig = () => `${(detail.value?.attributes ?? []).length}:${apks.value.length}:${apks.value.filter((x: any) => x.package_name || x.error).length}`
  let last = sig(), changed = false, idle = 0
  while (Date.now() - since < 6 * 60_000 && route.params.id === id) {
    await new Promise(r => setTimeout(r, 15_000))
    deepTick.value = Date.now()
    const d = await get<any>(`/sites/${id}`).catch(() => null)
    if (d) { d.attributes ??= []; d.scans ??= []; detail.value = d }
    const a = await get<any>(`/sites/${id}/apk`).catch(() => null)
    if (a) apks.value = a.apks ?? []
    const now = sig()
    if (now !== last) { changed = true; idle = 0; last = now } else if (changed && ++idle >= 2) break
  }
  deepSince.value = null
  try { sessionStorage.removeItem(`jutec_deep_${id}`) } catch {}
}

const collecting = computed(() => {
  const out: { key: string; text: string; sub?: string }[] = []
  if (!relations.value) out.push({ key: 'rel', text: 'Membangun peta relasi & situs terkait…' })
  if (extrasLoading.value) out.push({ key: 'extras', text: 'Memuat data lanjutan…', sub: 'Telegram, APK, perubahan antar scan, arsip Wayback, identitas, klaster operator & bukti forensik' })
  if (deepSince.value) {
    const min = Math.max(0, Math.round((deepTick.value - deepSince.value) / 60_000))
    out.push({ key: 'deep', text: 'Analisis lanjutan berjalan: dekode QRIS, OCR banner & analisis APK', sub: `Dimulai ${min ? `${min} menit` : 'baru saja'} lalu · biasanya 1–5 menit; card APK, Merchant QRIS & Artefak diperbarui otomatis.` })
  }
  if (enriching.value) out.push({ key: 'enrich', text: 'Mengambil reputasi dari URLScan & VirusTotal…', sub: '±30–60 detik' })
  if (waybackLoading.value) out.push({ key: 'wayback', text: 'Mengambil snapshot lama dari Wayback Machine…', sub: '±30–60 detik' })
  if (Object.values(vflRunning.value).some(Boolean)) out.push({ key: 'vfl', text: 'Pengecekan identitas berantai (VflOSINT)…', sub: '±1–3 menit' })
  if (evidenceBusy.value) out.push({ key: 'evidence', text: 'Mengambil bukti forensik (HTML, screenshot, log jaringan)…', sub: '±20–60 detik' })
  return out
})

// ── Telegram ──
const tgAccounts = ref<any[]>([])
const tgMessages = ref<any[]>([])
const tgOpen = ref(''), tgScraping = ref('')
const tgStatus: Record<string, { label: string; color: string; icon: string }> = {
  valid:     { label: 'Valid',            color: 'emerald', icon: 'i-lucide-badge-check' },
  not_found: { label: 'Tidak ditemukan',  color: 'rose',    icon: 'i-lucide-ban' },
  pending:   { label: 'Belum dicek',      color: 'gray',    icon: 'i-lucide-clock' },
}
const tgTypes: Record<string, string> = { channel: 'Channel', group: 'Grup', user: 'Akun pribadi (CS)', bot: 'Bot' }
async function loadTelegram(id: string | number) {
  tgAccounts.value = (await get<any>(`/sites/${id}/telegram`).catch(() => null))?.accounts ?? []
}
async function toggleTgMessages(u: string) {
  if (tgOpen.value === u) { tgOpen.value = ''; return }
  tgMessages.value = (await get<any>('/telegram/messages', { username: u, limit: 30 }).catch(() => null))?.messages ?? []
  tgOpen.value = u
}
async function scrapeTg(u: string) {
  tgScraping.value = u
  try {
    const r = await post<any>(`/telegram/scrape?username=${encodeURIComponent(u)}&limit=50`, {})
    toast.add({ title: `@${u} dibaca`, description: `${r.links.length} link · ${r.artifacts_found} artefak ditemukan`, icon: 'i-lucide-send', color: 'sky' })
    await loadTelegram(site.value.id)
  } catch (e: any) { toast.add({ title: 'Gagal membaca channel', description: e.message, color: 'rose' }) }
  finally { tgScraping.value = '' }
}

async function runDeep() {
  deepLoading.value = true
  try {
    await post(`/sites/${site.value.id}/deep`, {})
    toast.add({ title: 'Analisis lanjutan diantrekan', description: 'QRIS, OCR & APK diproses di background (1–5 menit). Hasilnya dimuat otomatis.', icon: 'i-lucide-microscope', color: 'primary' })
    try { sessionStorage.setItem(`jutec_deep_${site.value.id}`, String(Date.now())) } catch {}
    watchDeep(String(site.value.id), Date.now())
  } catch (e: any) { toast.add({ title: 'Gagal', description: e.message, color: 'rose' }) }
  finally { deepLoading.value = false }
}

async function findMirrors() {
  mirrorLoading.value = true
  try {
    const r = await post<any>(`/sites/${site.value.id}/mirrors`, {})
    toast.add({ title: r.queued.length ? `${r.queued.length} domain mirror baru diantrekan` : 'Tidak ada domain mirror baru',
      description: r.queued.length ? r.queued.slice(0, 5).join(', ') + (r.queued.length > 5 ? '…' : '') : `Brand "${r.brand}" — semua domain sudah pernah di-scan`,
      icon: 'i-lucide-copy-plus', color: r.queued.length ? 'emerald' : 'gray', timeout: 8000 })
  } catch (e: any) { toast.add({ title: 'Pencarian mirror gagal', description: e.message, color: 'rose' }) }
  finally { mirrorLoading.value = false }
}

async function fetchWayback() {
  waybackLoading.value = true
  try {
    const r = await post<any>(`/sites/${site.value.id}/wayback`, {})
    wayback.value = r.snapshots ?? []
    toast.add({ title: `${r.fetched} snapshot diambil`, description: `${r.artifacts} artefak historis ditambahkan ke data relasi`, icon: 'i-lucide-archive', color: 'emerald' })
  } catch (e: any) { toast.add({ title: 'Gagal mengambil arsip', description: e.message, color: 'rose' }) }
  finally { waybackLoading.value = false }
}

const riskyList = ['SYSTEM_ALERT_WINDOW', 'REQUEST_INSTALL_PACKAGES', 'READ_SMS', 'RECEIVE_SMS', 'SEND_SMS', 'READ_CONTACTS',
  'READ_PHONE_STATE', 'RECEIVE_BOOT_COMPLETED', 'ACCESS_FINE_LOCATION', 'CAMERA', 'RECORD_AUDIO', 'BIND_ACCESSIBILITY_SERVICE', 'QUERY_ALL_PACKAGES']
const riskyPerms = (perms?: string[]) => (perms ?? []).filter(p => riskyList.includes(p))
function fmtWayback(ts: string) {
  const d = new Date(`${ts.slice(0, 4)}-${ts.slice(4, 6)}-${ts.slice(6, 8)}T${ts.slice(8, 10) || '00'}:${ts.slice(10, 12) || '00'}:00Z`)
  return isNaN(d.getTime()) ? ts : d.toLocaleDateString('id-ID', { dateStyle: 'long' })
}
async function enrichNow() {
  enriching.value = true
  try {
    await post(`/sites/${site.value.id}/enrich`, {})
    toast.add({ title: 'Enrichment diantrekan', description: 'URLScan butuh ±30–60 detik; data dimuat otomatis.', icon: 'i-lucide-sparkles', color: 'primary' })
    for (let i = 0; i < 8; i++) {
      await new Promise(r => setTimeout(r, 10000))
      const d = await get<any>(`/sites/${site.value.id}`)
      if (d.enrichment?.urlscan || d.enrichment?.virustotal) {
        detail.value.enrichment = d.enrichment
        if (d.enrichment.urlscan && d.enrichment.virustotal) break
      }
    }
  } catch (e: any) {
    toast.add({ title: 'Gagal mengantrekan enrichment', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  } finally { enriching.value = false }
}

// Peta login & pendaftaran (null bila belum pernah di-scan dengan versi engine ini)
const auth = computed(() => {
  const a = site.value.auth_surface
  return a && (a.login_urls?.length || a.register_urls?.length || a.forms?.length || a.referral_codes?.length) ? a : null
})
// Metode pembayaran dari semua form daftar: nama dinormalkan & dikelompokkan
const payCats: { cat: string; icon: string; color: string; re: RegExp }[] = [
  { cat: 'Pulsa',    icon: 'i-lucide-card-sim',  color: 'amber',   re: /pulsa|telkomsel|indosat|\bxl\b|axis|\btri\b|smartfren/i },
  { cat: 'QRIS',     icon: 'i-lucide-qr-code',   color: 'cyan',    re: /qris/i },
  { cat: 'E-Wallet', icon: 'i-lucide-wallet',    color: 'emerald', re: /\b(dana|ovo|gopay|linkaja|link aja|shopeepay|sakuku|doku|isaku)\b/i },
  { cat: 'Crypto',   icon: 'i-lucide-bitcoin',   color: 'orange',  re: /usdt|bitcoin|btc|eth|crypto|tron|trc20/i },
  { cat: 'Bank',     icon: 'i-lucide-landmark',  color: 'violet',  re: /.*/ },
]
function normPayment(raw: string) {
  let n = raw.trim().replace(/^bank\s+/i, '').replace(/^pulsa[-\s]*/i, 'Pulsa ').replace(/\s+/g, ' ')
  if (n.length <= 5 || /^[A-Z0-9 -]+$/.test(n)) n = n.toUpperCase()
  return n.replace(/^Pulsa ([a-z])/i, (_, c) => 'Pulsa ' + c.toUpperCase())
}
const payments = computed(() => {
  const names = new Set<string>()
  for (const f of auth.value?.forms ?? []) for (const b of f.bank_options ?? []) names.add(normPayment(b))
  const groups = payCats.map(c => ({ ...c, items: [] as { name: string }[] }))
  for (const n of [...names].sort()) groups.find(g => g.re.test(n))!.items.push({ name: n })
  const order = ['Bank', 'E-Wallet', 'QRIS', 'Pulsa', 'Crypto']
  return { total: names.size, groups: groups.filter(g => g.items.length).sort((a, b) => order.indexOf(a.cat) - order.indexOf(b.cat)) }
})

function widgetIcon(u: string) {
  if (/\.apk/i.test(u)) return 'i-lucide-smartphone'
  if (/t\.me\//i.test(u)) return 'i-lucide-send'
  if (/wa\.me|whatsapp/i.test(u)) return 'i-lucide-message-circle'
  if (/promo|event/i.test(u)) return 'i-lucide-gift'
  return 'i-lucide-link'
}

const authLinks = computed(() => [
  { label: 'Halaman daftar', urls: auth.value?.register_urls ?? [] },
  { label: 'Halaman login', urls: auth.value?.login_urls ?? [] },
].filter(g => g.urls.length))

const composition = computed(() => {
  const attrs = relations.value?.attributes ?? detail.value?.attributes ?? []
  const rows = Object.entries(kindMeta).map(([kind, m]) => {
    const list = attrs.filter((a: any) => a.kind === kind)
    return { kind, label: m.label, color: m.color, n: list.length, linked: list.filter((a: any) => a.shared_sites > 0).length }
  }).filter(r => r.n)
  const max = Math.max(1, ...rows.map(r => r.n))
  // Urut dari jenis terbanyak; bila sama, yang lebih banyak terhubung ke situs lain duluan
  return rows.sort((a, b) => b.n - a.n || b.linked - a.linked).map(r => ({ ...r, pct: Math.round((r.n / max) * 100) }))
})

const missing = computed(() => [
  ['artifact', 'Rekening/kontak'], ['app', 'Link aplikasi'], ['tracking', 'Tracking ID'], ['fingerprint', 'Sidik jari'], ['ip', 'IP hosting'],
].filter(([k]) => !byKind(k).length).map(([, l]) => l))

const kpis = computed(() => {
  const w = detail.value?.whois
  return [
    { label: 'Confidence', value: `${Math.round((site.value.confidence ?? 0) * 100)}%`, hint: site.value.priority, icon: 'i-lucide-gauge',
      box: 'bg-rose-500/10', text: site.value.confidence >= 0.75 ? 'text-rose-400' : 'text-amber-400' },
    { label: 'Umur Domain', value: w?.domain_age_days >= 0 ? `${w.domain_age_days} hari` : '—', hint: w?.registrar || 'WHOIS n/a', icon: 'i-lucide-calendar-clock', box: 'bg-amber-500/10', text: 'text-amber-400' },
    { label: 'Artefak', value: byKind('artifact').length, hint: 'rekening & kontak', icon: 'i-lucide-wallet', box: 'bg-violet-500/10', text: 'text-violet-400' },
    { label: 'Sidik Jari', value: byKind('fingerprint').length, hint: 'CDN · favicon · API', icon: 'i-lucide-scan-face', box: 'bg-pink-500/10', text: 'text-pink-400' },
    { label: 'Situs Terkait', value: relations.value?.related.length ?? '—', hint: `${strongRelated.value} bukti kuat`, icon: 'i-lucide-waypoints', box: 'bg-indigo-500/10', text: 'text-indigo-400' },
    { label: 'Scan Terakhir', value: site.value.last_scanned_at ? new Date(site.value.last_scanned_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' }) : '—',
      hint: site.value.last_scanned_at ? new Date(site.value.last_scanned_at).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' }) : '', icon: 'i-lucide-clock', box: 'bg-slate-700/40', text: 'text-slate-300' },
  ]
})

// Poin temuan otomatis — dibaca sekilas tanpa membuka tiap kartu
const facts = computed(() => {
  const f: { icon: string; color: string; title: string; detail?: string }[] = []
  const s = site.value, w = detail.value?.whois
  f.push({ icon: s.is_gambling ? 'i-lucide-shield-alert' : 'i-lucide-shield-check', color: s.is_gambling ? 'text-rose-400' : 'text-emerald-400',
    title: s.is_gambling ? `Terdeteksi judol · prioritas ${s.priority} (${Math.round(s.confidence * 100)}%)` : `Tidak terdeteksi judol (${Math.round((s.confidence ?? 0) * 100)}%)` })
  if (w) f.push({ icon: 'i-lucide-calendar-clock', color: 'text-amber-400',
    title: `${w.domain_age_days >= 0 ? `Domain berumur ${w.domain_age_days} hari` : 'Umur domain tak diketahui'}${w.is_private ? ' · WHOIS disembunyikan' : ''}`,
    detail: [w.registrar, w.tld].filter(Boolean).join(' · ') })
  const rl = detail.value?.role_links
  if (s.role === 'landing' && rl?.targets?.length) f.push({ icon: 'i-lucide-door-open', color: 'text-sky-400', title: 'Landing page — situs utamanya di domain lain',
    detail: rl.targets.map((t: any) => t.host + (t.site ? '' : ' (belum dianalisis)')).join(' · ') })
  if (s.role === 'operator') f.push({ icon: 'i-lucide-castle', color: 'text-rose-400', title: `Situs utama (operator)${rl?.entries?.length ? ` · ${rl.entries.length} landing page mengarah ke sini` : ''}`,
    detail: (rl?.entries ?? []).slice(0, 4).map((e: any) => stripScheme(e.url)).join(' · ') })
  const reg = keyLinks.value.filter((a: any) => linkRank(a) === 0)
  if (reg.length) f.push({ icon: 'i-lucide-route', color: 'text-rose-400', title: 'Pendaftaran diarahkan ke domain lain', detail: reg.map((a: any) => a.value).join(' → ') })
  const au = auth.value
  if (au) {
    const reg = (au.forms ?? []).find((x: any) => x.type === 'register')
    f.push({ icon: 'i-lucide-key-round', color: 'text-amber-400',
      title: `Login & daftar: ${(au.forms ?? []).length} form${reg?.bank_options?.length ? `, ${reg.bank_options.length} pilihan bank` : ''}${au.referral_codes?.length ? `, referral ${au.referral_codes.join(', ')}` : ''}`,
      detail: reg ? `Daftar → ${reg.action}` : au.register_urls?.[0] })
  }
  if (payments.value.total) f.push({ icon: 'i-lucide-landmark', color: 'text-violet-400',
    title: `Menerima ${payments.value.total} metode pembayaran: ` + payments.value.groups.map(g => `${g.items.length} ${g.cat.toLowerCase()}`).join(', '),
    detail: payments.value.groups.flatMap(g => g.items.map(i => i.name)).slice(0, 10).join(' · ') })
  const contacts = byKind('artifact').filter((a: any) => a.group === 'Contact' || a.group === 'Phone')
  if (contacts.length) f.push({ icon: 'i-lucide-message-circle', color: 'text-emerald-400', title: `${contacts.length} kontak operator`,
    detail: contacts.map((a: any) => `${a.label || a.group}: ${a.value}`).join(' · ') })
  const pay = byKind('artifact').filter((a: any) => !['Contact', 'Phone'].includes(a.group))
  if (pay.length) f.push({ icon: 'i-lucide-landmark', color: 'text-violet-400', title: `${pay.length} rekening/wallet/QRIS`,
    detail: pay.slice(0, 3).map((a: any) => `${a.label || a.group} ${a.value}`).join(' · ') })
  if (byKind('app').length) f.push({ icon: 'i-lucide-smartphone', color: 'text-emerald-400', title: `${byKind('app').length} link aplikasi`, detail: byKind('app')[0].value })
  else if (s.app_promo) f.push({ icon: 'i-lucide-smartphone', color: 'text-amber-400', title: 'Mempromosikan APK tanpa link publik' })
  if (flaggedArtifacts.value.length) f.splice(1, 0, { icon: 'i-lucide-triangle-alert', color: 'text-rose-400',
    title: `${flaggedArtifacts.value.length} rekening/kontak juga dipakai situs judol terverifikasi`,
    detail: flaggedArtifacts.value.slice(0, 3).map((a: any) => `${a.value} (${a.verified_sites.length} situs)`).join(' · ') })
  const fp = byKind('fingerprint')
  if (fp.length) f.push({ icon: 'i-lucide-scan-face', color: 'text-pink-400', title: `${fp.length} sidik jari infrastruktur`,
    detail: fp.filter((a: any) => a.group !== 'favicon').slice(0, 3).map((a: any) => a.value).join(' · ') })
  const ips = byKind('ip')
  if (ips.length) f.push({ icon: 'i-lucide-server', color: 'text-amber-400', title: `Hosting di ${ips.length} IP`, detail: ips.slice(0, 2).map((a: any) => a.value).join(' · ') })
  const rel = relations.value?.related ?? []
  if (rel.length) f.push({ icon: 'i-lucide-waypoints', color: 'text-indigo-400', title: `Terhubung ke ${rel.length} situs lain${strongRelated.value ? `, ${strongRelated.value} dengan bukti kuat` : ''}`,
    detail: stripScheme(rel[0].url) })
  return f
})

// Rekening/kontak yang juga dipakai situs berstatus terverifikasi judol (dikonfirmasi manual, bukan skor otomatis)
const flaggedArtifacts = computed(() => byKind('artifact').filter((a: any) => a.verified_sites?.length)
  .sort((a: any, b: any) => b.verified_sites.length - a.verified_sites.length))

// ── Ekspor peta keterkaitan sebagai gambar (mengikuti filter yang sedang aktif) ──
const graphExportItems = [[
  { label: 'PNG · tema gelap', icon: 'i-lucide-image', click: () => exportGraph('png', 'dark') },
  { label: 'PNG · tema terang (dokumen)', icon: 'i-lucide-image', click: () => exportGraph('png', 'light') },
  { label: 'SVG · vektor', icon: 'i-lucide-pen-tool', click: () => exportGraph('svg', 'light') },
]]
async function exportGraph(fmt: 'png' | 'svg', theme: 'light' | 'dark') {
  const svg = buildRelationSvg(filteredRelations.value, kindMeta, { theme, showIsolated: showIsolated.value })
  if (!svg) return
  const name = `peta-${host.value.replace(/[^\w.-]+/g, '_')}-${new Date().toISOString().slice(0, 10)}.${fmt}`
  try {
    downloadBlob(fmt === 'svg' ? new Blob([svg], { type: 'image/svg+xml' }) : await svgToPng(svg), name)
    toast.add({ title: 'Peta diunduh', description: name, icon: 'i-lucide-image-down', color: 'emerald' })
  } catch (e: any) {
    toast.add({ title: 'Gagal mengunduh peta', description: e.message, color: 'rose' })
  }
}

// ── Analisis: dasar skor, pivot, keyakinan temuan, timeline (lihat utils/siteAnalysis.ts) ──
const scoreData = computed(() => explainScore(site.value, detail.value?.score_explain, detail.value?.whois, gameProviders.value))
const pivots = computed(() => rankPivots(detail.value?.attributes ?? [], kindMeta))
const confidenceOf = (a: any) => attrConfidence(a, { explain: detail.value?.score_explain, tgAccounts: tgAccounts.value, apks: apks.value })
const timeline = computed(() => buildTimeline({
  site: site.value, whois: detail.value?.whois, scans: detail.value?.scans ?? [], changes: changes.value, wayback: wayback.value,
  reachHistory: reachHistory.value, evidence: evidence.value, audit: siteAudit.value,
}))

const scanColumns = [
  { key: 'created_at', label: 'Waktu' }, { key: 'status', label: 'Status' },
  { key: 'score', label: 'Skor' }, { key: 'priority', label: 'Prioritas' },
]

function stripScheme(u: string) { return u.replace(/^https?:\/\//, '').replace(/\/$/, '') }

// Ekspor: bila masih ada proses berjalan, tanya dulu — tunggu selesai atau ekspor yang sudah siap.
const exportAsk = ref(false)
const exportWaiting = ref(false)
const exportFormat = ref<'pdf' | 'md'>('pdf')
const exportItems = [[
  { label: 'PDF (laporan lengkap)', icon: 'i-lucide-file-text', click: () => requestExport('pdf') },
  { label: 'Markdown', icon: 'i-lucide-file-code', click: () => requestExport('md') },
]]
const runExport = () => (exportFormat.value === 'pdf' ? exportPdf() : exportReport())
function requestExport(fmt?: 'pdf' | 'md') {
  if (fmt) exportFormat.value = fmt
  if (exportWaiting.value) {
    exportWaiting.value = false
    toast.add({ title: 'Ekspor otomatis dibatalkan', icon: 'i-lucide-x', color: 'gray' })
    return
  }
  if (collecting.value.length) exportAsk.value = true
  else runExport()
}
function waitForExport() {
  exportAsk.value = false
  exportWaiting.value = true
  toast.add({ title: 'Laporan akan diekspor otomatis', description: 'Setelah semua data selesai dikumpulkan.', icon: 'i-lucide-hourglass', color: 'primary' })
}
watch(() => collecting.value.length, n => {
  if (n === 0 && exportWaiting.value) {
    exportWaiting.value = false
    nextTick(() => runExport())
  }
})

// Ekspor laporan investigasi satu situs sebagai berkas Markdown (self-contained, siap dibagikan).
function exportReport() {
  const s = detail.value?.site ?? {}
  const L: string[] = []
  const line = (t = '') => L.push(t)
  const list = (arr: any[], f: (x: any) => string) => arr.forEach(x => line('- ' + f(x)))

  line(`# Laporan Investigasi — ${host.value}`)
  line()
  line(`- **URL:** ${s.url}`)
  line(`- **Situs #:** ${s.id}`)
  line(`- **Status judol:** ${s.is_gambling ? `Ya · prioritas ${s.priority} · ${Math.round((s.confidence ?? 0) * 100)}%` : 'Tidak terdeteksi'}`)
  line(`- **Pertama tercatat:** ${fmtDate(s.created_at)}`)
  line(`- **Scan terakhir:** ${fmtDate(s.last_scanned_at)} · rescan ${s.rescan_count ?? 0}×`)
  line(`- **Laporan dibuat:** ${new Date().toLocaleString('id-ID')}`)
  if (collecting.value.length) {
    line()
    line(`> **Catatan:** laporan ini dibuat saat analisis belum selesai. Bagian berikut mungkin belum lengkap:`)
    collecting.value.forEach(c => line(`> - ${c.text.replace(/…$/, '')}`))
  }
  line()

  const sec = (title: string) => { line(); line(`## ${title}`); line() }

  if (facts.value.length) { sec('Ringkasan Intelijen'); list(facts.value, (f: any) => `${f.title}${f.detail ? ` — ${f.detail}` : ''}`) }

  if (identities.value.some((i: any) => i.kominfo?.length || i.dukcapil?.length || i.related?.length)) {
    sec('Identitas Terkait (data bocoran)')
    identities.value.forEach((it: any) => {
      if (!(it.kominfo?.length || it.dukcapil?.length || it.related?.length)) return
      line(`### ${it.phone}${it.label ? ` (${it.label})` : ''}`)
      list(it.kominfo ?? [], (k: any) => `KomInfo — NIK ${k.nik} · ${k.provider} · reg ${k.reg_date} · sumber ${k.source}`)
      list(it.dukcapil ?? [], (d: any) => `Dukcapil — ${d.full_name} · NIK ${d.nik} · ${[d.address, d.district, d.location, d.region].filter(Boolean).join(', ')}`)
      list(it.related ?? [], (r: any) => `Nomor lain NIK sama — ${r.phone} · ${r.provider} · reg ${r.reg_date}`)
      line()
    })
  }

  if (payments.value.total) {
    sec('Metode Pembayaran')
    payments.value.groups.forEach((g: any) => list(g.items, (it: any) => `${g.cat}: ${it.name}`))
  }
  if (qrisMerchants.value.length) { sec('Merchant QRIS'); list(qrisMerchants.value, (q: any) => `${q.value}${q.label ? ` — ${q.label}` : ''}`) }

  const contacts = byKind('artifact').filter((a: any) => a.group === 'Contact' || a.group === 'Phone')
  if (contacts.length) { sec('Kontak Operator'); list(contacts, (a: any) => `${a.label || a.group}: ${a.value}`) }

  if (gameProviders.value.length) { sec('Provider Game'); line(gameProviders.value.join(', ')) }

  if (keyLinks.value.length) { sec('Alur Pendaftaran & Domain Cadangan'); list(keyLinks.value, (a: any) => `${a.label || a.group}: ${a.value}`) }
  if (byKind('app').length) { sec('Aplikasi'); list(byKind('app'), (a: any) => a.value) }
  if (byKind('fingerprint').length) { sec('Sidik Jari Infrastruktur'); list(byKind('fingerprint'), (a: any) => `${a.group}: ${a.value}`) }
  if (byKind('tracking').length || byKind('operator').length) { sec('Tracking & Operator'); list([...byKind('operator'), ...byKind('tracking')], (a: any) => `${a.group}: ${a.value}`) }

  const infra = [...byKind('ip'), ...byKind('ns'), ...byKind('ssl'), ...byKind('txt')]
  if (infra.length) { sec('Hosting & DNS'); list(infra, (a: any) => `${a.kind.toUpperCase()}: ${a.value}`) }

  if (detail.value?.whois) {
    sec('WHOIS')
    whoisRows.value.filter((r: any) => r.v && r.v !== '—').forEach((r: any) => line(`- **${r.k}:** ${r.v}`))
  }
  const e = detail.value?.enrichment ?? {}
  if (e.virustotal || e.urlscan) {
    sec('Reputasi')
    if (e.virustotal) line(`- **VirusTotal:** ${e.virustotal.malicious}/${e.virustotal.total} engine berbahaya`)
    if (e.urlscan) line(`- **URLScan:** ${e.urlscan.verdict || '—'} · skor ${e.urlscan.score}`)
  }

  if (clusters.value.length) {
    sec('Klaster Operator')
    clusters.value.forEach((op: any) => {
      line(`### ${op.name} — ${op.site_count} situs`)
      list(op.evidence ?? [], (ev: any) => `${ev.attr} (${ev.sites} situs)`)
      line()
    })
  }

  if (relations.value?.related?.length) {
    sec('Situs Terkait')
    list(relations.value.related, (r: any) => `${stripScheme(r.url)} — ${r.shared_count} atribut sama · ${r.shared_kinds.join(', ')}${r.is_gambling ? ' · judol' : ''}`)
  }

  if (apks.value.length) {
    sec('Analisis APK')
    apks.value.forEach((ap: any) => {
      line(`- **${ap.package_name || ap.url}** · ${ap.app_label || '—'} v${ap.version_name || '—'}`)
      if (ap.cert_sha256) line(`  - Sertifikat: ${ap.cert_sha256}`)
      if (ap.domains?.length) line(`  - Domain di kode: ${ap.domains.join(', ')}`)
    })
  }

  if (changes.value.length) {
    sec('Perubahan Antar Scan')
    changes.value.forEach((ch: any) => {
      line(`### ${fmtDate(ch.at)}`)
      Object.entries(ch.added ?? {}).forEach(([cat, vals]: any) => line(`- + ${cat}: ${vals.join(', ')}`))
      Object.entries(ch.removed ?? {}).forEach(([cat, vals]: any) => line(`- − ${cat}: ${vals.join(', ')}`))
      line()
    })
  }

  line(); line('---'); line('_Dihasilkan oleh Jutec — hanya untuk keperluan investigasi internal._')

  const blob = new Blob([L.join('\n')], { type: 'text/markdown;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `laporan-${host.value}-${new Date().toISOString().slice(0, 10)}.md`
  a.click()
  URL.revokeObjectURL(a.href)
  toast.add({ title: 'Laporan diekspor', description: a.download, icon: 'i-lucide-file-down', color: 'emerald' })
}
// Ekspor PDF: laporan A4 lengkap lewat dialog cetak browser ("Simpan sebagai PDF")
function reportHtml() {
  return buildSiteReportHtml({
    host: host.value, site: site.value, whois: detail.value?.whois, enrichment: detail.value?.enrichment,
    kindMeta, facts: facts.value, pending: collecting.value.map(c => c.text.replace(/…$/, '')),
    attributes: detail.value?.attributes ?? [], keyLinks: keyLinks.value, auth: auth.value, payments: payments.value,
    identities: identities.value, tgAccounts: tgAccounts.value, clusters: clusters.value, related: relations.value?.related ?? [],
    apks: apks.value, changes: changes.value, wayback: wayback.value, scans: detail.value?.scans ?? [],
    reachHistory: reachHistory.value, reachLabels: REACH, evidence: evidence.value,
    evidenceShot: ev => evFileURL(ev, 'screenshot-desktop.png'), gameProviders: gameProviders.value, operator: operator.value ?? '',
    revAnalytics: revResults.value, wallets: wallets.value,
    graphSvg: buildRelationSvg(relations.value, kindMeta, { theme: 'light', width: 680 }),
    score: scoreData.value, pivots: pivots.value, timeline: timeline.value, confidenceOf,
  })
}
async function exportPdf() {
  const html = reportHtml()
  try {
    await printSiteReport(html)
    toast.add({ title: 'Laporan PDF siap', description: 'Pilih "Simpan sebagai PDF" di dialog cetak.', icon: 'i-lucide-file-text', color: 'emerald' })
  } catch (e: any) {
    toast.add({ title: 'Gagal membuat PDF', description: e.message, color: 'rose', icon: 'i-lucide-circle-alert' })
  }
}
// Mode cetak langsung (/site/:id?export=pdf): setelah semua data terkumpul, halaman diganti dengan laporan A4.
// Dipakai untuk tautan cetak & pembuatan PDF otomatis (mis. Chrome headless --print-to-pdf).
const exportMode = route.query.export === 'pdf'
watch(() => [collecting.value.length, !!detail.value, !!relations.value] as const, ([n, hasDetail, hasRel]) => {
  if (!exportMode || n || !hasDetail || !hasRel) return
  nextTick(() => { const html = reportHtml(); document.open(); document.write(html); document.close() })
})

function fmtDate(d?: string) { return d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—' }

// Atribut → tipe di halaman Reverse Lookup
function lookupType(a: any): string | null {
  switch (a.kind) {
    case 'ip': return 'IP'
    case 'ns': return 'NS'
    case 'ssl': return 'SSL'
    case 'fingerprint': return 'FP'
    case 'artifact': return a.group
    case 'tracking': return ({ GoogleAnalytics: 'GA', GoogleTag: 'GA', GoogleTagManager: 'GTM', FacebookPixel: 'FB' } as any)[a.group] ?? null
    default: return null
  }
}
function openLookup(a: any) {
  const t = lookupType(a)
  if (t) navigateTo(`/reverse-lookup?type=${encodeURIComponent(t)}&v=${encodeURIComponent(a.value)}`)
  else if (a.kind === 'link') window.open(a.value.startsWith('http') ? a.value : `https://${a.value}`, '_blank')
}

async function load(id: string) {
  loading.value = true; error.value = ''; relations.value = null
  try {
    const d = await get<any>(`/sites/${id}`)
    d.attributes ??= []
    d.scans ??= []
    detail.value = d
    useHead({ title: `${host.value} — Jutec` })
    const rel = await get<any>(`/sites/${id}/relations`)
    rel.attributes ??= []
    rel.related ??= []
    relations.value = rel
    loadExtras(id)
    try {
      const ts = Number(sessionStorage.getItem(`jutec_deep_${id}`))
      if (ts && Date.now() - ts < 6 * 60_000) watchDeep(id, ts)
    } catch {}
    openKinds.value = new Set(treeGroups.value.filter(g => g.linked).map(g => g.kind))
  } catch (e: any) {
    error.value = e.message === 'situs tidak ditemukan' ? 'Situs tidak ditemukan.' : e.message
  } finally { loading.value = false }
}

watch(() => route.params.id as string, id => id && load(id), { immediate: true })
</script>

<style scoped>
.tree-node > summary { list-style: none; }
.tree-node > summary::-webkit-details-marker { display: none; }
.tree-summary { @apply flex flex-wrap items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm cursor-pointer hover:bg-slate-800/50 select-none; }
.tree-children { @apply ml-4 pl-3 border-l border-slate-800 space-y-0.5 py-1; }
</style>
