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

export type DnsResolution = 'recursive' | 'forward'
export type DnssecMode = 'off' | 'process' | 'validate'

export interface DnsForwarder {
  address: string
  tls: boolean
}

export interface DnsForwardZone {
  name: string
  servers: string[]
  recursive: boolean
}

export interface DnsSettings {
  enabled: boolean
  listenAddresses: string[]
  allowFrom: string[]
  resolution: DnsResolution
  forwarders: DnsForwarder[]
  forwardZones: DnsForwardZone[]
  dnssec: DnssecMode
}

export interface DnsStats {
  questions: number
  cacheHits: number
  cacheMisses: number
  cacheEntries: number
  noErrorAnswers: number
  nxdomainAnswers: number
  servfailAnswers: number
  uptimeSeconds: number
}

export interface DnsStatus {
  authoritative: ServiceState
  recursor: ServiceState
  stats: DnsStats | null
}

export interface DnsZone {
  name: string
  kind: string
  serial: number
}

export interface DnsRecord {
  content: string
  disabled: boolean
}

export interface DnsRRSet {
  name: string
  type: string
  ttl: number
  records: DnsRecord[]
}

export interface DnsZoneDetail extends DnsZone {
  rrsets: DnsRRSet[]
}
