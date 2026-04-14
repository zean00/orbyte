import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { QRCode } from '@/components/ui/QRCode'
import { useAuth } from '@/hooks/useAuth'
import { fetchAuthOptions } from '@/services/bootstrap'
import type { AuthOptions } from '@/services/generated/types'

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
  const { setAuthenticatedUser, isAuthenticated, hasCheckedAuth } = useAuth()
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
  const [authOptions, setAuthOptions] = useState<AuthOptions | null>(null)
  const [challenge, setChallenge] = useState<ChallengePayload | null>(null)
  const [expiredPasswordFlow, setExpiredPasswordFlow] = useState<ChallengePayload | null>(null)
  const [newPassword, setNewPassword] = useState('')

  useEffect(() => {
    let mounted = true
    async function loadOptions() {
      try {
        const options = await fetchAuthOptions()
        if (mounted) setAuthOptions(options)
      } catch {
        // fall back to default local labels and enabled providers
      }
    }
    void loadOptions()
    return () => {
      mounted = false
    }
  }, [])

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
    setAuthenticatedUser({ id: sessionData.user_id, name: sessionData.user_id, email: '', roles: [] })
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
  const passwordEnabled = authOptions?.password_enabled !== false
  const googleEnabled = !!authOptions?.google_enabled
  const loginTitle = authOptions?.login_title || 'Orbyte'
  const loginSubtitle = challenge
    ? 'Two-factor verification is required to continue.'
    : authOptions?.login_subtitle || 'Sign in to your account'
  const googleButtonLabel = authOptions?.google_button_label || 'Sign in with Google'
  const flowLabel = challenge
    ? challengeMode
      ? 'Authenticator setup'
      : 'Identity check'
    : expiredPasswordFlow
      ? 'Password reset'
      : 'Secure sign-in'
  const shellHints = [
    'Workspace routes, role context, and locale are restored after session hydration.',
    'Admin and operator surfaces stay on the same cookie-backed auth model.',
    'Two-factor enrollment and challenge states are resolved in the same session flow.',
  ]

  return (
    <div className="min-h-screen bg-shell px-4 py-4 dark:bg-ink sm:px-6 lg:px-8">
      <div className="mx-auto grid min-h-[calc(100vh-2rem)] max-w-7xl overflow-hidden rounded-[2rem] border border-line bg-surface shadow-panel lg:grid-cols-[1.08fr_0.92fr]">
        <section className="relative overflow-hidden border-b border-line bg-[radial-gradient(circle_at_top_left,_rgba(29,78,216,0.18),_transparent_34%),linear-gradient(180deg,color-mix(in_srgb,var(--color-shell)_90%,white_10%),var(--color-surface))] px-6 py-8 sm:px-8 lg:border-b-0 lg:border-r lg:px-10 lg:py-10">
          <div className="absolute inset-x-0 top-0 h-40 bg-[linear-gradient(180deg,rgba(255,255,255,0.28),transparent)] dark:bg-[linear-gradient(180deg,rgba(148,163,184,0.08),transparent)]" />
          <div className="relative flex h-full flex-col justify-between gap-8">
            <div className="space-y-8">
              <div className="inline-flex items-center gap-3 rounded-full border border-line/70 bg-surface/75 px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-muted backdrop-blur">
                <span className="flex h-2.5 w-2.5 rounded-full bg-accent" />
                Unified operations shell
              </div>

              <div className="max-w-xl space-y-4">
                <div className="text-sm font-semibold uppercase tracking-[0.18em] text-muted">Orbyte</div>
                <h1 className="max-w-lg font-display text-4xl font-semibold leading-tight text-body sm:text-5xl">
                  {loginTitle}
                </h1>
                <p className="max-w-md text-base leading-7 text-muted sm:text-lg">
                  {loginSubtitle}
                </p>
              </div>
            </div>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_220px]">
              <div className="space-y-5">
                <div className="grid gap-4 sm:grid-cols-3">
                  <MetricCard label="Surface access" value="Workspace + Admin" />
                  <MetricCard label="Session model" value="Cookie backed" />
                  <MetricCard label="Step-up auth" value="TOTP ready" />
                </div>

                <div className="space-y-3 border-t border-line/70 pt-5">
                  {shellHints.map((hint) => (
                    <div key={hint} className="flex items-start gap-3 text-sm leading-6 text-muted">
                      <span className="mt-1 flex h-2.5 w-2.5 rounded-full bg-accent" />
                      <span>{hint}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-[1.6rem] border border-line bg-surface/90 p-5 backdrop-blur">
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-muted">Current flow</div>
                <div className="mt-2 text-2xl font-semibold text-body">{flowLabel}</div>
                <div className="mt-4 space-y-3 text-sm leading-6 text-muted">
                  <p>Username and route intent are preserved through login, challenge, and post-auth redirection.</p>
                  <p>Locale and shell context are restored after bootstrap, not guessed from stale local state.</p>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="flex items-center px-6 py-8 sm:px-8 lg:px-10">
          <div className="mx-auto w-full max-w-md">
            <div className="mb-8 space-y-3">
              <div className="inline-flex rounded-full border border-line bg-shell/70 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-muted">
                {flowLabel}
              </div>
              <div>
                <h2 className="text-3xl font-semibold text-body">Continue to your session</h2>
                <p className="mt-2 text-sm leading-6 text-muted">
                  Complete the active authentication step for the requested workspace or admin route.
                </p>
              </div>
            </div>

            {error && (
              <div className="mb-5 rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-200">
                {error}
              </div>
            )}

            {challenge ? (
              <div className="space-y-5">
                {challengeMode ? (
                  <div className="space-y-4 rounded-[1.6rem] border border-line bg-shell/45 p-5 text-sm text-body">
                    <div>
                      <div className="text-base font-semibold">Set up Google Authenticator</div>
                      <div className="mt-1 leading-6 text-muted">
                        Scan the QR code or copy the secret into your authenticator app, then verify with the current 6-digit code.
                      </div>
                    </div>
                    {challenge.qr_uri ? (
                      <div className="rounded-[1.3rem] border border-line bg-white p-4">
                        <div className="mb-3 text-xs font-semibold uppercase tracking-[0.16em] text-muted">Authenticator QR</div>
                        <QRCode value={challenge.qr_uri} className="mx-auto rounded-xl bg-white p-3" />
                      </div>
                    ) : null}
                    {challenge.secret ? (
                      <div className="rounded-xl border border-line bg-surface px-3 py-2 font-mono text-xs text-body">
                        {challenge.secret}
                      </div>
                    ) : null}
                    {challenge.qr_uri ? (
                      <div className="break-all text-xs leading-5 text-muted">{challenge.qr_uri}</div>
                    ) : null}
                  </div>
                ) : (
                  <div className="rounded-[1.6rem] border border-line bg-shell/45 p-5 text-sm leading-6 text-body">
                    Enter the 6-digit code from your authenticator app to finish sign-in.
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
                  className="h-12 rounded-2xl"
                />

                <Button type="button" className="h-12 w-full rounded-2xl text-sm font-semibold" isLoading={isLoading} onClick={() => void handleChallengeVerify()}>
                  Verify Code
                </Button>
              </div>
            ) : expiredPasswordFlow ? (
              <div className="space-y-5">
                <div className="rounded-[1.6rem] border border-line bg-shell/45 p-5 text-sm text-body">
                  <div className="text-base font-semibold">Password change required</div>
                  <div className="mt-2 leading-6 text-muted">
                    Your current password no longer satisfies policy. Set a compliant password to continue.
                  </div>
                  <ul className="mt-4 list-disc space-y-1 pl-5 text-muted">
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
                  className="h-12 rounded-2xl"
                />

                <Button type="button" className="h-12 w-full rounded-2xl text-sm font-semibold" isLoading={isLoading} onClick={() => void handleExpiredPasswordChange()}>
                  Change Password
                </Button>
              </div>
            ) : (
              <>
                {passwordEnabled ? (
                  <form onSubmit={handleSubmit} className="space-y-5">
                    <Input
                      label="Username"
                      type="text"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      placeholder="Enter your username"
                      autoComplete="username"
                      required
                      className="h-12 rounded-2xl"
                    />

                    <Input
                      label="Password"
                      type="password"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="Enter your password"
                      autoComplete="current-password"
                      required
                      className="h-12 rounded-2xl"
                    />

                    <Button type="submit" className="h-12 w-full rounded-2xl text-sm font-semibold" isLoading={isLoading}>
                      Sign in
                    </Button>
                  </form>
                ) : null}

                {googleEnabled ? (
                  <div className={passwordEnabled ? 'mt-6' : ''}>
                    {passwordEnabled ? (
                      <div className="relative">
                        <div className="absolute inset-0 flex items-center">
                          <div className="w-full border-t border-line" />
                        </div>
                        <div className="relative flex justify-center text-sm">
                          <span className="bg-surface px-3 text-muted">Or continue with</span>
                        </div>
                      </div>
                    ) : null}

                    <Button
                      type="button"
                      variant="secondary"
                      className={passwordEnabled ? 'mt-4 h-12 w-full rounded-2xl text-sm font-semibold' : 'h-12 w-full rounded-2xl text-sm font-semibold'}
                      onClick={handleGoogleLogin}
                    >
                      <GoogleIcon className="mr-2 h-5 w-5" />
                      {googleButtonLabel}
                    </Button>
                  </div>
                ) : null}
              </>
            )}

            {!challenge ? (
              <p className="mt-6 text-sm text-muted">
                Need account recovery?{' '}
                <a href="/forgot-password" className="font-medium text-accent hover:underline">
                  Reset or recover access
                </a>
              </p>
            ) : null}
          </div>
        </section>
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

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[1.35rem] border border-line bg-surface/85 px-4 py-4 backdrop-blur">
      <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted">{label}</div>
      <div className="mt-2 text-base font-semibold text-body">{value}</div>
    </div>
  )
}
