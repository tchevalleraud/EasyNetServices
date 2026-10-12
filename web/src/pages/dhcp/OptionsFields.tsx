import { NumberInput, SimpleGrid, TagsInput, TextInput } from '@mantine/core'
import type { GetInputPropsReturnType } from '@mantine/form'
import type { DhcpFamily } from '../../api/types'
import { formatDuration } from '../../utils/format'

interface OptionsFieldsProps {
  family: DhcpFamily
  /** Returns the input props of an option field, e.g. "dnsServers". */
  inputProps: (field: string) => GetInputPropsReturnType
  /** Set in a subnet, where empty options inherit the global ones. */
  inherit?: boolean
}

/** Fields of the network options sent to the clients. */
export function OptionsFields({ family, inputProps, inherit }: OptionsFieldsProps) {
  const v4 = family === 'v4'
  const placeholder = (example: string) => (inherit ? 'Inherited from the DHCP settings' : example)
  return (
    <SimpleGrid cols={{ base: 1, sm: 2 }}>
      <TagsInput
        label="DNS servers"
        placeholder={placeholder(v4 ? '192.168.1.2' : '2001:db8:1::2')}
        {...inputProps('dnsServers')}
      />
      {v4 && <TextInput label="Domain name" placeholder={placeholder('home.lan')} {...inputProps('domainName')} />}
      <TagsInput label="Domain search list" placeholder={placeholder('home.lan')} {...inputProps('domainSearch')} />
      <TagsInput
        label={v4 ? 'NTP servers' : 'SNTP servers'}
        placeholder={placeholder(v4 ? '192.168.1.2' : '2001:db8:1::2')}
        {...inputProps('ntpServers')}
      />
    </SimpleGrid>
  )
}

interface LifetimeInputProps extends GetInputPropsReturnType {
  label: string
  description?: string
  inherit?: boolean
}

/** Input of a lifetime in seconds, showing it in a readable form. In a subnet,
 * 0 inherits the global lifetime and is shown as an empty input. */
export function LifetimeInput({ label, description, inherit, value, onChange, ...props }: LifetimeInputProps) {
  const seconds = typeof value === 'number' ? value : 0
  const readable = seconds > 0 ? formatDuration(seconds) : ''
  return (
    <NumberInput
      label={label}
      description={[description, readable].filter(Boolean).join(' · ')}
      placeholder={inherit ? 'Inherited from the DHCP settings' : undefined}
      suffix=" s"
      min={0}
      max={31536000}
      allowDecimal={false}
      allowNegative={false}
      thousandSeparator=" "
      value={inherit && seconds === 0 ? '' : value}
      onChange={(v: number | string) => onChange(v === '' ? 0 : v)}
      {...props}
    />
  )
}
