import { ActionIcon, Alert, Button, Group, Modal, NumberInput, Select, Stack, Text, TextInput } from '@mantine/core'
import { IconPlus, IconTrash } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, ApiError } from '../../api/client'
import type { DnsRRSet } from '../../api/types'
import { editableTypes, toAbsolute, toRelative, typeHints } from './names'
import { dnsKeys } from './queries'

interface RecordModalProps {
  zone: string
  /** RRSet to edit; undefined to create a new one. */
  rrset?: DnsRRSet
  existing: DnsRRSet[]
  onClose: () => void
}

export function RecordModal({ zone, rrset, existing, onClose }: RecordModalProps) {
  const queryClient = useQueryClient()
  const editing = rrset !== undefined
  const [name, setName] = useState(rrset ? toRelative(rrset.name, zone) : '')
  const [type, setType] = useState<string>(rrset?.type ?? 'A')
  const [ttl, setTtl] = useState<number>(rrset?.ttl ?? 3600)
  const [values, setValues] = useState<string[]>(rrset ? rrset.records.map((r) => r.content) : [''])
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const absolute = toAbsolute(name, zone)
  const conflict = !editing && existing.find((r) => r.name === absolute && r.type === type)
  const hint = typeHints[type]

  const setValue = (index: number, value: string) => setValues(values.map((v, i) => (i === index ? value : v)))

  const handleSave = async () => {
    setError(null)
    setErrors({})
    setSaving(true)
    try {
      const records = values.filter((v) => v.trim() !== '').map((content) => ({ content, disabled: false }))
      await api.put(`/dns/zones/${encodeURIComponent(zone)}/rrsets`, { name: absolute, type, ttl, records })
      await queryClient.invalidateQueries({ queryKey: dnsKeys.zone(zone) })
      onClose()
    } catch (err) {
      if (err instanceof ApiError && err.details.length > 0) {
        const fieldErrors: Record<string, string> = {}
        for (const d of err.details) {
          fieldErrors[d.field.replace(/\[\d+\]\.content$/, '')] = d.message
        }
        setErrors(fieldErrors)
      } else {
        setError(err instanceof Error ? err.message : 'Unable to save the record')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal opened onClose={onClose} title={editing ? 'Edit record' : 'New record'} centered size="lg">
      <Stack>
        {error && <Alert color="red">{error}</Alert>}
        {conflict && (
          <Alert color="yellow">
            A {type} record already exists for this name. Saving replaces its {conflict.records.length} value(s); edit it
            instead to keep them.
          </Alert>
        )}
        <Group grow align="flex-start">
          <TextInput
            label="Name"
            description={`Relative to ${zone}, @ for the zone itself`}
            placeholder="@"
            value={name}
            onChange={(e) => setName(e.currentTarget.value)}
            disabled={editing}
            error={errors.name}
            data-autofocus
          />
          <Select
            label="Type"
            description={hint?.help}
            data={[...editableTypes]}
            value={type}
            onChange={(v) => v && setType(v)}
            allowDeselect={false}
            disabled={editing}
            error={errors.type}
          />
        </Group>
        <Text size="xs" c="dimmed" mt={-8}>
          Full name: <Text span ff="monospace">{absolute}</Text>
        </Text>
        <NumberInput
          label="TTL"
          description="How long resolvers may cache the answer, in seconds"
          value={ttl}
          onChange={(v) => setTtl(typeof v === 'number' ? v : 0)}
          min={1}
          allowDecimal={false}
          maw={240}
          error={errors.ttl}
        />
        <Stack gap="xs">
          <Text size="sm" fw={500}>
            {values.length > 1 ? 'Values' : 'Value'}
          </Text>
          {values.map((value, index) => (
            <Group key={index} gap="xs" wrap="nowrap">
              <TextInput
                style={{ flex: 1 }}
                placeholder={hint?.placeholder}
                value={value}
                onChange={(e) => setValue(index, e.currentTarget.value)}
                aria-label={`Value ${index + 1}`}
                ff="monospace"
              />
              <ActionIcon
                variant="subtle"
                color="red"
                aria-label="Remove value"
                disabled={values.length === 1}
                onClick={() => setValues(values.filter((_, i) => i !== index))}
              >
                <IconTrash size={18} />
              </ActionIcon>
            </Group>
          ))}
          {errors.records && (
            <Text size="sm" c="red">
              {errors.records}
            </Text>
          )}
          {type !== 'CNAME' && (
            <Button
              variant="subtle"
              size="compact-sm"
              leftSection={<IconPlus size={14} />}
              onClick={() => setValues([...values, ''])}
              style={{ alignSelf: 'flex-start' }}
            >
              Add value
            </Button>
          )}
        </Stack>
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSave} loading={saving} disabled={values.every((v) => v.trim() === '')}>
            Save
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
