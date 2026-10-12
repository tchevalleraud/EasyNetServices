/** Record types that can be edited, mirroring the API. */
export const editableTypes = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'SRV', 'PTR', 'NS', 'CAA'] as const

export const typeHints: Record<string, { placeholder: string; help: string }> = {
  A: { placeholder: '192.168.1.10', help: 'IPv4 address' },
  AAAA: { placeholder: '2001:db8::10', help: 'IPv6 address' },
  CNAME: {
    placeholder: 'nas',
    help: 'Target host; a name without a dot is relative to the zone. A CNAME cannot coexist with other records',
  },
  MX: { placeholder: '10 mail', help: 'Priority and mail server; a name without a dot is relative to the zone' },
  TXT: { placeholder: 'v=spf1 -all', help: 'Free text; quotes are added automatically' },
  SRV: { placeholder: '0 5 5060 sip', help: 'Priority, weight, port and target' },
  PTR: { placeholder: 'nas.home.lan', help: 'Fully qualified host name of the address' },
  NS: { placeholder: 'ns1', help: 'Name server of the zone; a name without a dot is relative to the zone' },
  CAA: { placeholder: '0 issue "letsencrypt.org"', help: 'Flags, tag and value' },
}

export function isReverseZone(zone: string): boolean {
  return zone.endsWith('.in-addr.arpa') || zone.endsWith('.ip6.arpa')
}

/** Returns the name relative to the zone, "@" for the apex. */
export function toRelative(name: string, zone: string): string {
  if (name === zone) return '@'
  return name.endsWith(`.${zone}`) ? name.slice(0, -(zone.length + 1)) : name
}

/** Returns the absolute name of a name relative to the zone ("@" or empty for the apex). */
export function toAbsolute(relative: string, zone: string): string {
  const name = relative.trim().toLowerCase()
  if (name === '' || name === '@') return zone
  if (name.endsWith('.')) return name.slice(0, -1)
  return name === zone || name.endsWith(`.${zone}`) ? name : `${name}.${zone}`
}

/**
 * Returns the reverse zone of a network, e.g. 192.168.1.0/24 → 1.168.192.in-addr.arpa.
 * IPv4 prefixes must be a multiple of 8 and IPv6 prefixes a multiple of 4.
 */
export function reverseZone(cidr: string): string | null {
  const [address, lengthText] = cidr.trim().split('/')
  const length = Number(lengthText)
  if (!address || !Number.isInteger(length)) return null

  if (address.includes('.')) {
    const octets = address.split('.').map(Number)
    if (octets.length !== 4 || octets.some((o) => !Number.isInteger(o) || o < 0 || o > 255)) return null
    if (length < 8 || length > 24 || length % 8 !== 0) return null
    return `${octets.slice(0, length / 8).reverse().join('.')}.in-addr.arpa`
  }

  const nibbles = expandIPv6(address)
  if (!nibbles || length < 4 || length > 124 || length % 4 !== 0) return null
  return `${nibbles.slice(0, length / 4).split('').reverse().join('.')}.ip6.arpa`
}

/** Expands an IPv6 address to its 32 hexadecimal digits. */
function expandIPv6(address: string): string | null {
  const halves = address.toLowerCase().split('::')
  if (halves.length > 2) return null
  const head = halves[0] ? halves[0].split(':') : []
  const tail = halves.length === 2 && halves[1] ? halves[1].split(':') : []
  const missing = 8 - head.length - tail.length
  if (missing < 0 || (halves.length === 1 && missing !== 0)) return null
  const groups = [...head, ...Array(missing).fill('0'), ...tail]
  if (groups.some((g) => !/^[0-9a-f]{1,4}$/.test(g))) return null
  return groups.map((g) => g.padStart(4, '0')).join('')
}
