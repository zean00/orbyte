export default function HomePage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-body">Welcome to Orbyte</h1>
        <p className="text-muted mt-1">Document workflow automation platform</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-surface rounded-xl p-6 shadow-panel border border-line">
          <div className="text-3xl font-bold text-accent">0</div>
          <div className="text-sm text-muted mt-1">Active Workitems</div>
        </div>
        <div className="bg-surface rounded-xl p-6 shadow-panel border border-line">
          <div className="text-3xl font-bold text-accent">0</div>
          <div className="text-sm text-muted mt-1">Documents Today</div>
        </div>
        <div className="bg-surface rounded-xl p-6 shadow-panel border border-line">
          <div className="text-3xl font-bold text-accent">0</div>
          <div className="text-sm text-muted mt-1">Workflows Running</div>
        </div>
      </div>

      <div className="bg-surface rounded-xl p-6 shadow-panel border border-line">
        <h2 className="text-lg font-semibold text-body mb-4">Recent Activity</h2>
        <p className="text-muted text-sm">No recent activity</p>
      </div>
    </div>
  )
}
