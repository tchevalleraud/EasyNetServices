import { Center, Paper, Stack, Text } from '@mantine/core'
import type { ReactNode } from 'react'
import { ColorSchemeToggle } from './ColorSchemeToggle'
import { Logo } from './Logo'

/** Centered card used by the login and password change pages. */
export function AuthCard({ subtitle, children }: { subtitle: string; children: ReactNode }) {
  return (
    <Center mih="100vh" p="md" pos="relative">
      <div style={{ position: 'absolute', top: 16, right: 16 }}>
        <ColorSchemeToggle />
      </div>
      <Stack w="100%" maw={400} gap="lg">
        <Stack align="center" gap={4}>
          <Logo size="xl" />
          <Text c="dimmed" size="sm" ta="center">
            {subtitle}
          </Text>
        </Stack>
        <Paper withBorder shadow="sm" p="xl" radius="md">
          {children}
        </Paper>
      </Stack>
    </Center>
  )
}
