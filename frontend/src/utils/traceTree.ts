import type {
  JsonValue,
  TraceEvent,
} from '../types/execution'

export interface CallNode {
  id: string
  name: string
  depth: number
  parentId: string | null
  arguments: Record<string, JsonValue>
  variables: Record<string, JsonValue>
  children: string[]
  callStep: number
  returnStep?: number
  returnValue?: JsonValue
  status: 'active' | 'returned' | 'unwound'
}

export interface CallTreeState {
  nodes: CallNode[]
  roots: string[]
  stack: CallNode[]
  activeId: string | null
}

export function formatValue(
  value: JsonValue | undefined,
): string {
  if (value === undefined) return '—'

  return JSON.stringify(value)
}

export function formatCall(node: CallNode): string {
  const args = Object.entries(node.arguments)
    .map(([name, value]) => {
      const formatted = formatValue(value)

      return `${name}=${
        formatted.length > 30
          ? `${formatted.slice(0, 27)}...`
          : formatted
      }`
    })
    .join(', ')

  return `${node.name}(${args})`
}

export function buildCallTree(
  events: TraceEvent[],
  currentStep: number,
): CallTreeState {
  const nodes: CallNode[] = []
  const nodeMap = new Map<string, CallNode>()
  const roots: string[] = []
  const stack: CallNode[] = []

  const lastIndex = Math.min(
    currentStep,
    events.length - 1,
  )

  function unwindToDepth(depth: number) {
    while (
      stack.length > 0 &&
      stack[stack.length - 1].depth > depth
    ) {
      const removed = stack.pop()

      if (removed?.status === 'active') {
        removed.status = 'unwound'
      }
    }
  }

  for (let index = 0; index <= lastIndex; index++) {
    const event = events[index]

    if (!event) continue

    if (event.function === '<module>') {
      unwindToDepth(event.depth)
      continue
    }

    if (
      event.event === 'function_call' ||
      event.event === 'call'
    ) {
      unwindToDepth(event.depth - 1)

      const parent = stack[stack.length - 1]

      const node: CallNode = {
        id: `call-${index}`,
        name: event.function,
        depth: event.depth,
        parentId: parent?.id ?? null,
        arguments: { ...event.arguments },
        variables: { ...event.variables },
        children: [],
        callStep: index,
        status: 'active',
      }

      nodes.push(node)
      nodeMap.set(node.id, node)

      if (parent) {
        parent.children.push(node.id)
      } else {
        roots.push(node.id)
      }

      stack.push(node)

      continue
    }

    unwindToDepth(event.depth)

    const frame = stack[stack.length - 1]

    if (
      !frame ||
      frame.name !== event.function ||
      frame.depth !== event.depth
    ) {
      continue
    }

    if (event.variables) {
      frame.variables = { ...event.variables }
    }

    if (
      event.event === 'function_return' ||
      event.event === 'return'
    ) {
      frame.returnStep = index
      frame.returnValue = event.return_value
      frame.status = 'returned'

      stack.pop()
    }
  }

  return {
    nodes,
    roots: roots.filter(id => nodeMap.has(id)),
    stack: [...stack],
    activeId: stack[stack.length - 1]?.id ?? null,
  }
}