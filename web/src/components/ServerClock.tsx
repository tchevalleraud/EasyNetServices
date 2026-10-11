import { Button, Card, Group, Skeleton, Stack, Text } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { IconWorld } from '@tabler/icons-react'
import { formatUtcOffset, useServerTime } from '../hooks/useServerTime'
import { formatSeconds } from '../utils/format'
import { TimezoneModal } from './TimezoneModal'

function format(date: Date, timeZone: string, options: Intl.DateTimeFormatOptions): string {
  return new Intl.DateTimeFormat('en-GB', { timeZone, ...options }).format(date)
}

/** Describes how far the clock of this device is from the server clock. */
function DeviceSkew({ skewMillis, roundTripMillis }: { skewMillis: number; roundTripMillis: number }) {
  // The measurement cannot be more precise than half of the round trip.
  const tolerance = roundTripMillis / 2 + 100
  if (Math.abs(skewMillis) <= tolerance) {
    return (
      <Text size="xs" c="dimmed">
        The clock of this device matches the server.
      </Text>
    )
  }
  const direction = skewMillis > 0 ? 'behind' : 'ahead of'
  return (
    <Text size="xs" c="orange">
      The clock of this device is {formatSeconds(Math.abs(skewMillis) / 1000)} {direction} the server.
    </Text>
  )
}

/** Live clock showing the server time in the configured time zone. */
export function ServerClock() {
  const { data, serverNow, error } = useServerTime()
  const [opened, { open, close }] = useDisclosure()

  if (error) {
    return null
  }

  return (
    <Card withBorder radius="md" padding="lg">
      {!data || !serverNow ? (
        <Skeleton h={88} />
      ) : (
        <Group justify="space-between" align="flex-start" wrap="wrap" gap="md">
          <Stack gap={2}>
            <Text fz={40} fw={700} lh={1.1} ff="monospace" style={{ fontVariantNumeric: 'tabular-nums' }}>
              {format(serverNow, data.timezone, { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })}
            </Text>
            <Text size="sm">
              {format(serverNow, data.timezone, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}
            </Text>
            <Text size="sm" c="dimmed">
              UTC {format(serverNow, 'UTC', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })}
            </Text>
            <DeviceSkew skewMillis={data.skewMillis} roundTripMillis={data.roundTripMillis} />
          </Stack>
          <Stack gap={4} align="flex-end">
            <Text fw={600}>{data.timezone}</Text>
            <Text size="sm" c="dimmed">
              {data.abbreviation} · {formatUtcOffset(data.utcOffsetSeconds)}
            </Text>
            <Button variant="light" size="xs" leftSection={<IconWorld size={14} />} onClick={open} mt={4}>
              Change time zone
            </Button>
          </Stack>
        </Group>
      )}
      {data && opened && <TimezoneModal opened={opened} current={data.timezone} onClose={close} />}
    </Card>
  )
}
