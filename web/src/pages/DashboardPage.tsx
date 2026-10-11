import { Badge, Card, Group, SimpleGrid, Skeleton, Stack, Text, ThemeIcon, Title, UnstyledButton } from '@mantine/core'
import { IconClock, IconNetwork, IconServer, IconWorldWww, type Icon } from '@tabler/icons-react'
import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { api } from '../api/client'
import type { SystemInfo } from '../api/types'
import { ServiceStateBadge } from '../components/ServiceStateBadge'
import { formatDuration } from '../utils/format'
import { useNtpStatus } from './ntp/queries'

function ServiceCard({
  name,
  description,
  icon: ServiceIcon,
  to,
  badge,
  footer,
}: {
  name: string
  description: string
  icon: Icon
  to?: string
  badge: ReactNode
  footer?: ReactNode
}) {
  const card = (
    <Card withBorder radius="md" padding="lg" h="100%">
      <Group justify="space-between" mb="sm">
        <Group gap="sm">
          <ThemeIcon variant="light" size="lg">
            <ServiceIcon size={20} />
          </ThemeIcon>
          <Text fw={600}>{name}</Text>
        </Group>
        {badge}
      </Group>
      <Text size="sm" c="dimmed">
        {description}
      </Text>
      {footer}
    </Card>
  )
  return to ? (
    <UnstyledButton component={Link} to={to}>
      {card}
    </UnstyledButton>
  ) : (
    card
  )
}

function NtpCard() {
  const { data: status } = useNtpStatus()
  return (
    <ServiceCard
      name="NTP"
      description="Time synchronization for the local network"
      icon={IconClock}
      to="/ntp"
      badge={status ? <ServiceStateBadge state={status.state} /> : <Skeleton h={20} w={70} />}
      footer={
        status?.state === 'running' && (
          <Text size="sm" mt="sm" c={status.synchronized ? 'green' : 'orange'}>
            {status.synchronized
              ? `Synchronized · stratum ${status.tracking?.stratum} · ${status.sources.length} sources`
              : 'Not synchronized yet'}
          </Text>
        )
      }
    />
  )
}

function ComingSoonBadge() {
  return (
    <Badge variant="light" color="gray">
      Coming soon
    </Badge>
  )
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
        <ServiceCard
          name="DNS"
          description="Authoritative zones, recursion, dynamic DNS and DNS firewall"
          icon={IconWorldWww}
          badge={<ComingSoonBadge />}
        />
        <ServiceCard
          name="DHCP"
          description="DHCPv4 and DHCPv6 subnets, reservations and leases"
          icon={IconNetwork}
          badge={<ComingSoonBadge />}
        />
        <NtpCard />
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
          <InfoRow label="Uptime" value={system && formatDuration(system.uptimeSeconds)} />
        </Stack>
      </Card>
    </Stack>
  )
}
