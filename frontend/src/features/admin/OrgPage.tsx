import { Empty } from '@/components/feedback/Empty'

export default function OrgPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-body">Organization</h1>
      <Empty
        title="No organization data"
        description="Organization structure will appear here"
      />
    </div>
  )
}
