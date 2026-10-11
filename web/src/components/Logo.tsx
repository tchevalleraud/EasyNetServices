import { Group, Text } from '@mantine/core'
import { IconTopologyStar3 } from '@tabler/icons-react'

export function Logo({ size = 'lg' }: { size?: 'lg' | 'xl' }) {
  return (
    <Group gap="xs" wrap="nowrap">
      <IconTopologyStar3 size={size === 'xl' ? 34 : 26} color="var(--mantine-primary-color-filled)" />
      <Text fw={700} size={size}>
        EasyNetServices
      </Text>
    </Group>
  )
}
