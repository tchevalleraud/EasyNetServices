import { Alert, Badge, Card, Group, SimpleGrid, Skeleton, Stack, Table, Text, Tooltip } from '@mantine/core'
import { IconInfoCircle } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import type { NtpSourceStatus } from '../../api/types'
import { ServiceStateBadge } from '../../components/ServiceStateBadge'
import { formatDuration, formatSeconds } from '../../utils/format'
import { useNtpStatus } from './queries'

const sourceStateDisplay: Record<string, { color: string; label: string; help: string }> = {
  selected: { color: 'green', label: 'Selected', help: 'Source currently used to synchronize the clock' },
  combined: { color: 'teal', label: 'Combined', help: 'Acceptable source, combined with the selected one' },
  'not-combined': { color: 'gray', label: 'Not combined', help: 'Acceptable source, not used at the moment' },
  unreachable: { color: 'red', label: 'Unreachable', help: 'No reply received from this source' },
  falseticker: { color: 'orange', label: 'Falseticker', help: 'Time inconsistent with the other sources' },
  'too-variable': { color: 'yellow', label: 'Too variable', help: 'Measurements are too unstable' },
}

function SourceStateBadge({ state }: { state: string }) {
  const display = sourceStateDisplay[state] ?? { color: 'gray', label: state, help: '' }
  return (
    <Tooltip label={display.help} disabled={!display.help} withArrow>
      <Badge variant="light" color={display.color}>
        {display.label}
      </Badge>
    </Tooltip>
  )
}

/** Reach is an octal register of the last eight polls; counts the successful ones. */
function reachCount(reach: string): number {
  const value = parseInt(reach, 8)
  if (Number.isNaN(value)) return 0
  return value.toString(2).split('').filter((bit) => bit === '1').length
}

function Metric({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Card withBorder radius="md" padding="md">
      <Text size="xs" c="dimmed" tt="uppercase" fw={600} mb={6}>
        {label}
      </Text>
      {children}
    </Card>
  )
}

function SourcesTable({ sources }: { sources: NtpSourceStatus[] }) {
  if (sources.length === 0) {
    return (
      <Text size="sm" c="dimmed">
        No source reported yet.
      </Text>
    )
  }
  return (
    <Table.ScrollContainer minWidth={640}>
      <Table verticalSpacing="xs" highlightOnHover>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>State</Table.Th>
            <Table.Th>Address</Table.Th>
            <Table.Th>Stratum</Table.Th>
            <Table.Th>Poll</Table.Th>
            <Table.Th>Reach</Table.Th>
            <Table.Th>Last sample</Table.Th>
            <Table.Th>Offset</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {sources.map((source) => (
            <Table.Tr key={source.address}>
              <Table.Td>
                <SourceStateBadge state={source.state} />
              </Table.Td>
              <Table.Td ff="monospace">{source.address}</Table.Td>
              <Table.Td>{source.stratum || '—'}</Table.Td>
              <Table.Td>{formatDuration(source.pollSeconds)}</Table.Td>
              <Table.Td>
                <Tooltip label={`Register ${source.reach} (octal)`} withArrow>
                  <Text size="sm" span>
                    {reachCount(source.reach)}/8
                  </Text>
                </Tooltip>
              </Table.Td>
              <Table.Td>{source.lastRxSeconds < 0 ? '—' : `${formatDuration(source.lastRxSeconds)} ago`}</Table.Td>
              <Table.Td>
                {source.lastRxSeconds < 0
                  ? '—'
                  : `${formatSeconds(source.offsetSeconds, true)} ± ${formatSeconds(source.errorSeconds)}`}
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  )
}

export function NtpStatusPanel() {
  const { data: status, isPending, error } = useNtpStatus()

  if (error) {
    return <Alert color="red">Unable to load the NTP status: {error.message}</Alert>
  }
  if (isPending) {
    return <Skeleton h={200} />
  }

  const tracking = status.tracking

  return (
    <Stack>
      {status.state === 'unavailable' && (
        <Alert color="yellow" icon={<IconInfoCircle size={18} />}>
          The NTP daemon cannot be managed: the server is not running inside the EasyNetServices container.
        </Alert>
      )}

      <SimpleGrid cols={{ base: 2, md: 4 }}>
        <Metric label="Service">
          <ServiceStateBadge state={status.state} />
        </Metric>
        <Metric label="Synchronization">
          {status.state !== 'running' ? (
            <Text c="dimmed">—</Text>
          ) : status.synchronized ? (
            <Badge variant="light" color="green">
              Synchronized
            </Badge>
          ) : (
            <Badge variant="light" color="orange">
              Not synchronized
            </Badge>
          )}
        </Metric>
        <Metric label="Stratum">
          <Text fw={600}>{status.synchronized && tracking ? tracking.stratum : '—'}</Text>
        </Metric>
        <Metric label="Offset">
          <Text fw={600}>{status.synchronized && tracking ? formatSeconds(tracking.offsetSeconds, true) : '—'}</Text>
        </Metric>
      </SimpleGrid>

      {status.synchronized && tracking && (
        <Group gap="xs">
          <Text size="sm" c="dimmed">
            Reference:
          </Text>
          <Text size="sm" ff="monospace">
            {tracking.reference}
          </Text>
          <Text size="sm" c="dimmed">
            · root delay {formatSeconds(tracking.rootDelaySeconds)} · leap status {tracking.leapStatus}
          </Text>
        </Group>
      )}

      {status.state === 'running' && (
        <Card withBorder radius="md" padding="lg">
          <Text fw={600} mb="sm">
            Upstream sources
          </Text>
          <SourcesTable sources={status.sources} />
        </Card>
      )}
    </Stack>
  )
}
