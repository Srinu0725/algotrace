
import { create } from 'zustand'

import {
  executeCode,
  getJob,
  getTrace,
} from '../services/api'

import {
  subscribeToJob,
} from '../services/websocket'

import {
  isTerminalStatus,
} from '../types/execution'

import type {
  ExecutionStatus,
  Job,
  TraceResult,
} from '../types/execution'

interface ExecutionState {
  status: ExecutionStatus
  jobId: string | null
  job: Job | null
  trace: TraceResult | null
  error: string | null
  currentStep: number
  executedCode: string
  isExecuting: boolean
}

interface ExecutionStore extends ExecutionState {
  run: (code: string) => Promise<void>
  reset: () => void
  setStep: (step: number) => void
  nextStep: () => void
  previousStep: () => void
}

const initialState: ExecutionState = {
  status: 'idle',
  jobId: null,
  job: null,
  trace: null,
  error: null,
  currentStep: 0,
  executedCode: '',
  isExecuting: false,
}

let controller: AbortController | null = null

let unsubscribe: (() => void) | null = null

let pollTimer: ReturnType<typeof setTimeout> | null = null

function cleanup() {
  controller?.abort()
  controller = null

  unsubscribe?.()
  unsubscribe = null

  if (pollTimer !== null) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
}

export const useExecutionStore = create<ExecutionStore>(
  (set, get) => ({
    ...initialState,

    run: async (code: string) => {
      if (get().isExecuting) return

      if (!code.trim()) {
        set({
          status: 'error',
          error: 'Please enter Python code.',
        })

        return
      }

      cleanup()

      const activeController = new AbortController()

      controller = activeController

      const signal = activeController.signal

      let finalizing = false

      set({
        ...initialState,
        status: 'submitting',
        executedCode: code,
        isExecuting: true,
      })

      function handleError(error: unknown) {
        if (signal.aborted) return

        cleanup()

        set({
          status: 'error',
          isExecuting: false,
          error:
            error instanceof Error
              ? error.message
              : 'Unexpected execution error.',
        })
      }

      async function finalize(jobId: string) {
        if (signal.aborted || finalizing) return

        finalizing = true

        unsubscribe?.()
        unsubscribe = null

        if (pollTimer !== null) {
          clearTimeout(pollTimer)
          pollTimer = null
        }

        try {
          const job = await getJob(jobId, signal)

          if (signal.aborted) return

          set({
            job,
            status: 'loading_trace',
          })

          let trace: TraceResult | null = null
          let traceError: string | null = null

          if (job.trace_available) {
            try {
              trace = await getTrace(jobId, signal)
            } catch (error) {
              if (signal.aborted) return

              traceError =
                error instanceof Error
                  ? error.message
                  : 'Unable to retrieve execution trace.'
            }
          }

          if (signal.aborted) return

          cleanup()

          set({
            job,
            trace,
            status: job.status,
            currentStep: 0,
            isExecuting: false,
            error: traceError ?? job.error ?? null,
          })
        } catch (error) {
          handleError(error)
        }
      }

      async function poll(jobId: string) {
        if (signal.aborted || finalizing) return

        try {
          const job = await getJob(jobId, signal)

          if (signal.aborted || finalizing) return

          set({
            job,
            status: job.status,
          })

          if (isTerminalStatus(job.status)) {
            await finalize(jobId)
            return
          }

          pollTimer = setTimeout(() => {
            void poll(jobId)
          }, 500)
        } catch (error) {
          handleError(error)
        }
      }

      try {
        const response = await executeCode(
          code,
          'trace',
          signal,
        )

        if (signal.aborted) return

        const jobId = response.job_id

        set({
          jobId,
          status: response.status,
        })

        if (isTerminalStatus(response.status)) {
          await finalize(jobId)
          return
        }

        try {
          unsubscribe = subscribeToJob(jobId, {
            onEvent: (event) => {
              if (signal.aborted || finalizing) return

              set({
                status: event.status,
              })

              if (isTerminalStatus(event.status)) {
                void finalize(jobId)
              }
            },

            onFallback: () => {
              void poll(jobId)
            },
          })
        } catch {
          void poll(jobId)
        }
      } catch (error) {
        handleError(error)
      }
    },

    reset: () => {
      cleanup()

      set({
        ...initialState,
      })
    },

    setStep: (step: number) => {
      const count = get().trace?.events.length ?? 0

      if (count === 0 || !Number.isFinite(step)) {
        return
      }

      set({
        currentStep: Math.max(
          0,
          Math.min(count - 1, Math.trunc(step)),
        ),
      })
    },

    nextStep: () => {
      get().setStep(get().currentStep + 1)
    },

    previousStep: () => {
      get().setStep(get().currentStep - 1)
    },
  }),
)
