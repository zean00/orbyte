import { Empty } from '@/components/feedback/Empty'

export default function ModulesPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-body">Modules</h1>
      <Empty
        title="No modules"
        description="Platform modules will appear here"
      />
    </div>
  )
}
