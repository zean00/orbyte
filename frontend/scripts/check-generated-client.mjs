import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(__dirname, '..', '..')
const openAPIPath = path.join(repoRoot, 'contracts', 'openapi', 'latest', 'openapi.json')
const generatedClientPath = path.join(repoRoot, 'frontend', 'src', 'services', 'generated', 'client.ts')
const generatedTypesPath = path.join(repoRoot, 'frontend', 'src', 'services', 'generated', 'types.ts')
const apiPath = path.join(repoRoot, 'frontend', 'src', 'services', 'api.ts')

const openAPI = JSON.parse(fs.readFileSync(openAPIPath, 'utf8'))
const generatedClient = fs.readFileSync(generatedClientPath, 'utf8')
const generatedTypes = fs.readFileSync(generatedTypesPath, 'utf8')
const apiSource = fs.readFileSync(apiPath, 'utf8')

const requiredPaths = ['/auth/options', '/ui/bootstrap', '/admin/api/bootstrap']
const staleSnippets = ['/bootstrap/ui', '/bootstrap/admin', '/api/v1']

for (const endpoint of requiredPaths) {
  if (!openAPI.paths?.[endpoint]) {
    throw new Error(`OpenAPI artifact is missing required path ${endpoint}`)
  }
  if (!generatedClient.includes(`'${endpoint}'`)) {
    throw new Error(`Generated client is missing ${endpoint}`)
  }
  if (!generatedTypes.includes(`'${endpoint}'`)) {
    throw new Error(`Generated types are missing ${endpoint}`)
  }
}

if (!generatedTypes.includes('surface?: string')) {
  throw new Error('Generated types are missing the optional /ui/bootstrap surface query parameter')
}

if (!generatedTypes.includes('totp_enabled?: boolean')) {
  throw new Error('Generated types are missing totp_enabled from AuthOptions')
}

for (const snippet of staleSnippets) {
  if (generatedClient.includes(snippet) || generatedTypes.includes(snippet) || apiSource.includes(snippet)) {
    throw new Error(`Stale generated-client reference found: ${snippet}`)
  }
}
