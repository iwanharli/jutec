// Analisis turunan untuk halaman detail situs — dipakai bersama oleh tampilan & ekspor PDF
// agar angka di layar dan di laporan selalu sama.

const pct = (n: number) => `${Math.round(n * 100)}%`
const DAY = 86_400_000

// ── 1. Penjelasan skor ──
// Rumus mengikuti intelligence.Compute di backend:
// final = konten·0.45 + jaringan·0.30 + domain·0.25 + reputasi + hostname, lalu aturan prioritas bisa menaikkan batas bawah.
export const SCORE_RULES: Record<string, string> = {
  'game_provider+artifact': 'Provider game terdeteksi + ada artefak transaksi → High, skor ditetapkan 97%',
  'content+artifact': 'Konten judol kuat (≥70%) + artefak transaksi → High, skor minimal 88%',
  'template+artifact': 'Template mirip situs judol yang dikenal + artefak transaksi → High, skor minimal 85%',
  'enrichment+artifact': 'Reputasi buruk (URLScan/VirusTotal) + artefak transaksi → High, skor minimal 82%',
  'hostname+artifact': 'Nama domain mengandung istilah judol + artefak transaksi → High, skor minimal 80%',
  strong_signal: 'Satu sinyal kuat (skor ≥75%, provider game, konten kuat, atau reputasi buruk) → High',
  medium_signal: 'Sinyal sedang (skor ≥45%, TLD berisiko, kata judi di URL/hostname, atau template mirip) → Medium',
  not_gambling_cap: 'Sinyal kuat tetapi tidak dinilai judol (mis. situs pemerintah/berita) → dibatasi Medium',
  low: 'Tidak ada sinyal yang cukup → Low',
}

export interface ScoreRow { key: string; label: string; raw: number; weight: string; contrib: number; detail: string }

export function explainScore(site: any, explain: any, whois: any, gameProviders: string[]) {
  if (!explain?.score) return null
  const b = explain.score.breakdown ?? {}
  const kw: string[] = explain.keywords ?? []
  const rows: ScoreRow[] = [
    { key: 'content', label: 'Konten halaman', raw: b.content ?? 0, weight: '× 0.45', contrib: (b.content ?? 0) * 0.45,
      detail: [kw.length ? `${kw.length} kata kunci: ${kw.slice(0, 8).join(', ')}${kw.length > 8 ? '…' : ''}` : 'tidak ada kata kunci',
        b.url_path ? `kata judi di URL +${pct(b.url_path)}` : ''].filter(Boolean).join(' · ') },
    { key: 'network', label: 'Jaringan & template', raw: b.network ?? 0, weight: '× 0.30', contrib: (b.network ?? 0) * 0.30,
      detail: [gameProviders.length ? `provider game: ${gameProviders.join(', ')}` : '', b.simhash ? `template mirip situs judol (${pct(b.simhash)})` : '']
        .filter(Boolean).join(' · ') || 'tidak ada panggilan API provider game' },
    { key: 'domain', label: 'Risiko domain', raw: b.domain ?? 0, weight: '× 0.25', contrib: (b.domain ?? 0) * 0.25,
      detail: [whois?.domain_age_days >= 0 ? `umur ${whois.domain_age_days} hari` : '', whois?.is_private ? 'WHOIS privat' : '',
        b.tld_risk ? `TLD berisiko (${pct(b.tld_risk)})` : ''].filter(Boolean).join(' · ') || 'tidak ada data WHOIS' },
    { key: 'enrichment', label: 'Reputasi eksternal', raw: b.enrichment ?? 0, weight: '+ langsung', contrib: b.enrichment ?? 0,
      detail: b.enrichment ? 'URLScan / VirusTotal menandai berbahaya' : 'tidak ada tanda dari URLScan / VirusTotal' },
    { key: 'hostname', label: 'Istilah judol di hostname', raw: b.hostname ?? 0, weight: '+ langsung', contrib: b.hostname ?? 0,
      detail: b.hostname ? 'nama domain sengaja memakai istilah judol' : '—' },
  ]
  const sum = Math.min(1, rows.reduce((s, r) => s + r.contrib, 0))
  const final = explain.score.final ?? site.confidence ?? 0
  const rule = explain.score.rule as string | undefined
  const notes: string[] = []
  if (final - sum > 0.01) notes.push(`Aturan prioritas menaikkan skor dari ${pct(sum)} ke ${pct(final)}.`)
  if (!rule) notes.push('Scan ini dibuat sebelum aturan prioritas dicatat — scan ulang untuk melihat aturan yang dipakai.')
  if (site.verified_status) notes.push(`Status akhir ditetapkan manual: ${site.verified_status === 'judol' ? 'JUDOL' : 'BUKAN JUDOL'}${site.verified_by ? ` oleh ${site.verified_by}` : ''} — mengalahkan skor otomatis.`)
  if (Math.abs(final - (site.confidence ?? 0)) > 0.01) notes.push(`Skor situs saat ini (${pct(site.confidence ?? 0)}) berbeda dari scan lengkap terakhir — kemungkinan scan berikutnya gagal/tidak lengkap.`)
  return {
    rows, sum, final, priority: explain.score.priority as string, rule, ruleText: rule ? SCORE_RULES[rule] ?? rule : '',
    keywords: kw, patterns: (explain.patterns ?? []) as string[], scannedAt: explain.scanned_at as string, notes,
    top: [...rows].sort((a, b) => b.contrib - a.contrib)[0],
  }
}

