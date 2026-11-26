import { useId, useState } from 'react'
import { Network, Minus, Plus, RotateCcw } from 'lucide-react'

import type {
  JsonValue,
  ObjectSnapshot,
} from '../../types/execution'

interface ObjectVisualizerProps {
  references: Record<string, string>
  objects: Record<string, ObjectSnapshot>
  previousReferences?: Record<string, string>
  previousObjects?: Record<string, ObjectSnapshot>
  truncated?: boolean
}

interface NodePosition {
  x: number
  y: number
  height: number
}

const NODE_WIDTH = 190
const COLUMN_GAP = 100
const ROW_HEIGHT = 300

const MAX_NODES = 60
const MAX_FIELDS = 10
const COLUMNS = 3

function getReference(value: JsonValue): string | null {
  if (
    value !== null &&
    typeof value === 'object' &&
    !Array.isArray(value) &&
    typeof value.$ref === 'string'
  ) {
    return value.$ref
  }

  return null
}

function shorten(value: string, limit: number): string {
  return value.length > limit
    ? `${value.slice(0, limit - 1)}…`
    : value
}

export default function ObjectVisualizer({
  references,
  objects,
  previousReferences = {},
  previousObjects = {},
  truncated = false,
}: ObjectVisualizerProps) {
  const arrowId = useId()

  const [zoom, setZoom] = useState(1)

  const entries = Object.entries(objects).slice(0, MAX_NODES)

  const roots = Object.entries(references).slice(0, MAX_NODES)

  const rootHeight = Math.max(
    90,
    Math.ceil(roots.length / 6) * 42 + 35,
  )

  const positions = new Map<string, NodePosition>()

  entries.forEach(([id, object], index) => {
    const column = index % COLUMNS
    const row = Math.floor(index / COLUMNS)

    const fieldCount = Math.min(
      Object.keys(object.attributes).length,
      MAX_FIELDS,
    )

    positions.set(id, {
      x: 30 + column * (NODE_WIDTH + COLUMN_GAP),
      y: rootHeight + row * ROW_HEIGHT,
      height: 80 + fieldCount * 18,
    })
  })

  const width = Math.max(
    930,
    COLUMNS * (NODE_WIDTH + COLUMN_GAP) + 60,
  )

  const height =
    rootHeight +
    Math.ceil(entries.length / COLUMNS) * ROW_HEIGHT +
    30

  const displayLimited =
    truncated ||
    Object.keys(objects).length > MAX_NODES ||
    Object.keys(references).length > MAX_NODES ||
    entries.some(
      ([, object]) =>
        Object.keys(object.attributes).length > MAX_FIELDS,
    )

  if (entries.length === 0) {
    if (!truncated) {
      return null
    }

    return (
      <section className="trace-card object-card">
        <h3>Object References</h3>

        <p className="recursion-limit">
          Object snapshot exceeded the available trace limits.
        </p>
      </section>
    )
  }

  function zoomIn() {
    setZoom((current) =>
      Math.min(2, current + 0.1),
    )
  }

  function zoomOut() {
    setZoom((current) =>
      Math.max(0.3, current - 0.1),
    )
  }

  return (
    <section className="trace-card object-card">
      {/* Header */}

      <div className="recursion-header">
        <div className="recursion-title">
          <Network size={18} />

          <h3>Object References</h3>
        </div>

        <div className="recursion-zoom">
          <button
            type="button"
            onClick={zoomOut}
            disabled={zoom <= 0.3}
            title="Zoom out"
            aria-label="Zoom out"
          >
            <Minus size={15} />
          </button>

          <span>{Math.round(zoom * 100)}%</span>

          <button
            type="button"
            onClick={zoomIn}
            disabled={zoom >= 2}
            title="Zoom in"
            aria-label="Zoom in"
          >
            <Plus size={15} />
          </button>

          <button
            type="button"
            onClick={() => setZoom(1)}
            title="Reset zoom"
            aria-label="Reset zoom"
          >
            <RotateCcw size={15} />
          </button>
        </div>
      </div>

      {/* Statistics */}

      <div className="recursion-stats">
        <span>
          Objects: {Object.keys(objects).length}
        </span>

        <span>
          References: {Object.keys(references).length}
        </span>
      </div>

      {/* Visualization canvas */}

      <div className="object-canvas">
        <svg
          width={width * zoom}
          height={height * zoom}
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label="Object identity and reference graph"
        >
          <defs>
            <marker
              id={arrowId}
              markerWidth="8"
              markerHeight="8"
              refX="7"
              refY="4"
              orient="auto"
            >
              <path
                d="M 0 0 L 8 4 L 0 8 Z"
                fill="#64748b"
              />
            </marker>
          </defs>

          {/* Variable-to-object reference edges */}

          {roots.map(([name, targetId], index) => {
            const target = positions.get(targetId)

            if (!target) {
              return null
            }

            const rootX = 20 + (index % 6) * 145
            const rootY = 15 + Math.floor(index / 6) * 42

            const startX = rootX + 60
            const startY = rootY + 28

            const endX = target.x + NODE_WIDTH / 2
            const endY = target.y

            const middleY = (startY + endY) / 2

            return (
              <path
                key={`root-edge-${name}`}
                className="object-edge"
                d={`
                  M ${startX} ${startY}
                  C ${startX} ${middleY},
                    ${endX} ${middleY},
                    ${endX} ${endY}
                `}
                markerEnd={`url(#${arrowId})`}
              />
            )
          })}

          {/* Object-to-object reference edges */}

          {entries.flatMap(([id, object]) => {
            const source = positions.get(id)

            if (!source) {
              return []
            }

            return Object.entries(object.attributes)
              .slice(0, MAX_FIELDS)
              .map(([attribute, value], index) => {
                const targetId = getReference(value)

                if (!targetId) {
                  return null
                }

                const target = positions.get(targetId)

                if (!target) {
                  return null
                }

                const startX = source.x + NODE_WIDTH
                const startY = source.y + 64 + index * 18

                const endX = target.x
                const endY = target.y + target.height / 2

                let path: string

                if (id === targetId) {
                  // Self-referencing object

                  path = `
                    M ${startX} ${startY}
                    C ${startX + 65} ${startY},
                      ${startX + 65} ${source.y - 25},
                      ${source.x + NODE_WIDTH / 2} ${source.y}
                  `
                } else {
                  // Normal object reference

                  path = `
                    M ${startX} ${startY}
                    C ${startX + 45} ${startY},
                      ${endX - 45} ${endY},
                      ${endX} ${endY}
                  `
                }

                return (
                  <path
                    key={`${id}-${attribute}-${targetId}`}
                    className="object-edge"
                    d={path}
                    markerEnd={`url(#${arrowId})`}
                  >
                    <title>
                      {`${id}.${attribute} → ${targetId}`}
                    </title>
                  </path>
                )
              })
          })}

          {/* Variable reference labels */}

          {roots.map(([name, targetId], index) => {
            const x = 20 + (index % 6) * 145
            const y = 15 + Math.floor(index / 6) * 42

            const changed =
              previousReferences[name] !== targetId

            return (
              <g
                key={name}
                className={
                  changed
                    ? 'object-root changed'
                    : 'object-root'
                }
              >
                <title>
                  {`${name} → ${targetId}`}
                </title>

                <rect
                  x={x}
                  y={y}
                  width={120}
                  height={28}
                  rx={6}
                />

                <text
                  x={x + 60}
                  y={y + 18}
                  textAnchor="middle"
                >
                  {shorten(name, 16)}
                </text>
              </g>
            )
          })}

          {/* Object nodes */}

          {entries.map(([id, object]) => {
            const position = positions.get(id)

            if (!position) {
              return null
            }

            const attributes = Object.entries(
              object.attributes,
            )

            const visibleAttributes = attributes.slice(
              0,
              MAX_FIELDS,
            )

            const changed =
              JSON.stringify(previousObjects[id]) !==
              JSON.stringify(object)

            return (
              <g
                key={id}
                className={
                  changed
                    ? 'object-node changed'
                    : 'object-node'
                }
              >
                <title>
                  {`${object.type} (${id})`}
                </title>

                <rect
                  x={position.x}
                  y={position.y}
                  width={NODE_WIDTH}
                  height={position.height}
                  rx={8}
                />

                {/* Object type */}

                <text
                  x={position.x + 12}
                  y={position.y + 22}
                  className="object-type"
                >
                  {shorten(object.type, 20)}
                </text>

                {/* Stable object identity */}

                <text
                  x={position.x + 12}
                  y={position.y + 40}
                  className="object-id"
                >
                  {id}
                </text>

                <line
                  x1={position.x + 10}
                  y1={position.y + 48}
                  x2={position.x + NODE_WIDTH - 10}
                  y2={position.y + 48}
                  stroke="#334155"
                />

                {/* Object attributes */}

                {visibleAttributes.map(
                  ([attribute, value], index) => {
                    const reference = getReference(value)

                    const formattedValue = reference
                      ? `→ ${reference}`
                      : JSON.stringify(value)

                    const label =
                      `${attribute}: ${formattedValue}`

                    return (
                      <text
                        key={attribute}
                        x={position.x + 12}
                        y={position.y + 64 + index * 18}
                        className={
                          reference
                            ? 'object-reference'
                            : 'object-attribute'
                        }
                      >
                        <title>{label}</title>

                        {shorten(label, 25)}
                      </text>
                    )
                  },
                )}

                {/* Hidden attribute count */}

                {attributes.length > MAX_FIELDS && (
                  <text
                    x={position.x + 12}
                    y={
                      position.y +
                      position.height -
                      10
                    }
                    className="object-id"
                  >
                    +{attributes.length - MAX_FIELDS} more
                  </text>
                )}
              </g>
            )
          })}
        </svg>
      </div>

      {/* Legend */}

      <div
        className="recursion-legend"
        style={{
          display: 'flex',
          gap: 16,
          flexWrap: 'wrap',
        }}
      >
        <span style={{ color: '#60a5fa' }}>
          ● Object / Reference
        </span>

        <span style={{ color: '#f59e0b' }}>
          ● Created / Updated
        </span>

        <span style={{ color: '#94a3b8' }}>
          → Object relationship
        </span>
      </div>

      {displayLimited && (
        <p className="recursion-limit">
          Some objects, references or attributes have been
          omitted due to visualization or trace limits.
        </p>
      )}
    </section>
  )
}