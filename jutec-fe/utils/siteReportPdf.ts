// Laporan investigasi satu situs sebagai PDF: HTML A4 dirender di iframe tersembunyi lalu dicetak
// (dialog cetak browser → "Simpan sebagai PDF"). Teks tetap vektor & bisa dicari, tanpa dependensi tambahan.

import { SITE_CATEGORY } from './siteCategory'
import { explainScore, operatorProfile, TIMELINE_KINDS, type Confidence, type TimelineEvent } from './siteAnalysis'

export interface SiteReportData {
  host: string
  site: any
  whois: any
  enrichment: any
  kindMeta: Record<string, { label: string; color: string }>
  facts: { title: string; detail?: string }[]
  pending: string[]
  attributes: any[]
  keyLinks: any[]
  auth: any
  payments: { total: number; groups: { cat: string; items: { name: string }[] }[] }
  identities: any[]
  tgAccounts: any[]
  clusters: any[]
  related: any[]
  apks: any[]
  changes: any[]
  wayback: any[]
  scans: any[]
  reachHistory: any[]
  reachLabels: Record<string, { label: string }>
  evidence: any[]
  evidenceShot: (ev: any) => string
  gameProviders: string[]
  operator: string
  score: ReturnType<typeof explainScore>
  pivots: any[]
  graphSvg: string
  revAnalytics: any[]
  wallets: any[]
  timeline: TimelineEvent[]
  confidenceOf: (a: any) => Confidence
}

