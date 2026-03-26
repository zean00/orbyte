import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { QRCode } from '@/components/ui/QRCode'
import { useToast } from '@/components/ui/Toast'
import { useAuthStore } from '@/stores/authStore'

type TwoFactorState = {
  enabled: boolean
  enrollment_allowed: boolean
  issuer: string
  login_mode: string
  approval_mode: string
  approval_step_up_active: boolean
  approval_step_up_until?: string
  enrollment: {
    configured?: boolean
    verified?: boolean
    login_enabled?: boolean
    approval_enabled?: boolean
    issuer?: string
    account_name?: string
    verified_at?: string
    disabled_at?: string
  }
}

type EnrollmentPayload = {
  enrollment: TwoFactorState['enrollment']
  secret?: string
  qr_uri?: string
}

export default function SettingsPage() {
  const { addToast } = useToast()
  const navigate = useNavigate()
  const [state, setState] = useState<TwoFactorState | null>(null)
  const [loading, setLoading] = useState(true)
  const [verifyCode, setVerifyCode] = useState('')
  const [secret, setSecret] = useState('')
  const [qrURI, setQrURI] = useState('')
  const [loginEnabled, setLoginEnabled] = useState(false)
  const [approvalEnabled, setApprovalEnabled] = useState(false)

  async function load() {
    setLoading(true)
    try {
      const response = await fetch('/auth/2fa', { credentials: 'include' })
      if (!response.ok) throw new Error(`Failed to load settings: ${response.status}`)
      const payload = (await response.json()) as TwoFactorState
      setState(payload)
      setLoginEnabled(!!payload.enrollment?.login_enabled || payload.login_mode === 'required')
      setApprovalEnabled(!!payload.enrollment?.approval_enabled || payload.approval_mode === 'required')
    } catch (error) {
      addToast({ message: error instanceof Error ? error.message : 'Failed to load settings', variant: 'error' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  async function startEnrollment() {
    const response = await fetch('/auth/2fa/enroll', {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': readCookie('orbyte_csrf') },
    })
    if (!response.ok) throw new Error(`Failed to start enrollment: ${response.status}`)
    const payload = (await response.json()) as EnrollmentPayload
    setSecret(payload.secret || '')
    setQrURI(payload.qr_uri || '')
    setVerifyCode('')
    addToast({ message: 'Authenticator secret generated. Verify a code to finish setup.', variant: 'success' })
    await load()
  }

  async function verifyEnrollment() {
    const response = await fetch('/auth/2fa/verify-enrollment', {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': readCookie('orbyte_csrf'),
      },
      body: JSON.stringify({ code: verifyCode, login_enabled: loginEnabled, approval_enabled: approvalEnabled }),
    })
    if (!response.ok) throw new Error(await response.text())
    setSecret('')
    setQrURI('')
    setVerifyCode('')
    addToast({ message: 'Two-factor authentication verified.', variant: 'success' })
    await load()
  }

  async function savePreferences() {
    const response = await fetch('/auth/2fa/preferences', {
      method: 'PUT',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': readCookie('orbyte_csrf'),
      },
      body: JSON.stringify({ login_enabled: loginEnabled, approval_enabled: approvalEnabled }),
    })
    if (!response.ok) throw new Error(`Failed to save 2FA preferences: ${response.status}`)
    addToast({ message: 'Two-factor preferences updated.', variant: 'success' })
    await load()
  }

  async function disableEnrollment() {
    const response = await fetch('/auth/2fa', {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'X-CSRF-Token': readCookie('orbyte_csrf') },
    })
    if (!response.ok) throw new Error(`Failed to disable 2FA: ${response.status}`)
    const payload = (await response.json()) as { logged_out?: boolean }
    addToast({ message: 'Two-factor authentication disabled.', variant: 'success' })
    setSecret('')
    setQrURI('')
    setVerifyCode('')
    if (payload.logged_out) {
      useAuthStore.setState({
        user: null,
        token: null,
        isAuthenticated: false,
        isLoading: false,
        hasCheckedAuth: true,
      })
      navigate('/login', { replace: true })
      return
    }
    await load()
  }

  if (loading) {
    return (
      <section className="rounded-2xl border border-line bg-surface p-6 shadow-panel">
        <h1 className="text-2xl font-bold text-body">Settings</h1>
        <p className="mt-2 text-sm text-muted">Loading security settings.</p>
      </section>
    )
  }

  return (
    <section className="space-y-6 rounded-2xl border border-line bg-surface p-6 shadow-panel">
      <div>
        <h1 className="text-2xl font-bold text-body">Security Settings</h1>
        <p className="mt-1 text-sm text-muted">Manage Google Authenticator based two-factor authentication for sign-in and approval step-up.</p>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <Metric label="2FA Enabled" value={state?.enabled ? 'Yes' : 'No'} />
        <Metric label="Login Policy" value={humanize(state?.login_mode || 'disabled')} />
        <Metric label="Approval Policy" value={humanize(state?.approval_mode || 'disabled')} />
        <Metric label="Approval Step-Up" value={state?.approval_step_up_active ? 'Active' : 'Inactive'} />
      </div>

      <section className="rounded-xl border border-line p-4">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Enrollment</h2>
        <div className="mt-3 space-y-3 text-sm text-body">
          <div>Status: {state?.enrollment?.verified ? 'Verified' : state?.enrollment?.configured ? 'Pending verification' : 'Not configured'}</div>
          <div>Issuer: {state?.issuer || 'Orbyte'}</div>
          <div>Account: {state?.enrollment?.account_name || '-'}</div>
        </div>
        <div className="mt-4 flex flex-wrap gap-3">
          <button
            onClick={() => void startEnrollment().catch((error) => addToast({ message: error instanceof Error ? error.message : 'Enrollment failed', variant: 'error' }))}
            disabled={!state?.enabled || !state?.enrollment_allowed}
            className="rounded-lg bg-accent px-4 py-2 text-white disabled:opacity-50"
          >
            {state?.enrollment?.verified ? 'Rotate Secret' : 'Set Up Authenticator'}
          </button>
          <button
            onClick={() => void disableEnrollment().catch((error) => addToast({ message: error instanceof Error ? error.message : 'Disable failed', variant: 'error' }))}
            disabled={!state?.enrollment?.verified || state?.login_mode === 'required' || state?.approval_mode === 'required'}
            className="rounded-lg border border-line px-4 py-2 text-body disabled:opacity-50"
          >
            Disable 2FA
          </button>
        </div>
      </section>

      {(secret || state?.enrollment?.configured && !state?.enrollment?.verified) ? (
        <section className="rounded-xl border border-line p-4">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Verify Authenticator</h2>
          <div className="mt-3 space-y-3 text-sm text-body">
            {qrURI ? (
              <div className="rounded-xl border border-line bg-shell/40 p-4">
                <div className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted">Scan in Google Authenticator</div>
                <QRCode value={qrURI} className="mx-auto rounded-lg border border-line bg-white p-3" />
              </div>
            ) : null}
            {secret ? <div>Secret: <span className="font-mono">{secret}</span></div> : null}
            {qrURI ? <div className="break-all text-xs text-muted">Authenticator URI: {qrURI}</div> : null}
            <label className="flex flex-col gap-1">
              <span className="text-xs font-semibold uppercase tracking-wide text-muted">Verification Code</span>
              <input
                value={verifyCode}
                onChange={(event) => setVerifyCode(event.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                className="h-10 rounded-lg border border-line bg-surface px-3 py-2 text-sm text-body"
                placeholder="123456"
              />
            </label>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              <label className="flex items-center gap-2 text-sm text-body">
                <input type="checkbox" checked={loginEnabled} disabled={state?.login_mode === 'required'} onChange={(event) => setLoginEnabled(event.target.checked)} />
                Require code for sign-in
              </label>
              <label className="flex items-center gap-2 text-sm text-body">
                <input type="checkbox" checked={approvalEnabled} disabled={state?.approval_mode === 'required'} onChange={(event) => setApprovalEnabled(event.target.checked)} />
                Require code for approvals
              </label>
            </div>
            <button
              onClick={() => void verifyEnrollment().catch((error) => addToast({ message: error instanceof Error ? normalizeErrorText(error.message) : 'Verification failed', variant: 'error' }))}
              className="rounded-lg bg-accent px-4 py-2 text-white"
            >
              Verify and Enable
            </button>
          </div>
        </section>
      ) : null}

      {state?.enrollment?.verified ? (
        <section className="rounded-xl border border-line p-4">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Usage Preferences</h2>
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
            <label className="flex items-center gap-2 text-sm text-body">
              <input type="checkbox" checked={loginEnabled} disabled={state.login_mode === 'required' || state.login_mode === 'disabled'} onChange={(event) => setLoginEnabled(event.target.checked)} />
              Use 2FA for sign-in
            </label>
            <label className="flex items-center gap-2 text-sm text-body">
              <input type="checkbox" checked={approvalEnabled} disabled={state.approval_mode === 'required' || state.approval_mode === 'disabled'} onChange={(event) => setApprovalEnabled(event.target.checked)} />
              Use 2FA for approvals
            </label>
          </div>
          <button
            onClick={() => void savePreferences().catch((error) => addToast({ message: error instanceof Error ? error.message : 'Save failed', variant: 'error' }))}
            className="mt-4 rounded-lg bg-accent px-4 py-2 text-white"
          >
            Save Preferences
          </button>
        </section>
      ) : null}
    </section>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-xl border border-line bg-surface p-4 dark:bg-ink/60">
      <div className="text-xs font-semibold uppercase tracking-wide text-body">{label}</div>
      <div className="mt-2 text-xl font-bold text-body">{value}</div>
    </article>
  )
}

function readCookie(name: string): string {
  const cookie = document.cookie
    .split('; ')
    .find((entry) => entry.startsWith(`${name}=`))
  return cookie ? decodeURIComponent(cookie.split('=').slice(1).join('=')) : ''
}

function humanize(value: string): string {
  return value.replace(/[_./-]+/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function normalizeErrorText(value: string): string {
  try {
    const parsed = JSON.parse(value)
    return parsed?.error?.message || value
  } catch {
    return value
  }
}
