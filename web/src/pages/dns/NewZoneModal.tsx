import { Alert, Button, Group, Modal, SegmentedControl, Stack, Text, TextInput } from '@mantine/core'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { api, ApiError } from '../../api/client'
import type { DnsZoneDetail } from '../../api/types'
import { dnsKeys } from './queries'
import { reverseZone } from './names'

export function NewZoneModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [kind, setKind] = useState<'forward' | 'reverse'>('forward')
  const [name, setName] = useState('')
  const [network, setNetwork] = useState('')
  const [nameserver, setNameserver] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const reverse = network ? reverseZone(network) : null
  const zoneName = kind === 'forward' ? name.trim() : (reverse ?? '')

  const handleCreate = async () => {
    setError(null)
    setSaving(true)
    try {
      const zone = await api.post<DnsZoneDetail>('/dns/zones', {
        name: zoneName,
        nameservers: nameserver.trim() ? [nameserver.trim()] : [],
      })
      await queryClient.invalidateQueries({ queryKey: dnsKeys.zones })
      onClose()
      navigate(`/dns/zones/${encodeURIComponent(zone.name)}`)
    } catch (err) {
      if (err instanceof ApiError && err.details.length > 0) {
        setError(err.details.map((d) => d.message).join(', '))
      } else {
        setError(err instanceof Error ? err.message : 'Unable to create the zone')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal opened={opened} onClose={onClose} title="New zone" centered>
      <Stack>
        <SegmentedControl
          value={kind}
          onChange={(v) => setKind(v as 'forward' | 'reverse')}
          data={[
            { value: 'forward', label: 'Forward zone' },
            { value: 'reverse', label: 'Reverse zone' },
          ]}
        />
        {error && <Alert color="red">{error}</Alert>}
        {kind === 'forward' ? (
          <TextInput
            label="Zone name"
            description="Domain served by this server, e.g. home.lan or corp.example.com"
            placeholder="home.lan"
            value={name}
            onChange={(e) => setName(e.currentTarget.value)}
            data-autofocus
          />
        ) : (
          <TextInput
            label="Network"
            description="IPv4 prefix of /8, /16 or /24, or IPv6 prefix multiple of /4"
            placeholder="192.168.1.0/24"
            value={network}
            onChange={(e) => setNetwork(e.currentTarget.value)}
            error={network && !reverse ? 'Unsupported network or prefix length' : undefined}
            data-autofocus
          />
        )}
        {kind === 'reverse' && reverse && (
          <Text size="sm">
            Zone: <Text span ff="monospace">{reverse}</Text>
          </Text>
        )}
        <TextInput
          label="Name server (optional)"
          description="Creates the NS record of the zone"
          placeholder={zoneName ? `ns.${kind === 'forward' ? zoneName : 'home.lan'}` : 'ns.home.lan'}
          value={nameserver}
          onChange={(e) => setNameserver(e.currentTarget.value)}
        />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleCreate} loading={saving} disabled={!zoneName}>
            Create zone
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
