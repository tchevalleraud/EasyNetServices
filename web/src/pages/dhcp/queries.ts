import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { DhcpFamily, DhcpLease, DhcpSettings, DhcpStatus, DhcpSubnet, NetInterface } from '../../api/types'

export const dhcpKeys = {
  all: ['dhcp'],
  status: ['dhcp', 'status'],
  settings: ['dhcp', 'settings'],
  interfaces: ['dhcp', 'interfaces'],
  subnets: (family: DhcpFamily) => ['dhcp', family, 'subnets'],
  subnet: (family: DhcpFamily, id: number) => ['dhcp', family, 'subnets', id],
  leases: (family: DhcpFamily) => ['dhcp', family, 'leases'],
}

export function useDhcpStatus() {
  return useQuery({ queryKey: dhcpKeys.status, queryFn: () => api.get<DhcpStatus>('/dhcp/status'), refetchInterval: 10_000 })
}

export function useDhcpSettings() {
  return useQuery({ queryKey: dhcpKeys.settings, queryFn: () => api.get<DhcpSettings>('/dhcp/settings') })
}

export function useInterfaces() {
  return useQuery({ queryKey: dhcpKeys.interfaces, queryFn: () => api.get<NetInterface[]>('/dhcp/interfaces') })
}

export function useSubnets(family: DhcpFamily) {
  return useQuery({ queryKey: dhcpKeys.subnets(family), queryFn: () => api.get<DhcpSubnet[]>(`/dhcp/${family}/subnets`) })
}

export function useSubnet(family: DhcpFamily, id: number) {
  return useQuery({
    queryKey: dhcpKeys.subnet(family, id),
    queryFn: () => api.get<DhcpSubnet>(`/dhcp/${family}/subnets/${id}`),
  })
}

export function useLeases(family: DhcpFamily, enabled: boolean) {
  return useQuery({
    queryKey: dhcpKeys.leases(family),
    queryFn: () => api.get<DhcpLease[]>(`/dhcp/${family}/leases`),
    enabled,
    refetchInterval: 15_000,
    retry: false,
  })
}

export const familyLabel = (family: DhcpFamily) => (family === 'v4' ? 'DHCPv4' : 'DHCPv6')

export const emptyOptions = { dnsServers: [], domainName: '', domainSearch: [], ntpServers: [] }

/** Percentage of a pool in use, or null when the pool size is unknown. */
export function usagePercent(assigned: number, total: number): number | null {
  return total > 0 ? Math.min(100, (assigned / total) * 100) : null
}

const number = new Intl.NumberFormat('en-US')

/** Formats a pool size, which can be huge in IPv6. */
export function formatCount(n: number): string {
  return n >= 1e9 ? n.toExponential(1) : number.format(n)
}
