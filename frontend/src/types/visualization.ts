import type {
  JsonValue,
  ObjectSnapshot,
} from './execution'

export type VisualizationType =
  | 'array'
  | 'matrix'
  | 'linked-list'
  | 'recursion'
  | 'binary-tree'
  | 'graph'
  | 'stack'
  | 'queue'
  | 'heap'
  | 'object'

export type VisualizationMode =
  | 'auto'
  | 'all'
  | 'focus'

export type ActivityState =
  | 'created'
  | 'modified'
  | 'active'
  | 'unchanged'

export interface VisualizationBase {
  id: string
  name: string
  type: VisualizationType
  activity: ActivityState
}

export interface ArrayVisualization
  extends VisualizationBase {
  type: 'array'
  values: JsonValue[]
}

export interface MatrixVisualization
  extends VisualizationBase {
  type: 'matrix'
  values: JsonValue[][]
}

export interface LinkedListNode {
  id: string
  value: JsonValue
  next: string | null
  previous: string | null
  changed: boolean
}

export interface LinkedListVisualization
  extends VisualizationBase {
  type: 'linked-list'
  nodes: LinkedListNode[]
  pointers: Record<string, string>
  circular: boolean
  doublyLinked: boolean
}

export interface ObjectVisualization
  extends VisualizationBase {
  type: 'object'
  objects: Record<string, ObjectSnapshot>
  references: Record<string, string>
  truncated: boolean
}

export interface RecursionVisualization
  extends VisualizationBase {
  type: 'recursion'
  functionName: string
}

export type VisualizationDescriptor =
  | ArrayVisualization
  | MatrixVisualization
  | LinkedListVisualization
  | ObjectVisualization
  | RecursionVisualization