import { Badge, Card, Group, SimpleGrid, Skeleton, Stack, Text, ThemeIcon, Title } from '@mantine/core'
import { IconClock, IconNetwork, IconServer, IconWorldWww, type Icon } from '@tabler/icons-react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import type { SystemInfo } from '../api/types'

interface ServiceCard {
  name: string
  description: string
  icon: Icon
}

const services: ServiceCard[] = [
  { name: 'DNS', description: 'Authoritative zones, recursion, dynamic DNS and DNS firewall', icon: IconWorldWww },
  { name: 'DHCP', description: 'DHCPv4 and DHCPv6 subnets, reservations and leases', icon: IconNetwork },
  { name: 'NTP', description: 'Time synchronization for the local network', icon: IconClock },
]

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

function InfoRow({ label, value }: { label: string; value?: string }) {
  return (
    <Group justify="space-between">
      <Text size="sm" c="dimmed">
        {label}
      </Text>
      {value === undefined ? <Skeleton h={14} w={100} /> : <Text size="sm">{value}</Text>}
    </Group>
  )
}

export function DashboardPage() {
  const { data: system } = useQuery({
    queryKey: ['system'],
    queryFn: () => api.get<SystemInfo>('/system'),
    refetchInterval: 60_000,
  })

  return (
    <Stack gap="lg">
      <Title order={2}>Dashboard</Title>

      <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }}>
        {services.map((service) => (
          <Card key={service.name} withBorder radius="md" padding="lg">
            <Group justify="space-between" mb="sm">
              <Group gap="sm">
                <ThemeIcon variant="light" size="lg">
                  <service.icon size={20} />
                </ThemeIcon>
                <Text fw={600}>{service.name}</Text>
              </Group>
              <Badge variant="light" color="gray">
                Not available yet
              </Badge>
            </Group>
            <Text size="sm" c="dimmed">
              {service.description}
            </Text>
          </Card>
        ))}
      </SimpleGrid>

      <Card withBorder radius="md" padding="lg" maw={480}>
        <Group gap="sm" mb="md">
          <ThemeIcon variant="light" size="lg" color="gray">
            <IconServer size={20} />
          </ThemeIcon>
          <Text fw={600}>System</Text>
        </Group>
        <Stack gap="xs">
          <InfoRow label="Version" value={system?.version} />
          <InfoRow label="Role" value={system?.role} />
          <InfoRow label="Hostname" value={system?.hostname} />
          <InfoRow label="Uptime" value={system && formatUptime(system.uptimeSeconds)} />
        </Stack>
      </Card>
    </Stack>
  )
}
