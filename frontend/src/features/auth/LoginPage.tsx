import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { useAuth } from '@/hooks/useAuth'

export default function LoginPage() {
  const { login, isAuthenticated, hasCheckedAuth } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (hasCheckedAuth && isAuthenticated) navigate('/', { replace: true })
  }, [hasCheckedAuth, isAuthenticated, navigate])

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')

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

      if (!response.ok) {
        throw new Error('Invalid credentials')
      }

      // Get user info from session endpoint
      const sessionResponse = await fetch('/auth/session', {
        credentials: 'include',
      })
      if (sessionResponse.ok) {
        const sessionData = await sessionResponse.json()
        if (sessionData.authenticated && sessionData.user_id) {
          // Use session data to create user object - the token is in httpOnly cookie
          login(
            { id: sessionData.user_id, name: sessionData.user_id, email: '', roles: [] },
            sessionData.user_id
          )
          navigate('/', { replace: true })
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setIsLoading(false)
    }
  }

  const handleGoogleLogin = () => {
    window.location.href = `/auth/google/start?next=${encodeURIComponent('/ui')}`
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-shell dark:bg-ink p-4">
      <div className="w-full max-w-md">
        <div className="bg-surface rounded-xl shadow-panel border border-line p-8">
          <div className="text-center mb-8">
            <h1 className="text-2xl font-bold text-body font-display">Orbyte</h1>
            <p className="text-muted mt-1">Sign in to your account</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-red-800 text-sm">
                {error}
              </div>
            )}

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
        </div>

        <p className="text-center text-sm text-muted mt-4">
          <a href="/forgot-password" className="text-accent hover:underline">
            Forgot your password?
          </a>
        </p>
      </div>
    </div>
  )
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
