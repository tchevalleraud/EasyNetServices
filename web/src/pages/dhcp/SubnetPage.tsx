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
  SimpleGrid,
  Skeleton,
  Stack,
  Table,
  TagsInput,
  Text,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core'
import { useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import { IconAlertCircle, IconEdit, IconPlus, IconTrash } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { api, ApiError } from '../../api/client'
import type { DhcpFamily, DhcpSubnet } from '../../api/types'
import { toFormErrors } from '../../utils/formErrors'
import { LifetimeInput, OptionsFields } from './OptionsFields'
import { dhcpKeys, emptyOptions, familyLabel, useDhcpSettings, useSubnet } from './queries'
import { ReservationModal } from './ReservationModal'

const emptySubnet: DhcpSubnet = {
  id: 0,
  subnet: '',
  description: '',
  interface: '',
  pools: [],
  gateway: '',
  validLifetime: 0,
  options: emptyOptions,
  reservations: [],
}

const stringLists = ['pools', 'options\\.(dnsServers|domainSearch|ntpServers)']

export function SubnetPage() {
  const params = useParams()
  const family: DhcpFamily = params.family === 'v6' ? 'v6' : 'v4'
  const id = Number(params.id)
  const { data: subnet, isPending, error } = useSubnet(family, id)
  const { data: settings } = useDhcpSettings()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const form = useForm<DhcpSubnet>({ initialValues: emptySubnet })
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [editing, setEditing] = useState<number | null | undefined>(undefined)
  const [deleting, setDeleting] = useState<number | null>(null)
  const [deletingSubnet, setDeletingSubnet] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (subnet) {
      form.setInitialValues(subnet)
      form.setValues(subnet)
      form.resetDirty(subnet)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subnet])

  const handleSubmit = form.onSubmit(async (values) => {
    if (!subnet) return
    setSaveError(null)
    setSaving(true)
    try {
      // Reservations are saved on their own.
      const saved = await api.put<DhcpSubnet>(`/dhcp/${family}/subnets/${id}`, {
        ...values,
        reservations: subnet.reservations,
      })
      queryClient.setQueryData(dhcpKeys.subnet(family, id), saved)
      await queryClient.invalidateQueries({ queryKey: dhcpKeys.subnets(family) })
      notifications.show({ color: 'green', message: 'Subnet saved and applied' })
    } catch (err) {
      if (err instanceof ApiError && err.details.length > 0) {
        form.setErrors(toFormErrors(err.details, stringLists))
        setSaveError('Please fix the highlighted fields.')
      } else {
        setSaveError(err instanceof Error ? err.message : 'Unable to save the subnet')
      }
    } finally {
      setSaving(false)
    }
  })

  const deleteReservation = async () => {
    if (!subnet || deleting === null) return
    setBusy(true)
    try {
      await api.put(`/dhcp/${family}/subnets/${id}`, {
        ...subnet,
        reservations: subnet.reservations.filter((_, i) => i !== deleting),
      })
      await queryClient.invalidateQueries({ queryKey: dhcpKeys.all })
      setDeleting(null)
    } catch (err) {
      notifications.show({ color: 'red', message: err instanceof Error ? err.message : 'Unable to delete the reservation' })
    } finally {
      setBusy(false)
    }
  }

  const deleteSubnet = async () => {
    setBusy(true)
    try {
      await api.delete(`/dhcp/${family}/subnets/${id}`)
      await queryClient.invalidateQueries({ queryKey: dhcpKeys.all })
      notifications.show({ color: 'green', message: `Subnet ${subnet?.subnet ?? ''} deleted` })
      navigate('/dhcp?tab=subnets', { replace: true })
    } catch (err) {
      notifications.show({ color: 'red', message: err instanceof Error ? err.message : 'Unable to delete the subnet' })
      setBusy(false)
    }
  }

  return (
    <Stack gap="lg">
      <Breadcrumbs>
        <Anchor component={Link} to="/dhcp?tab=subnets" size="sm">
          DHCP subnets
        </Anchor>
        <Text size="sm">{subnet?.subnet ?? id}</Text>
      </Breadcrumbs>

      <Group justify="space-between" align="flex-start">
        <div>
          <Group gap="sm">
            <Title order={2} ff="monospace">
              {subnet?.subnet ?? '…'}
            </Title>
            <Badge variant="light" color={family === 'v4' ? 'blue' : 'grape'}>
              {familyLabel(family)}
            </Badge>
          </Group>
          {subnet && (
            <Text size="sm" c="dimmed">
              {subnet.description || 'No description'} · ID {subnet.id}
            </Text>
          )}
        </div>
        <Button variant="default" color="red" leftSection={<IconTrash size={16} />} onClick={() => setDeletingSubnet(true)} disabled={!subnet}>
          Delete subnet
        </Button>
      </Group>

      {error && <Alert color="red">Unable to load the subnet: {error.message}</Alert>}
      {isPending && <Skeleton h={300} />}

      {subnet && (
        <form onSubmit={handleSubmit}>
          <Stack>
            {saveError && (
              <Alert color="red" icon={<IconAlertCircle size={18} />}>
                {saveError}
              </Alert>
            )}
            <Card withBorder radius="md" padding="lg">
              <Text fw={600} mb="md">
                Addressing
              </Text>
              <Stack>
                <SimpleGrid cols={{ base: 1, sm: 2 }}>
                  <TextInput label="Description" placeholder="Home network" {...form.getInputProps('description')} />
                  <Select
                    label="Interface"
                    description={family === 'v6' ? 'Required for directly connected clients' : 'Optional'}
                    placeholder="Any"
                    data={settings?.interfaces ?? []}
                    clearable
                    {...form.getInputProps('interface')}
                    value={form.values.interface || null}
                    onChange={(v) => form.setFieldValue('interface', v ?? '')}
                  />
                </SimpleGrid>
                <TagsInput
                  label="Address pools"
                  description='Ranges "first - last" or networks in CIDR notation, assigned dynamically'
                  placeholder={family === 'v4' ? '192.168.1.100 - 192.168.1.199' : '2001:db8:1::1000 - 2001:db8:1::1fff'}
                  {...form.getInputProps('pools')}
                />
                <SimpleGrid cols={{ base: 1, sm: 2 }}>
                  {family === 'v4' && (
                    <TextInput label="Gateway" description="Default router of the clients" placeholder="192.168.1.1" {...form.getInputProps('gateway')} />
                  )}
                  <LifetimeInput
                    label="Lease time"
                    inherit
                    {...form.getInputProps('validLifetime')}
                  />
                </SimpleGrid>
              </Stack>
            </Card>
            <Card withBorder radius="md" padding="lg">
              <Text fw={600}>Options</Text>
              <Text size="sm" c="dimmed" mb="md">
                Override the options of the DHCP settings for this subnet; leave empty to inherit them.
              </Text>
              <OptionsFields family={family} inherit inputProps={(field) => form.getInputProps(`options.${field}`)} />
            </Card>
            <Group justify="flex-end">
              <Button variant="default" disabled={!form.isDirty() || saving} onClick={() => form.reset()}>
                Reset
              </Button>
              <Button type="submit" loading={saving} disabled={!form.isDirty()}>
                Save and apply
              </Button>
            </Group>
          </Stack>
        </form>
      )}

      {subnet && (
        <Card withBorder radius="md" padding={0}>
          <Group justify="space-between" p="md">
            <div>
              <Text fw={600}>Reservations</Text>
              <Text size="sm" c="dimmed">
                Fixed addresses assigned to known clients
              </Text>
            </div>
            <Button variant="light" size="xs" leftSection={<IconPlus size={14} />} onClick={() => setEditing(null)}>
              Add reservation
            </Button>
          </Group>
          {subnet.reservations.length === 0 ? (
            <Text size="sm" c="dimmed" px="md" pb="md">
              No reservation. Reserve the address of a client from the Leases tab, or add one here.
            </Text>
          ) : (
            <Table.ScrollContainer minWidth={640}>
              <Table highlightOnHover verticalSpacing="xs">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th pl="md">Address</Table.Th>
                    <Table.Th>Client</Table.Th>
                    <Table.Th>Host name</Table.Th>
                    <Table.Th>Description</Table.Th>
                    <Table.Th />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {subnet.reservations.map((r, i) => (
                    <Table.Tr key={`${r.ipAddress}-${i}`}>
                      <Table.Td pl="md" ff="monospace">
                        {r.ipAddress}
                      </Table.Td>
                      <Table.Td ff="monospace" fz="sm" style={{ wordBreak: 'break-all' }}>
                        {r.duid ? `DUID ${r.duid}` : r.hwAddress}
                      </Table.Td>
                      <Table.Td>{r.hostname || <Text c="dimmed">—</Text>}</Table.Td>
                      <Table.Td>{r.description || <Text c="dimmed">—</Text>}</Table.Td>
                      <Table.Td ta="right" pr="md" style={{ whiteSpace: 'nowrap' }}>
                        <Tooltip label="Edit" withArrow>
                          <ActionIcon variant="subtle" aria-label="Edit reservation" onClick={() => setEditing(i)}>
                            <IconEdit size={18} />
                          </ActionIcon>
                        </Tooltip>
                        <Tooltip label="Delete" withArrow>
                          <ActionIcon variant="subtle" color="red" aria-label="Delete reservation" onClick={() => setDeleting(i)}>
                            <IconTrash size={18} />
                          </ActionIcon>
                        </Tooltip>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          )}
        </Card>
      )}

      {subnet && editing !== undefined && (
        <ReservationModal family={family} subnet={subnet} index={editing ?? undefined} onClose={() => setEditing(undefined)} />
      )}

      <Modal opened={deleting !== null} onClose={() => setDeleting(null)} title="Delete reservation" centered>
        <Stack>
          <Text size="sm">
            Delete the reservation of{' '}
            <Text span ff="monospace">
              {deleting !== null ? subnet?.reservations[deleting]?.ipAddress : ''}
            </Text>
            ? The client keeps its address until its lease expires.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleting(null)}>
              Cancel
            </Button>
            <Button color="red" onClick={deleteReservation} loading={busy}>
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>

      <Modal opened={deletingSubnet} onClose={() => setDeletingSubnet(false)} title="Delete subnet" centered>
        <Stack>
          <Text size="sm">
            Delete the subnet <Text span ff="monospace">{subnet?.subnet}</Text> and its reservations? Its clients will
            not be able to renew their leases.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeletingSubnet(false)}>
              Cancel
            </Button>
            <Button color="red" onClick={deleteSubnet} loading={busy}>
              Delete subnet
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  )
}
