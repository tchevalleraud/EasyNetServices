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
