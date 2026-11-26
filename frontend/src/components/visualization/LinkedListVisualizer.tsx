import { useId, useState } from 'react'

import {
  Link2,
  Minus,
  Plus,
  RotateCcw,
} from 'lucide-react'

import type {
  LinkedListVisualization,
} from '../../types/visualization'

interface Props {
  data: LinkedListVisualization
}

const NODE_WIDTH = 110
const NODE_HEIGHT = 64
const NODE_GAP = 65
const PADDING = 50

export default function LinkedListVisualizer({
  data,
}: Props) {
  const markerId = useId()
  const [zoom, setZoom] = useState(1)

  // Preserve node order using object identities.
  // Reversing pointers must not reverse the layout.

  const nodes = [...data.nodes].sort(
    (a, b) =>
      a.id.localeCompare(
        b.id,
        undefined,
        { numeric: true },
      ),
  )

  const positions = new Map(
    nodes.map((node, index) => [
      node.id,
      PADDING + index * (NODE_WIDTH + NODE_GAP),
    ]),
  )

  const width = Math.max(
    320,
    nodes.length * (NODE_WIDTH + NODE_GAP) +
      PADDING * 2,
  )

  const nodeY = 100
  const height = 260

  return (
    <section className="ll-visualizer">
      <div className="ll-toolbar">
        <div className="ll-title">
          <Link2 size={16} />

          <span>
            {data.circular
              ? 'Circular Linked List'
              : data.doublyLinked
                ? 'Doubly Linked List'
                : 'Singly Linked List'}
          </span>
        </div>

        <div className="recursion-zoom">
          <button
            className="icon-button"
            title="Zoom out"
            onClick={() =>
              setZoom(z => Math.max(0.3, z - 0.1))
            }
          >
            <Minus size={15} />
          </button>

          <span>{Math.round(zoom * 100)}%</span>

          <button
            className="icon-button"
            title="Zoom in"
            onClick={() =>
              setZoom(z => Math.min(2, z + 0.1))
            }
          >
            <Plus size={15} />
          </button>

          <button
            className="icon-button"
            title="Reset zoom"
            onClick={() => setZoom(1)}
          >
            <RotateCcw size={14} />
          </button>
        </div>
      </div>

      <div className="ll-stats">
        {nodes.length} nodes
        {' · '}
        {Object.keys(data.pointers).length} pointers
      </div>

      <div className="ll-canvas">
        <svg
          width={width * zoom}
          height={height * zoom}
          viewBox={`0 0 ${width} ${height}`}
        >
          <defs>
            <marker
              id={markerId}
              markerWidth="8"
              markerHeight="8"
              refX="7"
              refY="4"
              orient="auto"
            >
              <path
                d="M0 0 L8 4 L0 8 Z"
                fill="#94a3b8"
              />
            </marker>
          </defs>

          {/* Actual next and previous references */}

          {nodes.map(node => {
            const x = positions.get(node.id)!

            const nextX = node.next
              ? positions.get(node.next)
              : undefined

            const previousX = node.previous
              ? positions.get(node.previous)
              : undefined

            return (
              <g key={`edges-${node.id}`}>
                {node.next === null && (
                  <>
                    <path
                      className="ll-edge"
                      d={`M ${x + NODE_WIDTH}
                        ${nodeY + 32}
                        h 25`}
                      markerEnd={`url(#${markerId})`}
                    />

                    <text
                      x={x + NODE_WIDTH + 30}
                      y={nodeY + 36}
                      className="ll-null"
                    >
                      null
                    </text>
                  </>
                )}

                {nextX !== undefined && (
                  <path
                    className={
                      node.changed
                        ? 'll-edge ll-edge-changed'
                        : 'll-edge'
                    }
                    d={
                      nextX === x
                        ? `M ${x + NODE_WIDTH} ${nodeY + 32} C ${x + NODE_WIDTH + 55} ${nodeY + 32}, ${x + NODE_WIDTH + 55} ${nodeY - 30}, ${x + NODE_WIDTH / 2} ${nodeY - 30} V ${nodeY}`
                        : nextX > x
                        ? `M ${x + NODE_WIDTH}
                            ${nodeY + 32}
                            C ${x + NODE_WIDTH + 25}
                            ${nodeY + 32},
                            ${nextX - 25}
                            ${nodeY + 32},
                            ${nextX}
                            ${nodeY + 32}`
                        : `M ${x + NODE_WIDTH / 2}
                            ${nodeY + NODE_HEIGHT}
                            V ${nodeY + 100}
                            H ${nextX + NODE_WIDTH / 2}
                            V ${nodeY + NODE_HEIGHT}`
                    }
                    markerEnd={`url(#${markerId})`}
                  />
                )}

                {previousX !== undefined && (
                  <path
                    className="ll-edge ll-edge-prev"
                    d={`M ${x + NODE_WIDTH / 2}
                        ${nodeY}
                        V ${nodeY - 25}
                        H ${previousX + NODE_WIDTH / 2}
                        V ${nodeY}`}
                    markerEnd={`url(#${markerId})`}
                  />
                )}

                {node.next !== null &&
                  nextX === undefined && (
                    <text
                      x={x + NODE_WIDTH + 8}
                      y={nodeY + 36}
                      className="ll-null"
                    >
                      ?
                    </text>
                  )}
              </g>
            )
          })}

          {/* Node boxes */}

          {nodes.map(node => {
            const x = positions.get(node.id)!

            const value =
              typeof node.value === 'string'
                ? node.value
                : JSON.stringify(node.value)

            return (
              <g
                key={node.id}
                className={
                  node.changed
                    ? 'll-node ll-node-changed'
                    : 'll-node'
                }
              >
                <title>
                  {node.id}: {value}
                </title>

                <rect
                  x={x}
                  y={nodeY}
                  width={NODE_WIDTH}
                  height={NODE_HEIGHT}
                  rx={8}
                />

                <line
                  x1={x + 80}
                  x2={x + 80}
                  y1={nodeY}
                  y2={nodeY + NODE_HEIGHT}
                  className="ll-divider"
                />

                <text
                  x={x + 40}
                  y={nodeY + 38}
                  className="ll-value"
                  textAnchor="middle"
                >
                  {value.length > 9
                    ? `${value.slice(0, 8)}â€¦`
                    : value}
                </text>

                <circle
                  cx={x + 95}
                  cy={nodeY + 32}
                  r={4}
                  fill="#94a3b8"
                />

                <text
                  x={x + NODE_WIDTH / 2}
                  y={nodeY + NODE_HEIGHT + 18}
                  className="ll-object-id"
                  textAnchor="middle"
                >
                  {node.id}
                </text>
              </g>
            )
          })}

          {/* Variable pointers */}

          {nodes.map(node => {
            const names = Object.entries(
              data.pointers,
            )
              .filter(([, id]) => id === node.id)
              .map(([name]) => name)

            if (names.length === 0) return null

            const x =
              positions.get(node.id)! +
              NODE_WIDTH / 2

            const label = names.join(', ')

            return (
              <g key={`pointer-${node.id}`}>
                <title>{label}</title>

                <text
                  x={x}
                  y={30}
                  className="ll-pointer"
                  textAnchor="middle"
                >
                  {label.length > 20
                    ? `${label.slice(0, 19)}â€¦`
                    : label}
                </text>

                <path
                  d={`M ${x} 40 V ${nodeY - 5}`}
                  className="ll-pointer-edge"
                  markerEnd={`url(#${markerId})`}
                />
              </g>
            )
          })}
        </svg>
      </div>

      <div className="ll-legend">
        <span>â— Existing node</span>
        <span className="ll-legend-changed">
          â— Created / modified
        </span>
        <span>â†’ Pointer</span>
      </div>
    </section>
  )
}
