import { Alert, Card, Group, SimpleGrid, Skeleton, Stack, Text } from '@mantine/core'
import { IconInfoCircle } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { ServiceStateBadge } from '../../components/ServiceStateBadge'
import { formatDuration } from '../../utils/format'
import { useDnsStatus } from './queries'

function Metric({ label, value, hint }: { label: string; value: ReactNode; hint?: string }) {
  return (
    <Card withBorder radius="md" padding="md">
      <Text size="xs" c="dimmed" tt="uppercase" fw={600} mb={6}>
        {label}
      </Text>
      <Text fw={600} size="lg">
        {value}
      </Text>
      {hint && (
        <Text size="xs" c="dimmed">
          {hint}
        </Text>
      )}
    </Card>
  )
}

const number = new Intl.NumberFormat('en-US')

export function DnsStatusPanel() {
  const { data: status, isPending, error } = useDnsStatus()

  if (error) return <Alert color="red">Unable to load the DNS status: {error.message}</Alert>
  if (isPending) return <Skeleton h={200} />

  const stats = status.stats
  const lookups = stats ? stats.cacheHits + stats.cacheMisses : 0
  const hitRate = stats && lookups > 0 ? `${((stats.cacheHits / lookups) * 100).toFixed(1)} %` : '—'

  return (
    <Stack>
      {status.recursor === 'unavailable' && (
        <Alert color="yellow" icon={<IconInfoCircle size={18} />}>
          The DNS services cannot be managed: the server is not running inside the EasyNetServices container.
        </Alert>
      )}
      {status.recursor === 'disabled' && (
        <Alert color="blue" icon={<IconInfoCircle size={18} />}>
          The resolver is disabled: clients cannot query the DNS server. Local zones can still be edited. Enable it in
          the Resolver tab.
        </Alert>
      )}

      <SimpleGrid cols={{ base: 1, sm: 2 }}>
        <Card withBorder radius="md" padding="md">
          <Group justify="space-between">
            <div>
              <Text fw={600}>Resolver</Text>
              <Text size="sm" c="dimmed">
                Answers the clients on port 53
              </Text>
            </div>
            <ServiceStateBadge state={status.recursor} />
          </Group>
        </Card>
        <Card withBorder radius="md" padding="md">
          <Group justify="space-between">
            <div>
              <Text fw={600}>Authoritative server</Text>
              <Text size="sm" c="dimmed">
                Hosts the local zones
              </Text>
            </div>
            <ServiceStateBadge state={status.authoritative} />
          </Group>
        </Card>
      </SimpleGrid>

      {stats && (
        <SimpleGrid cols={{ base: 2, md: 3 }}>
          <Metric label="Queries" value={number.format(stats.questions)} hint={`since ${formatDuration(stats.uptimeSeconds)}`} />
          <Metric label="Cache hit rate" value={hitRate} hint={`${number.format(stats.cacheEntries)} entries cached`} />
          <Metric label="Successful answers" value={number.format(stats.noErrorAnswers)} />
          <Metric label="Unknown names" value={number.format(stats.nxdomainAnswers)} hint="NXDOMAIN answers" />
          <Metric label="Failures" value={number.format(stats.servfailAnswers)} hint="SERVFAIL answers" />
          <Metric label="Uptime" value={formatDuration(stats.uptimeSeconds)} />
        </SimpleGrid>
      )}
    </Stack>
  )
}
