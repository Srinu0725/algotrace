import { useCallback, useEffect, useRef, useState } from 'react'

import { useExecutionStore } from '../store/executionStore'

const BASE_DELAY = 700
const SEEK_EVENT = 'algotrace:manual-seek'
export function seekFromVisualization(step: number) {
  window.dispatchEvent(new Event(SEEK_EVENT))
  useExecutionStore.getState().setStep(step)
}

export const PLAYBACK_SPEEDS = [
  0.25,
  0.5,
  1,
  1.5,
  2,
  4,
] as const

export function usePlayback() {
  // ==========================================
  // EXECUTION STORE
  // ==========================================

  const trace = useExecutionStore(state => state.trace)

  const jobId = useExecutionStore(state => state.jobId)

  const currentStep = useExecutionStore(
    state => state.currentStep,
  )

  const isExecuting = useExecutionStore(
    state => state.isExecuting,
  )

  const setStep = useExecutionStore(
    state => state.setStep,
  )

  // ==========================================
  // PLAYBACK STATE
  // ==========================================

  const [playingTrace, setPlayingTrace] =
    useState<typeof trace>(null)

  const playingRef = useRef(false)
  const [speed, updateSpeed] = useState(1)

  const totalSteps = trace?.events.length ?? 0

  const lastStep = Math.max(0, totalSteps - 1)

  const hasTrace = Boolean(trace) && totalSteps > 0

  const isFirstStep = currentStep <= 0

  const isLastStep = currentStep >= lastStep

  const canPlay = hasTrace && !isExecuting

  const isPlaying =
    canPlay &&
    playingTrace === trace &&
    !isLastStep

  const progress = totalSteps > 0
    ? ((currentStep + 1) / totalSteps) * 100
    : 0

  // ==========================================
  // SPEED CONTROL
  // ==========================================

  const setSpeed = useCallback((value: number) => {
    if (!Number.isFinite(value) || value <= 0) {
      return
    }

    updateSpeed(
      Math.max(0.25, Math.min(4, value)),
    )
  }, [])

  // ==========================================
  // PAUSE
  // ==========================================

  const pause = useCallback(() => {
    playingRef.current = false
    setPlayingTrace(null)
  }, [])

  useEffect(() => {
    window.addEventListener(SEEK_EVENT, pause)
    return () => window.removeEventListener(SEEK_EVENT, pause)
  }, [pause])

  // ==========================================
  // PLAY
  // ==========================================

  const play = useCallback(() => {
    if (!canPlay || totalSteps <= 1) {
      return
    }

    // Replay from the beginning if execution
    // playback has already finished.

    if (currentStep >= lastStep) {
      setStep(0)
    }

    playingRef.current = true
    setPlayingTrace(trace)

  }, [
    canPlay,
    totalSteps,
    currentStep,
    lastStep,
    setStep,
    trace,
  ])

  // ==========================================
  // TOGGLE PLAYBACK
  // ==========================================

  const toggle = useCallback(() => {
    if (isPlaying) {
      pause()
    } else {
      play()
    }
  }, [isPlaying, pause, play])

  // ==========================================
  // SEEK TO A SPECIFIC EXECUTION STEP
  // ==========================================

  const seek = useCallback((step: number) => {
    if (!canPlay || !Number.isFinite(step)) {
      return
    }

    pause()

    const nextStep = Math.max(
      0,
      Math.min(
        lastStep,
        Math.trunc(step),
      ),
    )

    setStep(nextStep)

  }, [
    canPlay,
    lastStep,
    pause,
    setStep,
  ])

  // ==========================================
  // MANUAL STEP NAVIGATION
  // ==========================================

  const next = useCallback(() => {
    seek(currentStep + 1)
  }, [seek, currentStep])

  const previous = useCallback(() => {
    seek(currentStep - 1)
  }, [seek, currentStep])

  const restart = useCallback(() => {
    seek(0)
  }, [seek])

  const finish = useCallback(() => {
    seek(lastStep)
  }, [seek, lastStep])

  // ==========================================
  // AUTOMATIC PLAYBACK ENGINE
  // ==========================================

  useEffect(() => {
    if (!isPlaying || !trace || isExecuting) {
      return
    }

    const delay = BASE_DELAY / speed

    // Capture the execution identity so an old
    // timer cannot update a newer execution.

    const activeTrace = trace

    const activeJobId = jobId

    const timer = window.setTimeout(() => {

      const latestState =
        useExecutionStore.getState()

      // Prevent stale execution updates.

      if (
        !playingRef.current ||
        latestState.trace !== activeTrace ||
        latestState.jobId !== activeJobId ||
        latestState.isExecuting
      ) {
        return
      }

      // Prevent a scheduled timer from overriding
      // a manually selected execution step.

      if (latestState.currentStep !== currentStep) {
        return
      }

      const nextStep = Math.min(
        latestState.currentStep + 1,
        lastStep,
      )

      latestState.setStep(nextStep)

      // Stop automatically after the final step.

      if (nextStep >= lastStep) {
        playingRef.current = false
        setPlayingTrace(null)
      }

    }, delay)

    // Cancel the previous timer whenever the step,
    // speed, trace or playback state changes.

    return () => {
      window.clearTimeout(timer)
    }

  }, [
    isPlaying,
    isExecuting,
    trace,
    jobId,
    currentStep,
    lastStep,
    speed,
  ])

  // ==========================================
  // PUBLIC PLAYBACK CONTROLS
  // ==========================================

  return {
    // Execution state

    currentStep,
    totalSteps,
    lastStep,
    hasTrace,
    canPlay,
    progress,

    // Playback state

    isPlaying,
    playing: isPlaying,
    speed,

    // Boundary state

    isFirstStep,
    isLastStep,

    canGoNext:
      canPlay && !isLastStep,

    canGoPrevious:
      canPlay && !isFirstStep,

    // Playback actions

    play,
    pause,
    toggle,
    togglePlayback: toggle,

    // Speed control

    setSpeed,
    playbackSpeeds: PLAYBACK_SPEEDS,

    // Navigation

    seek,
    restart,
    finish,
    next,
    previous,

    // Compatibility aliases

    nextStep: next,
    previousStep: previous,
    goToStep: seek,
    goToStart: restart,
    goToEnd: finish,
  }
}

export default usePlayback
