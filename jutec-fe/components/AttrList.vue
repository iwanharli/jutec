<template>
  <div>
    <p v-if="!items.length" class="muted">{{ empty }}</p>
    <ul v-else class="divide-y divide-slate-800/60 -my-1.5">
      <li v-for="a in visible" :key="a.kind + a.id" class="py-2 flex items-start gap-2 group/attr">
        <div class="flex-1 min-w-0">
          <p class="mono text-[15px] text-slate-100 break-words [overflow-wrap:anywhere] leading-snug">{{ a.value }}</p>
          <div class="flex flex-wrap items-center gap-1 mt-1">
            <span class="text-[13px] text-slate-500">{{ kindLabel(a) }}</span>
            <UBadge v-if="a.label && a.kind !== 'link'" color="violet" variant="subtle" size="xs">{{ a.label }}</UBadge>
            <UBadge v-if="isRegister(a)" color="rose" variant="subtle" size="xs">Pendaftaran</UBadge>
            <UBadge v-if="a.shared_sites" :color="a.shared_sites > 40 ? 'amber' : 'primary'" variant="subtle" size="xs">
              +{{ a.shared_sites }} situs{{ a.shared_sites > 40 ? ' · umum' : '' }}
            </UBadge>
            <UTooltip v-if="a.verified_sites?.length" :text="'Terverifikasi judol: ' + a.verified_sites.map((s: any) => s.url.replace(/^https?:\/\//, '')).join(', ')"
              :ui="{ width: 'max-w-xs', base: 'h-auto whitespace-normal' }">
              <UBadge color="rose" variant="solid" size="xs">
                <UIcon name="i-lucide-triangle-alert" class="w-3 h-3 mr-1" />{{ a.verified_sites.length }} situs judol terverifikasi
              </UBadge>
            </UTooltip>
            <UTooltip v-if="confidence" :text="confidence(a).why" :ui="{ width: 'max-w-xs', base: 'h-auto whitespace-normal' }">
              <UBadge :color="(CONF_COLOR[confidence(a).level] as any)" variant="outline" size="xs">
                <span class="w-1.5 h-1.5 rounded-full mr-1 bg-current" />{{ confidence(a).source }} · {{ confidence(a).level }}
              </UBadge>
            </UTooltip>
          </div>
        </div>
        <!-- Aksi: ikon ringkas dengan tooltip -->
        <div class="flex flex-wrap justify-end gap-0.5 shrink-0 max-w-[140px] opacity-70 group-hover/attr:opacity-100 transition">
          <UTooltip v-for="act in actions(a)" :key="act.label" :text="act.label">
            <UButton size="2xs" :color="act.color ?? 'gray'" variant="ghost" :icon="act.icon"
              :to="act.to" :target="act.external ? '_blank' : undefined" @click="act.onClick?.()" />
          </UTooltip>
        </div>
      </li>
    </ul>
    <UButton v-if="max && items.length > max" class="mt-2" size="xs" color="gray" variant="ghost"
      :label="expanded ? 'Tampilkan lebih sedikit' : `+${items.length - max} lainnya`" @click="expanded = !expanded" />
  </div>
</template>

<script setup lang="ts">
const { confirm } = useConfirm()
import type { Confidence } from '~/utils/siteAnalysis'
const props = defineProps<{ items: any[]; empty: string; max?: number; confidence?: (a: any) => Confidence }>()
const emit = defineEmits<{ (e: 'lookup', a: any): void }>()
const toast = useToast()

const expanded = ref(false)
const visible = computed(() => (props.max && !expanded.value ? props.items.slice(0, props.max) : props.items))