const esc = (v: any) => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' } as any)[c])
const fmt = (d?: string) => d ? new Date(d).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—'
const fmtDay = (d?: string) => d ? new Date(d).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' }) : '—'
const pct = (n?: number) => `${Math.round((n ?? 0) * 100)}%`
const strip = (u: string) => String(u ?? '').replace(/^https?:\/\//, '').replace(/\/$/, '')
const mono = (v: any) => `<span class="mono">${esc(v)}</span>`

function table(head: string[], rows: string[][], widths: string[] = []) {
  if (!rows.length) return ''
  return `<table><colgroup>${head.map((_, i) => `<col${widths[i] ? ` style="width:${widths[i]}"` : ''}>`).join('')}</colgroup>
    <thead><tr>${head.map(h => `<th>${esc(h)}</th>`).join('')}</tr></thead>
    <tbody>${rows.map(r => `<tr>${r.map(c => `<td>${c}</td>`).join('')}</tr>`).join('')}</tbody></table>`
}
function kv(rows: [string, string][]) {
  const r = rows.filter(([, v]) => v && v !== '—' && v !== esc('—'))
  return r.length ? `<dl class="kv">${r.map(([k, v]) => `<dt>${esc(k)}</dt><dd>${v}</dd>`).join('')}</dl>` : ''
}
const chip = (t: string, tone = 'gray') => `<span class="chip ${tone}">${esc(t)}</span>`
const CONF_TONE = { tinggi: 'green', sedang: 'amber', rendah: 'rose' } as const

export function buildSiteReportHtml(d: SiteReportData) {
  const s = d.site ?? {}, w = d.whois, e = d.enrichment ?? {}
  const by = (k: string) => d.attributes.filter(a => a.kind === k)
  const contacts = by('artifact').filter(a => a.group === 'Contact' || a.group === 'Phone')
  const money = by('artifact').filter(a => !['Contact', 'Phone'].includes(a.group))
  const flagged = by('artifact').filter(a => a.verified_sites?.length)
  const now = new Date()
  const reportId = `JTC-${s.id}-${now.toISOString().slice(0, 10).replace(/-/g, '')}`

  let n = 0
  const sections: string[] = []
  const sec = (title: string, body: string, sub = '') => {
    if (!body.trim()) return
    n++
    sections.push(`<section><h2><span class="num">${String(n).padStart(2, '0')}</span>${esc(title)}</h2>${sub ? `<p class="sub">${esc(sub)}</p>` : ''}${body}</section>`)
  }

  // ── Ringkasan
  sec('Ringkasan Temuan', d.facts.length
    ? `<ul class="facts">${d.facts.map(f => `<li><b>${esc(f.title)}</b>${f.detail ? `<span>${esc(f.detail)}</span>` : ''}</li>`).join('')}</ul>` : '')

  // ── Dasar skor
  const sc = d.score
  sec('Dasar Penilaian Skor', sc ? [
    table(['Sinyal', 'Nilai', 'Bobot', 'Kontribusi', 'Keterangan'], sc.rows.map(r => [
      `<b>${esc(r.label)}</b>`, pct(r.raw), esc(r.weight),
      `<div class="bar"><i style="width:${Math.min(100, r.contrib * 100)}%"></i></div><b>+${pct(r.contrib)}</b>`, esc(r.detail)]), ['22%', '9%', '11%', '16%']),
    kv([['Jumlah sinyal', pct(sc.sum)], ['Skor akhir', `<b>${pct(sc.final)}</b> · prioritas ${esc(sc.priority)}`], ['Aturan prioritas', esc(sc.ruleText)],
      ['Catatan', sc.notes.map(esc).join('<br>')], ['Kata kunci cocok', sc.keywords.map(k => chip(k, 'rose')).join(' ')], ['Pola', sc.patterns.map(k => chip(k)).join(' ')]]),
  ].join('') : '<p class="muted">Belum ada scan lengkap dengan rincian skor.</p>',
  `Dihitung dari scan lengkap ${fmt(sc?.scannedAt)} · rumus: konten×0.45 + jaringan×0.30 + domain×0.25 + reputasi + hostname`)

  // ── Pivot
  sec('Pivot Terkuat', table(['#', 'Atribut', 'Jenis', 'Dipakai', 'Kekuatan'], d.pivots.map((a, i) => [
    String(i + 1), mono(a.value), esc(a.kindLabel), `${esc(a.shared_sites)} situs lain`, chip(a.level, a.level === 'kuat' ? 'rose' : a.level === 'sedang' ? 'amber' : 'gray')]),
    ['6%', '', '24%', '16%', '12%']), 'Atribut paling khas yang juga dipakai situs lain — prioritas penelusuran ke operator')

  // ── Alur pendaftaran
  const authBody = [
    table(['Jenis', 'Tujuan'], d.keyLinks.map(a => [esc(a.label || a.group), mono(a.value)]), ['28%']),
    d.auth ? kv([
      ['Halaman daftar', (d.auth.register_urls ?? []).map(mono).join('<br>')],
      ['Halaman login', (d.auth.login_urls ?? []).map(mono).join('<br>')],
      ['Kode referral', (d.auth.referral_codes ?? []).map(mono).join(', ')],
    ]) : '',
    ...(d.auth?.forms ?? []).map((f: any) => `<div class="box"><div class="box-h">${chip(({ register: 'Form Daftar', login: 'Form Login', change_password: 'Ganti Password' } as any)[f.type] ?? f.type, f.type === 'register' ? 'rose' : 'blue')}
      ${chip(f.method || 'GET')}${f.external_action ? chip('submit ke domain lain', 'amber') : ''}${f.has_captcha ? chip('captcha') : ''}${f.has_otp ? chip('OTP', 'violet') : ''}${f.needs_phone ? chip('wajib no. HP/WA', 'green') : ''}${f.needs_bank ? chip('minta rekening', 'cyan') : ''}</div>
      ${kv([['Endpoint', mono(f.action)], ['Halaman', f.page_url ? mono(f.page_url) : ''], ['Field', (f.fields ?? []).map(esc).join(', ')], ['Bank/e-wallet', (f.bank_options ?? []).map(esc).join(', ')]])}</div>`),
  ].join('')
  sec('Alur Pendaftaran & Login', authBody, 'Tujuan tombol DAFTAR/LOGIN, domain cadangan, dan form yang terbaca')

  const shared = (a: any) => `${esc(a.shared_sites ?? 0)}${a.verified_sites?.length ? `<div>${chip(`${a.verified_sites.length} judol terverifikasi`, 'rose')}</div>` : ''}`
  const conf = (a: any) => { const c = d.confidenceOf(a); return `${chip(c.level, CONF_TONE[c.level])}<div class="muted">${esc(c.source)}</div>` }

  // ── Keuangan
  const finBody = [
    table(['Jenis', 'Nama / Label', 'Nilai', 'Situs lain', 'Keyakinan'], money.map(a => [esc(a.group), esc(a.label || '—'), mono(a.value), shared(a), conf(a)]), ['14%', '22%', '', '15%', '17%']),
    d.payments.total ? `<h3>Pilihan pembayaran di form daftar (${d.payments.total})</h3>${kv(d.payments.groups.map(g => [g.cat, g.items.map(i => esc(i.name)).join(', ')] as [string, string]))}` : '',
  ].join('')
  const usd = (n: number) => `$${Number(n ?? 0).toLocaleString('en-US', { maximumFractionDigits: 2 })}`
  const day = (t: number) => t ? new Date(t).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—'
  const cpTable = (w: any, list: any[]) => table(['Alamat', 'Jumlah', 'Tx', 'Catatan'], (list ?? []).map((cp: any) => [mono(cp.address), usd(cp.amount), esc(cp.count),
    w.linked_sites?.[cp.address] ? chip(`dompet ${w.linked_sites[cp.address].map((s: any) => strip(s.url)).join(', ')}`, 'rose') : '']), ['', '16%', '7%', '28%'])
  const walletBody = d.wallets.map(w => {
    const p = w.profile
    if (!p) return `<div class="box"><b class="mono">${esc(w.address)}</b><div class="muted">${esc(w.error || 'tidak ada data')}</div></div>`
    return `<div class="box"><div class="box-h"><b class="mono">${esc(w.address)}</b>${Object.keys(w.linked_sites ?? {}).length ? chip(`terhubung ${Object.keys(w.linked_sites).length} dompet situs lain`, 'rose') : ''}</div>
      ${p.transfers ? `<div class="mini">${[['Masuk', usd(p.total_in)], ['Keluar', usd(p.total_out)], ['Transfer', `${p.transfers}${p.complete ? '' : ' terbaru'}`],
        ['Aktif', `${day(p.first_at)} – ${day(p.last_at)}`]].map(([k, v]) => `<div><span>${esc(k)}</span><b>${esc(v)}</b></div>`).join('')}</div>
      <h3>Pengirim terbesar (dana masuk)</h3>${cpTable(w, p.top_senders) || '<p class="muted">—</p>'}
      <h3>Penerima terbesar (dana keluar)</h3>${cpTable(w, p.top_receivers) || '<p class="muted">—</p>'}` : '<p class="muted">Belum ada transfer USDT.</p>'}</div>`
  }).join('')
  const finAll = finBody + (walletBody ? `<h3>Jejak dana USDT (TRC20) — sumber: TronGrid</h3>${walletBody}` : '')
  sec('Keuangan', finAll, 'Rekening bank, e-wallet, QRIS & kripto yang tercantum di situs')

  // ── Kontak & identitas
  const owner = by('tracking')
  const idn = d.identities.filter(i => i.kominfo?.length || i.dukcapil?.length || i.related?.length)
  const contactBody = [
    table(['Jenis', 'Kontak', 'Situs lain', 'Keyakinan'], contacts.map(a => [esc(a.label || a.group), mono(a.value), shared(a), conf(a)]), ['20%', '', '15%', '17%']),
    d.tgAccounts.length ? `<h3>Akun Telegram</h3>${table(['Username', 'Status', 'Nama / Info', 'Situs'],
      d.tgAccounts.map(a => [mono('@' + a.username), esc(a.status === 'valid' ? (a.type || 'valid') : a.status === 'not_found' ? 'tidak ditemukan' : 'belum dicek'),
        `${esc(a.name || '—')}${a.members ? ` · ${Number(a.members).toLocaleString('id-ID')} anggota` : ''}${a.bio ? `<div class="muted">${esc(a.bio)}</div>` : ''}`, esc(a.site_count ?? 1)]), ['22%', '16%', '', '10%'])}` : '',
    (s.widget_links ?? []).length ? `<h3>Link widget / CS</h3><ul class="plain">${s.widget_links.map((u: string) => `<li>${mono(u)}</li>`).join('')}</ul>` : '',
    idn.length ? `<h3>Identitas terkait (data bocoran)</h3>${idn.map(it => `<div class="box"><div class="box-h"><b class="mono">${esc(it.phone)}</b>${it.label ? ` <span class="muted">${esc(it.label)}</span>` : ''}</div>
      ${table(['Sumber', 'Detail'], [
        ...(it.kominfo ?? []).map((k: any) => ['KomInfo', `NIK ${mono(k.nik)} · ${esc(k.provider)} · reg ${esc(k.reg_date)} · ${esc(k.source)}`]),
        ...(it.dukcapil ?? []).map((x: any) => ['Dukcapil', `<b>${esc(x.full_name)}</b> · NIK ${mono(x.nik)}<div class="muted">${esc([x.address, x.district, x.location, x.region].filter(Boolean).join(', '))}</div>`]),
        ...(it.related ?? []).map((r: any) => ['NIK sama', `${mono(r.phone)} · ${esc(r.provider)} · reg ${esc(r.reg_date)}`]),
      ], ['18%'])}</div>`).join('')}` : '',
  ].join('')
  sec('Kontak & Identitas', contactBody, 'Kontak operator, akun Telegram, dan identitas yang terhubung')

  // ── Infrastruktur
  const infra = [...by('ip'), ...by('ns'), ...by('ssl'), ...by('txt')]
  const infraBody = [
    infra.length ? `<h3>Hosting & DNS</h3>${table(['Jenis', 'Nilai', 'Situs lain'], infra.map(a => [esc(d.kindMeta[a.kind]?.label ?? a.kind), mono(a.value), esc(a.shared_sites ?? 0)]), ['22%', '', '14%'])}` : '',
    by('fingerprint').length ? `<h3>Sidik jari infrastruktur</h3>${table(['Jenis', 'Nilai', 'Situs lain', 'Keyakinan'], by('fingerprint').map(a => [esc(a.group), mono(a.value), esc(a.shared_sites ?? 0), conf(a)]), ['18%', '', '11%', '18%'])}` : '',
    owner.length || by('operator').length ? `<h3>Tracking, identifier pengelola & operator</h3>${table(['Jenis', 'ID', 'Situs lain'], [...by('operator'), ...owner].map(a => [esc(a.group), mono(a.value), esc(a.shared_sites ?? 0)]), ['22%', '', '14%'])}` : '',
  ].join('')
  sec('Infrastruktur', infraBody, 'IP, nameserver, sertifikat, sidik jari & ID tracking — penanda pengelola yang sama')

  // ── WHOIS & reputasi
  const whoisBody = [
    w ? kv([
      ['Domain', mono(w.domain)], ['Registrar', esc(w.registrar)], ['Terdaftar', esc(fmtDay(w.registered_at))], ['Kedaluwarsa', esc(fmtDay(w.expires_at))],
      ['Umur domain', w.domain_age_days >= 0 ? `${esc(w.domain_age_days)} hari` : ''], ['WHOIS privat', w.is_private ? 'Ya' : 'Tidak'],
      ['Registrant', esc([w.registrant, w.org].filter(Boolean).join(' · '))], ['Email', w.email ? mono(w.email) : ''],
      ['Skor risiko domain', w.risk_score != null ? pct(w.risk_score) : ''],
    ]) : '',
    e.virustotal || e.urlscan ? `<h3>Reputasi</h3>${kv([
      ['VirusTotal', e.virustotal ? `${esc(e.virustotal.malicious)}/${esc(e.virustotal.total)} engine menandai berbahaya` : ''],
      ['URLScan', e.urlscan ? `${esc(e.urlscan.verdict || '—')} · skor ${esc(e.urlscan.score)}` : ''],
    ])}` : '',
  ].join('')
  sec('WHOIS & Reputasi', whoisBody)

  // ── Jaringan
  const netBody = [
    d.graphSvg ? `<figure class="graph">${d.graphSvg}<figcaption>Peta keterkaitan: situs ini (tengah) → atribut → situs lain yang memakai atribut yang sama. Garis putus-putus = atribut umum (CDN, nameserver publik). Label hanya untuk situs dengan keterkaitan terbanyak.</figcaption></figure>` : '',
    d.clusters.length ? `<h3>Klaster operator</h3>${d.clusters.map(op => { const p = operatorProfile(op); const day = (t: number | null) => t ? new Date(t).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—'; return `<div class="box"><div class="box-h"><b>${esc(op.name)}</b> ${chip(`${op.site_count} situs`, 'rose')}</div>
      <div class="mini">${[['Situs', `${p.total} (${p.judol} judol)`], ['Masih hidup', `${p.online + p.blockedAlive}${p.blockedAlive ? ` · ${p.blockedAlive} lolos blokir` : ''}`],
        ['Diblokir / mati', `${p.blocked} / ${p.dead}`], ['Respawn', p.respawnDays == null ? '—' : `${Math.max(1, Math.round(p.respawnDays))} hari (median ${p.respawnSamples})`],
        ['Aktif', `${day(p.firstSeen)} – ${day(p.lastSeen)}`], ['Baru 30 hari', String(p.recent)]].map(([k, v]) => `<div><span>${esc(k)}</span><b>${esc(v)}</b></div>`).join('')}</div>
      ${p.sharedArtifacts.length ? `<div class="muted" style="margin-top:6px">Rekening & kontak dipakai bergantian</div>${table(['Jenis', 'Nilai', 'Situs'], p.sharedArtifacts.slice(0, 15).map(a => [esc(a.name || a.type), mono(a.value), esc(a.sites)]), ['22%', '', '12%'])}` : ''}
      ${(op.evidence ?? []).length ? `<div class="muted" style="margin-top:6px">Bukti penghubung</div><ul class="plain">${op.evidence.map((ev: any) => `<li>${esc(ev.attr)} <span class="muted">· ${esc(ev.sites)} situs</span></li>`).join('')}</ul>` : ''}</div>` }).join('')}` : '',
    d.related.length ? `<h3>Situs terkait (${d.related.length})</h3>${table(['#', 'Situs', 'Atribut sama', 'Status'],
      d.related.map((r, i) => [String(i + 1), `${mono(strip(r.url))}<div class="muted">${esc((r.shared_kinds ?? []).map((k: string) => d.kindMeta[k]?.label ?? k).join(' · '))}</div>`,
        `${chip(`${r.shared_count} sama`, r.strength >= 4 ? 'rose' : r.strength >= 2 ? 'amber' : 'gray')}`, r.is_gambling ? chip('Judol', 'rose') : chip('Normal', 'green')]), ['6%', '', '18%', '14%'])}` : '',
    by('link').length ? `<h3>Domain tertaut</h3><ul class="cols">${by('link').map(a => `<li>${mono(a.value)}</li>`).join('')}</ul>` : '',
  ].join('')
  const revBody = d.revAnalytics.filter(r => r.fetched_at).map(r => `<div class="box"><div class="box-h"><b class="mono">${esc(r.value)}</b>${chip(r.platform)}${chip(`${r.domains.length} domain`, 'blue')}${
      r.domains.filter((x: any) => x.site?.is_gambling).length ? chip(`${r.domains.filter((x: any) => x.site?.is_gambling).length} judol di Jutec`, 'rose') : ''}<span class="muted">dicek ${esc(day(new Date(r.fetched_at).getTime()))}</span></div>
      ${r.error ? `<p class="muted">${esc(r.error)}</p>` : r.domains.length ? `<ul class="cols">${r.domains.map((x: any) => `<li>${mono(x.domain)} <span class="muted">${x.site ? (x.site.is_gambling ? '· judol' : '· normal') : '· baru'}</span></li>`).join('')}</ul>` : '<p class="muted">Tidak ada domain lain.</p>'}</div>`).join('')
  const netAll = netBody + (revBody ? `<h3>ID analytics di situs lain — sumber: HackerTarget</h3>${revBody}` : '')
  sec('Jaringan & Relasi', netAll, 'Situs lain yang berbagi atribut dengan situs ini')

  // ── Konten & aplikasi
  const contentBody = [
    by('app').length ? `<h3>Link aplikasi</h3><ul class="plain">${by('app').map(a => `<li>${mono(a.value)}</li>`).join('')}</ul>` : '',
    d.apks.length ? `<h3>Analisis APK</h3>${d.apks.map(ap => `<div class="box">${kv([
      ['Paket', mono(ap.package_name || '—')], ['Aplikasi', `${esc(ap.app_label || '—')} v${esc(ap.version_name || '—')}`], ['Sumber', mono(ap.url)],
      ['Sertifikat', ap.cert_sha256 ? mono(ap.cert_sha256) : ''], ['Izin', (ap.permissions ?? []).map(esc).join(', ')],
      ['Domain di kode', (ap.domains ?? []).map(mono).join(', ')], ['Galat', esc(ap.error)],
    ])}</div>`).join('')}` : '',
    d.gameProviders.length ? `<h3>Provider game</h3><p>${d.gameProviders.map(g => chip(g)).join(' ')}</p>` : '',
    (s.internal_pages ?? []).length ? `<h3>Halaman internal (${s.internal_pages.length})</h3><ul class="cols">${s.internal_pages.map((p: string) => `<li>${mono(strip(p))}</li>`).join('')}</ul>` : '',
  ].join('')
  sec('Konten & Aplikasi', contentBody)

  // ── Riwayat
  const histBody = [
    d.timeline.length ? `<h3>Timeline gabungan</h3>${table(['Waktu', 'Jenis', 'Kejadian'], d.timeline.map(ev => [
      esc(new Date(ev.at).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })) + (ev.future ? ' <span class="muted">(akan datang)</span>' : ''),
      `<span class="dot ${ev.tone}"></span>${esc(TIMELINE_KINDS[ev.kind].label)}`, `<b>${esc(ev.title)}</b>${ev.detail ? `<div class="muted">${esc(ev.detail)}</div>` : ''}`]), ['20%', '16%'])}` : '',
    d.scans.length ? `<h3>Riwayat scan</h3>${table(['Waktu', 'Status', 'Skor', 'Prioritas'],
      d.scans.map(sc => [esc(fmt(sc.created_at)), esc(sc.status), sc.score != null ? pct(sc.score) : 'tidak terjangkau', esc(sc.priority || '—')]))}` : '',
    d.reachHistory.length ? `<h3>Keterjangkauan</h3>${table(['Waktu', 'Status', 'HTTP'],
      d.reachHistory.map(x => [esc(fmt(x.at)), esc(d.reachLabels[x.status]?.label ?? x.status), esc(x.http_status || '—')]))}` : '',
    d.changes.length ? `<h3>Perubahan antar scan</h3>${table(['Waktu', 'Perubahan'], d.changes.map(ch => [esc(fmt(ch.at)), [
      ...Object.entries(ch.added ?? {}).map(([c, v]: any) => `<div><span class="add">+ ${esc(c)}</span> ${v.map(mono).join(', ')}</div>`),
      ...Object.entries(ch.removed ?? {}).map(([c, v]: any) => `<div><span class="del">− ${esc(c)}</span> ${v.map(mono).join(', ')}</div>`),
    ].join('')]), ['22%'])}` : '',
    d.wayback.length ? `<h3>Arsip Wayback</h3>${table(['Snapshot', 'Artefak & pendaftaran'], d.wayback.map(x => [mono(x.timestamp),
      `${(x.artifacts ?? []).map((a: any) => `${esc(a.name || a.type)}: ${mono(a.value)}`).join('<br>') || '<span class="muted">—</span>'}${(x.register ?? []).length ? `<div class="muted">Daftar → ${x.register.map(esc).join(' · ')}</div>` : ''}`]), ['22%'])}` : '',
  ].join('')
  sec('Riwayat', histBody)

  // ── Bukti forensik
  const done = d.evidence.filter(ev => ev.status === 'done')
  const failed = d.evidence.filter(ev => ev.status === 'error')
  const evNote = !done.length && failed.length
    ? `<div class="warn">Bukti forensik belum berhasil diambil — ${failed.length} percobaan gagal (terakhir ${esc(fmt(failed[0].created_at))}: <span class="mono">${esc(failed[0].error)}</span>). Situs kemungkinan lambat atau memblokir browser otomatis; ambil ulang dari halaman detail atau rekam manual.</div>`
    : failed.length ? `<p class="muted">${failed.length} percobaan pengambilan bukti lain gagal dan tidak disertakan.</p>` : ''
  sec('Bukti Forensik', evNote + done.map(ev => `<div class="box ev">
    <img src="${esc(d.evidenceShot(ev))}" alt="" onerror="this.remove()">
    <div>${kv([['Bukti', `#${esc(ev.id)}`], ['Diambil', esc(fmt(ev.captured_at || ev.created_at))], ['Oleh', esc(ev.requested_by)],
      ['HTTP', esc(ev.http_status)], ['IP server', mono(ev.server_ip)], ['URL akhir', mono(ev.final_url)],
      ['Redirect', (ev.redirects ?? []).map(mono).join(' → ')], ['SHA-256 manifest', mono(ev.manifest_sha256)],
      ['File', (ev.files ?? []).map((f: any) => esc(f.name)).join(', ')]])}</div></div>`).join(''),
    'Rekaman halaman dengan hash SHA-256 — integritas dapat diverifikasi ulang di Jutec')

  const status = s.is_gambling ? { t: 'SITUS JUDOL', c: 'rose' } : { t: 'NORMAL', c: 'green' }
  const kpis = [
    ['Confidence', pct(s.confidence)], ['Prioritas', s.priority || '—'],
    ['Umur domain', w?.domain_age_days >= 0 ? `${w.domain_age_days} hari` : '—'],
    ['Artefak', String(by('artifact').length)], ['Situs terkait', String(d.related.length)], ['Scan', `${(s.rescan_count ?? 0) + 1}×`],
  ]

  return `<!doctype html><html lang="id"><head><meta charset="utf-8"><title>Laporan Investigasi — ${esc(d.host)}</title>
<style>
@page { size: A4; margin: 16mm 14mm 18mm; }
* { box-sizing: border-box; }
html { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
body { margin: 0; font: 9.5pt/1.5 -apple-system, "Segoe UI", Inter, Roboto, Helvetica, Arial, sans-serif; color: #1e293b; background: #fff; }
.mono { font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace; font-size: 8.6pt; overflow-wrap: anywhere; word-break: break-word; }
.muted { color: #64748b; font-size: 8.5pt; }
.cover { background: #0f172a; color: #e2e8f0; border-radius: 10px; padding: 18px 22px; position: relative; overflow: hidden; }
.cover::after { content: ""; position: absolute; right: -40px; top: -40px; width: 180px; height: 180px; border-radius: 50%; background: ${s.is_gambling ? '#f43f5e' : '#10b981'}; opacity: .18; }
.brand { font-size: 8pt; letter-spacing: .18em; text-transform: uppercase; color: #94a3b8; display: flex; justify-content: space-between; }
.cover h1 { margin: 10px 0 2px; font-size: 20pt; line-height: 1.2; color: #fff; overflow-wrap: anywhere; }
.cover .url { color: #94a3b8; }
.cover .chips { margin-top: 10px; display: flex; gap: 6px; flex-wrap: wrap; }
.kpis { display: grid; grid-template-columns: repeat(6, 1fr); gap: 8px; margin: 12px 0 4px; }
.kpi { border: 1px solid #e2e8f0; border-radius: 8px; padding: 8px 10px; }
.kpi span { display: block; font-size: 7.5pt; text-transform: uppercase; letter-spacing: .06em; color: #64748b; }
.kpi b { font-size: 13pt; color: #0f172a; }
.meta { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; font-size: 8.5pt; color: #475569; margin: 8px 0 0; }
.meta b { display: block; color: #0f172a; font-weight: 600; }
.warn { margin-top: 12px; border: 1px solid #fcd34d; background: #fffbeb; color: #92400e; border-radius: 8px; padding: 8px 12px; font-size: 8.8pt; }
.warn ul { margin: 4px 0 0 16px; padding: 0; }
section { margin-top: 18px; }
h2 { font-size: 12.5pt; color: #0f172a; margin: 0 0 2px; padding-bottom: 5px; border-bottom: 2px solid #0f172a; display: flex; align-items: baseline; gap: 8px; break-after: avoid; }
h2 .num { font-size: 9pt; color: #fff; background: ${s.is_gambling ? '#e11d48' : '#059669'}; border-radius: 4px; padding: 1px 6px; font-variant-numeric: tabular-nums; }
.sub { margin: 4px 0 8px; color: #64748b; font-size: 8.6pt; break-after: avoid; }
h3 { font-size: 10pt; color: #334155; margin: 12px 0 6px; break-after: avoid; }
table { width: 100%; border-collapse: collapse; table-layout: fixed; margin: 4px 0 8px; font-size: 8.8pt; }
thead { display: table-header-group; }
th { text-align: left; font-size: 7.6pt; text-transform: uppercase; letter-spacing: .05em; color: #475569; background: #f1f5f9; padding: 5px 8px; border-bottom: 1px solid #cbd5e1; }
td { padding: 5px 8px; border-bottom: 1px solid #e2e8f0; vertical-align: top; overflow-wrap: anywhere; }
tr { break-inside: avoid; }
tbody tr:nth-child(even) td { background: #f8fafc; }
.kv { display: grid; grid-template-columns: 34% 1fr; margin: 4px 0 8px; font-size: 8.8pt; border-top: 1px solid #e2e8f0; }
.kv dt, .kv dd { margin: 0; padding: 4px 8px; border-bottom: 1px solid #e2e8f0; }
.kv dt { color: #64748b; background: #f8fafc; }
.kv dd { overflow-wrap: anywhere; }
.box { border: 1px solid #e2e8f0; border-radius: 8px; padding: 8px 10px; margin: 6px 0; break-inside: avoid; }
.box .kv { margin-bottom: 0; }
.box-h { margin-bottom: 6px; display: flex; gap: 5px; flex-wrap: wrap; align-items: center; }
.ev { display: grid; grid-template-columns: 38% 1fr; gap: 10px; }
.ev img { width: 100%; max-height: 230px; object-fit: cover; object-position: top; border: 1px solid #e2e8f0; border-radius: 6px; }
.chip { display: inline-block; font-size: 7.6pt; font-weight: 600; padding: 1px 7px; border-radius: 999px; background: #e2e8f0; color: #334155; white-space: nowrap; }
.chip.rose { background: #ffe4e6; color: #be123c; } .chip.green { background: #d1fae5; color: #047857; }
.chip.amber { background: #fef3c7; color: #b45309; } .chip.blue { background: #dbeafe; color: #1d4ed8; }
.chip.violet { background: #ede9fe; color: #6d28d9; } .chip.cyan { background: #cffafe; color: #0e7490; }
.cover .chip { background: rgba(255,255,255,.12); color: #e2e8f0; } .cover .chip.rose { background: #e11d48; color: #fff; } .cover .chip.green { background: #059669; color: #fff; }
.facts { list-style: none; margin: 6px 0 0; padding: 0; }
.facts li { padding: 5px 0 5px 14px; border-bottom: 1px dashed #e2e8f0; position: relative; break-inside: avoid; }
.facts li::before { content: ""; position: absolute; left: 0; top: 11px; width: 6px; height: 6px; border-radius: 50%; background: ${s.is_gambling ? '#e11d48' : '#059669'}; }
.facts span { display: block; color: #64748b; font-size: 8.6pt; overflow-wrap: anywhere; }
ul.plain { margin: 4px 0 8px 16px; padding: 0; } ul.plain li { margin: 2px 0; }
ul.cols { columns: 2; column-gap: 18px; margin: 4px 0 8px 16px; padding: 0; } ul.cols li { break-inside: avoid; margin: 1px 0; }
.add { color: #047857; font-weight: 600; } .del { color: #be123c; font-weight: 600; }
.bar { height: 5px; background: #e2e8f0; border-radius: 3px; overflow: hidden; margin: 4px 0 2px; } .bar i { display: block; height: 100%; background: ${s.is_gambling ? '#e11d48' : '#059669'}; }
.mini { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; } .mini div { background: #f8fafc; border-radius: 6px; padding: 4px 8px; }
.mini span { display: block; font-size: 7.4pt; text-transform: uppercase; letter-spacing: .05em; color: #64748b; } .mini b { font-size: 9.5pt; }
.dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; margin-right: 5px; background: #94a3b8; }
.dot.rose { background: #e11d48; } .dot.amber { background: #f59e0b; } .dot.emerald { background: #10b981; } .dot.sky { background: #0ea5e9; } .dot.violet { background: #8b5cf6; }
.warn.alert { border-color: #fda4af; background: #fff1f2; color: #9f1239; }
figure.graph { margin: 6px 0 10px; break-inside: avoid; border: 1px solid #e2e8f0; border-radius: 8px; overflow: hidden; }
figure.graph svg { display: block; width: 100%; height: auto; max-height: 200mm; }
figcaption { font-size: 8pt; color: #64748b; padding: 6px 10px; background: #f8fafc; border-top: 1px solid #e2e8f0; }
footer.end { margin-top: 24px; padding-top: 8px; border-top: 1px solid #cbd5e1; font-size: 8pt; color: #64748b; display: flex; justify-content: space-between; }
</style></head><body>
<div class="cover">
  <div class="brand"><span>Jutec · Laporan Investigasi</span><span>${esc(reportId)}</span></div>
  <h1>${esc(d.host)}</h1>
  <div class="url mono">${esc(s.url)}</div>
  <div class="chips">${chip(status.t, status.c)}${s.priority ? chip(`Prioritas ${s.priority}`) : ''}${s.reach_status ? chip(d.reachLabels[s.reach_status]?.label ?? s.reach_status) : ''}${s.category && s.category !== 'clean' && s.category !== 'unclear' ? chip(SITE_CATEGORY[s.category]?.label.toUpperCase() ?? '', s.category === 'betting' ? 'rose' : 'gray') : ''}${s.app_promo ? chip('Promosi APK') : ''}${s.lifecycle === 'inactive' ? chip('TIDAK AKTIF') : ''}${s.revived_at ? chip(`Hidup lagi ${fmtDay(s.revived_at)}`, 'rose') : ''}${s.verified_status ? chip(`Verifikasi: ${s.verified_status}`) : ''}</div>
</div>
<div class="kpis">${kpis.map(([k, v]) => `<div class="kpi"><span>${esc(k)}</span><b>${esc(v)}</b></div>`).join('')}</div>
<div class="meta">
  <div>Situs #<b>${esc(s.id)}</b></div>
  <div>Pertama tercatat<b>${esc(fmt(s.created_at))}</b></div>
  <div>Scan terakhir<b>${esc(fmt(s.last_scanned_at))}</b></div>
  <div>Dibuat<b>${esc(now.toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }))}${d.operator ? ` · ${esc(d.operator)}` : ''}</b></div>
</div>
${flagged.length ? `<div class="warn alert"><b>Peringatan:</b> ${flagged.length} rekening/kontak situs ini juga dipakai situs yang sudah diverifikasi manual sebagai judol:<ul>${flagged.map(a =>
  `<li>${esc(a.label || a.group)} <span class="mono">${esc(a.value)}</span> → ${a.verified_sites.slice(0, 4).map((v: any) => `<span class="mono">${esc(strip(v.url))}</span>`).join(', ')}${a.verified_sites.length > 4 ? ` +${a.verified_sites.length - 4} lainnya` : ''}</li>`).join('')}</ul></div>` : ''}
${d.pending.length ? `<div class="warn"><b>Catatan:</b> laporan dibuat saat analisis belum selesai — bagian berikut mungkin belum lengkap:<ul>${d.pending.map(p => `<li>${esc(p)}</li>`).join('')}</ul></div>` : ''}
${sections.join('\n')}
<footer class="end"><span>Dihasilkan oleh Jutec — hanya untuk keperluan investigasi internal. RAHASIA.</span><span>${esc(reportId)}</span></footer>
</body></html>`
}

// Render di iframe tersembunyi, tunggu gambar dimuat, lalu buka dialog cetak.
export async function printSiteReport(html: string) {
  const frame = document.createElement('iframe')
  frame.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0;visibility:hidden'
  document.body.appendChild(frame)
  const doc = frame.contentDocument!
  doc.open(); doc.write(html); doc.close()
  await Promise.race([
    Promise.all([...doc.images].map(img => img.complete ? null : new Promise(r => { img.onload = img.onerror = r }))),
    new Promise(r => setTimeout(r, 8000)),
  ])
  await doc.fonts?.ready
  const win = frame.contentWindow!
  win.focus()
  win.print()
  setTimeout(() => frame.remove(), 1000)
}