// ── 3. Atribut paling unik (pivot) ──
// Kekuatan = seberapa khas jenis atributnya × seberapa jarang dipakai situs lain.
// Rekening yang dipakai 2 situs jauh lebih bermakna daripada IP Cloudflare yang dipakai 3.000 situs.
const OWNER_GROUPS = new Set(['ReCaptcha', 'GoogleAPIKey', 'FirebaseProject', 'AdSense', 'TelegramBot', 'TelegramChat', 'CloudflareAnalytics',
  'Clarity', 'TikTokPixel', 'YandexMetrica', 'OneSignal', 'Sentry', 'Jivo', 'Tidio', 'Zendesk', 'Smartsupp'])
export function attrWeight(a: any): number {
  switch (a.kind) {
    case 'operator': return 1
    case 'artifact': return ['Contact', 'Phone'].includes(a.group) ? 0.9 : 1
    case 'tracking': return OWNER_GROUPS.has(a.group) ? 0.95 : 0.8
    case 'fingerprint': return ({ api: 0.85, apk_cert: 0.95, apk_pkg: 0.85, referral: 0.9, favicon: 0.6, cdn: 0.25 } as any)[a.group] ?? 0.6
    case 'app': return 0.75
    case 'ssl': return 0.6
    case 'txt': return 0.55
    case 'link': return 0.4
    case 'ip': return 0.35
    case 'ns': return 0.1
    default: return 0.3
  }
}
export function rankPivots(attrs: any[], kindMeta: Record<string, { label: string }>, limit = 6) {
  return attrs
    .filter(a => a.shared_sites > 0 && a.shared_sites <= 40)
    .map(a => {
      const strength = attrWeight(a) / (1 + Math.log2(a.shared_sites))
      return {
        ...a, strength,
        level: strength >= 0.6 ? 'kuat' : strength >= 0.3 ? 'sedang' : 'lemah',
        kindLabel: `${kindMeta[a.kind]?.label ?? a.kind}${a.group && a.kind !== 'link' ? ` · ${a.group}` : ''}`,
        why: `dipakai ${a.shared_sites} situs lain · bobot jenis ${Math.round(attrWeight(a) * 100)}%`,
      }
    })
    .sort((a, b) => b.strength - a.strength)
    .slice(0, limit)
}

