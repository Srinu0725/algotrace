
import type {
  ExecuteResponse,
  ExecutionMode,
  Job,
  TraceResult,
} from '../types/execution'

const API_BASE = '/api'

async function request<T>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const response = await fetch(
    `${API_BASE}${path}`,
    options,
  )

  if (!response.ok) {
    const message = await response.text()

    throw new Error(
      message.trim() ||
      `Request failed: HTTP ${response.status}`,
    )
  }

  return response.json() as Promise<T>
}

export async function executeCode(
  code: string,
  mode: ExecutionMode = 'trace',
  signal?: AbortSignal,
): Promise<ExecuteResponse> {
  return request<ExecuteResponse>('/execute', {
    method: 'POST',

    headers: {
      'Content-Type': 'application/json',
    },

    body: JSON.stringify({
      code,
      mode,
    }),

    signal,
  })
}

export async function getJob(
  jobId: string,
  signal?: AbortSignal,
): Promise<Job> {
  return request<Job>(
    `/jobs/${encodeURIComponent(jobId)}`,
    { signal },
  )
}

export async function getTrace(
  jobId: string,
  signal?: AbortSignal,
): Promise<TraceResult> {
  return request<TraceResult>(
    `/jobs/${encodeURIComponent(jobId)}/trace`,
    { signal },
  )
}

export async function checkHealth(): Promise<boolean> {
  try {
    const response = await fetch('/api/health')
    return response.ok
  } catch {
    return false
  }
}
