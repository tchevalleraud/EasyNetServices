/** Formats a duration in seconds with a unit adapted to its magnitude (ns to s). */
export function formatSeconds(value: number, signed = false): string {
  const abs = Math.abs(value)
  const sign = signed && value > 0 ? '+' : value < 0 ? '-' : ''
  if (abs === 0) return '0 s'
  if (abs < 1e-6) return `${sign}${(abs * 1e9).toFixed(0)} ns`
  if (abs < 1e-3) return `${sign}${(abs * 1e6).toFixed(1)} µs`
  if (abs < 1) return `${sign}${(abs * 1e3).toFixed(2)} ms`
  return `${sign}${abs.toFixed(2)} s`
}

/** Formats a duration in whole seconds, e.g. 3725 → "1h 2m". */
export function formatDuration(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const rest = seconds % 60
  const join = (major: string, minor: string, value: number) => (value > 0 ? `${major} ${minor}` : major)
  if (days > 0) return join(`${days}d`, `${hours}h`, hours)
  if (hours > 0) return join(`${hours}h`, `${minutes}m`, minutes)
  if (minutes > 0) return join(`${minutes}m`, `${rest}s`, rest)
  return `${seconds}s`
}

/** Converts an API field path ("sources[0].address") to a form path ("sources.0.address"). */
export function toFormPath(field: string): string {
  return field.replace(/\[(\d+)\]/g, '.$1')
}
