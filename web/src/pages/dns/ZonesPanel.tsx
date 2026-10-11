import { ActionIcon, Alert, Badge, Button, Card, Group, Skeleton, Stack, Table, Text, Tooltip } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { IconChevronRight, IconPlus } from '@tabler/icons-react'
import { Link } from 'react-router'
import { NewZoneModal } from './NewZoneModal'
import { isReverseZone } from './names'
import { useZones } from './queries'

export function ZonesPanel() {
  const { data: zones, isPending, error } = useZones()
  const [opened, { open, close }] = useDisclosure()

  return (
    <Stack>
      <Group justify="space-between">
        <Text size="sm" c="dimmed">
          Zones hosted by this server. Names of these zones are answered locally.
        </Text>
        <Button leftSection={<IconPlus size={16} />} onClick={open}>
          New zone
        </Button>
      </Group>

      {error && <Alert color="red">Unable to load the zones: {error.message}</Alert>}
      {isPending && <Skeleton h={160} />}

      {zones && zones.length === 0 && (
        <Card withBorder radius="md" padding="xl">
          <Stack align="center" gap="xs">
            <Text fw={600}>No zone yet</Text>
            <Text size="sm" c="dimmed" ta="center">
              Create a zone such as home.lan to give names to the devices of your network.
            </Text>
          </Stack>
        </Card>
      )}

      {zones && zones.length > 0 && (
        <Card withBorder radius="md" padding={0}>
          <Table highlightOnHover verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th pl="lg">Zone</Table.Th>
                <Table.Th>Type</Table.Th>
                <Table.Th>Serial</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {zones.map((zone) => (
                <Table.Tr key={zone.name}>
                  <Table.Td pl="lg">
                    <Text component={Link} to={`/dns/zones/${encodeURIComponent(zone.name)}`} ff="monospace" fw={500}>
                      {zone.name}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Badge variant="light" color={isReverseZone(zone.name) ? 'grape' : 'blue'}>
                      {isReverseZone(zone.name) ? 'Reverse' : 'Forward'}
                    </Badge>
                  </Table.Td>
                  <Table.Td ff="monospace">{zone.serial}</Table.Td>
                  <Table.Td ta="right" pr="lg">
                    <Tooltip label="Open the zone" withArrow>
                      <ActionIcon
                        variant="subtle"
                        component={Link}
                        to={`/dns/zones/${encodeURIComponent(zone.name)}`}
                        aria-label={`Open ${zone.name}`}
                      >
                        <IconChevronRight size={18} />
                      </ActionIcon>
                    </Tooltip>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Card>
      )}

      {opened && <NewZoneModal opened={opened} onClose={close} />}
    </Stack>
  )
}
