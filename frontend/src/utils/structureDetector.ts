import { detectBinaryTrees } from './binaryTreeDetector'
import type {
  JsonValue,
  TraceEvent,
  ObjectSnapshot,
} from '../types/execution'

import type {
  VisualizationDescriptor,
  LinkedListVisualization,
  LinkedListNode,
  ActivityState,
} from '../types/visualization'

// ==========================================
// SHARED HELPERS
// ==========================================

function equal(
  first: unknown,
  second: unknown,
): boolean {
  return JSON.stringify(first) ===
    JSON.stringify(second)
}

function getReference(
  value: JsonValue | undefined,
): string | null {
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

function isMatrix(
  value: JsonValue,
): value is JsonValue[][] {
  return (
    Array.isArray(value) &&
    value.length > 0 &&
    value.every(row => Array.isArray(row))
  )
}

function getActivity(
  current: unknown,
  previous: unknown,
): ActivityState {
  if (previous === undefined) {
    return 'created'
  }

  return equal(current, previous)
    ? 'unchanged'
    : 'modified'
}

// ==========================================
// LINKED LIST DETECTION
// ==========================================

function isLinkedListNode(
  object: ObjectSnapshot,
): boolean {
  const attributes = object.attributes

  if (
    object.type === 'list' ||
    object.type === 'dict'
  ) {
    return false
  }

  if (!Object.hasOwn(attributes, 'next')) {
    return false
  }

  return (
    attributes.next === null ||
    getReference(attributes.next) !== null
  )
}

function getNodeValue(
  object: ObjectSnapshot,
): JsonValue {
  const attributes = object.attributes

  for (const key of [
    'value',
    'val',
    'data',
    'key',
    'item',
  ]) {
    if (Object.hasOwn(attributes, key)) {
      return attributes[key]
    }
  }

  return object.type
}

function getPreviousReference(
  object: ObjectSnapshot,
): string | null {
  return getReference(
    object.attributes.prev ??
    object.attributes.previous,
  )
}

// ==========================================
// DETECT ALL LINKED LIST COMPONENTS
// ==========================================

function detectLinkedLists(
  event: TraceEvent,
  previousEvent?: TraceEvent,
): LinkedListVisualization[] {
  const objects = event.objects ?? {}
  const references = event.references ?? {}

  const previousObjects =
    previousEvent?.objects ?? {}

  const previousReferences =
    previousEvent?.references ?? {}

  const candidates = new Map(
    Object.entries(objects).filter(
      ([, object]) => isLinkedListNode(object),
    ),
  )

  const adjacency = new Map<string, Set<string>>()

  for (const id of candidates.keys()) {
    adjacency.set(id, new Set())
  }

  // Build connected components using both
  // forward and backward pointers.

  for (const [id, object] of candidates) {
    const neighbours = [
      getReference(object.attributes.next),
      getPreviousReference(object),
    ]

    for (const neighbour of neighbours) {
      if (
        neighbour &&
        candidates.has(neighbour)
      ) {
        adjacency.get(id)?.add(neighbour)
        adjacency.get(neighbour)?.add(id)
      }
    }
  }

  const visited = new Set<string>()

  const visualizations:
    LinkedListVisualization[] = []

  for (const startId of candidates.keys()) {
    if (visited.has(startId)) continue

    const component: string[] = []
    const queue = [startId]

    visited.add(startId)

    for (let index = 0; index < queue.length; index++) {
      const id = queue[index]

      component.push(id)

      for (const neighbour of adjacency.get(id) ?? []) {
        if (!visited.has(neighbour)) {
          visited.add(neighbour)
          queue.push(neighbour)
        }
      }
    }

    const componentIds = new Set(component)

    const pointers = Object.fromEntries(
      Object.entries(references).filter(
        ([, id]) => componentIds.has(id),
      ),
    )

    // Prefer the head variable when naming
    // a linked list.

    const pointerNames = Object.keys(pointers)

    const name =
      pointerNames.find(
        name => name.toLowerCase() === 'head',
      ) ??
      pointerNames[0] ??
      'Linked List'

    const incoming = new Set<string>()

    for (const id of component) {
      const object = candidates.get(id)!

      const next = getReference(
        object.attributes.next,
      )

      if (next && componentIds.has(next)) {
        incoming.add(next)
      }
    }

    // Find a natural head node.
    // Circular lists may not have one.

    const headId =
      component.find(id => !incoming.has(id)) ??
      pointers[name] ??
      component[0]

    const orderedIds: string[] = []
    const orderedSet = new Set<string>()

    let current: string | null = headId

    while (
      current &&
      componentIds.has(current) &&
      !orderedSet.has(current)
    ) {
      orderedIds.push(current)
      orderedSet.add(current)

      const object = candidates.get(current)!

      current = getReference(
        object.attributes.next,
      )
    }

    const circular =
      current !== null &&
      orderedSet.has(current)

    // Preserve disconnected traversal branches
    // rather than silently dropping nodes.

    for (const id of component) {
      if (!orderedSet.has(id)) {
        orderedIds.push(id)
      }
    }

    const nodes: LinkedListNode[] =
      orderedIds.map(id => {
        const object = candidates.get(id)!

        return {
          id,
          value: getNodeValue(object),
          next: getReference(
            object.attributes.next,
          ),
          previous: getPreviousReference(object),
          changed: !equal(
            object,
            previousObjects[id],
          ),
        }
      })

    const doublyLinked = component.some(id => {
      const attributes =
        candidates.get(id)!.attributes

      return (
        Object.hasOwn(attributes, 'prev') ||
        Object.hasOwn(attributes, 'previous')
      )
    })

    const pointerChanged =
      Object.entries(pointers).some(
        ([name, id]) =>
          previousReferences[name] !== id,
      ) ||
      Object.entries(previousReferences).some(
        ([name, id]) =>
          componentIds.has(id) &&
          pointers[name] !== id,
      )

    const hasExistingNodes = component.some(
      id => previousObjects[id] !== undefined,
    )

    const activity: ActivityState =
      !hasExistingNodes
        ? 'created'
        : nodes.some(node => node.changed) ||
            pointerChanged
          ? 'modified'
          : 'unchanged'

    // Use the earliest tracked object identity
    // instead of the current head pointer.

    const anchor = [...component].sort(
      (a, b) =>
        a.localeCompare(
          b,
          undefined,
          { numeric: true },
        ),
    )[0]

    visualizations.push({
      id: `linked-list:${anchor}`,
      type: 'linked-list',
      name,
      activity,
      nodes,
      pointers,
      circular,
      doublyLinked,
    })
  }

  return visualizations
}

// ==========================================
// MAIN STRUCTURE DETECTION ENGINE
// ==========================================

export function detectStructures(
  event: TraceEvent,
  previousEvent?: TraceEvent,
): VisualizationDescriptor[] {
  const visualizations:
    VisualizationDescriptor[] = []

  const variables = {
    ...(event.arguments ?? {}),
    ...(event.variables ?? {}),
  }

  const previousVariables = {
    ...(previousEvent?.arguments ?? {}),
    ...(previousEvent?.variables ?? {}),
  }

  const references = event.references ?? {}
  const objects = event.objects ?? {}

  const consumedObjects = new Set<string>()
  const renderedCollections = new Set<string>()

  // ========================================
  // ARRAYS AND MATRICES
  // ========================================

  for (const [name, value] of Object.entries(variables)) {
    if (!Array.isArray(value)) continue

    const objectId = references[name]

    const identity =
      objectId ?? `${event.function}:${name}`

    // Avoid rendering aliases of the same
    // array as separate visualizations.

    if (renderedCollections.has(identity)) {
      continue
    }

    renderedCollections.add(identity)

    const activity = getActivity(
      value,
      previousVariables[name],
    )

    if (isMatrix(value)) {
      visualizations.push({
        id: `matrix:${identity}`,
        type: 'matrix',
        name,
        values: value,
        activity,
      })
    } else {
      visualizations.push({
        id: `array:${identity}`,
        type: 'array',
        name,
        values: value,
        activity,
      })
    }

    if (objectId) {
      consumedObjects.add(objectId)

      // Matrix rows should not appear again
      // as generic object nodes.

      if (isMatrix(value)) {
        const matrixObject = objects[objectId]

        for (const attribute of Object.values(
          matrixObject?.attributes ?? {},
        )) {
          const rowId = getReference(attribute)

          if (rowId) {
            consumedObjects.add(rowId)
          }
        }
      }
    }
  }

  // ========================================
  // LINKED LISTS
  // ========================================

  for (const tree of detectBinaryTrees(event, previousEvent)) {
    visualizations.push(tree)
    for (const node of tree.nodes) consumedObjects.add(node.id)
  }

  const linkedLists = detectLinkedLists(
    event,
    previousEvent,
  )

  for (const linkedList of linkedLists) {
    if (linkedList.nodes.some(node => consumedObjects.has(node.id))) continue
    visualizations.push(linkedList)

    for (const node of linkedList.nodes) {
      consumedObjects.add(node.id)
    }
  }

  // ========================================
  // GENERIC OBJECT FALLBACK
  // ========================================

  const remainingObjects = Object.fromEntries(
    Object.entries(objects).filter(
      ([id]) => !consumedObjects.has(id),
    ),
  )

  const remainingReferences = Object.fromEntries(
    Object.entries(references).filter(
      ([, id]) => !consumedObjects.has(id),
    ),
  )

  // Retain the generic graph when an object
  // references a specialized structure.

  const hasCrossStructureReferences =
    Object.values(remainingObjects).some(
      object =>
        Object.values(object.attributes).some(
          value => {
            const id = getReference(value)

            return (
              id !== null &&
              consumedObjects.has(id)
            )
          },
        ),
    )

  const genericObjects =
    hasCrossStructureReferences
      ? objects
      : remainingObjects

  const genericReferences =
    hasCrossStructureReferences
      ? references
      : remainingReferences

  if (
    Object.keys(genericObjects).length > 0 ||
    event.objects_truncated
  ) {
    const previousObjects = Object.fromEntries(
      Object.keys(genericObjects).map(
        id => [
          id,
          previousEvent?.objects?.[id],
        ],
      ),
    )

    visualizations.push({
      id: 'objects',
      type: 'object',
      name: 'Object References',
      objects: genericObjects,
      references: genericReferences,
      truncated: event.objects_truncated ?? false,
      activity: getActivity(
        genericObjects,
        previousObjects,
      ),
    })
  }

  return visualizations
}
