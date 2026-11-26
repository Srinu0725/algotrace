
import {
  useEffect,
  useState,
} from 'react'

import {
  useExecutionStore,
} from '../store/executionStore'

export function usePlayback() {
  const [playing, setPlaying] =
    useState(false)

  const [speed, setSpeed] =
    useState(1)

  const trace = useExecutionStore(
    (state) => state.trace,
  )

  const currentStep = useExecutionStore(
    (state) => state.currentStep,
  )

  const setStep = useExecutionStore(
    (state) => state.setStep,
  )

  const totalSteps =
    trace?.events.length ?? 0

  const isPlaying =
    playing &&
    totalSteps > 0 &&
    currentStep < totalSteps - 1

  useEffect(() => {
    if (!isPlaying) return

    const timer = window.setTimeout(() => {
      const state =
        useExecutionStore.getState()

      if (state.trace !== trace) {
        setPlaying(false)
        return
      }

      const lastStep =
        (trace?.events.length ?? 1) - 1

      const next = Math.min(
        state.currentStep + 1,
        lastStep,
      )

      state.setStep(next)

      if (next >= lastStep) {
        setPlaying(false)
      }
    }, 700 / speed)

    return () => {
      window.clearTimeout(timer)
    }
  }, [
    isPlaying,
    speed,
    currentStep,
    trace,
  ])

  function play() {
    if (totalSteps < 2) return

    if (currentStep >= totalSteps - 1) {
      setStep(0)
    }

    setPlaying(true)
  }

  function pause() {
    setPlaying(false)
  }

  function togglePlayback() {
    if (isPlaying) {
      pause()
    } else {
      play()
    }
  }

  function seek(step: number) {
    pause()
    setStep(step)
  }

  function restart() {
    pause()
    setStep(0)
  }

  function next() {
    seek(currentStep + 1)
  }

  function previous() {
    seek(currentStep - 1)
  }

  function finish() {
    seek(totalSteps - 1)
  }

  return {
    isPlaying,
    speed,
    setSpeed,

    currentStep,
    totalSteps,

    play,
    pause,
    togglePlayback,

    seek,
    restart,
    next,
    previous,
    finish,
  }
}
