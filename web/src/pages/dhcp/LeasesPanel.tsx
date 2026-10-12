import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Group,
  Modal,
  SegmentedControl,
  Select,
  Skeleton,
  Stack,
  Table,
  Text,
  TextInput,
  Tooltip,
} from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconBookmark, IconInfoCircle, IconSearch, IconTrash } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { api, ApiError } from '../../api/client'
import type { DhcpFamily, DhcpLease } from '../../api/types'
import { formatDuration } from '../../utils/format'
import { dhcpKeys, familyLabel, useDhcpStatus, useLeases, useSubnets } from './queries'
import { ReservationModal } from './ReservationModal'

const stateColors: Record<DhcpLease['state'], string> = {
  active: 'green',
  declined: 'red',
  expired: 'gray',
  released: 'gray',
  registered: 'blue',
  unknown: 'yellow',
}

function expiresIn(expires: string): string {
  const seconds = Math.round((new Date(expires).getTime() - Date.now()) / 1000)
  return seconds > 0 ? `in ${formatDuration(seconds)}` : 'expired'
}

export function LeasesPanel() {
  const [family, setFamily] = useState<DhcpFamily>('v4')
  const [subnetFilter, setSubnetFilter] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [reserving, setReserving] = useState<DhcpLease | null>(null)
  const [releasing, setReleasing] = useState<DhcpLease | null>(null)
  const [busy, setBusy] = useState(false)
  const queryClient = useQueryClient()

  const { data: status } = useDhcpStatus()
  const running = (family === 'v4' ? status?.dhcpv4.state : status?.dhcpv6.state) === 'running'
  const { data: subnets } = useSubnets(family)
  const { data: leases, isPending, error } = useLeases(family, running)

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase()
    return (leases ?? []).filter(
      (l) =>
        (!subnetFilter || String(l.subnetId) === subnetFilter) &&
        (!term || [l.address, l.hwAddress, l.duid, l.hostname].some((v) => v.toLowerCase().includes(term))),
    )
  }, [leases, subnetFilter, search])

  const subnetName = (id: number) => subnets?.find((s) => s.id === id)?.subnet ?? `#${id}`
  const reservingSubnet = reserving ? subnets?.find((s) => s.id === reserving.subnetId) : undefined

  const release = async () => {
    if (!releasing) return
    setBusy(true)
    try {
      await api.delete(`/dhcp/${family}/leases/${encodeURIComponent(releasing.address)}`)
      await queryClient.invalidateQueries({ queryKey: dhcpKeys.leases(family) })
      setReleasing(null)
    } catch (err) {
      notifications.show({ color: 'red', message: err instanceof Error ? err.message : 'Unable to delete the lease' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Stack>
      <Group gap="sm">
        <SegmentedControl
          value={family}
          onChange={(v) => {
            setFamily(v as DhcpFamily)
            setSubnetFilter(null)
          }}
          data={[
            { value: 'v4', label: 'DHCPv4' },
            { value: 'v6', label: 'DHCPv6' },
          ]}
        />
        <TextInput
          placeholder="Search addresses, MAC, DUID and host names"
          leftSection={<IconSearch size={16} />}
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
          style={{ flex: 1, minWidth: 200 }}
        />
        <Select
          placeholder="All subnets"
          data={(subnets ?? []).map((s) => ({ value: String(s.id), label: s.subnet }))}
          value={subnetFilter}
          onChange={setSubnetFilter}
          clearable
          w={220}
        />
      </Group>

      {status && !running && (
        <Alert color="blue" icon={<IconInfoCircle size={18} />}>
          The {familyLabel(family)} server is not running: enable it in the Settings tab to see its leases.
        </Alert>
      )}
      {running && error && !(error instanceof ApiError && error.code === 'dhcp_unavailable') && (
        <Alert color="red">Unable to load the leases: {error.message}</Alert>
      )}
      {running && isPending && <Skeleton h={160} />}

      {running && leases && (
        <Card withBorder radius="md" padding={0}>
          <Table.ScrollContainer minWidth={760}>
            <Table highlightOnHover verticalSpacing="xs">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th pl="md">Address</Table.Th>
                  <Table.Th>{family === 'v4' ? 'MAC address' : 'DUID'}</Table.Th>
                  <Table.Th>Host name</Table.Th>
                  <Table.Th>Subnet</Table.Th>
                  <Table.Th>Expires</Table.Th>
                  <Table.Th>State</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {filtered.map((l) => (
                  <Table.Tr key={l.address}>
                    <Table.Td pl="md" ff="monospace">
                      {l.address}
                    </Table.Td>
                    <Table.Td ff="monospace" fz="sm" style={{ wordBreak: 'break-all' }}>
                      {family === 'v4' ? l.hwAddress : l.duid}
                    </Table.Td>
                    <Table.Td>{l.hostname || <Text c="dimmed">—</Text>}</Table.Td>
                    <Table.Td ff="monospace" fz="sm">
                      {subnetName(l.subnetId)}
                    </Table.Td>
                    <Table.Td fz="sm">{expiresIn(l.expires)}</Table.Td>
                    <Table.Td>
                      <Group gap={4}>
                        <Badge variant="light" color={stateColors[l.state]}>
                          {l.state}
                        </Badge>
                        {l.reserved && (
                          <Badge variant="outline" color="grape">
                            reserved
                          </Badge>
                        )}
                      </Group>
                    </Table.Td>
                    <Table.Td ta="right" pr="md" style={{ whiteSpace: 'nowrap' }}>
                      {!l.reserved && (
                        <Tooltip label="Reserve this address for the client" withArrow>
                          <ActionIcon variant="subtle" aria-label="Reserve" onClick={() => setReserving(l)}>
                            <IconBookmark size={18} />
                          </ActionIcon>
                        </Tooltip>
                      )}
                      <Tooltip label="Delete the lease" withArrow>
                        <ActionIcon variant="subtle" color="red" aria-label="Delete lease" onClick={() => setReleasing(l)}>
                          <IconTrash size={18} />
                        </ActionIcon>
                      </Tooltip>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
          {filtered.length === 0 && (
            <Text size="sm" c="dimmed" ta="center" p="md">
              {leases.length === 0 ? 'No lease yet.' : 'No matching lease.'}
            </Text>
          )}
        </Card>
      )}

      {reserving && reservingSubnet && (
        <ReservationModal
          family={family}
          subnet={reservingSubnet}
          initial={{
            ipAddress: reserving.address,
            hostname: reserving.hostname,
            hwAddress: family === 'v4' ? reserving.hwAddress : '',
            duid: family === 'v6' ? reserving.duid : '',
          }}
          onClose={() => setReserving(null)}
          onSaved={() => notifications.show({ color: 'green', message: `${reserving.address} reserved` })}
        />
      )}

      <Modal opened={releasing !== null} onClose={() => setReleasing(null)} title="Delete lease" centered>
        <Stack>
          <Text size="sm">
            Delete the lease of <Text span ff="monospace">{releasing?.address}</Text>? The address can then be assigned to
            another client, while the current one may keep using it until it renews.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setReleasing(null)}>
              Cancel
            </Button>
            <Button color="red" onClick={release} loading={busy}>
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  )
}
