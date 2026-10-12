import { Alert, Card, Group, SimpleGrid, Skeleton, Stack, Table, Text } from '@mantine/core'
import { IconAlertTriangle, IconInfoCircle } from '@tabler/icons-react'
import { Link } from 'react-router'
import type { DhcpFamily, DhcpFamilyStatus } from '../../api/types'
import { ServiceStateBadge } from '../../components/ServiceStateBadge'
import { familyLabel, formatCount, useDhcpStatus } from './queries'
import { UsageBar } from './UsageBar'

function FamilyCard({ family, status }: { family: DhcpFamily; status: DhcpFamilyStatus }) {
  return (
    <Card withBorder radius="md" padding="md">
      <Group justify="space-between" mb="sm">
        <div>
          <Text fw={600}>{familyLabel(family)} server</Text>
          <Text size="sm" c="dimmed">
            {status.state === 'running'
              ? `${formatCount(status.packetsReceived)} messages received, ${formatCount(status.packetsSent)} sent`
              : family === 'v4'
                ? 'Assigns IPv4 addresses'
                : 'Assigns IPv6 addresses'}
          </Text>
        </div>
        <ServiceStateBadge state={status.state} />
      </Group>
      {status.socketErrors.length > 0 && (
        <Alert color="orange" icon={<IconAlertTriangle size={18} />} mb="sm" title="Cannot listen on every interface">
          {status.socketErrors.map((e) => (
            <div key={e}>{e}</div>
          ))}
        </Alert>
      )}
      {status.subnets.length === 0 ? (
        <Text size="sm" c="dimmed">
          No subnet.{' '}
          <Text component={Link} to="/dhcp?tab=subnets" size="sm" c="blue">
            Add one
          </Text>
        </Text>
      ) : (
        <Table verticalSpacing={6}>
          <Table.Tbody>
            {status.subnets.map((s) => (
              <Table.Tr key={s.id}>
                <Table.Td ff="monospace">
                  <Text component={Link} to={`/dhcp/subnets/${family}/${s.id}`} size="sm" ff="monospace">
                    {s.subnet}
                  </Text>
                </Table.Td>
                <Table.Td w="50%">
                  {status.state === 'running' ? (
                    <UsageBar assigned={s.assigned} total={s.total} />
                  ) : (
                    <Text size="xs" c="dimmed">
                      —
                    </Text>
                  )}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Card>
  )
}

export function DhcpStatusPanel() {
  const { data: status, isPending, error } = useDhcpStatus()

  if (error) return <Alert color="red">Unable to load the DHCP status: {error.message}</Alert>
  if (isPending) return <Skeleton h={200} />

  return (
    <Stack>
      {status.dhcpv4.state === 'unavailable' && (
        <Alert color="yellow" icon={<IconInfoCircle size={18} />}>
          The DHCP services cannot be managed: the server is not running inside the EasyNetServices container.
        </Alert>
      )}
      {status.dhcpv4.state === 'disabled' && status.dhcpv6.state === 'disabled' && (
        <Alert color="blue" icon={<IconInfoCircle size={18} />}>
          The DHCP servers are disabled. Choose the interfaces to serve and enable them in the Settings tab, then add
          a subnet.
        </Alert>
      )}
      <SimpleGrid cols={{ base: 1, md: 2 }}>
        <FamilyCard family="v4" status={status.dhcpv4} />
        <FamilyCard family="v6" status={status.dhcpv6} />
      </SimpleGrid>
    </Stack>
  )
}
