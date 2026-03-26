import { Routes, Route } from 'react-router-dom'
import { Empty } from '@/components/feedback/Empty'

function DocumentList() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-body">Documents</h1>
      <Empty
        title="No documents"
        description="Documents will appear here when available"
      />
    </div>
  )
}

export default function DocumentsPage() {
  return (
    <Routes>
      <Route index element={<DocumentList />} />
    </Routes>
  )
}