const labels: Record<string, string> = {
  ip: 'IP Address', ns: 'Nameserver', ssl: 'Sertifikat SSL', txt: 'TXT', tracking: 'Tracking',
  artifact: 'Artefak', link: 'Domain', app: 'Aplikasi', operator: 'Operator', fingerprint: 'Sidik jari',
}
function kindLabel(a: any) {
  if (a.kind === 'link') return `${a.group.replace('register', 'pendaftaran')} · ${a.label} URL`
  if (a.kind === 'fingerprint') return ({ cdn: 'Sidik jari · CDN', favicon: 'Sidik jari · Favicon', api: 'Sidik jari · API backend', referral: 'Sidik jari · Kode referral', apk_cert: 'Sidik jari · Sertifikat APK', apk_pkg: 'Sidik jari · Paket APK' } as any)[a.group] ?? 'Sidik jari'
  return a.group ? `${labels[a.kind] ?? a.kind} · ${a.group}` : labels[a.kind] ?? a.kind
}
const isRegister = (a: any) => a.kind === 'link' && String(a.group).includes('register')

// Atribut yang bisa dicari di Reverse Lookup
function canLookup(a: any) {
  if (a.kind === 'tracking') return ['GoogleAnalytics', 'GoogleTag', 'GoogleTagManager', 'FacebookPixel'].includes(a.group)
  return ['ip', 'ns', 'ssl', 'artifact', 'fingerprint'].includes(a.kind)
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.add({ title: 'Disalin', description: text.length > 80 ? text.slice(0, 80) + '…' : text, icon: 'i-lucide-check', color: 'emerald', timeout: 2000 })
  } catch { toast.add({ title: 'Gagal menyalin', color: 'rose' }) }
}

const digits = (v: string) => v.replace(/\D/g, '').replace(/^0/, '62')
const asUrl  = (v: string) => (/^https?:\/\//.test(v) ? v : `https://${v}`)

type Action = { label: string; icon: string; color?: string; to?: string; external?: boolean; onClick?: () => void }
function actions(a: any): Action[] {
  const acts: Action[] = [{ label: 'Salin', icon: 'i-lucide-copy', onClick: () => copy(a.value) }]
  const contact = a.kind === 'artifact' ? String(a.label).toLowerCase() : ''

  if (a.kind === 'artifact' && (contact === 'whatsapp' || a.group === 'Phone')) {
    acts.push({ label: 'Buka WhatsApp', icon: 'i-lucide-message-circle', color: 'emerald', to: `https://wa.me/${digits(a.value)}`, external: true })
    acts.push({ label: 'Cek Identitas', icon: 'i-lucide-fingerprint', color: 'cyan', to: `/leak?q=${digits(a.value)}` })
  }
  if (a.kind === 'artifact' && contact === 'telegram') {
    acts.push({ label: 'Buka Telegram', icon: 'i-lucide-send', color: 'sky', to: `https://t.me/${a.value.replace(/^@/, '')}`, external: true })
  }
  if (a.kind === 'link' || (a.kind === 'fingerprint' && a.group === 'api')) {
    acts.push({ label: 'Buka', icon: 'i-lucide-external-link', to: asUrl(a.value), external: true })
    acts.push({ label: 'Analisis', icon: 'i-lucide-scan-line', color: 'primary', to: `/discovery?url=${encodeURIComponent(asUrl(a.value))}` })
  }
  if (a.kind === 'app') {
    acts.push({ label: 'Unduh APK', icon: 'i-lucide-download', color: 'amber', onClick: () => {
      confirm({ title: 'Unduh file APK?', tone: 'warning', icon: 'i-lucide-shield-alert', confirmLabel: 'Saya mengerti, unduh',
        description: 'APK dari situs judol bisa berisi malware (pencuri SMS/OTP, akses layar, dan sejenisnya).',
        details: ['Jangan dipasang di HP pribadi atau kantor', 'Analisis hanya di lingkungan terisolasi (sandbox/emulator)'] })
        .then(ok => ok && window.open(a.value, '_blank', 'noopener'))
    } })
  }
  if (a.kind === 'ip') acts.push({ label: 'Info IP', icon: 'i-lucide-map-pin', to: `https://ipinfo.io/${a.value}`, external: true })
  if (canLookup(a)) acts.push({ label: 'Situs lain', icon: 'i-lucide-scan-search', color: 'primary', onClick: () => emit('lookup', a) })
  return acts
}
</script>
