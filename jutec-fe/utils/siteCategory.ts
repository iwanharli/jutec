// Kategori situs — satu definisi untuk kolom "Jenis" di tabel dan header halaman detail.
// Nilai dihitung backend (siteCategorySQL di store/dashboard.go).
export const SITE_CATEGORY: Record<string, { label: string; icon: string; tag: string; on: string; desc: string }> = {
  betting:  { label: 'Betting', icon: 'i-lucide-dices', desc: 'Situs judi utama: form login/daftar sendiri, provider game, atau meminta rekening',
    tag: 'bg-rose-500/10 text-rose-300 ring-rose-500/30', on: 'bg-rose-500/15 text-rose-200 ring-rose-500/40' },
  landing:  { label: 'Landing Page', icon: 'i-lucide-door-open', desc: 'Pintu masuk: tombol daftar/login mengarah ke domain lain',
    tag: 'bg-sky-500/10 text-sky-300 ring-sky-500/30', on: 'bg-sky-500/15 text-sky-200 ring-sky-500/40' },
  injected: { label: 'Disusupi (korban)', icon: 'i-lucide-bug', desc: 'Situs sah yang dibobol: konten judol disusupkan (halaman sisipan, cloaking, link tersembunyi, atau parasite SEO di subdomain/path)',
    tag: 'bg-orange-500/10 text-orange-300 ring-orange-500/30', on: 'bg-orange-500/15 text-orange-200 ring-orange-500/40' },
  info:     { label: 'Info RTP / Togel', icon: 'i-lucide-chart-column', desc: 'Situs pendukung: RTP, prediksi, bocoran, live draw, paito',
    tag: 'bg-violet-500/10 text-violet-300 ring-violet-500/30', on: 'bg-violet-500/15 text-violet-200 ring-violet-500/40' },
  intl:     { label: 'Judi Internasional', icon: 'i-lucide-globe-2', desc: 'Judi luar negeri (ulasan kasino, operator legal, provider game) — tidak menyasar Indonesia, tidak dihitung sebagai judol',
    tag: 'bg-teal-500/10 text-teal-300 ring-teal-500/30', on: 'bg-teal-500/15 text-teal-200 ring-teal-500/40' },
  unclear:  { label: 'Belum Jelas', icon: 'i-lucide-circle-help', desc: 'Terdeteksi judi, perannya belum bisa ditentukan',
    tag: 'bg-slate-700/40 text-slate-300 ring-slate-600', on: 'bg-slate-700/60 text-slate-100 ring-slate-500' },
  clean:    { label: 'Bukan Judi', icon: 'i-lucide-shield-check', desc: 'Tidak terdeteksi judi',
    tag: 'bg-emerald-500/10 text-emerald-300 ring-emerald-500/30', on: 'bg-emerald-500/15 text-emerald-200 ring-emerald-500/40' },
}
