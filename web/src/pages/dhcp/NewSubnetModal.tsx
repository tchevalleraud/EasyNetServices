import { Alert, Button, Group, Modal, Select, Stack, TextInput } from '@mantine/core'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { api, ApiError } from '../../api/client'
import type { DhcpFamily, DhcpSubnet } from '../../api/types'
import { suggestSubnet } from './addresses'
import { dhcpKeys, emptyOptions, familyLabel, useDhcpSettings } from './queries'

export function NewSubnetModal({ family, onClose }: { family: DhcpFamily; onClose: () => void }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { data: settings } = useDhcpSettings()
  const interfaces = settings?.interfaces ?? []
  const [subnet, setSubnet] = useState('')
  const [description, setDescription] = useState('')
  // undefined until the operator chooses an interface, or clears it.
  const [chosenInterface, setIface] = useState<string | null | undefined>(undefined)
  const [pool, setPool] = useState('')
  const [gateway, setGateway] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  // Directly connected DHCPv6 clients need the interface of the subnet:
  // default to the first one.
  const defaultInterface = family === 'v6' ? (settings?.interfaces[0] ?? null) : null
  const iface = chosenInterface === undefined ? defaultInterface : chosenInterface

  // Fill the pool and gateway once the network is typed, unless already set.
  const handleSubnetBlur = () => {
    const suggestion = suggestSubnet(subnet)
    if (!suggestion) return
    if (!pool) setPool(suggestion.pool)
    if (!gateway && family === 'v4') setGateway(suggestion.gateway)
  }

  const handleCreate = async () => {
    setError(null)
    setErrors({})
    setSaving(true)
    try {
      const created = await api.post<DhcpSubnet>(`/dhcp/${family}/subnets`, {
        subnet,
        description,
        interface: iface ?? '',
        pools: pool.trim() ? [pool.trim()] : [],
        gateway: family === 'v4' ? gateway.trim() : '',
        validLifetime: 0,
        options: emptyOptions,
        reservations: [],
      })
      await queryClient.invalidateQueries({ queryKey: dhcpKeys.all })
      onClose()
      navigate(`/dhcp/subnets/${family}/${created.id}`)
    } catch (err) {
      if (err instanceof ApiError && err.details.length > 0) {
        const fieldErrors: Record<string, string> = {}
        for (const d of err.details) fieldErrors[d.field.replace(/\[\d+\]$/, '')] = d.message
        setErrors(fieldErrors)
      } else {
        setError(err instanceof Error ? err.message : 'Unable to create the subnet')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal opened onClose={onClose} title={`New ${familyLabel(family)} subnet`} centered>
      <Stack>
        {error && <Alert color="red">{error}</Alert>}
        <TextInput
          label="Network"
          description="Network in CIDR notation"
          placeholder={family === 'v4' ? '192.168.1.0/24' : '2001:db8:1::/64'}
          value={subnet}
          onChange={(e) => setSubnet(e.currentTarget.value)}
          onBlur={handleSubnetBlur}
          error={errors.subnet}
          data-autofocus
        />
        <TextInput
          label="Description"
          placeholder="Home network"
          value={description}
          onChange={(e) => setDescription(e.currentTarget.value)}
          error={errors.description}
        />
        <Select
          label="Interface"
          description={
            family === 'v6'
              ? 'Required for directly connected clients; leave empty only for clients behind a relay'
              : 'Optional: by default the subnet matching the address of the interface is used'
          }
          placeholder="Any"
          data={interfaces}
          value={iface}
          onChange={setIface}
          clearable
          error={errors.interface}
        />
        <TextInput
          label="Address pool"
          description="Range of addresses assigned dynamically"
          placeholder={family === 'v4' ? '192.168.1.100 - 192.168.1.199' : '2001:db8:1::1000 - 2001:db8:1::1fff'}
          value={pool}
          onChange={(e) => setPool(e.currentTarget.value)}
          error={errors.pools}
        />
        {family === 'v4' && (
          <TextInput
            label="Gateway"
            description="Default router of the clients"
            placeholder="192.168.1.1"
            value={gateway}
            onChange={(e) => setGateway(e.currentTarget.value)}
            error={errors.gateway}
          />
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleCreate} loading={saving} disabled={!subnet.trim()}>
            Create subnet
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
