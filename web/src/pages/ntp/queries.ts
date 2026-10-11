import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { NtpConfig, NtpStatus } from '../../api/types'

export const ntpConfigKey = ['ntp', 'config']
export const ntpStatusKey = ['ntp', 'status']

export function useNtpStatus() {
  return useQuery({
    queryKey: ntpStatusKey,
    queryFn: () => api.get<NtpStatus>('/ntp/status'),
    refetchInterval: 10_000,
  })
}

export function useNtpConfig() {
  return useQuery({ queryKey: ntpConfigKey, queryFn: () => api.get<NtpConfig>('/ntp/config') })
}
