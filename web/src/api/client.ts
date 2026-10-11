/** Invalid field reported by a validation_failed error. */
export interface FieldError {
  field: string
  message: string
}

/** Error returned by the API, mirroring the server error payload. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details: FieldError[]

  constructor(status: number, code: string, message: string, details: FieldError[] = []) {
    super(message)
    this.status = status
    this.code = code
    this.details = details
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    method,
    credentials: 'same-origin',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (!response.ok) {
    let code = 'http_error'
    let message = `Request failed with status ${response.status}`
    let details: FieldError[] = []
    try {
      const payload = await response.json()
      code = payload.error?.code ?? code
      message = payload.error?.message ?? message
      details = payload.error?.details ?? details
    } catch {
      // Not a JSON error payload; keep the generic message.
    }
    throw new ApiError(response.status, code, message, details)
  }

  if (response.status === 204) {
    return undefined as T
  }
  return response.json() as Promise<T>
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
}
