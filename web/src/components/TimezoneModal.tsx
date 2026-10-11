import { Alert, Button, Group, Modal, Select, Stack, Text } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { api } from '../api/client'
import type { SystemSettings } from '../api/types'
import { systemTimeKey } from '../hooks/useServerTime'

function timezoneList(current: string): string[] {
  const zones = new Set(Intl.supportedValuesOf('timeZone'))
  zones.add('UTC')
  zones.add(current)
  return [...zones].sort()
}

interface TimezoneModalProps {
  opened: boolean
  current: string
  onClose: () => void
}

export function TimezoneModal({ opened, current, onClose }: TimezoneModalProps) {
  const queryClient = useQueryClient()
  const [value, setValue] = useState<string | null>(current)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const zones = useMemo(() => timezoneList(current), [current])
  const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone

  const handleSave = async () => {
    if (!value) return
    setError(null)
    setSaving(true)
    try {
      await api.put<SystemSettings>('/system/settings', { timezone: value })
      await queryClient.invalidateQueries({ queryKey: systemTimeKey })
      notifications.show({ color: 'green', message: `Time zone set to ${value}` })
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to save the time zone')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal opened={opened} onClose={onClose} title="Time zone" centered>
      <Stack>
        <Text size="sm" c="dimmed">
          Used to display dates and in the logs. NTP clients always receive UTC time and apply their own time zone.
        </Text>
        {error && <Alert color="red">{error}</Alert>}
        <Select
          label="Time zone"
          data={zones}
          value={value}
          onChange={setValue}
          searchable
          allowDeselect={false}
          limit={100}
          nothingFoundMessage="No matching time zone"
          comboboxProps={{ withinPortal: true }}
        />
        {browserZone && browserZone !== value && (
          <Button variant="subtle" size="compact-sm" onClick={() => setValue(browserZone)} style={{ alignSelf: 'flex-start' }}>
            Use the time zone of this device ({browserZone})
          </Button>
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSave} loading={saving} disabled={!value || value === current}>
            Save
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
