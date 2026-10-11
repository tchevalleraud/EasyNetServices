import { Stack, Tabs, Text, Title } from '@mantine/core'
import { IconActivity, IconSettings } from '@tabler/icons-react'
import { useSearchParams } from 'react-router'
import { NtpConfigForm } from './NtpConfigForm'
import { NtpStatusPanel } from './NtpStatusPanel'

export function NtpPage() {
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'config' ? 'config' : 'status'

  return (
    <Stack gap="lg">
      <div>
        <Title order={2}>NTP</Title>
        <Text c="dimmed" size="sm">
          Time server for the local network
        </Text>
      </div>
      <Tabs value={tab} onChange={(value) => setParams(value === 'config' ? { tab: 'config' } : {}, { replace: true })}>
        <Tabs.List mb="md">
          <Tabs.Tab value="status" leftSection={<IconActivity size={16} />}>
            Status
          </Tabs.Tab>
          <Tabs.Tab value="config" leftSection={<IconSettings size={16} />}>
            Configuration
          </Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="status">
          <NtpStatusPanel />
        </Tabs.Panel>
        <Tabs.Panel value="config">
          <NtpConfigForm />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  )
}
