export interface User {
  username: string
  source: string
  mustChangePassword: boolean
}

export interface SystemInfo {
  version: string
  role: string
  hostname: string
  uptimeSeconds: number
}

export type NtpSourceType = 'server' | 'pool'

export interface NtpSource {
  type: NtpSourceType
  address: string
  iburst: boolean
  prefer: boolean
  nts: boolean
}

export type NtpAccessAction = 'allow' | 'deny'

export interface NtpAccessRule {
  action: NtpAccessAction
  network: string
}

export interface NtpConfig {
  enabled: boolean
  sources: NtpSource[]
  access: NtpAccessRule[]
  localStratum: number
}

export type ServiceState = 'running' | 'stopped' | 'disabled' | 'unavailable'

export interface NtpTracking {
  referenceId: string
  reference: string
  stratum: number
  offsetSeconds: number
  rootDelaySeconds: number
  rootDispersionSeconds: number
  updateIntervalSeconds: number
  leapStatus: string
}

export interface NtpSourceStatus {
  mode: string
  state: string
  address: string
  stratum: number
  pollSeconds: number
  reach: string
  lastRxSeconds: number
  offsetSeconds: number
  errorSeconds: number
}

export interface NtpStatus {
  state: ServiceState
  synchronized: boolean
  tracking: NtpTracking | null
  sources: NtpSourceStatus[]
}

export interface SystemSettings {
  timezone: string
}

export interface SystemTime {
  unixMillis: number
  timezone: string
  abbreviation: string
  utcOffsetSeconds: number
  formattedLocal: string
  formattedUtc: string
}
