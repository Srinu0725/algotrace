import type {
  VisualizationDescriptor,
} from '../types/visualization'

const ACTIVITY_PRIORITY = {
  created: 3,
  modified: 2,
  active: 1,
  unchanged: 0,
} as const

export function getActiveVisualizations(
  visualizations: VisualizationDescriptor[],
): VisualizationDescriptor[] {
  return visualizations.filter(
    visualization =>
      visualization.activity !== 'unchanged',
  )
}

export function selectActiveVisualization(
  visualizations: VisualizationDescriptor[],
  previousActiveId: string | null,
): string | null {
  if (visualizations.length === 0) {
    return null
  }

  const changed = getActiveVisualizations(
    visualizations,
  )

  // If nothing meaningful changed, retain
  // the previously selected visualization.

  if (changed.length === 0) {
    return (
      visualizations.find(
        item => item.id === previousActiveId,
      )?.id ??
      visualizations[0].id
    )
  }

  // Retain the current visualization if
  // it is still being modified.

  const previousActive = changed.find(
    item => item.id === previousActiveId,
  )

  if (previousActive) {
    return previousActive.id
  }

  // Prefer specialized renderers over the
  // generic object inspector.

  const specialized = changed.filter(
    item => item.type !== 'object',
  )

  const candidates =
    specialized.length > 0
      ? specialized
      : changed

  return [...candidates].sort(
    (first, second) =>
      ACTIVITY_PRIORITY[second.activity] -
      ACTIVITY_PRIORITY[first.activity],
  )[0].id
}