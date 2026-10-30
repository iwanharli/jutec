// ensureOk melempar Error berisi pesan dari backend ({"error": "..."}) bila status bukan 2xx.
async function ensureOk(res: Response) {
  if (res.ok) return
  let msg = `API error ${res.status}`
  try {
    const body = await res.json()
    if (body?.error) msg = body.error
  } catch {}
  throw new Error(msg)
}

// Header identitas pengguna untuk log audit
function auditHeaders(extra: Record<string, string> = {}) {
  const u = operatorName()
  return u ? { ...extra, 'X-Jutec-User': u } : extra
}

export function useApi() {
  const { apiBase } = useRuntimeConfig().public

  async function get<T>(path: string, params?: Record<string, string | number | boolean | null | undefined>): Promise<T> {
    const url = new URL(apiBase + path)
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== null && v !== undefined) url.searchParams.set(k, String(v))
      })
    }
    const res = await fetch(url.toString(), { headers: auditHeaders() })
    await ensureOk(res)
    return res.json()
  }

  async function post<T>(path: string, body: unknown): Promise<T> {
    const res = await fetch(apiBase + path, {
      method: 'POST',
      headers: auditHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(body),
    })
    await ensureOk(res)
    return res.json()
  }

  async function del(path: string): Promise<void> {
    const res = await fetch(apiBase + path, { method: 'DELETE', headers: auditHeaders() })
    await ensureOk(res)
  }

  async function put<T>(path: string, body: unknown): Promise<T> {
    const res = await fetch(apiBase + path, {
      method: 'PUT',
      headers: auditHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(body),
    })
    await ensureOk(res)
    return res.json()
  }

  async function patch<T>(path: string, body: unknown): Promise<T> {
    const res = await fetch(apiBase + path, {
      method: 'PATCH',
      headers: auditHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(body),
    })
    await ensureOk(res)
    return res.json()
  }

  // Unduh file (dengan header audit) lalu simpan lewat browser
  async function download(path: string, fallbackName: string) {
    const res = await fetch(apiBase + path, { headers: auditHeaders() })
    await ensureOk(res)
    const cd = res.headers.get('Content-Disposition') ?? ''
    const name = /filename="([^"]+)"/.exec(cd)?.[1] ?? fallbackName
    const url = URL.createObjectURL(await res.blob())
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  }

  return { get, post, put, patch, del, download, apiBase }
}
