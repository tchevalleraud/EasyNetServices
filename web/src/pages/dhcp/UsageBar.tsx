import { Group, Progress, Stack, Text } from '@mantine/core'
import { formatCount, usagePercent } from './queries'

export function UsageBar({ assigned, total }: { assigned: number; total: number }) {
  const percent = usagePercent(assigned, total)
  const color = percent === null ? 'gray' : percent >= 90 ? 'red' : percent >= 75 ? 'orange' : 'teal'
  return (
    <Stack gap={4} miw={140}>
      <Progress value={percent ?? 0} color={color} size="sm" />
      <Group justify="space-between" gap="xs">
        <Text size="xs" c="dimmed">
          {formatCount(assigned)} / {total > 0 ? formatCount(total) : '—'}
        </Text>
        <Text size="xs" c="dimmed">
          {percent === null ? '' : percent < 1 && percent > 0 ? '< 1 %' : `${percent.toFixed(0)} %`}
        </Text>
      </Group>
    </Stack>
  )
}
