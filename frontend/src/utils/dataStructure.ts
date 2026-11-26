import type { JsonValue } from '../types/execution'

export type DataStructureType =
  | 'scalar'
  | 'array'
  | 'matrix'
  | 'object'

export function isMatrix(
  value: JsonValue,
): value is JsonValue[][] {
  return (
    Array.isArray(value) &&
    value.length > 0 &&
    value.every(row => Array.isArray(row))
  )
}

export function detectDataStructure(
  value: JsonValue,
): DataStructureType {
  if (isMatrix(value)) {
    return 'matrix'
  }

  if (Array.isArray(value)) {
    return 'array'
  }

  if (
    value !== null &&
    typeof value === 'object'
  ) {
    return 'object'
  }

  return 'scalar'
}