
import type {
  CallTreeState,
} from './traceTree'

export function extractRecursiveTree(
  tree: CallTreeState,
): CallTreeState {
  const nodeMap = new Map(
    tree.nodes.map(node => [node.id, node]),
  )

  const recursiveRoots = new Set<string>()

  // Find functions that appear again
  // within their own ancestor chain.

  for (const node of tree.nodes) {
    let parentId = node.parentId

    while (parentId) {
      const parent = nodeMap.get(parentId)

      if (!parent) break

      if (parent.name === node.name) {
        recursiveRoots.add(parent.id)
      }

      parentId = parent.parentId
    }
  }

  const included = new Set<string>()

  function includeSubtree(id: string) {
    if (included.has(id)) return

    const node = nodeMap.get(id)

    if (!node) return

    included.add(id)

    for (const child of node.children) {
      includeSubtree(child)
    }
  }

  for (const id of recursiveRoots) {
    includeSubtree(id)
  }

  const nodes = tree.nodes
    .filter(node => included.has(node.id))
    .map(node => ({
      ...node,

      parentId:
        node.parentId &&
        included.has(node.parentId)
          ? node.parentId
          : null,

      children: node.children.filter(
        id => included.has(id),
      ),
    }))

  const roots = nodes
    .filter(node => node.parentId === null)
    .map(node => node.id)

  return {
  nodes,
  roots,
  stack: tree.stack.filter(
    node => included.has(node.id),
  ),
  activeId:
    tree.activeId &&
    included.has(tree.activeId)
      ? tree.activeId
      : null,
}
}
