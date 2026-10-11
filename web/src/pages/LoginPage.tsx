import { Alert, Button, PasswordInput, Stack, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { IconAlertCircle } from '@tabler/icons-react'
import { useState } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { AuthCard } from '../components/AuthCard'

export function LoginPage() {
  const { user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const form = useForm({
    initialValues: { username: '', password: '' },
    validate: {
      username: (v) => (v.trim() ? null : 'Username is required'),
      password: (v) => (v ? null : 'Password is required'),
    },
  })

  const from = (location.state as { from?: string } | null)?.from ?? '/'

  if (user) {
    return <Navigate to={user.mustChangePassword ? '/change-password' : from} replace />
  }

  const handleSubmit = form.onSubmit(async ({ username, password }) => {
    setError(null)
    setSubmitting(true)
    try {
      const loggedIn = await login(username.trim(), password)
      navigate(loggedIn.mustChangePassword ? '/change-password' : from, { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Unable to reach the server')
    } finally {
      setSubmitting(false)
    }
  })

  return (
    <AuthCard subtitle="Sign in to manage your network services">
      <form onSubmit={handleSubmit}>
        <Stack>
          {error && (
            <Alert color="red" icon={<IconAlertCircle size={18} />}>
              {error}
            </Alert>
          )}
          <TextInput label="Username" autoComplete="username" autoFocus {...form.getInputProps('username')} />
          <PasswordInput label="Password" autoComplete="current-password" {...form.getInputProps('password')} />
          <Button type="submit" loading={submitting} fullWidth mt="sm">
            Sign in
          </Button>
        </Stack>
      </form>
    </AuthCard>
  )
}
