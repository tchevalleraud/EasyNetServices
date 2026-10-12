import { Stack, Tabs, Text, Title } from '@mantine/core'
import { IconActivity, IconAddressBook, IconNetwork, IconSettings } from '@tabler/icons-react'
import { useSearchParams } from 'react-router'
import { DhcpSettingsForm } from './DhcpSettingsForm'
import { DhcpStatusPanel } from './DhcpStatusPanel'
import { LeasesPanel } from './LeasesPanel'
import { SubnetsPanel } from './SubnetsPanel'

const tabs = ['status', 'subnets', 'leases', 'settings']

export function DhcpPage() {
  const [params, setParams] = useSearchParams()
  const tab = tabs.includes(params.get('tab') ?? '') ? params.get('tab')! : 'status'

  return (
    <Stack gap="lg">
      <div>
        <Title order={2}>DHCP</Title>
        <Text c="dimmed" size="sm">
          Address assignment for the clients of the network
        </Text>
      </div>
      <Tabs value={tab} onChange={(value) => setParams(value === 'status' ? {} : { tab: value ?? 'status' }, { replace: true })}>
        <Tabs.List mb="md">
          <Tabs.Tab value="status" leftSection={<IconActivity size={16} />}>
            Status
          </Tabs.Tab>
          <Tabs.Tab value="subnets" leftSection={<IconNetwork size={16} />}>
            Subnets
          </Tabs.Tab>
          <Tabs.Tab value="leases" leftSection={<IconAddressBook size={16} />}>
            Leases
          </Tabs.Tab>
          <Tabs.Tab value="settings" leftSection={<IconSettings size={16} />}>
            Settings
          </Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="status">
          <DhcpStatusPanel />
        </Tabs.Panel>
        <Tabs.Panel value="subnets">
          <SubnetsPanel />
        </Tabs.Panel>
        <Tabs.Panel value="leases">
          <LeasesPanel />
        </Tabs.Panel>
        <Tabs.Panel value="settings">
          <DhcpSettingsForm />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  )
}
