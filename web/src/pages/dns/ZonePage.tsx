import {
  ActionIcon,
  Alert,
  Anchor,
  Badge,
  Breadcrumbs,
  Button,
  Card,
  Group,
  Modal,
  Select,
  Skeleton,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconEdit, IconPlus, IconSearch, IconTrash } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { api } from '../../api/client'
import type { DnsRRSet } from '../../api/types'
import { formatDuration } from '../../utils/format'
import { editableTypes, isReverseZone, toRelative } from './names'
import { dnsKeys, useZone } from './queries'
import { RecordModal } from './RecordModal'

const typeOrder = ['SOA', 'NS', ...editableTypes.filter((t) => t !== 'NS')]

function sortRRSets(rrsets: DnsRRSet[], zone: string): DnsRRSet[] {
  return [...rrsets].sort((a, b) => {
    // Apex records first, then by name and type.
    const apex = Number(b.name === zone) - Number(a.name === zone)
    if (apex !== 0) return apex
    return a.name.localeCompare(b.name) || typeOrder.indexOf(a.type) - typeOrder.indexOf(b.type)
  })
}

export function ZonePage() {
  const zoneName = useParams().zone ?? ''
  const { data: zone, isPending, error } = useZone(zoneName)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string | null>(null)
  const [editing, setEditing] = useState<DnsRRSet | null | undefined>(undefined)
  const [deleting, setDeleting] = useState<DnsRRSet | null>(null)
  const [deletingZone, setDeletingZone] = useState(false)
  const [busy, setBusy] = useState(false)

  const rrsets = useMemo(() => {
    if (!zone) return []
    const term = search.trim().toLowerCase()
    return sortRRSets(zone.rrsets, zone.name).filter(
      (rr) =>
        (!typeFilter || rr.type === typeFilter) &&
        (!term || rr.name.includes(term) || rr.records.some((r) => r.content.toLowerCase().includes(term))),
    )
  }, [zone, search, typeFilter])

  const deleteRRSet = async () => {
    if (!deleting) return
    setBusy(true)
    try {
      await api.delete(
        `/dns/zones/${encodeURIComponent(zoneName)}/rrsets/${encodeURIComponent(deleting.name)}/${deleting.type}`,
      )
      await queryClient.invalidateQueries({ queryKey: dnsKeys.zone(zoneName) })
      setDeleting(null)
    } catch (err) {
      notifications.show({ color: 'red', message: err instanceof Error ? err.message : 'Unable to delete the record' })
    } finally {
      setBusy(false)
    }
  }

  const deleteZone = async () => {
    setBusy(true)
    try {
      await api.delete(`/dns/zones/${encodeURIComponent(zoneName)}`)
      await queryClient.invalidateQueries({ queryKey: dnsKeys.zones })
      notifications.show({ color: 'green', message: `Zone ${zoneName} deleted` })
      navigate('/dns?tab=zones', { replace: true })
    } catch (err) {
      notifications.show({ color: 'red', message: err instanceof Error ? err.message : 'Unable to delete the zone' })
      setBusy(false)
    }
  }

  return (
    <Stack gap="lg">
      <Breadcrumbs>
        <Anchor component={Link} to="/dns?tab=zones" size="sm">
          DNS zones
        </Anchor>
        <Text size="sm">{zoneName}</Text>
      </Breadcrumbs>

      <Group justify="space-between" align="flex-start">
        <div>
          <Group gap="sm">
            <Title order={2} ff="monospace">
              {zoneName}
            </Title>
            <Badge variant="light" color={isReverseZone(zoneName) ? 'grape' : 'blue'}>
              {isReverseZone(zoneName) ? 'Reverse' : 'Forward'}
            </Badge>
          </Group>
          {zone && (
            <Text size="sm" c="dimmed">
              Serial {zone.serial} · {zone.rrsets.length} record sets
            </Text>
          )}
        </div>
        <Group>
          <Button variant="default" color="red" leftSection={<IconTrash size={16} />} onClick={() => setDeletingZone(true)}>
            Delete zone
          </Button>
          <Button leftSection={<IconPlus size={16} />} onClick={() => setEditing(null)} disabled={!zone}>
            Add record
          </Button>
        </Group>
      </Group>

      {error && <Alert color="red">Unable to load the zone: {error.message}</Alert>}
      {isPending && <Skeleton h={240} />}

      {zone && (
        <Card withBorder radius="md" padding={0}>
          <Group p="md" gap="sm">
            <TextInput
              placeholder="Search names and values"
              leftSection={<IconSearch size={16} />}
              value={search}
              onChange={(e) => setSearch(e.currentTarget.value)}
              style={{ flex: 1, minWidth: 200 }}
            />
            <Select
              placeholder="All types"
              data={typeOrder}
              value={typeFilter}
              onChange={setTypeFilter}
              clearable
              w={140}
            />
          </Group>
          <Table.ScrollContainer minWidth={640}>
            <Table highlightOnHover verticalSpacing="xs">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th pl="md">Name</Table.Th>
                  <Table.Th>Type</Table.Th>
                  <Table.Th>TTL</Table.Th>
                  <Table.Th>Value</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {rrsets.map((rr) => {
                  const editable = (editableTypes as readonly string[]).includes(rr.type)
                  return (
                    <Table.Tr key={`${rr.name}/${rr.type}`}>
                      <Table.Td pl="md" ff="monospace">
                        {toRelative(rr.name, zone.name)}
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="outline" radius="sm">
                          {rr.type}
                        </Badge>
                      </Table.Td>
                      <Table.Td>{formatDuration(rr.ttl)}</Table.Td>
                      <Table.Td ff="monospace" style={{ wordBreak: 'break-all' }}>
                        {rr.records.map((r) => (
                          <div key={r.content}>{r.content}</div>
                        ))}
                      </Table.Td>
                      <Table.Td ta="right" pr="md" style={{ whiteSpace: 'nowrap' }}>
                        {editable ? (
                          <>
                            <Tooltip label="Edit" withArrow>
                              <ActionIcon variant="subtle" aria-label="Edit record" onClick={() => setEditing(rr)}>
                                <IconEdit size={18} />
                              </ActionIcon>
                            </Tooltip>
                            <Tooltip label="Delete" withArrow>
                              <ActionIcon variant="subtle" color="red" aria-label="Delete record" onClick={() => setDeleting(rr)}>
                                <IconTrash size={18} />
                              </ActionIcon>
                            </Tooltip>
                          </>
                        ) : (
                          <Text size="xs" c="dimmed">
                            Managed automatically
                          </Text>
                        )}
                      </Table.Td>
                    </Table.Tr>
                  )
                })}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
          {rrsets.length === 0 && (
            <Text size="sm" c="dimmed" ta="center" p="md">
              No matching record.
            </Text>
          )}
        </Card>
      )}

      {zone && editing !== undefined && (
        <RecordModal zone={zone.name} rrset={editing ?? undefined} existing={zone.rrsets} onClose={() => setEditing(undefined)} />
      )}

      <Modal opened={deleting !== null} onClose={() => setDeleting(null)} title="Delete record" centered>
        <Stack>
          <Text size="sm">
            Delete the {deleting?.type} record of <Text span ff="monospace">{deleting?.name}</Text>?
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleting(null)}>
              Cancel
            </Button>
            <Button color="red" onClick={deleteRRSet} loading={busy}>
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>

      <Modal opened={deletingZone} onClose={() => setDeletingZone(false)} title="Delete zone" centered>
        <Stack>
          <Text size="sm">
            Delete the zone <Text span ff="monospace">{zoneName}</Text> and all its records? This cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeletingZone(false)}>
              Cancel
            </Button>
            <Button color="red" onClick={deleteZone} loading={busy}>
              Delete zone
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  )
}
