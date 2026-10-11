import {
  ActionIcon,
  Alert,
  Button,
  Card,
  Checkbox,
  Group,
  NumberInput,
  Select,
  Skeleton,
  Stack,
  Switch,
  Text,
  TextInput,
  Tooltip,
} from '@mantine/core'
import { useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import { IconAlertCircle, IconPlus, IconTrash } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type ReactNode } from 'react'
import { api, ApiError } from '../../api/client'
import type { NtpConfig } from '../../api/types'
import { toFormPath } from '../../utils/format'
import { ntpConfigKey, ntpStatusKey, useNtpConfig } from './queries'

const emptyConfig: NtpConfig = { enabled: true, sources: [], access: [], localStratum: 0 }

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

export function NtpConfigForm() {
  const { data, isPending, error } = useNtpConfig()
  const queryClient = useQueryClient()
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  const form = useForm<NtpConfig>({ initialValues: emptyConfig })

  // Load the stored configuration into the form once it is fetched.
  useEffect(() => {
    if (data) {
      form.setInitialValues(data)
      form.setValues(data)
      form.resetDirty(data)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  if (error) {
    return <Alert color="red">Unable to load the NTP configuration: {error.message}</Alert>
  }
  if (isPending) {
    return <Skeleton h={300} />
  }

  const handleSubmit = form.onSubmit(async (values) => {
    setSaveError(null)
    setSaving(true)
    try {
      const saved = await api.put<NtpConfig>('/ntp/config', values)
      queryClient.setQueryData(ntpConfigKey, saved)
      await queryClient.invalidateQueries({ queryKey: ntpStatusKey })
      notifications.show({ color: 'green', message: 'NTP configuration applied' })
    } catch (err) {
      if (err instanceof ApiError && err.details.length > 0) {
        form.setErrors(Object.fromEntries(err.details.map((d) => [toFormPath(d.field), d.message])))
        setSaveError('Please fix the highlighted fields.')
      } else {
        setSaveError(err instanceof Error ? err.message : 'Unable to save the configuration')
      }
    } finally {
      setSaving(false)
    }
  })

  const { sources, access } = form.values

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
            label="Enable the NTP server"
            description="Serve time to the clients of the network on UDP port 123"
            {...form.getInputProps('enabled', { type: 'checkbox' })}
          />
        </Card>

        <Section title="Upstream sources" description="Servers the local clock is synchronized with.">
          <Stack gap="sm">
            {form.errors.sources && (
              <Text size="sm" c="red">
                {form.errors.sources}
              </Text>
            )}
            {sources.map((_, index) => (
              <Group key={index} align="flex-start" gap="sm" wrap="wrap">
                <Select
                  w={110}
                  aria-label="Type"
                  data={[
                    { value: 'pool', label: 'Pool' },
                    { value: 'server', label: 'Server' },
                  ]}
                  allowDeselect={false}
                  {...form.getInputProps(`sources.${index}.type`)}
                />
                <TextInput
                  style={{ flex: 1, minWidth: 200 }}
                  aria-label="Address"
                  placeholder="pool.ntp.org"
                  {...form.getInputProps(`sources.${index}.address`)}
                />
                <Group gap="md" mt={8}>
                  <Tooltip label="Send a burst of requests at startup for a faster synchronization" withArrow>
                    <Checkbox label="iburst" {...form.getInputProps(`sources.${index}.iburst`, { type: 'checkbox' })} />
                  </Tooltip>
                  <Tooltip label="Prefer this source over the others" withArrow>
                    <Checkbox label="Prefer" {...form.getInputProps(`sources.${index}.prefer`, { type: 'checkbox' })} />
                  </Tooltip>
                  <Tooltip label="Authenticate the source with Network Time Security (the server must support it)" withArrow>
                    <Checkbox label="NTS" {...form.getInputProps(`sources.${index}.nts`, { type: 'checkbox' })} />
                  </Tooltip>
                </Group>
                <ActionIcon
                  variant="subtle"
                  color="red"
                  mt={4}
                  aria-label="Remove source"
                  onClick={() => form.removeListItem('sources', index)}
                >
                  <IconTrash size={18} />
                </ActionIcon>
              </Group>
            ))}
            <Group>
              <Button
                variant="light"
                leftSection={<IconPlus size={16} />}
                onClick={() =>
                  form.insertListItem('sources', { type: 'server', address: '', iburst: true, prefer: false, nts: false })
                }
              >
                Add source
              </Button>
            </Group>
          </Stack>
        </Section>

        <Section
          title="Access control"
          description="Client networks allowed to query the server. Clients matching no rule are denied; the most specific rule wins."
        >
          <Stack gap="sm">
            {access.length === 0 && (
              <Text size="sm" c="dimmed">
                No rule: the server does not answer any client.
              </Text>
            )}
            {access.map((_, index) => (
              <Group key={index} align="flex-start" gap="sm">
                <Select
                  w={110}
                  aria-label="Action"
                  data={[
                    { value: 'allow', label: 'Allow' },
                    { value: 'deny', label: 'Deny' },
                  ]}
                  allowDeselect={false}
                  {...form.getInputProps(`access.${index}.action`)}
                />
                <TextInput
                  style={{ flex: 1, minWidth: 180 }}
                  aria-label="Network"
                  placeholder="192.168.1.0/24"
                  {...form.getInputProps(`access.${index}.network`)}
                />
                <ActionIcon
                  variant="subtle"
                  color="red"
                  mt={4}
                  aria-label="Remove rule"
                  onClick={() => form.removeListItem('access', index)}
                >
                  <IconTrash size={18} />
                </ActionIcon>
              </Group>
            ))}
            <Group>
              <Button
                variant="light"
                leftSection={<IconPlus size={16} />}
                onClick={() => form.insertListItem('access', { action: 'allow', network: '' })}
              >
                Add rule
              </Button>
            </Group>
          </Stack>
        </Section>

        <Section
          title="Fallback"
          description="Keep serving time from the local clock when no upstream source is reachable, e.g. on an isolated network."
        >
          <NumberInput
            maw={260}
            label="Local stratum"
            description="1 to 15, or 0 to disable. A high value such as 10 is recommended."
            min={0}
            max={15}
            allowDecimal={false}
            {...form.getInputProps('localStratum')}
          />
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
