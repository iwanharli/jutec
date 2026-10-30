// Nama penyidik/analis yang sedang memakai aplikasi. Dikirim di setiap request (header X-Jutec-User)
// agar log audit mencatat siapa melakukan apa. Ini penanda, bukan autentikasi.
const KEY = 'jutec_operator'

export function useOperator() {
  // Diisi dari localStorage setelah mount (lihat layout) agar render server & klien sama
  const name = useState<string>('operator', () => '')
  function set(v: string) {
    name.value = v.trim().slice(0, 64)
    try { localStorage.setItem(KEY, name.value) } catch {}
  }
  return { name, set }
}

// Dipakai di luar setup (fetch biasa): baca langsung dari localStorage
export function operatorName(): string {
  if (import.meta.server) return ''
  try { return localStorage.getItem(KEY) ?? '' } catch { return '' }
}
