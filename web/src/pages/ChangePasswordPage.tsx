import { Alert, Button, Group, PasswordInput, Stack } from '@mantine/core'
import { useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import { IconAlertCircle, IconInfoCircle } from '@tabler/icons-react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { AuthCard } from '../components/AuthCard'

const MIN_LENGTH = 8

export function ChangePasswordPage() {
  const { user, changePassword, logout } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const forced = user?.mustChangePassword ?? false

  const form = useForm({
    initialValues: { currentPassword: '', newPassword: '', confirmPassword: '' },
    validate: {
      currentPassword: (v) => (v ? null : 'Current password is required'),
      newPassword: (v, values) => {
        if (v.length < MIN_LENGTH) return `Must be at least ${MIN_LENGTH} characters long`
        if (v === values.currentPassword) return 'Must differ from the current password'
        return null
      },
      confirmPassword: (v, values) => (v === values.newPassword ? null : 'Passwords do not match'),
    },
  })

  const handleSubmit = form.onSubmit(async ({ currentPassword, newPassword }) => {
    setError(null)
    setSubmitting(true)
    try {
      await changePassword(currentPassword, newPassword)
      notifications.show({ color: 'green', message: 'Password changed successfully' })
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Unable to reach the server')
    } finally {
      setSubmitting(false)
    }
  })

  const handleCancel = async () => {
    if (forced) {
      await logout()
      navigate('/login', { replace: true })
    } else {
      navigate(-1)
    }
  }

  return (
    <AuthCard subtitle={forced ? 'A new password is required before continuing' : 'Change your password'}>
      <form onSubmit={handleSubmit}>
        <Stack>
          {forced && (
            <Alert color="blue" icon={<IconInfoCircle size={18} />}>
              You are using a default or temporary password. Please choose a new one.
            </Alert>
          )}
          {error && (
            <Alert color="red" icon={<IconAlertCircle size={18} />}>
              {error}
            </Alert>
          )}
          <PasswordInput
            label="Current password"
            autoComplete="current-password"
            autoFocus
            {...form.getInputProps('currentPassword')}
          />
          <PasswordInput
            label="New password"
            description={`At least ${MIN_LENGTH} characters`}
            autoComplete="new-password"
            {...form.getInputProps('newPassword')}
          />
          <PasswordInput
            label="Confirm new password"
            autoComplete="new-password"
            {...form.getInputProps('confirmPassword')}
          />
          <Group justify="space-between" mt="sm">
            <Button variant="default" onClick={handleCancel}>
              {forced ? 'Log out' : 'Cancel'}
            </Button>
            <Button type="submit" loading={submitting}>
              Change password
            </Button>
          </Group>
        </Stack>
      </form>
    </AuthCard>
  )
}
