export interface DiscoveryLog {
  time: string
  text: string
  type: 'INFO' | 'FOUND' | 'SKIP'
}

export const useDiscoveryState = () => {
  const loading = useState<boolean>('disc-loading', () => false)
  const result = useState<any>('disc-result', () => null)
  const logs = useState<any[]>('disc-logs', () => [])
  const elapsed = useState<string>('disc-elapsed', () => '00:00')
  const startTime = useState<number | null>('disc-start-time', () => null)
  
  const addLog = (text: string, type: 'INFO' | 'FOUND' | 'SKIP' = 'INFO') => {
    logs.value.push({
      time: new Date().toLocaleTimeString(),
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
