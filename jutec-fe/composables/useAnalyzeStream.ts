export interface ProgressEvent {
  type: 'start' | 'step' | 'done' | 'error'
  step: string
  message: string
  data?: any
  ts: number
}

export interface StreamState {
  running: boolean
  logs: ProgressEvent[]
  result: any | null
  error: string | null
}

export function useAnalyzeStream() {
  const { apiBase, wsBase } = useRuntimeConfig().public

  const state = reactive<StreamState>({
    running: false,
    logs: [],
    result: null,
    error: null,
  })

  let ws: WebSocket | null = null

  function reset() {
    state.logs = []
    state.result = null
    state.error = null
  }

  function analyze(url: string, depth = 0) {
    reset()
    state.running = true

    // Prefer WebSocket, fall back to SSE
    const wsURL = `${wsBase}/ws/analyze?url=${encodeURIComponent(url)}&depth=${depth}`

    try {
      ws = new WebSocket(wsURL)

      ws.onmessage = (evt) => {
        const ev: ProgressEvent = JSON.parse(evt.data)
        state.logs.push(ev)
        if (ev.type === 'done') {
          state.result = ev.data
          state.running = false
          ws?.close()
        }
        if (ev.type === 'error') {
          state.error = ev.message
          state.running = false
          ws?.close()
        }
      }

      ws.onerror = () => {
        // Fallback to SSE
        ws?.close()
        analyzeSSE(url, depth)
      }

      ws.onclose = () => {
        if (state.running) state.running = false
      }
    } catch {
      analyzeSSE(url, depth)
    }
  }

  function analyzeSSE(url: string, depth = 0) {
    const sseURL = `${apiBase}/sse/analyze?url=${encodeURIComponent(url)}&depth=${depth}`
    const es = new EventSource(sseURL)

    es.onmessage = (evt) => {
      const ev: ProgressEvent = JSON.parse(evt.data)
      state.logs.push(ev)
      if (ev.type === 'done') {
        state.result = ev.data
        state.running = false
        es.close()
      }
      if (ev.type === 'error') {
        state.error = ev.message
        state.running = false
        es.close()
      }
    }

    es.onerror = () => {
      state.error = 'Koneksi ke server terputus'
      state.running = false
      es.close()
    }
  }

  function stop() {
    ws?.close()
    state.running = false
  }

  return { state, analyze, stop }
}
