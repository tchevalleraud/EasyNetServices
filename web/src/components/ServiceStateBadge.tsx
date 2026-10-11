import { Badge } from '@mantine/core'
import type { ServiceState } from '../api/types'

const stateDisplay: Record<ServiceState, { color: string; label: string }> = {
  running: { color: 'green', label: 'Running' },
  stopped: { color: 'red', label: 'Stopped' },
  disabled: { color: 'gray', label: 'Disabled' },
  unavailable: { color: 'yellow', label: 'Unavailable' },
}

export function ServiceStateBadge({ state }: { state: ServiceState }) {
  const { color, label } = stateDisplay[state]
  return (
    <Badge variant="light" color={color}>
      {label}
    </Badge>
  )
}
