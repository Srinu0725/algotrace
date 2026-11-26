import { useLayoutEffect, useRef } from 'react'

interface AutoFollowOptions {
  enabled: boolean
  currentStep: number
  jobId: string | null
  activeTarget: string | null
}

export default function useAutoFollow({
  enabled,
  currentStep,
  jobId,
  activeTarget,
}: AutoFollowOptions) {
  const containerRef = useRef<HTMLDivElement>(null)

  // Reset scrolling whenever a new execution starts.
  useLayoutEffect(() => {
    const container = containerRef.current

    if (container) {
      container.scrollTop = 0
    }
  }, [jobId])

  // Follow the currently active visualization.
  useLayoutEffect(() => {
    if (!enabled || !activeTarget) {
      return
    }

    const container = containerRef.current

    if (!container) {
      return
    }

    const target = Array.from(
      container.querySelectorAll<HTMLElement>(
        '[data-visualization-id]',
      ),
    ).find(
      element =>
        element.dataset.visualizationId === activeTarget,
    )

    if (!target) {
      return
    }

    const containerRect =
      container.getBoundingClientRect()

    const targetRect =
      target.getBoundingClientRect()

    const visibleTop =
      containerRect.top + container.clientTop

    const visibleBottom =
      visibleTop + container.clientHeight

    const fullyVisible =
      targetRect.top >= visibleTop &&
      targetRect.bottom <= visibleBottom

    if (fullyVisible) {
      return
    }

    const oversized =
      targetRect.height > container.clientHeight

    // Do not repeatedly reposition large visualizations.
    if (
      oversized &&
      targetRect.top >= visibleTop &&
      targetRect.top < visibleBottom
    ) {
      return
    }

    const nextScrollTop =
      container.scrollTop +
      targetRect.top -
      visibleTop -
      12

    container.scrollTop = Math.max(
      0,
      nextScrollTop,
    )
  }, [
    enabled,
    currentStep,
    jobId,
    activeTarget,
  ])

  return containerRef
}