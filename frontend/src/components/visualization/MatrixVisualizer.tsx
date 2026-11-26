import { useMemo, useState } from 'react'

import {
  Grid3X3,
  Minus,
  Plus,
} from 'lucide-react'

import type {
  JsonValue,
  VariableChange,
} from '../../types/execution'

interface Props {
  name: string
  values: JsonValue[][]
  changes: VariableChange[]
}

const MAX_ROWS = 50
const MAX_COLUMNS = 50

export default function MatrixVisualizer({
  name,
  values,
  changes,
}: Props) {
  const [cellSize, setCellSize] = useState(46)

  const rows = values.length

  const columns = Math.max(
    0,
    ...values.map(row => row.length),
  )

  const visibleRows = Math.min(rows, MAX_ROWS)

  const visibleColumns = Math.min(
    columns,
    MAX_COLUMNS,
  )

  const changedCells = useMemo(() => {
    const result = new Set<string>()

    const relevantChanges = changes.filter(
      change => change.name === name,
    )

    for (const change of relevantChanges) {
      const oldValue = change.old_value
      const newValue = change.new_value

      if (
        !Array.isArray(oldValue) ||
        !Array.isArray(newValue)
      ) {
        continue
      }

      for (
        let row = 0;
        row < Math.min(
          newValue.length,
          MAX_ROWS,
        );
        row++
      ) {
        const oldRow = oldValue[row]
        const newRow = newValue[row]

        if (!Array.isArray(newRow)) continue

        for (
          let col = 0;
          col < Math.min(
            newRow.length,
            MAX_COLUMNS,
          );
          col++
        ) {
          const previous = Array.isArray(oldRow)
            ? oldRow[col]
            : undefined

          const current = newRow[col]

          if (
            JSON.stringify(previous) !==
            JSON.stringify(current)
          ) {
            result.add(`${row}-${col}`)
          }
        }
      }
    }

    return result
  }, [changes, name])

  return (
    <div className="matrix-visualizer">
      <div className="recursion-header">
        <div className="recursion-title">
          <Grid3X3 size={18} />

          <h3>{name}</h3>
        </div>

        <div className="recursion-zoom">
          <button
            className="icon-button"
            title="Smaller cells"
            onClick={() =>
              setCellSize(size =>
                Math.max(30, size - 6),
              )
            }
          >
            <Minus size={15} />
          </button>

          <button
            className="icon-button"
            title="Larger cells"
            onClick={() =>
              setCellSize(size =>
                Math.min(100, size + 6),
              )
            }
          >
            <Plus size={15} />
          </button>
        </div>
      </div>

      <div className="recursion-stats">
        <span>{rows} rows</span>
        <span>{columns} max columns</span>
        <span>{changedCells.size} visible updates</span>
      </div>

      <div className="matrix-scroll">
        <div
          className="matrix-grid"
          style={{
            gridTemplateColumns:
              `30px repeat(${visibleColumns}, ${cellSize}px)`,
          }}
        >
          <div />

          {Array.from(
            { length: visibleColumns },
            (_, col) => (
              <div
                key={`column-${col}`}
                className="matrix-index"
              >
                {col}
              </div>
            ),
          )}

          {values
            .slice(0, visibleRows)
            .map((row, rowIndex) => (
              <div
                className="matrix-row"
                key={rowIndex}
              >
                <div className="matrix-index">
                  {rowIndex}
                </div>

                {Array.from(
                  { length: visibleColumns },
                  (_, colIndex) => {
                    const exists =
                      colIndex < row.length

                    const value = row[colIndex]

                    const changed =
                      changedCells.has(
                        `${rowIndex}-${colIndex}`,
                      )

                    return (
                      <div
                        key={colIndex}
                        className={[
                          'matrix-cell',
                          changed ? 'changed' : '',
                          !exists ? 'missing' : '',
                        ].join(' ')}
                        style={{
                          height: cellSize,
                        }}
                        title={
                          exists
                            ? `${name}[${rowIndex}][${colIndex}] = ${JSON.stringify(value)}`
                            : 'No element'
                        }
                      >
                        {exists
                          ? JSON.stringify(value)
                          : '—'}
                      </div>
                    )
                  },
                )}
              </div>
            ))}
        </div>
      </div>

      <div className="recursion-legend">
        <span>
          <i className="legend-waiting" />
          Cell value
        </span>

        <span>
          <i className="legend-current" />
          Updated cell
        </span>
      </div>

      {(rows > MAX_ROWS ||
        columns > MAX_COLUMNS) && (
        <p className="recursion-limit">
          Display limited to the first 50 rows
          and 50 columns.
        </p>
      )}
    </div>
  )
}