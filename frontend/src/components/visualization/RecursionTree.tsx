import { useMemo, useState } from 'react'

import {
  Network,
  Minus,
  Plus,
  RotateCcw,
} from 'lucide-react'

import type {
  CallNode,
  CallTreeState,
} from '../../utils/traceTree'

import {
  formatCall,
  formatValue,
} from '../../utils/traceTree'

interface Props {
  tree: CallTreeState
  onSelectStep: (step: number) => void
}

interface PositionedNode {
  node: CallNode
  x: number
  y: number
}

const NODE_WIDTH = 180
const NODE_HEIGHT = 68
const HORIZONTAL_GAP = 32
const VERTICAL_GAP = 55
const PADDING = 30
const MAX_VISIBLE_NODES = 250

function createLayout(tree: CallTreeState) {
  const visibleNodes = tree.nodes.slice(
    0,
    MAX_VISIBLE_NODES,
  )

  const nodeMap = new Map(
    visibleNodes.map(node => [node.id, node]),
  )

  const positioned: PositionedNode[] = []

  let nextLeaf = 0
  let maxLevel = 0

  function visit(id: string, level: number): number {
    const node = nodeMap.get(id)

    if (!node) return 0

    maxLevel = Math.max(maxLevel, level)

    const children = node.children.filter(
      childId => nodeMap.has(childId),
    )

    let x: number

    if (children.length === 0) {
      x =
        PADDING +
        NODE_WIDTH / 2 +
        nextLeaf *
          (NODE_WIDTH + HORIZONTAL_GAP)

      nextLeaf++
    } else {
      const childPositions = children.map(
        childId => visit(childId, level + 1),
      )

      x =
        (childPositions[0] +
          childPositions[childPositions.length - 1]) /
        2
    }

    positioned.push({
      node,
      x,
      y:
        PADDING +
        level * (NODE_HEIGHT + VERTICAL_GAP),
    })

    return x
  }

  for (const root of tree.roots) {
    if (nodeMap.has(root)) {
      visit(root, 0)
    }
  }

  return {
    nodes: positioned,
    width: Math.max(
      320,
      PADDING * 2 +
        nextLeaf * (NODE_WIDTH + HORIZONTAL_GAP) -
        HORIZONTAL_GAP,
    ),
    height:
      PADDING * 2 +
      maxLevel * (NODE_HEIGHT + VERTICAL_GAP) +
      NODE_HEIGHT,
  }
}

export default function RecursionTree({
  tree,
  onSelectStep,
}: Props) {
  const [zoom, setZoom] = useState(1)

  const layout = useMemo(
    () => createLayout(tree),
    [tree],
  )

  const positions = new Map(
    layout.nodes.map(item => [
      item.node.id,
      item,
    ]),
  )

  if (tree.nodes.length === 0) {
    return null
  }

  return (
    <section className="trace-card recursion-card">
      <div className="recursion-header">
        <div className="recursion-title">
          <Network size={18} />

          <h3>Function Call Tree</h3>
        </div>

        <div className="recursion-zoom">
          <button
            className="icon-button"
            title="Zoom out"
            onClick={() =>
              setZoom(value =>
                Math.max(0.3, value - 0.15),
              )
            }
          >
            <Minus size={15} />
          </button>

          <span>{Math.round(zoom * 100)}%</span>

          <button
            className="icon-button"
            title="Zoom in"
            onClick={() =>
              setZoom(value =>
                Math.min(2, value + 0.15),
              )
            }
          >
            <Plus size={15} />
          </button>

          <button
            className="icon-button"
            title="Reset zoom"
            onClick={() => setZoom(1)}
          >
            <RotateCcw size={15} />
          </button>
        </div>
      </div>

      <div className="recursion-stats">
        <span>{tree.nodes.length} calls</span>

        <span>
          {
            tree.nodes.filter(
              node => node.status === 'returned',
            ).length
          }{' '}
          returned
        </span>

        <span>{tree.stack.length} active</span>
      </div>

      <div className="recursion-canvas">
        <svg
          width={layout.width * zoom}
          height={layout.height * zoom}
          viewBox={`0 0 ${layout.width} ${layout.height}`}
        >
          {layout.nodes.map(({ node, x, y }) => {
            if (!node.parentId) return null

            const parent = positions.get(
              node.parentId,
            )

            if (!parent) return null

            const startY = parent.y + NODE_HEIGHT
            const endY = y
            const middleY = (startY + endY) / 2

            return (
              <path
                key={`edge-${node.id}`}
                d={`
                  M ${parent.x} ${startY}
                  C ${parent.x} ${middleY},
                    ${x} ${middleY},
                    ${x} ${endY}
                `}
                className={
                  node.status === 'returned'
                    ? 'recursion-edge returned'
                    : 'recursion-edge'
                }
              />
            )
          })}

          {layout.nodes.map(({ node, x, y }) => {
            const active =
              tree.activeId === node.id

            const status = active
              ? 'current'
              : node.status

            const call = formatCall(node)

            const description =
              node.status === 'returned'
                ? `Returned: ${formatValue(
                    node.returnValue,
                  )}`
                : node.status === 'unwound'
                  ? 'Frame exited'
                  : active
                    ? 'Executing'
                    : 'Waiting'

            return (
              <g
                key={node.id}
                className={`recursion-node ${status}`}
                role="button"
                tabIndex={0}
                aria-label={call}
                onClick={() =>
                  onSelectStep(node.callStep)
                }
                onKeyDown={event => {
                  if (
                    event.key === 'Enter' ||
                    event.key === ' '
                  ) {
                    event.preventDefault()
                    onSelectStep(node.callStep)
                  }
                }}
              >
                <title>
                  {call} — {description}
                </title>

                <rect
                  x={x - NODE_WIDTH / 2}
                  y={y}
                  width={NODE_WIDTH}
                  height={NODE_HEIGHT}
                  rx={9}
                />

                <text
                  x={x}
                  y={y + 27}
                  textAnchor="middle"
                  className="recursion-node-label"
                >
                  {call.length > 23
                    ? `${call.slice(0, 20)}...`
                    : call}
                </text>

                <text
                  x={x}
                  y={y + 48}
                  textAnchor="middle"
                  className="recursion-node-status"
                >
                  {description.length > 26
                    ? `${description.slice(0, 23)}...`
                    : description}
                </text>
              </g>
            )
          })}
        </svg>
      </div>

      {tree.nodes.length > MAX_VISIBLE_NODES && (
        <p className="recursion-limit">
          Showing the first {MAX_VISIBLE_NODES} calls.
          Playback and stack inspection remain available.
        </p>
      )}

      <div className="recursion-legend">
        <span>
          <i className="legend-current" />
          Executing
        </span>

        <span>
          <i className="legend-waiting" />
          Waiting
        </span>

        <span>
          <i className="legend-returned" />
          Returned
        </span>

        <span>
          <i className="legend-unwound" />
          Exited
        </span>
      </div>

      <p className="recursion-hint">
        Click any function node to jump to its call step.
      </p>
    </section>
  )
}