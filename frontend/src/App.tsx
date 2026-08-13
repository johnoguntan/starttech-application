import { useState, useEffect } from 'react'
import { apiClient } from './lib/apiClient'

interface HealthStatus {
  status: string
  service: string
  checks: {
    mongodb: string
    redis: string
  }
  timestamp: string
}

export default function App() {
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    apiClient.get<HealthStatus>('/v1/health')
      .then((res) => setHealth(res.data))
      .catch((err) => setError(err.message))
  }, [])

  return (
    <div style={{ fontFamily: 'system-ui', padding: '2rem', maxWidth: '600px', margin: '0 auto' }}>
      <h1>MuchToDo</h1>
      <h2>API Health</h2>
      {error && <pre style={{ color: 'red' }}>{error}</pre>}
      {health && (
        <pre style={{ background: '#f4f4f4', padding: '1rem', borderRadius: '8px' }}>
          {JSON.stringify(health, null, 2)}
        </pre>
      )}
    </div>
  )
}