// ── 5. Tingkat keyakinan per temuan ──
export type Confidence = { level: 'tinggi' | 'sedang' | 'rendah'; source: string; why: string }
export function attrConfidence(a: any, ctx: { explain: any; tgAccounts: any[]; apks: any[] }): Confidence {
  const inPage = new Set((ctx.explain?.artifact_values ?? []).map((v: string) => String(v).toLowerCase()))
  const technical = (source: string): Confidence => ({ level: 'tinggi', source, why: 'Diambil langsung dari respons server / DNS — tidak bergantung tafsiran.' })
  switch (a.kind) {
    case 'artifact': {
      if (String(a.label).toLowerCase() === 'telegram' || /t\.me|^@/.test(a.value)) {
        const u = String(a.value).replace(/^@|.*t\.me\//g, '').toLowerCase()
        const tg = ctx.tgAccounts.find(x => String(x.username).toLowerCase() === u)
        if (tg?.status === 'valid') return { level: 'tinggi', source: 'Telegram (terverifikasi)', why: 'Akun dicek ke Telegram dan memang ada.' }
        if (tg?.status === 'not_found') return { level: 'rendah', source: 'Telegram (tidak ditemukan)', why: 'Akun tidak ditemukan di Telegram — mungkin sudah dihapus atau salah baca.' }
      }
      if (inPage.has(String(a.value).toLowerCase())) return { level: 'tinggi', source: 'Teks halaman', why: 'Tertulis di HTML halaman pada scan lengkap terakhir.' }
      if (a.group === 'QRIS') return { level: 'tinggi', source: 'Dekode QRIS', why: 'Dibaca dari kode QR di gambar — data merchant tertanam di kode, bukan tebakan.' }
      if (!ctx.explain) return { level: 'sedang', source: 'Tidak diketahui', why: 'Belum ada scan lengkap untuk dibandingkan.' }
      return { level: 'sedang', source: 'OCR gambar / scan lama', why: 'Tidak ada di teks halaman terakhir: hasil baca gambar (bisa salah baca) atau sudah dihapus dari situs. Cek ulang sebelum dipakai.' }
    }
    case 'link':
      return String(a.group).includes('register')
        ? { level: 'tinggi', source: 'Tujuan tombol daftar', why: 'Diikuti dari tombol DAFTAR/LOGIN, termasuk hasil shortener.' }
        : { level: 'sedang', source: 'Tautan di halaman', why: 'Hanya ditautkan — belum tentu milik operator yang sama.' }
    case 'app': {
      const analyzed = ctx.apks.some(x => x.url === a.value && x.package_name)
      return analyzed ? { level: 'tinggi', source: 'APK dianalisis', why: 'File berhasil diunduh dan dibongkar.' }
        : { level: 'sedang', source: 'Link unduhan', why: 'Link tercantum, file belum berhasil dianalisis.' }
    }
    case 'fingerprint': return a.group === 'cdn'
      ? { level: 'rendah', source: 'CDN', why: 'CDN dipakai banyak situs tak terkait — hanya pendukung.' }
      : technical('Sidik jari teknis')
    case 'ip': case 'ns': case 'ssl': case 'txt': return technical('DNS / sertifikat')
    case 'tracking': case 'operator': return technical('Kode di halaman')
    default: return { level: 'sedang', source: '—', why: '' }
  }
}
export const CONF_COLOR: Record<Confidence['level'], string> = { tinggi: 'emerald', sedang: 'amber', rendah: 'rose' }

// ── 2. Timeline gabungan ──
export type TimelineKind = 'domain' | 'scan' | 'perubahan' | 'akses' | 'arsip' | 'bukti' | 'aktivitas'
export interface TimelineEvent { at: number; kind: TimelineKind; title: string; detail?: string; tone: 'rose' | 'amber' | 'emerald' | 'sky' | 'violet' | 'gray'; future?: boolean }
export const TIMELINE_KINDS: Record<TimelineKind, { label: string; icon: string }> = {
  domain: { label: 'Domain', icon: 'i-lucide-globe' },
  scan: { label: 'Scan', icon: 'i-lucide-scan-line' },
  perubahan: { label: 'Perubahan', icon: 'i-lucide-git-compare' },
  akses: { label: 'Akses & blokir', icon: 'i-lucide-shield-ban' },
  arsip: { label: 'Arsip Wayback', icon: 'i-lucide-archive' },
  bukti: { label: 'Bukti', icon: 'i-lucide-file-lock-2' },
  aktivitas: { label: 'Aktivitas analis', icon: 'i-lucide-user-round' },
}
const REACH_LABEL: Record<string, string> = { online: 'Online', blocked_alive: 'Diblokir (server masih hidup)', blocked: 'Diblokir', offline: 'Offline', nxdomain: 'Domain hilang' }

export function buildTimeline(d: {
  site: any; whois: any; scans: any[]; changes: any[]; wayback: any[]; reachHistory: any[]; evidence: any[]; audit: any[]
}): TimelineEvent[] {
  const ev: TimelineEvent[] = []
  const t = (x?: string) => (x ? new Date(x).getTime() : NaN)
  const add = (at: number, e: Omit<TimelineEvent, 'at'>) => { if (!isNaN(at)) ev.push({ at, ...e, future: at > Date.now() }) }
  const s = d.site ?? {}, w = d.whois

  if (w?.registered_at) add(t(w.registered_at), { kind: 'domain', tone: 'violet', title: 'Domain didaftarkan', detail: [w.registrar, w.is_private && 'WHOIS privat'].filter(Boolean).join(' · ') })
  if (w?.expires_at) add(t(w.expires_at), { kind: 'domain', tone: 'gray', title: 'Domain kedaluwarsa', detail: w.registrar })
  add(t(s.created_at), { kind: 'domain', tone: 'sky', title: 'Pertama tercatat di Jutec' })

  // Scan: tampilkan yang pertama, yang gagal, dan yang mengubah prioritas — sisanya hanya mengulang
  const scans = [...d.scans].sort((a, b) => t(a.created_at) - t(b.created_at))
  scans.forEach((sc, i) => {
    const prev = scans[i - 1]
    if (i > 0 && sc.status === prev.status && sc.priority === prev.priority) return
    add(t(sc.created_at), sc.status === 'ok'
      ? { kind: 'scan', tone: sc.priority === 'High' ? 'rose' : sc.priority === 'Medium' ? 'amber' : 'emerald',
          title: i === 0 ? `Scan pertama · ${sc.priority}` : `Prioritas berubah: ${prev.priority || '—'} → ${sc.priority}`, detail: sc.score != null ? `skor ${pct(sc.score)}` : '' }
      : { kind: 'scan', tone: 'gray', title: 'Scan gagal — situs tidak terjangkau' })
  })
  if (scans.length > 1) {
    const last = scans[scans.length - 1]
    if (!ev.some(e => e.kind === 'scan' && e.at === t(last.created_at)))
      add(t(last.created_at), { kind: 'scan', tone: 'gray', title: `Scan terakhir · ${last.priority || last.status}`, detail: `${scans.length} scan tercatat` })
  }

  for (const ch of d.changes) {
    const parts = [
      ...Object.entries(ch.added ?? {}).map(([c, v]: any) => `+${v.length} ${c}`),
      ...Object.entries(ch.removed ?? {}).map(([c, v]: any) => `−${v.length} ${c}`),
    ]
    const vals = [...Object.values(ch.added ?? {}), ...Object.values(ch.removed ?? {})].flat() as string[]
    add(t(ch.at), { kind: 'perubahan', tone: 'amber', title: `Konten berubah: ${parts.join(', ')}`, detail: vals.slice(0, 4).join(' · ') })
  }

  // Keterjangkauan: hanya transisi status
  const reach = [...d.reachHistory].sort((a, b) => t(a.at) - t(b.at))
  reach.forEach((r, i) => {
    if (i > 0 && reach[i - 1].status === r.status) return
    add(t(r.at), { kind: 'akses', tone: r.status === 'online' ? 'emerald' : r.status === 'blocked_alive' ? 'rose' : 'gray',
      title: i === 0 ? `Status akses: ${REACH_LABEL[r.status] ?? r.status}` : `${REACH_LABEL[reach[i - 1].status] ?? reach[i - 1].status} → ${REACH_LABEL[r.status] ?? r.status}`,
      detail: r.http_status ? `HTTP ${r.http_status}` : '' })
  })
  if (s.revived_at) add(t(s.revived_at), { kind: 'akses', tone: 'rose', title: 'Situs hidup lagi setelah tidak aktif', detail: 'operator kemungkinan memakai ulang domain lama' })
  if (s.lifecycle === 'inactive' && s.offline_since) add(t(s.offline_since), { kind: 'akses', tone: 'gray', title: 'Mulai tidak bisa diakses', detail: 'dianggap tidak aktif setelah 14 hari' })
  if (s.blocked_since && !reach.some(r => Math.abs(t(r.at) - t(s.blocked_since)) < 60_000))
    add(t(s.blocked_since), { kind: 'akses', tone: 'rose', title: 'Pertama terdeteksi diblokir ISP' })

  for (const wb of d.wayback) {
    const ts = String(wb.timestamp)
    const at = Date.UTC(+ts.slice(0, 4), +ts.slice(4, 6) - 1, +ts.slice(6, 8) || 1)
    const n = (wb.artifacts ?? []).length
    add(at, { kind: 'arsip', tone: 'gray', title: `Snapshot Wayback${n ? ` · ${n} artefak` : ''}`,
      detail: (wb.artifacts ?? []).slice(0, 3).map((a: any) => `${a.name || a.type}: ${a.value}`).join(' · ') })
  }
  for (const e of d.evidence.filter(x => x.status === 'done'))
    add(t(e.captured_at || e.created_at), { kind: 'bukti', tone: 'emerald', title: `Bukti forensik #${e.id} diambil`, detail: [e.requested_by, e.manifest_sha256?.slice(0, 16) + '…'].filter(Boolean).join(' · ') })
  if (s.verified_at) add(t(s.verified_at), { kind: 'aktivitas', tone: s.verified_status === 'judol' ? 'rose' : 'emerald',
    title: `Diverifikasi manual: ${s.verified_status === 'judol' ? 'JUDOL' : 'bukan judol'}`, detail: [s.verified_by, s.verified_note].filter(Boolean).join(' · ') })
  for (const a of d.audit) {
    if (a.status >= 400) continue
    add(t(a.at), { kind: 'aktivitas', tone: 'gray', title: a.action, detail: a.actor === 'anonymous' ? 'Tanpa nama' : a.actor })
  }
  return ev.sort((a, b) => b.at - a.at)
}

// ── 4. Profil operator ──
export function operatorProfile(op: any) {
  const m: any[] = op.members ?? []
  const t = (x?: string) => (x ? new Date(x).getTime() : NaN)
  const blocked = m.filter(x => x.reach_status === 'blocked' || x.reach_status === 'blocked_alive')
  const created = m.map(x => t(x.created_at)).filter(x => !isNaN(x)).sort((a, b) => a - b)
  // Kecepatan respawn: jarak dari sebuah situs diblokir hingga situs baru klaster yang sama muncul
  const gaps = blocked.map(b => {
    const bt = t(b.blocked_since)
    const next = created.find(c => c > bt)
    return isNaN(bt) || next == null ? null : (next - bt) / DAY
  }).filter((x): x is number => x != null).sort((a, b) => a - b)
  const median = gaps.length ? gaps[Math.floor(gaps.length / 2)] : null
  const recent = created.filter(c => c > Date.now() - 30 * DAY).length
  return {
    total: m.length || op.site_count,
    judol: m.filter(x => x.is_gambling).length,
    online: m.filter(x => x.reach_status === 'online').length,
    blockedAlive: m.filter(x => x.reach_status === 'blocked_alive').length,
    blocked: blocked.length,
    dead: m.filter(x => x.reach_status === 'offline' || x.reach_status === 'nxdomain').length,
    unchecked: m.filter(x => !x.reach_status).length,
    firstSeen: created[0] ?? null,
    lastSeen: created[created.length - 1] ?? null,
    recent,
    respawnDays: median,
    respawnSamples: gaps.length,
    sharedArtifacts: (op.shared_artifacts ?? []) as any[],
    members: m,
  }
}
