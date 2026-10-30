// Dialog konfirmasi bergaya aplikasi — pengganti window.confirm().
// Pemakaian: if (!(await confirm({ title: 'Hapus?', tone: 'danger' }))) return
export interface ConfirmOptions {
  title: string
  description?: string
  details?: string[]          // poin tambahan (mis. apa yang akan terjadi)
  confirmLabel?: string
  cancelLabel?: string
  tone?: 'danger' | 'warning' | 'primary'
  icon?: string
}

interface ConfirmState extends ConfirmOptions { open: boolean }

let resolver: ((ok: boolean) => void) | null = null

export const useConfirm = () => {
  const state = useState<ConfirmState>('confirm-dialog', () => ({ open: false, title: '' }))

  function confirm(opts: ConfirmOptions): Promise<boolean> {
    resolver?.(false) // dialog sebelumnya (bila ada) dianggap batal
    state.value = { tone: 'primary', confirmLabel: 'Lanjutkan', cancelLabel: 'Batal', ...opts, open: true }
    return new Promise(res => { resolver = res })
  }
  function settle(ok: boolean) {
    state.value = { ...state.value, open: false }
    resolver?.(ok)
    resolver = null
  }
  return { state, confirm, settle }
}
