import {
  ActionIcon,
  Alert,
  Button,
  Card,
  Checkbox,
  Group,
  SegmentedControl,
  Select,
  Skeleton,
  Stack,
  Switch,
  TagsInput,
  Text,
  TextInput,
} from '@mantine/core'
import { useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import { IconAlertCircle, IconPlus, IconTrash } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type ReactNode } from 'react'
import { api, ApiError } from '../../api/client'
import type { DnsForwarder, DnsSettings } from '../../api/types'
import { toFormErrors } from '../../utils/formErrors'
import { dnsKeys, useDnsSettings } from './queries'

const emptySettings: DnsSettings = {
  enabled: false,
  listenAddresses: [],
  allowFrom: [],
  resolution: 'recursive',
  forwarders: [],
  forwardZones: [],
  dnssec: 'process',
}

const presets: { label: string; forwarders: DnsForwarder[] }[] = [
  { label: 'Cloudflare', forwarders: [{ address: '1.1.1.1', tls: true }, { address: '1.0.0.1', tls: true }] },
  { label: 'Quad9', forwarders: [{ address: '9.9.9.9', tls: true }, { address: '149.112.112.112', tls: true }] },
  { label: 'Google', forwarders: [{ address: '8.8.8.8', tls: true }, { address: '8.8.4.4', tls: true }] },
]

// Lists of strings edited with a single TagsInput.
const stringLists = ['listenAddresses', 'allowFrom', 'forwardZones\\.\\d+\\.servers']

function Section({ title, description, children }: { title: string; description: string; children: ReactNode }) {
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

export function ResolverForm() {
  const { data, isPending, error } = useDnsSettings()
  const queryClient = useQueryClient()
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const form = useForm<DnsSettings>({ initialValues: emptySettings })

  useEffect(() => {
    if (data) {
      form.setInitialValues(data)
      form.setValues(data)
      form.resetDirty(data)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  if (error) return <Alert color="red">Unable to load the resolver settings: {error.message}</Alert>
  if (isPending) return <Skeleton h={300} />

  const handleSubmit = form.onSubmit(async (values) => {
    setSaveError(null)
    setSaving(true)
    try {
      const saved = await api.put<DnsSettings>('/dns/settings', values)
      queryClient.setQueryData(dnsKeys.settings, saved)
      await queryClient.invalidateQueries({ queryKey: dnsKeys.status })
      notifications.show({ color: 'green', message: 'Resolver settings applied' })
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

  const { forwarders, forwardZones, resolution } = form.values

  return (
    <form onSubmit={handleSubmit}>
      <Stack>
        {saveError && (
          <Alert color="red" icon={<IconAlertCircle size={18} />}>
            {saveError}
          </Alert>
        )}

        <Card withBorder radius="md" padding="lg">
          <Switch
            label="Enable the resolver"
            description="Answer DNS queries from the clients on port 53"
            {...form.getInputProps('enabled', { type: 'checkbox' })}
          />
        </Card>

        <Section
          title="Listening"
          description="Addresses the resolver listens on, and client networks allowed to query it."
        >
          <Stack>
            <TagsInput
              label="Listen addresses"
              description="Leave empty to listen on all addresses. Set the address of the LAN interface when port 53 is already used on the host, e.g. by systemd-resolved."
              placeholder="All addresses"
              {...form.getInputProps('listenAddresses')}
            />
            <TagsInput
              label="Allowed clients"
              description="Networks or addresses allowed to query the resolver; others are ignored."
              placeholder="192.168.1.0/24"
              {...form.getInputProps('allowFrom')}
            />
          </Stack>
        </Section>

        <Section title="Resolution" description="How names outside the local and forward zones are resolved.">
          <Stack>
            <SegmentedControl
              value={resolution}
              onChange={(v) => form.setFieldValue('resolution', v as DnsSettings['resolution'])}
              data={[
                { value: 'recursive', label: 'Recursive (root servers)' },
                { value: 'forward', label: 'Forward to resolvers' },
              ]}
              style={{ alignSelf: 'flex-start' }}
            />
            {resolution === 'forward' && (
              <Stack gap="sm">
                {form.errors.forwarders && (
                  <Text size="sm" c="red">
                    {form.errors.forwarders}
                  </Text>
                )}
                {forwarders.map((_, index) => (
                  <Group key={index} gap="sm" align="flex-start">
                    <TextInput
                      style={{ flex: 1, minWidth: 180 }}
                      aria-label="Resolver address"
                      placeholder="1.1.1.1"
                      {...form.getInputProps(`forwarders.${index}.address`)}
                    />
                    <Checkbox
                      mt={8}
                      label="DNS over TLS"
                      {...form.getInputProps(`forwarders.${index}.tls`, { type: 'checkbox' })}
                    />
                    <ActionIcon
                      variant="subtle"
                      color="red"
                      mt={4}
                      aria-label="Remove resolver"
                      onClick={() => form.removeListItem('forwarders', index)}
                    >
                      <IconTrash size={18} />
                    </ActionIcon>
                  </Group>
                ))}
                <Group gap="xs">
                  <Button
                    variant="light"
                    leftSection={<IconPlus size={16} />}
                    onClick={() => form.insertListItem('forwarders', { address: '', tls: true })}
                  >
                    Add resolver
                  </Button>
                  {presets.map((p) => (
                    <Button key={p.label} variant="subtle" onClick={() => form.setFieldValue('forwarders', p.forwarders)}>
                      Use {p.label}
                    </Button>
                  ))}
                </Group>
              </Stack>
            )}
            <Select
              label="DNSSEC validation"
              maw={420}
              data={[
                { value: 'process', label: 'When requested by the client (recommended)' },
                { value: 'validate', label: 'Always, refuse invalid answers' },
                { value: 'off', label: 'Disabled' },
              ]}
              allowDeselect={false}
              {...form.getInputProps('dnssec')}
            />
          </Stack>
        </Section>

        <Section
          title="Forward zones"
          description="Send the queries for specific domains to other servers, e.g. the domain controllers of an Active Directory domain. DNSSEC validation is disabled for these domains."
        >
          <Stack gap="sm">
            {forwardZones.length === 0 && (
              <Text size="sm" c="dimmed">
                No forward zone.
              </Text>
            )}
            {forwardZones.map((_, index) => (
              <Card key={index} withBorder radius="md" padding="sm">
                <Group align="flex-start" gap="sm">
                  <TextInput
                    style={{ flex: 1, minWidth: 180 }}
                    label="Domain"
                    placeholder="corp.example.com"
                    {...form.getInputProps(`forwardZones.${index}.name`)}
                  />
                  <TagsInput
                    style={{ flex: 2, minWidth: 220 }}
                    label="Servers"
                    placeholder="10.0.0.10 or 10.0.0.10:5353"
                    {...form.getInputProps(`forwardZones.${index}.servers`)}
                  />
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    mt={28}
                    aria-label="Remove forward zone"
                    onClick={() => form.removeListItem('forwardZones', index)}
                  >
                    <IconTrash size={18} />
                  </ActionIcon>
                </Group>
                <Checkbox
                  mt="sm"
                  label="The servers are resolvers (recursive), not authoritative servers for this domain"
                  {...form.getInputProps(`forwardZones.${index}.recursive`, { type: 'checkbox' })}
                />
              </Card>
            ))}
            <Group>
              <Button
                variant="light"
                leftSection={<IconPlus size={16} />}
                onClick={() => form.insertListItem('forwardZones', { name: '', servers: [], recursive: true })}
              >
                Add forward zone
              </Button>
            </Group>
          </Stack>
        </Section>

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
