import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { DnsSettings, DnsStatus, DnsZone, DnsZoneDetail } from '../../api/types'

export const dnsKeys = {
  status: ['dns', 'status'],
  settings: ['dns', 'settings'],
  zones: ['dns', 'zones'],
  zone: (name: string) => ['dns', 'zones', name],
}

export function useDnsStatus() {
  return useQuery({ queryKey: dnsKeys.status, queryFn: () => api.get<DnsStatus>('/dns/status'), refetchInterval: 10_000 })
}

export function useDnsSettings() {
  return useQuery({ queryKey: dnsKeys.settings, queryFn: () => api.get<DnsSettings>('/dns/settings') })
}

export function useZones() {
  return useQuery({ queryKey: dnsKeys.zones, queryFn: () => api.get<DnsZone[]>('/dns/zones') })
}

export function useZone(name: string) {
  return useQuery({
    queryKey: dnsKeys.zone(name),
    queryFn: () => api.get<DnsZoneDetail>(`/dns/zones/${encodeURIComponent(name)}`),
  })
}
