function parseIPv4(s: string): number | null {
  const parts = s.split('.')
  if (parts.length !== 4) return null
  let n = 0
  for (const p of parts) {
    if (!/^\d{1,3}$/.test(p) || Number(p) > 255) return null
    n = n * 256 + Number(p)
  }
  return n
}

function formatIPv4(n: number): string {
  return [24, 16, 8, 0].map((shift) => Math.floor(n / 2 ** shift) % 256).join('.')
}

export interface SubnetSuggestion {
  pool: string
  gateway: string
}

/**
 * Suggests a pool and a gateway for a new subnet: the gateway is the first
 * address and the pool the second half of an IPv4 network; IPv6 networks
 * get a pool of 4096 addresses.
 */
export function suggestSubnet(cidr: string): SubnetSuggestion | null {
  const [address, bitsText] = cidr.trim().split('/')
  const bits = Number(bitsText)
  if (!address || !bitsText || !Number.isInteger(bits)) return null
  if (address.includes(':')) {
    if (bits > 112 || !address.endsWith('::')) return null
    return { pool: `${address}1000 - ${address}1fff`, gateway: '' }
  }
  const ip = parseIPv4(address)
  if (ip === null || bits < 8 || bits > 29) return null
  const size = 2 ** (32 - bits)
  const network = ip - (ip % size)
  return {
    gateway: formatIPv4(network + 1),
    pool: `${formatIPv4(network + size / 2)} - ${formatIPv4(network + size - 2)}`,
  }
}
