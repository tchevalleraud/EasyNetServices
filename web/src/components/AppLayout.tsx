import { AppShell, Avatar, Burger, Group, Menu, NavLink, Text, UnstyledButton } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import {
  IconClock,
  IconDashboard,
  IconKey,
  IconLogout,
  IconNetwork,
  IconWorldWww,
  type Icon,
} from '@tabler/icons-react'
import { NavLink as RouterNavLink, Outlet, useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthContext'
import { ColorSchemeToggle } from './ColorSchemeToggle'
import { Logo } from './Logo'

interface NavItem {
  label: string
  icon: Icon
  to?: string
}

// Service pages are listed but disabled until they are implemented.
const navItems: NavItem[] = [
  { label: 'Dashboard', icon: IconDashboard, to: '/' },
  { label: 'DNS', icon: IconWorldWww },
  { label: 'DHCP', icon: IconNetwork },
  { label: 'NTP', icon: IconClock, to: '/ntp' },
]

export function AppLayout() {
  const [opened, { toggle, close }] = useDisclosure()
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = async () => {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <AppShell
      header={{ height: 60 }}
      navbar={{ width: 240, breakpoint: 'sm', collapsed: { mobile: !opened } }}
      padding="lg"
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Group>
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" aria-label="Toggle navigation" />
            <Logo />
          </Group>
          <Group gap="sm">
            <ColorSchemeToggle />
            <Menu position="bottom-end" width={200}>
              <Menu.Target>
                <UnstyledButton aria-label="User menu">
                  <Group gap="xs">
                    <Avatar size="sm" color="blue">
                      {user?.username.charAt(0).toUpperCase()}
                    </Avatar>
                    <Text size="sm" visibleFrom="xs">
                      {user?.username}
                    </Text>
                  </Group>
                </UnstyledButton>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Item leftSection={<IconKey size={16} />} onClick={() => navigate('/change-password')}>
                  Change password
                </Menu.Item>
                <Menu.Divider />
                <Menu.Item color="red" leftSection={<IconLogout size={16} />} onClick={handleLogout}>
                  Log out
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="sm">
        {navItems.map((item) =>
          item.to ? (
            <NavLink
              key={item.label}
              component={RouterNavLink}
              to={item.to}
              end={item.to === '/'}
              label={item.label}
              leftSection={<item.icon size={18} />}
              onClick={close}
            />
          ) : (
            <NavLink
              key={item.label}
              label={item.label}
              description="Coming soon"
              leftSection={<item.icon size={18} />}
              disabled
            />
          ),
        )}
      </AppShell.Navbar>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
