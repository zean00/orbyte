import { Empty } from '@/components/feedback/Empty'

export default function WorklistPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-body">Worklist</h1>
      <Empty
        title="No workitems"
        description="Workitems assigned to you will appear here"
      />
    </div>
  )
}
