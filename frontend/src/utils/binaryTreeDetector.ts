import type { JsonValue, TraceEvent } from '../types/execution'
import type { BinaryTreeVisualization } from '../types/visualization'

function ref(value: JsonValue | undefined): string | null {
  return value && typeof value === 'object' && !Array.isArray(value) && typeof value.$ref === 'string' ? value.$ref : null
}

export function detectBinaryTrees(event: TraceEvent, previous?: TraceEvent): BinaryTreeVisualization[] {
  const objects = event.objects ?? {}
  const candidates = new Map(Object.entries(objects).filter(([, o]) =>
    o.type !== 'list' && o.type !== 'dict' &&
    ['left', 'right'].every(k => Object.prototype.hasOwnProperty.call(o.attributes, k) && (o.attributes[k] === null || ref(o.attributes[k]) !== null))))
  const adjacency = new Map([...candidates.keys()].map(id => [id, new Set<string>()]))
  for (const [id, o] of candidates) {
    for (const child of [ref(o.attributes.left), ref(o.attributes.right)]) {
      if (child && candidates.has(child)) { adjacency.get(id)!.add(child); adjacency.get(child)!.add(id) }
    }
  }
  const seen = new Set<string>()
  const result: BinaryTreeVisualization[] = []
  for (const id of candidates.keys()) {
    if (seen.has(id)) continue
    const ids = [id]; seen.add(id)
    for (let i = 0; i < ids.length; i++) for (const child of adjacency.get(ids[i])!) {
      if (!seen.has(child)) { seen.add(child); ids.push(child) }
    }
    const members = new Set(ids)
    const pointers = Object.fromEntries(Object.entries(event.references ?? {}).filter(([, target]) => members.has(target)))
    const nodes = ids.map(id => {
      const o = candidates.get(id)!
      const key = ['value', 'val', 'data', 'key', 'item'].find(k => Object.prototype.hasOwnProperty.call(o.attributes, k))
      return { id, value: key ? o.attributes[key] : o.type, left: ref(o.attributes.left), right: ref(o.attributes.right), changed: JSON.stringify(o) !== JSON.stringify(previous?.objects?.[id]) }
    })
    const incoming = new Map(ids.map(id => [id, 0]))
    let incomplete = Boolean(event.objects_truncated)
    for (const n of nodes) for (const child of [n.left, n.right]) if (child) {
      if (members.has(child)) incoming.set(child, incoming.get(child)! + 1)
      else incomplete = true
    }
    const roots = ids.filter(id => incoming.get(id) === 0)
    const anchor = [...ids].sort((a,b) => a.localeCompare(b, undefined, {numeric:true}))[0]
    const root = roots[0] ?? pointers.root ?? anchor
    const visited = new Set<string>(); const pending = [root]
    while (pending.length) { const current = pending.pop()!; if (visited.has(current) || !members.has(current)) continue; visited.add(current); const n = nodes.find(n => n.id === current)!; for (const child of [n.left,n.right]) if (child) pending.push(child) }
    const irregular = roots.length !== 1 || [...incoming.values()].some(n => n > 1) || visited.size !== ids.length
    const pointerChanged = Object.entries(pointers).some(([k,v]) => previous?.references?.[k] !== v) || Object.entries(previous?.references ?? {}).some(([k,v]) => members.has(v) && pointers[k] !== v)
    result.push({type:'binary-tree', id:`binary-tree:${anchor}`, name: Object.keys(pointers).find(k => pointers[k] === root) ?? 'Binary Tree', nodes, root, pointers, incomplete, irregular, activity: ids.every(id => !previous?.objects?.[id]) ? 'created' : nodes.some(n => n.changed) || pointerChanged ? 'modified' : 'unchanged'})
  }
  return result
}
