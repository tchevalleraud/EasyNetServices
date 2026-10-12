import { Alert, Button, Group, Modal, SegmentedControl, Stack, TextInput } from '@mantine/core'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, ApiError } from '../../api/client'
import type { DhcpFamily, DhcpReservation, DhcpSubnet } from '../../api/types'
import { dhcpKeys } from './queries'

interface ReservationModalProps {
  family: DhcpFamily
  subnet: DhcpSubnet
  /** Index of the reservation to edit; undefined to add one. */
  index?: number
  /** Initial values of a new reservation, e.g. taken from a lease. */
  initial?: Partial<DhcpReservation>
  onClose: () => void
  onSaved?: () => void
}

const emptyReservation: DhcpReservation = { hwAddress: '', duid: '', ipAddress: '', hostname: '', description: '' }

export function ReservationModal({ family, subnet, index, initial, onClose, onSaved }: ReservationModalProps) {
  const queryClient = useQueryClient()
  const editing = index !== undefined
  const [values, setValues] = useState<DhcpReservation>(
    editing ? subnet.reservations[index] : { ...emptyReservation, ...initial },
  )
  // DHCPv6 clients are identified by their DUID, or by their MAC address.
  const [identifier, setIdentifier] = useState<'duid' | 'mac'>(family === 'v6' && (values.duid || !values.hwAddress) ? 'duid' : 'mac')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const set = (field: keyof DhcpReservation) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setValues({ ...values, [field]: e.currentTarget.value })

  const handleSave = async () => {
    setError(null)
    setErrors({})
    setSaving(true)
    const reservation = {
      ...values,
      hwAddress: identifier === 'mac' ? values.hwAddress : '',
      duid: identifier === 'duid' ? values.duid : '',
    }
    const reservations = editing
      ? subnet.reservations.map((r, i) => (i === index ? reservation : r))
      : [...subnet.reservations, reservation]
    const position = editing ? index : reservations.length - 1
    try {
      await api.put(`/dhcp/${family}/subnets/${subnet.id}`, { ...subnet, reservations })
      await queryClient.invalidateQueries({ queryKey: dhcpKeys.all })
      onSaved?.()
      onClose()
    } catch (err) {
      if (err instanceof ApiError && err.details.length > 0) {
        const prefix = `reservations[${position}].`
        const fieldErrors: Record<string, string> = {}
        const others: string[] = []
        for (const d of err.details) {
          if (d.field.startsWith(prefix)) fieldErrors[d.field.slice(prefix.length)] = d.message
          else others.push(`${d.field}: ${d.message}`)
        }
        setErrors(fieldErrors)
        if (others.length > 0) setError(others.join('; '))
      } else {
        setError(err instanceof Error ? err.message : 'Unable to save the reservation')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal opened onClose={onClose} title={editing ? 'Edit reservation' : 'New reservation'} centered>
      <Stack>
        {error && <Alert color="red">{error}</Alert>}
        {family === 'v6' && (
          <SegmentedControl
            value={identifier}
            onChange={(v) => setIdentifier(v as 'duid' | 'mac')}
            data={[
              { value: 'duid', label: 'DUID' },
              { value: 'mac', label: 'MAC address' },
            ]}
          />
        )}
        {identifier === 'mac' ? (
          <TextInput
            label="MAC address"
            placeholder="00:11:22:33:44:55"
            value={values.hwAddress}
            onChange={set('hwAddress')}
            error={errors.hwAddress}
            data-autofocus
          />
        ) : (
          <TextInput
            label="DUID"
            description="DHCP unique identifier of the client, shown in its leases"
            placeholder="00:01:00:01:2c:5f:3c:29:aa:bb:cc:dd:ee:ff"
            value={values.duid}
            onChange={set('duid')}
            error={errors.duid}
            data-autofocus
          />
        )}
        <TextInput
          label="IP address"
          description={`Inside ${subnet.subnet}, preferably outside the pools`}
          placeholder={family === 'v4' ? '192.168.1.10' : '2001:db8:1::10'}
          value={values.ipAddress}
          onChange={set('ipAddress')}
          error={errors.ipAddress}
        />
        <TextInput
          label="Host name (optional)"
          description="Sent to the client"
          placeholder="nas"
          value={values.hostname}
          onChange={set('hostname')}
          error={errors.hostname}
        />
        <TextInput
          label="Description (optional)"
          placeholder="Living room NAS"
          value={values.description}
          onChange={set('description')}
          error={errors.description}
        />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSave} loading={saving}>
            {editing ? 'Save' : 'Add reservation'}
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
