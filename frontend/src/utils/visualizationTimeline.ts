import type {
  TraceEvent,
} from '../types/execution'

import {
  detectStructures,
} from './structureDetector'

import {
  selectActiveVisualization,
} from './activityDetector'

export interface TimelineFrame {
  step: number
  activeId: string | null
  visibleIds: string[]
  changedIds: string[]
  retainedSteps: number[]
}

export interface VisualizationTimeline {
  frames: TimelineFrame[]
  order: Map<string, number>
}

export function buildVisualizationTimeline(
  events: TraceEvent[],
): VisualizationTimeline {
  const frames: TimelineFrame[] = []

  const order = new Map<string, number>()

  const frameSteps = new Map<number, number>()
  let activeId: string | null = null

  for (let index = 0; index < events.length; index++) {
    const event = events[index]
    const previousEvent = events[index - 1]

    const depth = event.depth
    for (const key of frameSteps.keys()) {
      if (key > depth || (key === depth && ['call', 'function_call'].includes(event.event))) frameSteps.delete(key)
    }
    const retainedSteps = [...frameSteps.entries()]
      .filter(([key]) => key < depth)
      .map(([, step]) => step)
      .reverse()
    frameSteps.set(depth, index)
    if (['return', 'function_return'].includes(event.event)) frameSteps.delete(depth)
    const structures = detectStructures(
      event,
      previousEvent,
    )

    // Register structures in first-seen order.

    for (const structure of structures) {
      if (!order.has(structure.id)) {
        order.set(
          structure.id,
          order.size,
        )
      }
    }

    // Select meaningful activity.

    activeId = selectActiveVisualization(
      structures,
      activeId,
    )

    frames.push({
      step: index,
      retainedSteps,

      activeId,

      visibleIds: structures.map(
        structure => structure.id,
      ),

      changedIds: structures
        .filter(
          structure =>
            structure.activity !== 'unchanged',
        )
        .map(
          structure => structure.id,
        ),
    })
  }

  return {
    frames,
    order,
  }
}

export function getTimelineFrame(
  timeline: VisualizationTimeline,
  step: number,
): TimelineFrame | undefined {
  if (
    !Number.isInteger(step) ||
    step < 0 ||
    step >= timeline.frames.length
  ) {
    return undefined
  }

  return timeline.frames[step]
}
