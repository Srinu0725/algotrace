
import { useState } from 'react'

import {
  ChartNoAxesColumn,
  Grid2X2,
} from 'lucide-react'

import type {
  JsonValue,
  VariableChange,
} from '../../types/execution'

interface ArrayVisualizerProps {
  name: string
  values: JsonValue[]
  changes: VariableChange[]
}

type ViewMode = 'bars' | 'cells'

export default function ArrayVisualizer({
  name,
  values,
  changes,
}: ArrayVisualizerProps) {
  const [view, setView] =
    useState<ViewMode>('bars')

  const isNumeric = values.every(
    (value) =>
      typeof value === 'number' &&
      Number.isFinite(value),
  )

  const numericValues = isNumeric
    ? (values as number[])
    : []

  const maxValue = Math.max(
    1,
    ...numericValues.map(Math.abs),
  )

  const hasNegative = numericValues.some(
    (value) => value < 0,
  )

  const changedIndices = new Set<number>()

  for (const change of changes) {
    if (
      change.name !== name ||
      !Array.isArray(change.old_value) ||
      !Array.isArray(change.new_value)
    ) {
      continue
    }

    const oldValues = change.old_value
    const newValues = change.new_value

    newValues.forEach((value, index) => {
      if (
        index >= oldValues.length ||
        JSON.stringify(value) !==
          JSON.stringify(oldValues[index])
      ) {
        changedIndices.add(index)
      }
    })
  }

  const showBars =
    view === 'bars' && isNumeric

  return (
    <div className="array-visualizer">
      <div className="array-header">
        <div className="array-title">
          <span>{name}</span>

          <small>
            {values.length} elements
          </small>
        </div>

        <div className="array-view-toggle">
          <button
            className={
              showBars ? 'view-active' : ''
            }
            disabled={!isNumeric}
            title="Bar visualization"
            aria-label="Bar visualization"
            aria-pressed={showBars}
            onClick={() => setView('bars')}
          >
            <ChartNoAxesColumn size={16} />
          </button>

          <button
            className={
              !showBars ? 'view-active' : ''
            }
            title="Cell visualization"
            aria-label="Cell visualization"
            aria-pressed={!showBars}
            onClick={() => setView('cells')}
          >
            <Grid2X2 size={16} />
          </button>
        </div>
      </div>

      {values.length === 0 ? (
        <div className="array-empty">
          Empty array
        </div>
      ) : showBars ? (
        <div className="array-bars">
          {numericValues.map((value, index) => {
            const changed =
              changedIndices.has(index)

            const height = Math.max(
              2,
              (Math.abs(value) / maxValue) *
                (hasNegative ? 85 : 170),
            )

            return (
              <div
                className="array-bar-column"
                key={index}
              >
                <div className="array-bar-value">
                  {value}
                </div>

                <div className="array-bar-track">
                  <div
                    className="array-zero-axis"
                    style={{
                      bottom: hasNegative
                        ? '50%'
                        : 0,
                    }}
                  />

                  <div
                    className={`array-bar ${
                      changed
                        ? 'array-bar-changed'
                        : ''
                    }`}
                    style={{
                      height: `${height}px`,

                      bottom:
                        value < 0
                          ? undefined
                          : hasNegative
                            ? '50%'
                            : 0,

                      top:
                        value < 0
                          ? '50%'
                          : undefined,
                    }}
                  />
                </div>

                <span className="array-index">
                  {index}
                </span>
              </div>
            )
          })}
        </div>
      ) : (
        <div className="array-cells">
          {values.map((value, index) => (
            <div
              className="array-cell-column"
              key={index}
            >
              <div
                className={`array-cell ${
                  changedIndices.has(index)
                    ? 'array-cell-changed'
                    : ''
                }`}
                title={JSON.stringify(value)}
              >
                {JSON.stringify(value)}
              </div>

              <span className="array-index">
                {index}
              </span>
            </div>
          ))}
        </div>
      )}

      <div className="array-legend">
        <span>
          <i className="legend-normal" />
          Element
        </span>

        <span>
          <i className="legend-changed" />
          Updated element
        </span>
      </div>
    </div>
  )
}
