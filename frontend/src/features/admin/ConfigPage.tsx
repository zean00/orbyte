import { Empty } from '@/components/feedback/Empty'

export default function ConfigPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-body">Configuration</h1>
      <Empty
        title="No configuration"
        description="Platform configuration will appear here"
      />
    </div>
  )
}
