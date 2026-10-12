import { Stack, Tabs, Text, Title } from '@mantine/core'
import { IconActivity, IconList, IconSettings } from '@tabler/icons-react'
import { useSearchParams } from 'react-router'
import { DnsStatusPanel } from './DnsStatusPanel'
import { ResolverForm } from './ResolverForm'
import { ZonesPanel } from './ZonesPanel'

const tabs = ['status', 'zones', 'resolver']

export function DnsPage() {
  const [params, setParams] = useSearchParams()
  const tab = tabs.includes(params.get('tab') ?? '') ? params.get('tab')! : 'status'

  return (
    <Stack gap="lg">
      <div>
        <Title order={2}>DNS</Title>
        <Text c="dimmed" size="sm">
          Local zones and name resolution for the network
        </Text>
      </div>
      <Tabs value={tab} onChange={(value) => setParams(value === 'status' ? {} : { tab: value ?? 'status' }, { replace: true })}>
        <Tabs.List mb="md">
          <Tabs.Tab value="status" leftSection={<IconActivity size={16} />}>
            Status
          </Tabs.Tab>
          <Tabs.Tab value="zones" leftSection={<IconList size={16} />}>
            Zones
          </Tabs.Tab>
          <Tabs.Tab value="resolver" leftSection={<IconSettings size={16} />}>
            Resolver
          </Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="status">
          <DnsStatusPanel />
        </Tabs.Panel>
        <Tabs.Panel value="zones">
          <ZonesPanel />
        </Tabs.Panel>
        <Tabs.Panel value="resolver">
          <ResolverForm />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  )
}
