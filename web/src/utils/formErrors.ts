import type { FieldError } from '../api/client'
import { toFormPath } from './format'

/**
 * Converts API field errors to form errors. Errors on an item of a list of
 * strings edited with a single input (e.g. "allowFrom[2]") are reported on the
 * list itself, prefixed with the item number.
 */
export function toFormErrors(details: FieldError[], stringLists: string[] = []): Record<string, string> {
  const errors: Record<string, string> = {}
  for (const d of details) {
    let path = toFormPath(d.field)
    let message = d.message
    const match = path.match(/^(.*)\.(\d+)$/)
    if (match && stringLists.some((list) => new RegExp(`^${list}$`).test(match[1]))) {
      path = match[1]
      message = `Item ${Number(match[2]) + 1}: ${message}`
    }
    errors[path] = errors[path] ? `${errors[path]}; ${message}` : message
  }
  return errors
}
