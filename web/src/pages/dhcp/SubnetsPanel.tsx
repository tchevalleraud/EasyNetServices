import { Alert, Badge, Button, Card, Group, Skeleton, Stack, Table, Text } from '@mantine/core'
import { IconPlus } from '@tabler/icons-react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import type { DhcpFamily } from '../../api/types'
import { NewSubnetModal } from './NewSubnetModal'
import { familyLabel, useDhcpStatus, useSubnets } from './queries'
import { UsageBar } from './UsageBar'

function FamilySubnets({ family, onAdd }: { family: DhcpFamily; onAdd: () => void }) {
  const { data: subnets, isPending, error } = useSubnets(family)
  const { data: status } = useDhcpStatus()
  const navigate = useNavigate()
  const familyStatus = family === 'v4' ? status?.dhcpv4 : status?.dhcpv6

  return (
    <Card withBorder radius="md" padding={0}>
      <Group justify="space-between" p="md">
        <Group gap="sm">
          <Text fw={600}>{familyLabel(family)} subnets</Text>
          {familyStatus && familyStatus.state !== 'running' && (
            <Badge variant="light" color="gray">
              Server {familyStatus.state}
            </Badge>
          )}
        </Group>
        <Button variant="light" size="xs" leftSection={<IconPlus size={14} />} onClick={onAdd}>
          Add subnet
        </Button>
      </Group>
      {error && (
        <Alert color="red" m="md">
          Unable to load the subnets: {error.message}
        </Alert>
      )}
      {isPending && <Skeleton h={80} m="md" />}
      {subnets && subnets.length === 0 && (
        <Text size="sm" c="dimmed" px="md" pb="md">
          No {familyLabel(family)} subnet.
        </Text>
      )}
      {subnets && subnets.length > 0 && (
        <Table.ScrollContainer minWidth={720}>
          <Table highlightOnHover verticalSpacing="xs">
            <Table.Thead>
              <Table.Tr>
                <Table.Th pl="md">Network</Table.Th>
                <Table.Th>Description</Table.Th>
                <Table.Th>Interface</Table.Th>
                <Table.Th>Pools</Table.Th>
                <Table.Th>Reservations</Table.Th>
                <Table.Th pr="md">Usage</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {subnets.map((s) => {
                const usage = familyStatus?.subnets.find((u) => u.id === s.id)
                return (
                  <Table.Tr
                    key={s.id}
                    style={{ cursor: 'pointer' }}
                    onClick={() => navigate(`/dhcp/subnets/${family}/${s.id}`)}
                  >
                    <Table.Td pl="md" ff="monospace">
                      {s.subnet}
                    </Table.Td>
                    <Table.Td>{s.description || <Text c="dimmed">—</Text>}</Table.Td>
                    <Table.Td>{s.interface || <Text c="dimmed">Any</Text>}</Table.Td>
                    <Table.Td ff="monospace" fz="sm">
                      {s.pools.length > 0 ? s.pools.map((p) => <div key={p}>{p}</div>) : <Text c="dimmed">None</Text>}
                    </Table.Td>
                    <Table.Td>{s.reservations.length}</Table.Td>
                    <Table.Td pr="md">
                      {usage && familyStatus?.state === 'running' ? (
                        <UsageBar assigned={usage.assigned} total={usage.total} />
                      ) : (
                        <Text size="xs" c="dimmed">
                          —
                        </Text>
                      )}
                    </Table.Td>
                  </Table.Tr>
                )
              })}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
    </Card>
  )
}

export function SubnetsPanel() {
  const [adding, setAdding] = useState<DhcpFamily | null>(null)
  return (
    <Stack>
      <FamilySubnets family="v4" onAdd={() => setAdding('v4')} />
      <FamilySubnets family="v6" onAdd={() => setAdding('v6')} />
      {adding && <NewSubnetModal family={adding} onClose={() => setAdding(null)} />}
    </Stack>
  )
}
