export type JobStatus =
  | 'queued'
  | 'running'
  | 'completed'
  | 'failed'
  | 'timed_out'
  | 'limit_reached'

export type ExecutionMode = 'run' | 'trace'

export type ExecutionStatus =
  | 'idle'
  | 'submitting'
  | 'loading_trace'
  | 'error'
  | JobStatus

export type JsonValue =
  | null
  | boolean
  | number
  | string
  | JsonValue[]
  | { [key: string]: JsonValue }

export interface ExecuteResponse {
  job_id: string
  status: JobStatus
  mode: ExecutionMode
}

export interface Job {
  job_id: string
  code: string
  mode: ExecutionMode
  status: JobStatus

  stdout: string
  stderr: string

  exit_code: number
  duration_ms: number
  timed_out: boolean

  stdout_truncated: boolean
  stderr_truncated: boolean

  trace_available: boolean
  trace_status?: string
  total_steps: number
  trace_size_bytes: number

  error?: string

  created_at: string
  updated_at: string
}

export interface JobEvent {
  type: 'job_updated'
  job_id: string
  mode: ExecutionMode
  status: JobStatus

  trace_available: boolean
  trace_status?: string

  total_steps: number
  duration_ms: number
  exit_code: number
  timed_out: boolean

  error?: string
  updated_at: string
}

export interface VariableChange {
  type: string
  name: string
  value?: JsonValue
  old_value?: JsonValue
  new_value?: JsonValue
}

export interface TraceException {
  type: string
  message: string
}

export interface TraceError {
  type: string
  message: string
  line: number | null
  column?: number
  function: string
}

export interface TraceEvent {
  step: number
  line: number
  event: string
  function: string
  depth: number
  call_stack: string[]

  variables?: Record<string, JsonValue>
  arguments?: Record<string, JsonValue>

  changes?: VariableChange[]
  return_value?: JsonValue
  exception?: TraceException
}

export interface TraceResult {
  status: string
  limit: string | null
  error: TraceError | null

  stdout: string
  stderr: string

  stdout_truncated: boolean
  stderr_truncated: boolean

  total_steps: number
  trace_size_bytes: number

  events: TraceEvent[]
}

export function isTerminalStatus(
  status: JobStatus,
): boolean {
  return [
    'completed',
    'failed',
    'timed_out',
    'limit_reached',
  ].includes(status)
}
