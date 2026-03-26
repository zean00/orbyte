import { Empty } from '@/components/feedback/Empty'

export default function WorkflowsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-body">Workflows</h1>
      <Empty
        title="No workflows"
        description="Workflows will appear here when available"
      />
    </div>
  )
}
