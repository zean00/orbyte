import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { QRCode } from '@/components/ui/QRCode'
import { useAuth } from '@/hooks/useAuth'

type ChallengePayload = {
  status?: string
  username?: string
  password_policy?: {
    min_length?: number
    require_uppercase?: boolean
    require_number?: boolean
    require_special?: boolean
    max_age_days?: number
  }
  challenge?: {
    id: string
    status: string
    purpose: string
    expires_at?: string
    username?: string
    auth_method?: string
  }
  enrollment?: {
    configured?: boolean
    verified?: boolean
    account_name?: string
  }
  secret?: string
  qr_uri?: string
}

export default function LoginPage() {
  const { login, isAuthenticated, hasCheckedAuth } = useAuth()
  const navigate = useNavigate()
  const nextPath = resolveNextPath()

  useEffect(() => {
    if (hasCheckedAuth && isAuthenticated) {
      redirectAfterAuthentication(navigate, nextPath)
    }
  }, [hasCheckedAuth, isAuthenticated, navigate, nextPath])

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')
  const [challenge, setChallenge] = useState<ChallengePayload | null>(null)
  const [expiredPasswordFlow, setExpiredPasswordFlow] = useState<ChallengePayload | null>(null)
  const [newPassword, setNewPassword] = useState('')

  useEffect(() => {
    let mounted = true
    async function loadChallenge() {
      try {
        const response = await fetch('/auth/2fa/challenge', { credentials: 'include' })
        if (!response.ok) return
        const payload = (await response.json()) as ChallengePayload
        if (mounted) setChallenge(payload)
      } catch {
        // no pending challenge
      }
    }
    void loadChallenge()
    return () => {
      mounted = false
    }
  }, [])

  async function hydrateAuthenticatedUser() {
    const sessionResponse = await fetch('/auth/session', { credentials: 'include' })
    if (!sessionResponse.ok) throw new Error(`Session check failed: ${sessionResponse.status}`)
    const sessionData = await sessionResponse.json()
    if (!sessionData.authenticated || !sessionData.user_id) throw new Error('Authentication did not complete')
    login({ id: sessionData.user_id, name: sessionData.user_id, email: '', roles: [] }, sessionData.user_id)
    redirectAfterAuthentication(navigate, nextPath)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError('')

    try {
      const response = await fetch('/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
        credentials: 'include',
      })
      const payload = await response.json().catch(() => ({}))
      if (response.status === 202) {
        setExpiredPasswordFlow(null)
        setChallenge(payload as ChallengePayload)
        return
      }
      if (response.status === 403 && (payload as ChallengePayload).status === 'password_change_required') {
        setChallenge(null)
        setExpiredPasswordFlow(payload as ChallengePayload)
        return
      }
      if (!response.ok) {
        throw new Error((payload as { error?: { message?: string } })?.error?.message || 'Invalid credentials')
      }
      await hydrateAuthenticatedUser()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setIsLoading(false)
    }
  }

  const handleChallengeVerify = async () => {
    setIsLoading(true)
    setError('')
    try {
      const response = await fetch('/auth/2fa/challenge/verify', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code }),
      })
      const payload = await response.json().catch(() => ({}))
      if (!response.ok) {
        throw new Error((payload as { error?: { message?: string } })?.error?.message || 'Verification failed')
      }
      setChallenge(null)
      setCode('')
      await hydrateAuthenticatedUser()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed')
    } finally {
      setIsLoading(false)
    }
  }

  const handleExpiredPasswordChange = async () => {
    setIsLoading(true)
    setError('')
    const targetUsername = expiredPasswordFlow?.username || username
    try {
      const response = await fetch('/auth/password/change', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: targetUsername,
          current_password: password,
          new_password: newPassword,
        }),
      })
      const payload = await response.json().catch(() => ({}))
      if (!response.ok) {
        throw new Error((payload as { error?: { message?: string } })?.error?.message || 'Password change failed')
      }
      const retryResponse = await fetch('/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: targetUsername, password: newPassword }),
        credentials: 'include',
      })
      const retryPayload = await retryResponse.json().catch(() => ({}))
      if (retryResponse.status === 202) {
        setExpiredPasswordFlow(null)
        setChallenge(retryPayload as ChallengePayload)
        setPassword(newPassword)
        setNewPassword('')
        return
      }
      if (!retryResponse.ok) {
        throw new Error((retryPayload as { error?: { message?: string } })?.error?.message || 'Login failed after password change')
      }
      setExpiredPasswordFlow(null)
      setPassword(newPassword)
      setNewPassword('')
      await hydrateAuthenticatedUser()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Password change failed')
    } finally {
      setIsLoading(false)
    }
  }

  const handleGoogleLogin = () => {
    window.location.href = `/auth/google/start?next=${encodeURIComponent(nextPath || '/ui')}`
  }

  const challengeMode = challenge?.challenge?.purpose === 'totp_enroll'

  return (
    <div className="min-h-screen flex items-center justify-center bg-shell dark:bg-ink p-4">
      <div className="w-full max-w-md">
        <div className="bg-surface rounded-xl shadow-panel border border-line p-8">
          <div className="text-center mb-8">
            <h1 className="text-2xl font-bold text-body font-display">Orbyte</h1>
            <p className="text-muted mt-1">{challenge ? 'Two-factor verification is required to continue.' : 'Sign in to your account'}</p>
          </div>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-800 text-sm">
              {error}
            </div>
          )}

          {challenge ? (
            <div className="space-y-4">
              {challengeMode ? (
                <div className="space-y-2 rounded-lg border border-line p-4 text-sm text-body">
                  <div className="font-semibold">Set up Google Authenticator</div>
                  {challenge.qr_uri ? (
                    <div className="rounded-lg border border-line bg-shell/40 p-3">
                      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">Scan QR Code</div>
                      <QRCode value={challenge.qr_uri} className="mx-auto rounded-lg border border-line bg-white p-3" />
                    </div>
                  ) : null}
                  {challenge.secret ? <div>Secret: <span className="font-mono">{challenge.secret}</span></div> : null}
                  {challenge.qr_uri ? <div className="break-all text-xs text-muted">Authenticator URI: {challenge.qr_uri}</div> : null}
                  <div className="text-muted">Add this secret to Google Authenticator, then enter the 6-digit code below.</div>
                </div>
              ) : (
                <div className="rounded-lg border border-line p-4 text-sm text-body">
                  Enter the 6-digit code from Google Authenticator to complete sign-in.
                </div>
              )}

              <Input
                label="Authentication Code"
                type="text"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                placeholder="123456"
                autoComplete="one-time-code"
                required
              />

              <Button type="button" className="w-full" isLoading={isLoading} onClick={() => void handleChallengeVerify()}>
                Verify Code
              </Button>
            </div>
          ) : expiredPasswordFlow ? (
            <div className="space-y-4">
              <div className="rounded-lg border border-line p-4 text-sm text-body">
                <div className="font-semibold">Password change required</div>
                <div className="mt-2 text-muted">Your password has expired. Set a new password to continue signing in.</div>
                <ul className="mt-3 list-disc space-y-1 pl-5 text-muted">
                  <li>Minimum length: {expiredPasswordFlow.password_policy?.min_length || 8}</li>
                  {expiredPasswordFlow.password_policy?.require_uppercase ? <li>Must include an uppercase letter</li> : null}
                  {expiredPasswordFlow.password_policy?.require_number ? <li>Must include a number</li> : null}
                  {expiredPasswordFlow.password_policy?.require_special ? <li>Must include a special character</li> : null}
                </ul>
              </div>

              <Input
                label="New Password"
                type="password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                placeholder="Enter a new password"
                autoComplete="new-password"
                required
              />

              <Button type="button" className="w-full" isLoading={isLoading} onClick={() => void handleExpiredPasswordChange()}>
                Change Password
              </Button>
            </div>
          ) : (
            <>
              <form onSubmit={handleSubmit} className="space-y-4">
                <Input
                  label="Username"
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="Enter your username"
                  autoComplete="username"
                  required
                />

                <Input
                  label="Password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter your password"
                  autoComplete="current-password"
                  required
                />

                <Button type="submit" className="w-full" isLoading={isLoading}>
                  Sign in
                </Button>
              </form>

              <div className="mt-6">
                <div className="relative">
                  <div className="absolute inset-0 flex items-center">
                    <div className="w-full border-t border-line" />
                  </div>
                  <div className="relative flex justify-center text-sm">
                    <span className="px-2 bg-surface text-muted">Or continue with</span>
                  </div>
                </div>

                <Button
                  type="button"
                  variant="secondary"
                  className="w-full mt-4"
                  onClick={handleGoogleLogin}
                >
                  <GoogleIcon className="w-5 h-5 mr-2" />
                  Sign in with Google
                </Button>
              </div>
            </>
          )}
        </div>

        {!challenge ? (
          <p className="text-center text-sm text-muted mt-4">
            <a href="/forgot-password" className="text-accent hover:underline">
              Forgot your password?
            </a>
          </p>
        ) : null}
      </div>
    </div>
  )
}

function resolveNextPath(): string {
  if (typeof window === 'undefined') return '/ui'
  const next = new URLSearchParams(window.location.search).get('next') || '/ui'
  return sanitizeNextPath(next)
}

function sanitizeNextPath(next: string): string {
  const trimmed = next.trim()
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) return '/ui'
  if (trimmed.startsWith('/ui/login')) return '/ui'
  return trimmed
}

function redirectAfterAuthentication(navigate: ReturnType<typeof useNavigate>, nextPath: string) {
  if (nextPath.startsWith('/admin') || nextPath.startsWith('/ui/')) {
    window.location.assign(nextPath)
    return
  }
  if (nextPath === '/ui') {
    navigate('/', { replace: true })
    return
  }
  navigate(nextPath, { replace: true })
}

function GoogleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24">
      <path
        fill="#4285F4"
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
      />
      <path
        fill="#34A853"
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
      />
      <path
        fill="#FBBC05"
        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
      />
      <path
        fill="#EA4335"
        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
      />
    </svg>
  )
}
