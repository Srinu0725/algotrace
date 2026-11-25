import { isTerminalStatus } from '../types/execution'

import type {
  JobEvent,
  JobStatus,
} from '../types/execution'

interface WebSocketCallbacks {
  onEvent: (event: JobEvent) => void
  onFallback: () => void
}

const CONNECTION_TIMEOUT_MS = 5000
const EVENT_TIMEOUT_MS = 15000

const VALID_STATUSES = new Set<JobStatus>([
  'queued',
  'running',
  'completed',
  'failed',
  'timed_out',
  'limit_reached',
])

export function subscribeToJob(
  jobId: string,
  callbacks: WebSocketCallbacks,
): () => void {
  // Use secure WebSocket when HTTPS is enabled

  const protocol =
    window.location.protocol === 'https:'
      ? 'wss:'
      : 'ws:'

  // Same-origin WebSocket through Vite proxy

  const url =
    `${protocol}//${window.location.host}` +
    `/ws/jobs/${encodeURIComponent(jobId)}`

  const socket = new WebSocket(url)

  let disposed = false
  let terminalReceived = false
  let fallbackStarted = false

  let connectionTimer: number | null = null
  let eventTimer: number | null = null

  // Clear all active timers

  function clearTimers() {
    if (connectionTimer !== null) {
      window.clearTimeout(connectionTimer)

      connectionTimer = null
    }

    if (eventTimer !== null) {
      window.clearTimeout(eventTimer)

      eventTimer = null
    }
  }

  // Safely close the WebSocket

  function closeSocket() {
    if (
      socket.readyState === WebSocket.OPEN ||
      socket.readyState === WebSocket.CONNECTING
    ) {
      socket.close()
    }
  }

  // Switch to REST polling

  function fallback() {
    if (
      disposed ||
      terminalReceived ||
      fallbackStarted
    ) {
      return
    }

    fallbackStarted = true

    clearTimers()
    closeSocket()

    callbacks.onFallback()
  }

  // Detect inactive connections

  function restartEventTimer() {
    if (eventTimer !== null) {
      window.clearTimeout(eventTimer)
    }

    eventTimer = window.setTimeout(() => {
      fallback()
    }, EVENT_TIMEOUT_MS)
  }

  // Connection timeout

  connectionTimer = window.setTimeout(() => {
    fallback()
  }, CONNECTION_TIMEOUT_MS)

  // Connection established

  socket.onopen = () => {
    if (disposed || fallbackStarted) {
      closeSocket()
      return
    }

    if (connectionTimer !== null) {
      window.clearTimeout(connectionTimer)

      connectionTimer = null
    }

    restartEventTimer()
  }

  // Receive job events

  socket.onmessage = (message) => {
    if (
      disposed ||
      fallbackStarted ||
      terminalReceived
    ) {
      return
    }

    let event: JobEvent

    try {
      event = JSON.parse(
        message.data,
      ) as JobEvent
    } catch {
      fallback()
      return
    }

    // Validate event type and job identity

    if (
      !event ||
      event.type !== 'job_updated' ||
      event.job_id !== jobId ||
      !VALID_STATUSES.has(event.status)
    ) {
      return
    }

    restartEventTimer()

    // Terminal execution state

    if (isTerminalStatus(event.status)) {
      terminalReceived = true

      clearTimers()
    }

    // Forward event to Zustand store

    callbacks.onEvent(event)
  }

  // Connection error

  socket.onerror = () => {
    fallback()
  }

  // Connection closed

  socket.onclose = () => {
    clearTimers()

    if (
      !disposed &&
      !terminalReceived
    ) {
      fallback()
    }
  }

  // Subscription cleanup

  return () => {
    if (disposed) return

    disposed = true

    clearTimers()

    socket.onopen = null
    socket.onmessage = null
    socket.onerror = null
    socket.onclose = null

    closeSocket()
  }
}
