export interface DiscoveryLog {
  time: string
  text: string
  type: LogType
}
// PHASE = pergantian tahap (judul), WARN = galat yang tidak menghentikan proses
export type LogType = 'INFO' | 'FOUND' | 'SKIP' | 'PHASE' | 'WARN'
const fmtTime = (d: Date) => d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })

export const useDiscoveryState = () => {
  const loading = useState<boolean>('disc-loading', () => false)
  const result = useState<any>('disc-result', () => null)
  const logs = useState<any[]>('disc-logs', () => [])
  const elapsed = useState<string>('disc-elapsed', () => '00:00')
  const startTime = useState<number | null>('disc-start-time', () => null)
  
  const addLog = (text: string, type: LogType = 'INFO', at?: Date | string) => {
    logs.value.push({
      time: fmtTime(at ? new Date(at) : new Date()),
      text,
      type
    })
    
    // Auto scroll logic must be in the component but logs are here
  }

  const resetState = () => {
    result.value = null
    logs.value = []
    elapsed.value = '00:00'
    startTime.value = Date.now()
  }

  return {
    loading,
    result,
    logs,
    elapsed,
    startTime,
    addLog,
    resetState
  }
}
