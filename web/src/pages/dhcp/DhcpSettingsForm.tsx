import { Alert, Button, Card, Group, MultiSelect, Skeleton, Stack, Switch, Text } from '@mantine/core'
import { useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import { IconAlertCircle, IconInfoCircle } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type ReactNode } from 'react'
import { api, ApiError } from '../../api/client'
import type { DhcpFamily, DhcpSettings } from '../../api/types'
import { toFormErrors } from '../../utils/formErrors'
import { LifetimeInput, OptionsFields } from './OptionsFields'
import { dhcpKeys, emptyOptions, useDhcpSettings, useInterfaces } from './queries'

const emptySettings: DhcpSettings = {
  interfaces: [],
  dhcpv4: { enabled: false, validLifetime: 86400, preferredLifetime: 0, options: emptyOptions },
  dhcpv6: { enabled: false, validLifetime: 86400, preferredLifetime: 43200, options: emptyOptions },
}

// Lists of strings edited with a single input.
const stringLists = ['interfaces', 'dhcpv[46]\\.options\\.(dnsServers|domainSearch|ntpServers)']

function Section({ title, description, children }: { title: string; description: ReactNode; children: ReactNode }) {
  return (
    <Card withBorder radius="md" padding="lg">
      <Text fw={600}>{title}</Text>
      <Text size="sm" c="dimmed" mb="md">
        {description}
      </Text>
      {children}
    </Card>
  )
}

export function DhcpSettingsForm() {
  const { data, isPending, error } = useDhcpSettings()
  const { data: interfaces } = useInterfaces()
  const queryClient = useQueryClient()
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const form = useForm<DhcpSettings>({ initialValues: emptySettings })

  useEffect(() => {
    if (data) {
      form.setInitialValues(data)
      form.setValues(data)
      form.resetDirty(data)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  if (error) return <Alert color="red">Unable to load the DHCP settings: {error.message}</Alert>
  if (isPending) return <Skeleton h={300} />

  const handleSubmit = form.onSubmit(async (values) => {
    setSaveError(null)
    setSaving(true)
    try {
      const saved = await api.put<DhcpSettings>('/dhcp/settings', values)
      queryClient.setQueryData(dhcpKeys.settings, saved)
      await queryClient.invalidateQueries({ queryKey: dhcpKeys.status })
      notifications.show({ color: 'green', message: 'DHCP settings applied' })
    } catch (err) {
      if (err instanceof ApiError && err.details.length > 0) {
        form.setErrors(toFormErrors(err.details, stringLists))
        setSaveError('Please fix the highlighted fields.')
      } else {
        setSaveError(err instanceof Error ? err.message : 'Unable to save the settings')
      }
    } finally {
      setSaving(false)
    }
  })

  // Interfaces of the host, plus configured ones that disappeared.
  const interfaceOptions = (interfaces ?? []).map((i) => ({
    value: i.name,
    label: `${i.name}${i.addresses.length > 0 ? ` — ${i.addresses.join(', ')}` : ''}${i.up ? '' : ' (down)'}`,
  }))
  for (const name of form.values.interfaces) {
    if (!interfaceOptions.some((o) => o.value === name)) interfaceOptions.push({ value: name, label: `${name} (not found)` })
  }

  const familySection = (family: DhcpFamily) => {
    const key = family === 'v4' ? 'dhcpv4' : 'dhcpv6'
    return (
      <Section
        title={family === 'v4' ? 'DHCPv4' : 'DHCPv6'}
        description={
          family === 'v4'
            ? 'Assigns IPv4 addresses. Default settings of every DHCPv4 subnet.'
            : 'Assigns IPv6 addresses. Default settings of every DHCPv6 subnet.'
        }
      >
        <Stack>
          <Switch
            label={`Enable the ${family === 'v4' ? 'DHCPv4' : 'DHCPv6'} server`}
            description={family === 'v4' ? 'Listens on UDP port 67' : 'Listens on UDP port 547'}
            {...form.getInputProps(`${key}.enabled`, { type: 'checkbox' })}
          />
          {family === 'v6' && form.values.dhcpv6.enabled && (
            <Alert color="blue" icon={<IconInfoCircle size={18} />}>
              DHCPv6 clients learn their default router from the router advertisements of the network, which must have
              the managed (M) flag set for the clients to request an address.
            </Alert>
          )}
          <Group grow align="flex-start">
            <LifetimeInput label="Lease time" {...form.getInputProps(`${key}.validLifetime`)} />
            {family === 'v6' && (
              <LifetimeInput
                label="Preferred lifetime"
                description="Shorter than the lease time"
                {...form.getInputProps('dhcpv6.preferredLifetime')}
              />
            )}
          </Group>
          <OptionsFields family={family} inputProps={(field) => form.getInputProps(`${key}.options.${field}`)} />
        </Stack>
      </Section>
    )
  }

  return (
    <form onSubmit={handleSubmit}>
      <Stack>
        {saveError && (
          <Alert color="red" icon={<IconAlertCircle size={18} />}>
            {saveError}
          </Alert>
        )}

        <Section
          title="Interfaces"
          description="Network interfaces the DHCP servers listen on. Clients of other networks can be served through a DHCP relay."
        >
          <MultiSelect
            placeholder="Choose the interfaces"
            data={interfaceOptions}
            searchable
            {...form.getInputProps('interfaces')}
          />
        </Section>

        {familySection('v4')}
        {familySection('v6')}

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
  )
}
