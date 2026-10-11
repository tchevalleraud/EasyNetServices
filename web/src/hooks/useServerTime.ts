import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { SystemTime } from '../api/types'

export const systemTimeKey = ['system', 'time']

export interface ServerTime extends SystemTime {
  /** Server clock minus device clock, in milliseconds. */
  skewMillis: number
  /** Duration of the request, which bounds the precision of skewMillis. */
  roundTripMillis: number
}

async function fetchServerTime(): Promise<ServerTime> {
  const start = Date.now()
  const time = await api.get<SystemTime>('/system/time')
  const end = Date.now()
  // Assume the server read its clock halfway through the request.
  return { ...time, skewMillis: time.unixMillis - (start + end) / 2, roundTripMillis: end - start }
}

/**
 * Returns the server time, refreshed from the API every minute and advanced
 * locally in between, so a clock can tick without polling the server.
 */
export function useServerTime() {
  const query = useQuery({ queryKey: systemTimeKey, queryFn: fetchServerTime, refetchInterval: 60_000 })
  const [deviceNow, setDeviceNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = setInterval(() => setDeviceNow(Date.now()), 250)
    return () => clearInterval(timer)
  }, [])

  const serverNow = query.data ? new Date(deviceNow + query.data.skewMillis) : undefined
  return { ...query, serverNow }
}

/** Formats a UTC offset in seconds as "UTC+02:00". */
export function formatUtcOffset(seconds: number): string {
  const sign = seconds < 0 ? '-' : '+'
  const abs = Math.abs(seconds)
  const hours = String(Math.floor(abs / 3600)).padStart(2, '0')
  const minutes = String(Math.floor((abs % 3600) / 60)).padStart(2, '0')
  return `UTC${sign}${hours}:${minutes}`
}
