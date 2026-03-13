# SDaaS Platform — Full Architecture Design
*Software Department as a Service · v0.1 draft*

---

## Overview

The SDaaS platform is a product-agnostic software operations system composed of five layers. Each layer has a single responsibility and communicates with adjacent layers through defined interfaces and events. The architecture is designed so that the only layer that changes per application is the Adapter Layer — everything above and below it is universal.

```
┌─────────────────────────────────────────────────┐
│  01 · HUMAN CONTROL PLANE          👤  gold      │
│  Task Backlog · Staging Preview · Command Center │
│  Monitoring Dashboard · Visual Design Studio     │
├─────────────────────────────────────────────────┤
│  02 · AGENT ORCHESTRATION LAYER    🤖  red       │
│  Developer · QA · Sysadmin · DevOps · UI Design  │
├─────────────────────────────────────────────────┤
│  03 · PLATFORM CORE                ⚡  purple    │
│  Event Bus · Pipeline · Observability · Auth     │
├─────────────────────────────────────────────────┤
│  04 · APPLICATION ADAPTER LAYER    🔌  blue      │
│  Odoo · Generic App · Adapter SDK               │
├─────────────────────────────────────────────────┤
│  05 · INFRASTRUCTURE LAYER         🏗️  green     │
│  Environments · Containers · Storage · Network   │
└─────────────────────────────────────────────────┘
```

---

## Layer 1 — Human Control Plane 👤

> **Design Principle:** The human never writes code, touches infrastructure, or makes deployment decisions under pressure. Every interaction is a deliberate, informed choice — submitting intent (task) or authorizing outcomes (approve / deploy). The platform makes consequences fully visible before confirmation is requested.

---

### 📋 Task Backlog
`UI` `queue` `feedback-loop`

The primary contract between humans and agents. A task must contain: intent (what), acceptance criteria (how to know it's done), and optionally: priority and deadline. Agents never act on ambiguous tasks — the system prompts for clarification before dispatch.

| Sub-component | Type | Description |
|---|---|---|
| TaskSubmitForm | UI | Natural language input. Optional: attach screenshot, reference URL, priority tag. Submitted as event to Event Bus. |
| PriorityQueue | UI | Kanban-style view. Columns: Backlog → In Progress → In Staging → Awaiting Review → Done. Drag-to-reorder for priority. |
| TaskDetailPanel | UI | Opens on click. Shows: description, linked branch, agent activity log, staging preview link, test results, comment thread. |
| FeedbackThread | UI | Inline comments on a task. Human feedback stored as structured event, re-ingested by Developer Agent for revision. |
| TaskStateEngine | service | Manages state transitions. Emits task.state_changed events. Prevents illegal transitions (e.g. Done → In Progress without a new task). |

**Data In:** `user.task_submitted` · `agent.task_status_update` · `qa.review_ready`  
**Data Out:** `task.created` · `task.feedback_added` · `task.approved_for_staging`

---

### 🖥️ Staging Environment Preview
`preview` `diff` `approval`

Staging is treated as a read-only artifact for the human. Any feedback goes back through the task thread and re-triggers the developer agent — humans do not directly modify staging.

| Sub-component | Type | Description |
|---|---|---|
| EmbeddedPreview | UI | Full iframe of staging environment. Isolated subdomain with staging data. Visually identical to production. |
| ChangeSummaryPanel | UI | Auto-generated diff: files changed, routes added/removed, DB migrations, dependency changes. Human-readable summary by LLM. |
| TestResultsBadge | UI | QA Agent's test run results inline. Pass/fail per test suite. Expandable log. Links to full distributed trace. |
| ApprovalWidget | UI | Approve / Request Changes / Reject. Requires text input if rejecting. Approval triggers pipeline.stage_approved event. |
| StagingLifecycle | service | Manages staging spinup/teardown. TTL-based cleanup. Snapshot capability for replay and forensics. |

**Data In:** `pipeline.staging_ready` · `qa.tests_complete` · `agent.change_summary`  
**Data Out:** `human.approved_staging` · `human.requested_changes` · `human.rejected`

---

### 🚀 Command Center
`deploy` `rollback` `audit`

The only place production deploys originate. It is never called by an agent. The architecture physically prevents automated production deployment — `pipeline.deploy_authorized` can only be emitted from an authenticated human session.

| Sub-component | Type | Description |
|---|---|---|
| DeployButton | UI | Confirmation-gated. Shows: what's being deployed, from which staging snapshot, diff from current prod. Two-step: review → confirm. |
| EnvironmentStatusBar | UI | Live status of DEV / STAGING / PROD. Git SHA, deploy time, health checks, active user count per environment. |
| RollbackTimeline | UI | Visual timeline of all production deployments. Each entry: timestamp, deployer, tasks included, health snapshot. One-click rollback. |
| DeploymentStrategy | config | Per-app config: canary (5%→25%→100%), blue-green, or direct. Strategy shown to human before confirming deploy. |
| AuditLog | service | Append-only log of every action taken in Command Center. Stored immutably. Queryable. Required for compliance. |

**Data In:** `pipeline.ready_for_production` · `human.approved_staging`  
**Data Out:** `pipeline.deploy_authorized` · `pipeline.rollback_requested`

---

### 📊 Monitoring Dashboard
`observability` `metrics` `agent-status`

Read-only for most users. Surfaces what the Sysadmin Agent sees — the human looks through the agent's eyes with full raw access available one click deeper.

| Sub-component | Type | Description |
|---|---|---|
| RequestRateChart | UI | Live req/s, p50/p95/p99 latency, error rate. Time range selector: 1h/6h/24h/7d. Annotated with deploy events. |
| ResourceGauge | UI | CPU, memory, disk, network per service. Color-coded threshold bands. |
| ErrorFeed | UI | Live stream of errors. Grouped by type. Click to expand full stack trace and trace ID. |
| AgentActivityPanel | UI | Real-time view of all agent states. Which task is dev agent working on? What anomaly did sysadmin flag? |
| IncidentTimeline | UI | Chronological incidents raised by Sysadmin Agent. Status: open/mitigated/resolved. Links to task if fix was created. |

**Data In:** `obs.metrics_stream` · `obs.log_stream` · `agent.activity_event` · `incident.created`  
**Data Out:** `human.alert_acknowledged` · `human.incident_comment`

---

### 🎨 Visual Design Studio
`UI` `design-intent` `no-code`

The non-technical human's primary customization surface. A business owner should be able to describe a UI change entirely through visual annotation — and the UI Designer Agent translates that into working, tested, staged code.

| Sub-component | Type | Description |
|---|---|---|
| ScreenAnnotator | UI | Upload a screenshot of any app screen. Draw directly on it: highlight areas to change, add sticky-note annotations, mark elements to move/remove/redesign. Annotations become structured design intent consumed by UI Designer Agent. |
| ComponentPalette | UI | Visual library of the app's existing UI components: buttons, forms, tables, modals. Drag onto a canvas to describe layout changes without writing any code. |
| DesignDiffPreview | UI | Side-by-side: current screen (live from staging) vs. proposed design. Human marks approval per screen area. Unapproved areas route back to UI Designer Agent for revision. |
| BrandTokenEditor | UI | Visual controls for design tokens: colors, typography scale, spacing units, border radius, shadows. Changes propagate to all components automatically. No CSS knowledge required. |
| ResponsiveChecker | UI | Preview any staged UI change at mobile, tablet, and desktop breakpoints before approving. Flags layout regressions automatically. |
| DesignTaskBridge | service | Converts visual annotations, token changes, and layout instructions into structured design.task.submitted events on the Event Bus, consumed by the UI Designer Agent. |

**Data In:** `human.design_annotation` · `human.brand_token_change` · `human.layout_instruction`  
**Data Out:** `design.task.submitted` · `design.feedback_added` · `design.approved`

---

### Task State Machine

```
Draft → Submitted → In Progress → In Staging → In Review → Feedback → Approved → Deployed
```

| State | Description | Trigger |
|---|---|---|
| Draft | Human composing the request. Not yet submitted. Stored locally. | User opens submission form |
| Submitted | Task emitted to Event Bus. Awaiting pickup by Developer Agent. | Human clicks submit |
| In Progress | Developer Agent claimed the task, actively working in DEV branch. | dev_agent.task_claimed event |
| In Staging | Agent pushed code to staging. QA Agent running test suite. | pipeline.staging_deployed event |
| In Review | QA Agent approved. Human can access staging preview and give feedback. | qa_agent.review_ready event |
| Feedback | Human left comments. Routes back to Developer Agent with revision context. | human.requested_changes event |
| Approved | Human approved staging. In queue for next production deployment window. | human.approved_staging event |
| Deployed | Production deployment completed. Task closed. In rollback history. | pipeline.prod_deployed event |

---

## Layer 2 — Agent Orchestration Layer 🤖

> **Design Principle:** Each agent has exactly one role, one trigger condition, one output type, and one escalation path. Agents never call each other directly — they publish events to the Event Bus. This keeps each agent independently testable, replaceable, and auditable. An agent that fails does not cascade to others.

---

### Shared Agent Contract

Every agent implements this interface:

| Field | Type | Description |
|---|---|---|
| trigger_event | EventType | The single event type this agent wakes on |
| context_builder() | fn → Context | Fetches all relevant state before acting: task, codebase, logs, metrics |
| action_loop() | fn → Actions | Core LLM reasoning + tool execution loop |
| output_event | EventType | The single event type this agent emits on completion |
| escalate(reason) | fn → Incident | Called when agent cannot complete. Raises incident for next agent or human. |
| health_check() | fn → Status | Heartbeat endpoint. Platform checks this to detect stuck/crashed agents. |

---

### 👨‍💻 Developer Agent
**Trigger:** `task.submitted` · **Output:** `pipeline.dev_ready`

Operates in strict loop: plan → code → test → push. Never deploys beyond DEV. If tests fail after 3 self-correction attempts, escalates with a detailed failure report rather than looping indefinitely.

| Sub-component | Type | Description |
|---|---|---|
| ContextLoader | module | Fetches task description, acceptance criteria, linked codebase snapshot, relevant past tasks, and test suite config. |
| PlanningModule | LLM | Breaks task into atomic code changes. Identifies affected files, modules, test cases. Emits structured JSON plan before coding. |
| CodeExecutor | tool | Git branch creation, file writes, file edits. Operates inside DEV container. Sandboxed — no network access except internal APIs. |
| TestRunner | tool | Executes the app's test suite against changes. Parses results. If failure: attempts self-correction up to 3 times before escalating. |
| PRSummarizer | LLM | Generates human-readable summary: what changed, why, what to test manually, known limitations. Attached to task detail panel. |
| StagingPusher | tool | Triggers pipeline.dev_ready event with branch reference. Hands off to Pipeline Engine. |

**Data In:** `task.submitted` · `human.requested_changes (revision)`  
**Data Out:** `pipeline.dev_ready` · `task.status_update` · `agent.escalation (if stuck)`

**State Machine:** `Idle → Claimed → Planning → Coding → Testing → Self-Correcting → Pushing → Escalated`

---

### 🔍 QA Agent
**Trigger:** `pipeline.staging_deployed` · **Output:** `qa.review_ready`

Scope is strictly automated validation. Does not make subjective judgments. If acceptance criteria require human judgment (e.g. 'the UI looks good'), it flags those explicitly for human review rather than auto-approving.

| Sub-component | Type | Description |
|---|---|---|
| RegressionRunner | tool | Runs full regression suite against staging. Compares to baseline from last production deploy. Flags any new failures. |
| E2ERunner | tool | Executes end-to-end browser tests using headless browser. Tests critical user journeys defined in test config. |
| APIValidator | tool | Validates all API endpoints: response schemas, status codes, response time SLAs. Compares to OpenAPI spec if available. |
| AcceptanceCriteria | LLM | Maps test results to original task's acceptance criteria. Determines if each criterion is satisfied, partial, or unmet. |
| ReviewReporter | LLM | Generates structured test report: summary, passed/failed, coverage delta, manual test suggestions. Attached to task. |

**Data In:** `pipeline.staging_deployed`  
**Data Out:** `qa.review_ready (pass)` · `qa.review_failed (fail → back to developer)`

---

### 🛡️ Sysadmin Agent
**Trigger:** `obs.anomaly_detected` · **Output:** `incident.created`

Always running — never goes idle. For P1 severity, both creates an incident AND notifies the human simultaneously. Never delays human notification to attempt a fix first.

| Sub-component | Type | Description |
|---|---|---|
| LogTailer | tool | Continuously tails structured logs from Observability Engine. Applies pattern matching for known error signatures. |
| AnomalyClassifier | LLM | Classifies anomaly: type, severity (P1–P3), likely root cause, affected service. |
| IncidentCreator | tool | Creates structured incident: classification, timeline, affected services, raw log samples. Emits incident.created. |
| HotfixProposer | LLM | For known fix patterns (cache clear, restart pod, rollback config), proposes and can auto-apply in DEV for review. |
| EscalationRouter | service | Determines if incident needs immediate human notification (P1), Developer Agent involvement, or DevOps Agent involvement. |

**Data In:** `obs.anomaly_detected` · `obs.log_stream (continuous)`  
**Data Out:** `incident.created` · `task.submitted (hotfix)` · `human.p1_alert`

---

### ⚙️ DevOps Agent
**Trigger:** `obs.metric_threshold` · **Output:** `infra.scaling_applied`

Only agent with direct infrastructure write access. All changes logged with before/after state. Cost threshold breaches always require human acknowledgment — agent never makes spend decisions above configured limits autonomously.

| Sub-component | Type | Description |
|---|---|---|
| MetricWatcher | tool | Subscribes to metrics stream. Evaluates scaling rules: CPU > 80% for 3min, memory > 85%, request queue depth > 500. |
| ScalingDecider | LLM | Determines appropriate scaling action: scale out (add pods), scale up (increase resources), or scale in (reduce during low traffic). |
| InfraController | tool | Calls Environment Manager API to execute scaling decisions. Validates new state after change. |
| DeploymentManager | tool | Executes canary promotion, blue-green switches, health-check monitoring during rollouts. Can pause mid-deploy on degradation. |
| CostMonitor | service | Tracks infra spend vs. budget. Alerts human if monthly projection exceeds threshold. Recommends right-sizing optimizations. |

**Data In:** `obs.metric_threshold` · `pipeline.deploy_authorized`  
**Data Out:** `infra.scaling_applied` · `deployment.completed` · `human.cost_alert`

---

### 🎨 UI Designer Agent
**Trigger:** `design.task.submitted` · **Output:** `pipeline.dev_ready`

Not a general-purpose developer. Operates exclusively on UI layer files: components, stylesheets, design tokens, layout templates. Never touches backend logic, API routes, or database schema. If a design task implies a data change (e.g. add a new field to a form), it creates a linked backend task for the Developer Agent rather than implementing it itself.

| Sub-component | Type | Description |
|---|---|---|
| DesignContextLoader | module | Fetches the design task: screen annotations, brand token changes, layout instructions, component palette selections, and the current app UI component tree. |
| DesignPlanner | LLM | Interprets visual intent into a concrete implementation plan: which CSS/component files to change, which tokens to update, what markup to restructure. Produces diff plan before writing code. |
| TokenApplicator | tool | Applies brand token changes (colors, fonts, spacing) to the design system config. Propagates changes to all components that reference those tokens. |
| ComponentRewriter | tool | Modifies component markup, layout, and styles based on DesignPlanner output. Operates in DEV environment. Works within the app's existing component system. |
| ScreenshotValidator | tool | Takes headless browser screenshots of modified screens at all breakpoints. Compares against the human's original annotation to validate visual alignment with intent. |
| A11yChecker | tool | Runs accessibility audit on modified screens: contrast ratios, ARIA labels, keyboard navigation, focus order. Flags regressions before pushing to staging. |
| DesignSummarizer | LLM | Generates human-readable design change summary: what changed visually, before/after screenshots embedded in task detail, known limitations or deviations from annotation. |

**Data In:** `design.task.submitted` · `human.design_feedback (revision)`  
**Data Out:** `pipeline.dev_ready` · `design.task.status_update` · `agent.escalation (if ambiguous intent)`

**State Machine:** `Idle → Claimed → Interpreting → Implementing → Screenshotting → A11y Check → Pushing`  
*Exception paths:* `Clarifying` (ambiguous annotation) · `Escalated` (3 revision cycles without convergence)

---

### Inter-Agent Event Flow
*Via Event Bus — never direct calls*

| From | Event | To | Note |
|---|---|---|---|
| Human | `task.submitted` | Developer Agent | New feature or fix request |
| Developer Agent | `pipeline.staging_deployed` | QA Agent | Code ready for validation |
| QA Agent | `qa.review_failed` | Developer Agent | Tests failed, revision needed |
| QA Agent | `qa.review_ready` | Human | Ready for human preview |
| Sysadmin Agent | `task.submitted (hotfix)` | Developer Agent | Programmatically created incident task |
| Sysadmin Agent | `incident.infra_involved` | DevOps Agent | Anomaly requires infra action |
| DevOps Agent | `infra.scaling_applied` | Sysadmin Agent | Infra change for log correlation |
| Obs Engine | `obs.anomaly_detected` | Sysadmin Agent | Automatic anomaly classification input |
| Human | `design.task.submitted` | UI Designer Agent | Visual annotation or token change |
| UI Designer Agent | `pipeline.dev_ready` | QA Agent | UI change ready for visual regression |
| UI Designer Agent | `task.submitted (backend link)` | Developer Agent | Form/data changes implied by UI design |
| QA Agent | `qa.review_failed` | UI Designer Agent | Visual regression detected, revision needed |

---

## Layer 3 — Platform Core ⚡

> **Design Principle:** Entirely product-agnostic — knows nothing about Odoo, task contents, or business logic. Provides durable, observable, and policy-enforced infrastructure for events, pipelines, observability, and authorization. Must be more reliable than the agents and applications it serves.

---

### ⚡ Event Bus
`async` `durable` `event-sourced`

The single source of truth for "what happened and when." All agent actions are traceable to an event chain. The append-only log enables full system replay — useful for debugging, auditing, and disaster recovery.

| Sub-component | Type | Description |
|---|---|---|
| Publisher API | service | Authenticated REST/gRPC endpoint. Accepts events with: event_type, source_agent, payload, idempotency_key. Returns ack with event_id. |
| TopicRouter | service | Routes events to subscriber queues by event_type. Fan-out supported. Dead-letter queue for unroutable events. |
| DurableStore | storage | Events persisted before delivery. Append-only log (Kafka / NATS JetStream). Replay any event range. Retention: 90 days. |
| Subscriber SDK | library | Agents use this to subscribe. Handles: at-least-once delivery, ack/nack, backoff/retry, consumer group management. |
| DeadLetterQueue | service | Receives events that failed delivery after max retries. Triggers ops.dlq_event alert. Human reviewable in dashboard. |
| EventSchemaRegistry | service | All event schemas versioned and validated at publish time. Breaking schema changes require version bump. |

#### Core Event Schemas

**`task.submitted`** — Publishers: Human UI · Subscribers: Dev Agent
```
event_id              uuid
task_id               uuid
title                 string
description           string
acceptance_criteria   string[]
priority              low|medium|high|critical
submitted_by          user_id
timestamp             ISO8601
```

**`pipeline.staging_deployed`** — Publishers: Pipeline Engine · Subscribers: QA Agent, Human UI
```
event_id              uuid
pipeline_run_id       uuid
task_id               uuid
branch                string
staging_url           string
diff_summary          DiffSummary
deployed_at           ISO8601
```

**`obs.anomaly_detected`** — Publishers: Observability Engine · Subscribers: Sysadmin Agent
```
event_id              uuid
anomaly_id            uuid
service               string
anomaly_type          string
severity              P1|P2|P3
log_samples           LogLine[]
metric_snapshot       MetricSnapshot
detected_at           ISO8601
```

---

### 🔄 Pipeline Engine
`stages` `gates` `rollback`

Enforces the invariant that no code reaches production without passing every gate. Gates cannot be bypassed even by admins. Emergency rollbacks still require human authorization.

#### Stages

| Stage | Mode | Trigger | Gate Conditions |
|---|---|---|---|
| 🔧 DEV | AUTO | Agent push | Unit tests pass · Lint clean · No secrets in code |
| 🔬 STAGING | AUTO | Gate pass from DEV | QA agent approval · Regression suite pass · Performance baseline met |
| 🚀 PROD | HUMAN GATE | Human authorization | Human approves · Canary health check · Rollback plan confirmed |

| Sub-component | Type | Description |
|---|---|---|
| StageController | service | Manages DEV→STAGING→PROD progression. Each stage is isolated. Enforces gate conditions before advancing. |
| GateEvaluator | service | Evaluates gate conditions for each stage transition. Gates are configurable rules: test thresholds, performance baselines, human approvals. |
| SnapshotManager | service | Creates immutable snapshots at key points: pre-deploy, post-deploy. Used for rollback and replay. |
| DeploymentExecutor | service | Executes deployment against Infrastructure Layer via Environment Manager. Supports canary, blue-green, direct strategies. |
| RollbackEngine | service | Restores any prior snapshot on request. Validates rollback target before executing. Emits pipeline.rollback_completed. |
| PipelineAuditLog | storage | Immutable record of every pipeline run: stages, gate results, durations, who approved, what deployed. Queryable via API. |

**Data In:** `pipeline.dev_ready` · `human.approved_staging` · `pipeline.deploy_authorized` · `pipeline.rollback_requested`  
**Data Out:** `pipeline.staging_deployed` · `pipeline.ready_for_production` · `pipeline.prod_deployed` · `pipeline.rollback_completed`

---

### 🔭 Observability Engine
`logs` `metrics` `traces` `alerts`

Shared nervous system. Every agent reads from it and every service writes to it. Never a single point of failure — deployed redundantly with its own health monitored by a lightweight watchdog.

**LOGS**
- Structured JSON — level, timestamp, trace_id, service, message
- Tailed from each container via sidecar agent
- Indexed in time-series store (Loki / OpenSearch)
- Retention: 30 days hot, 1 year cold archive

**METRICS**
- Prometheus-compatible scrape endpoints on all services
- Agent work metrics: tasks/hr, cycle time, error rate
- Infra metrics: CPU, memory, disk, network per pod
- Business metrics: deploy frequency, MTTR, change failure rate

**TRACES**
- OpenTelemetry spans across all agent actions
- Trace ID propagated from task creation → deployment
- Flame graph available per deployment in dashboard
- Stored in compatible backend (Tempo / Jaeger)

**ALERTS**
- Alert rules defined as code (AlertManager / Grafana)
- Severity: P1 (page) → P2 (ticket) → P3 (log)
- Sysadmin agent is alert receiver — triages before human escalation
- Human notified only for P1 or agent-unresolvable P2

| Sub-component | Type | Description |
|---|---|---|
| LogAggregator | service | Collects structured logs from all containers via sidecar. Parses, indexes, stores in time-series log store. Exposes query API. |
| MetricsCollector | service | Scrapes Prometheus endpoints on all services. Stores in time-series DB (VictoriaMetrics). Exposes PromQL API. |
| TraceCollector | service | Receives OTLP trace spans. Assembles into full traces. Links to log lines and metrics via trace_id correlation. |
| AlertEngine | service | Evaluates alert rules against metrics and log patterns. Emits obs.anomaly_detected events. Rules defined as code. |
| DashboardAPI | service | Unified query API consumed by Monitoring Dashboard. Handles metric queries, log search, trace lookup, alert status. |
| RetentionManager | service | Enforces data retention policies. Compresses and tiers data to cold storage. Compliant deletion on request. |

---

### 🔐 Auth & Policy Engine
`RBAC` `audit` `approval-workflow`

The constitutional layer. Cannot be overridden by agents. No agent has production write access except via the Pipeline Engine, which itself requires a human-authorized event.

| Sub-component | Type | Description |
|---|---|---|
| IdentityProvider | service | SSO/SAML/OIDC integration. JWTs issued per session. Short-lived tokens with refresh. MFA enforced for Approver and Admin roles. |
| RBACEnforcer | service | Middleware injected into all API routes. Validates JWT and checks role permission matrix on every request. No permission = 403. |
| ApprovalWorkflow | service | Manages multi-party approval flows. Configurable: single approver, quorum (N of M), sequential chain. |
| PolicyEngine | service | OPA-compatible policy rules. Evaluates: time-based deploy windows, required approvals per env, cost spend limits. |
| ImmutableAuditLog | storage | Every action appended to tamper-evident log. Signed with HMAC. Exportable for compliance. |
| SecretsVault | service | Centralized secrets management. Agents receive short-lived credentials. Rotation handled automatically. |

#### RBAC Matrix

| Action | Viewer | Operator | Approver | Admin |
|---|:---:|:---:|:---:|:---:|
| View staging | ✓ | ✓ | ✓ | ✓ |
| Submit task | — | ✓ | ✓ | ✓ |
| Add task feedback | ✓ | ✓ | ✓ | ✓ |
| Approve staging | — | — | ✓ | ✓ |
| Deploy to prod | — | — | ✓ | ✓ |
| Rollback prod | — | — | — | ✓ |
| Manage agents | — | — | — | ✓ |
| View audit log | — | ✓ | ✓ | ✓ |
| Edit RBAC | — | — | — | ✓ |
| Submit design task | — | ✓ | ✓ | ✓ |
| Edit brand tokens | — | — | ✓ | ✓ |
| Approve design | — | — | ✓ | ✓ |

---

## Layer 4 — Application Adapter Layer 🔌

> **Design Principle:** The only part of the platform that knows anything about a specific application. It translates the platform's generic commands into app-specific API calls, and translates app-specific events back into the platform's event vocabulary. Swap the adapter and the entire platform above it continues to work unchanged. This is the architectural seam that makes the platform product-agnostic.

### The Invariant

| Platform sends | Adapter translates |
|---|---|
| `deploy.execute(branch, env)` | → app-specific API call |
| `config.set(key, value)` | → app config file write |
| `health.check()` | → app health endpoint |
| `log.stream.subscribe()` | → app log format parser |
| `backup.create()` | → app DB dump command |
| `backup.restore(snapshot_id)` | → app DB restore procedure |

---

### Adapter Interface Contract

Every adapter must implement these 11 methods:

| Method | Returns | Description |
|---|---|---|
| `deploy(branch, environment, strategy)` | DeployResult | Takes a git branch and deploys it to the specified environment using the given strategy (canary/blue-green/direct). Returns deploy status, URL, and health. |
| `rollback(snapshot_id, environment)` | RollbackResult | Restores the application to the state at the given snapshot. Must validate snapshot integrity before applying. |
| `health_check(environment)` | HealthStatus | Returns current health of the app in the given environment: status (healthy/degraded/down), response time, error rate, active sessions. |
| `get_logs(environment, since, filters)` | LogStream | Returns a structured stream of application logs. Adapter is responsible for normalizing app-native log format to platform schema. |
| `get_metrics(environment, metric_names, range)` | MetricSeries[] | Returns time-series metric data in Prometheus-compatible format. Adapter maps app-native metrics to platform metric vocabulary. |
| `set_config(key, value, environment)` | ConfigResult | Sets a configuration value in the target environment. Adapter handles: file writes, env vars, or app config API as appropriate. |
| `create_snapshot(environment, label)` | Snapshot | Creates a named point-in-time snapshot of the application state: code, DB, config. Used by Pipeline Engine before every deploy. |
| `restore_snapshot(snapshot_id, environment)` | RestoreResult | Full restore of the application to a prior snapshot state. Must handle DB restore, config restore, and code rollback atomically. |
| `run_tests(branch, test_suite)` | TestResults | Executes the app's test suite against a given branch. Returns structured results: pass/fail per test, coverage delta, duration. |
| `list_extensions()` | Extension[] | Returns all currently installed extensions/plugins with version and compatibility status. |
| `install_extension(id, version)` | InstallResult | Installs a 3rd-party extension. Adapter handles: download, compatibility check, install, activation, and post-install health check. |

#### Adapter Lifecycle State Machine

```
Unregistered → Registered → Validating → Active → Degraded / Unavailable
                                                  ↕
                                               Updating
```

| State | Description | Trigger |
|---|---|---|
| Unregistered | Adapter package exists but not yet registered with the platform. | Adapter SDK installed |
| Registered | Adapter declared in platform config with app endpoint, credentials, and environment map. | Admin registers adapter |
| Validating | Platform calls health_check() and run_tests() to verify adapter and app is reachable. | Registration complete |
| Active | Adapter passed validation. Platform can now dispatch commands. | Validation passes |
| Degraded | health_check() returning degraded status. Sysadmin Agent alerted. Non-critical ops continue. | health_check returns degraded |
| Unavailable | health_check() failing. Platform suspends dispatching. Incident raised automatically. | health_check fails 3x |
| Updating | Adapter version update in progress. Platform pauses dispatching. Returns to Active when complete. | Admin initiates update |

---

### 🟣 Odoo Adapter
`XML-RPC` `ORM` `module-system` `multi-instance`

Reference implementation. Bridges the platform's generic commands to Odoo's specific APIs. All complexity of Odoo's deployment model is hidden behind the standard adapter interface.

| Sub-component | Type | Description |
|---|---|---|
| OdooRPCClient | client | Wraps Odoo XML-RPC API. Handles auth (uid + db), method dispatch, error translation. Used for: res.users, ir.module, base.automation. |
| ORM Bridge | service | Maps platform schema operations to Odoo ORM calls. Used for: reading module states, querying configuration values, checking DB integrity. |
| ModuleInstaller | tool | Implements install_extension() using Odoo's module system. Handles: module download from app store or private registry, dependency resolution, install, upgrade. |
| LogNormalizer | service | Odoo logs are multi-format (PostgreSQL logs, Python logs, Nginx access logs). Normalizes them into platform's structured JSON schema. |
| MetricsAdapter | service | Odoo doesn't natively expose Prometheus metrics. This sidecar scrapes Odoo's /web/database/manager and internal APIs to synthesize metrics. |
| ConfigManager | service | Maps set_config() to Odoo's ir.config_parameter model for system-level settings and res.company for company-level settings. |
| DBBackupRestore | tool | Implements create_snapshot() / restore_snapshot() using Odoo's /web/database/backup and /web/database/restore endpoints + pg_dump. |
| UpgradeCompatCheck | service | Before deploying a new module version, checks Odoo version compatibility, dependency conflicts, and migration script existence. |

**Odoo Environment Topology**

| Env | Database | Description |
|---|---|---|
| DEV | odoo_dev | Developer Agent's working environment. Ephemeral — spun up per task branch. Seeded with anonymized copy of staging data. |
| STAGING | odoo_staging | QA Agent and human preview environment. Full data clone from last production backup, anonymized. Persistent between deploys. |
| PROD | odoo_prod | Live environment. Multiple worker processes. HAProxy load balancer in front. Read replicas for reporting queries. |

---

### 🔧 Generic Application Adapter
`REST` `GraphQL` `Docker` `any-stack`

Targets custom-built applications — any app built using any stack. Assumes the app exposes standard interfaces: a REST/GraphQL API, a health endpoint, structured logs, and a CI/CD-compatible deploy process.

| Sub-component | Type | Description |
|---|---|---|
| AppManifest | config | YAML config the customer provides: API base URL, auth method, health endpoint path, log format, metrics endpoint, deploy command, rollback command. |
| REST/GraphQL Client | client | Generic HTTP client. Reads API contracts from AppManifest. Used for: health checks, config reads, status queries. Handles pagination and auth. |
| DockerDeployer | tool | Implements deploy() by pulling the new image tag and doing a rolling update of containers in the target environment. Supports compose and k8s manifests. |
| LogParser | service | Configurable log parser. Supports: JSON structured logs (direct), logfmt, Apache combined format, custom regex pattern. Normalizes to platform schema. |
| HealthProber | service | HTTP health checker. Polls configured health endpoint. Evaluates: status code, response time, optional JSON path assertion. |
| ShellRunner | tool | For apps without API-driven deployment. Executes configured shell commands for deploy, rollback, backup, restore. |
| MetricsScraper | service | Scrapes Prometheus /metrics endpoint if available. Falls back to: StatsD listener, custom metrics API, or derived metrics from logs. |
| MigrationRunner | tool | Executes database migration commands (Django migrate, Alembic, Flyway, Liquibase) as part of deploy. Rolls back migration on deploy failure. |

**Supported stacks (no adapter code changes required):**
Django/Python · Laravel/PHP · Rails/Ruby · Express/Node.js · Spring Boot/Java · FastAPI/Python · NestJS/TypeScript · Go (any framework) · Elixir/Phoenix · Docker Compose · Kubernetes/Helm · Any app with `/health` endpoint

---

### 🧩 Adapter SDK
`open-source` `typed` `validated` `publishable`

Enables any software vendor, partner, or customer to build and publish a first-class adapter. The platform becomes the operating system — the ecosystem builds the drivers.

| Sub-component | Type | Description |
|---|---|---|
| AdapterBaseClass | library | Base class (TypeScript/Python) that implements the boilerplate: auth, retry, logging, health reporting. Adapter author only implements the 11 interface methods. |
| SchemaTypes | library | Typed definitions for all platform request/response schemas. Enforced at compile time. |
| TestHarness | tool | CLI: `adapter-test --adapter ./my-adapter`. Runs the platform's full contract test suite against any adapter implementation. Pass = publishable. |
| LocalSimulator | tool | Spins up a local mock Platform Core. Enables full adapter development without a real platform instance. |
| AdapterRegistry | service | Hosted registry of published adapters (like npm / Docker Hub). Versioned. Signed. |
| ValidationPipeline | service | Before publishing: contract tests, security scan (no exfiltration, no undeclared network calls), performance benchmark. |
| AdapterDocs | service | Auto-generated documentation from the adapter's type signatures. Published alongside the adapter in the registry. |

**Author Workflow:**

| Step | Command | Description |
|---|---|---|
| 01 Scaffold | `adapter-sdk new my-app-adapter` | Generates typed project with all 11 methods stubbed |
| 02 Implement | — | Fill in each method. SDK handles auth, retry, logging. |
| 03 Test | `adapter-sdk test` | Runs full contract suite locally. |
| 04 Simulate | `adapter-sdk simulate` | Connects adapter to local Platform Core mock. End-to-end validation. |
| 05 Publish | `adapter-sdk publish` | Submits to registry. Auto-validation pipeline runs. On pass: available to all platform instances. |

---

## Layer 5 — Infrastructure Layer 🏗️

> **Design Principle:** The execution substrate — it runs whatever the Platform Core and Adapter Layer instruct it to run. The same four components exist in all deployment modes. Only the underlying cloud provider and ownership model changes. The platform code never changes; the infrastructure target does.

---

### Deployment Modes

**☁️ Shared Cloud** — *Startups / early stage*  
Platform manages all infrastructure on behalf of the customer. Fastest onboarding. Zero infra management.
- Namespace-isolated containers per customer — no data mixing
- Shared Kubernetes cluster with resource quotas per namespace
- Storage encrypted at rest, isolated per customer DB
- Egress to internet allowed; ingress via platform-managed TLS terminator
- *Constraints:* No custom networking rules · No dedicated compute · SLA: 99.5% uptime

**🖥️ Dedicated Server** — *Growth / scale-up*  
Customer gets a dedicated VM or bare-metal instance. Isolated compute. Platform still manages the software stack.
- Dedicated VM(s) provisioned in platform cloud or customer-preferred region
- Customer controls: instance type, CPU/RAM allocation, storage tier
- Private networking between services on the dedicated node
- Customer can install additional services (Redis, Elasticsearch, etc.)
- *Constraints:* Platform still manages OS/k8s layer · Single cloud provider region · SLA: 99.9% uptime

**🏢 Customer Cloud** — *Enterprise / full ownership*  
Customer brings their own cloud account (AWS, GCP, Azure, on-prem). Platform deploys the full stack into the customer's environment. Customer owns everything. Platform provides AI agent layer only.
- Customer provides cloud account credentials (IAM role, service principal)
- Platform provisions full stack via Terraform/Pulumi into customer's cloud
- Customer retains full cloud account ownership — no lock-in
- Multi-region, multi-AZ deployment supported
- VPC, subnets, security groups, and IAM all in customer's account
- Platform agent connects via secure outbound tunnel (no inbound firewall rules)
- *Constraints:* Customer manages cloud billing · Customer responsible for cloud-level compliance · SLA: 99.99% (enterprise contract)

---

### 🏗️ Environment Manager
`lifecycle` `isolation` `namespace`

Primary abstraction between the platform and the underlying compute provider. Never speaks Kubernetes directly — it speaks environments. The Container Runtime translates environments into actual k8s namespaces.

| Sub-component | Type | Description |
|---|---|---|
| EnvironmentProvisioner | service | Creates isolated environments on demand: namespace, resource quota, network policy, service accounts, secrets scope. Supports: dev (ephemeral), staging (persistent), prod (HA). |
| EnvironmentRegistry | storage | Source of truth for all environments: owner, app, created_at, resource allocation, current state, linked snapshots. |
| ResourceQuotaEnforcer | service | Enforces CPU, memory, storage, and network limits per environment. Tied to customer's subscription tier. |
| EnvironmentSeeder | tool | Populates a new DEV or STAGING environment with seed data: anonymized copy of production DB, fixture data, or blank schema. |
| TTLManager | service | Manages ephemeral environment cleanup. DEV environments for completed tasks are destroyed after configurable TTL (default: 24h after task closes). |
| EnvironmentCloner | tool | Creates an exact copy of an environment for: hotfix branches, A/B testing, parallel feature development. |

**Data In:** `platform.env_create_requested` · `platform.env_destroy_requested` · `pipeline.snapshot_create`  
**Data Out:** `env.created` · `env.destroyed` · `env.snapshot_ready` · `infra.quota_exceeded`

---

### 🐳 Container Runtime
`Kubernetes` `Docker` `scaling` `service-mesh`

The only component that directly modifies running workloads. The execution boundary — anything above it is coordination; anything here is physical compute. All mutations are logged to the immutable audit log.

| Sub-component | Type | Description |
|---|---|---|
| K8sOperator | service | Custom Kubernetes operator that watches for platform EnvironmentResource CRDs and reconciles the desired state: pods, services, ingress, PVCs. |
| ImageRegistry | service | Private container image registry. Images signed and scanned for CVEs before allowed to run. |
| HorizontalScaler | service | Kubernetes HPA configured per service. Scales pod count based on CPU utilization, memory, and custom metrics. Called by DevOps Agent. |
| ServiceMesh | service | Sidecar-based service mesh (Istio / Linkerd). Provides: mTLS between services, request tracing, traffic shaping for canary deployments, circuit breaker. |
| HealthController | service | Manages liveness and readiness probes. Restarts unhealthy pods. Reports health state to Observability Engine. |
| ResourceGovernor | service | Enforces container resource requests and limits. Prevents OOM kills from cascading. Reports resource pressure to DevOps Agent. |

---

### 💾 Storage & State
`DB` `object-storage` `vault` `backup`

Most critical layer for data integrity. Each environment has fully isolated storage — there is no shared state between DEV, STAGING, and PROD at any layer. Cross-environment data flows only through the controlled EnvironmentSeeder.

| Sub-component | Type | Description |
|---|---|---|
| RelationalDB | storage | PostgreSQL primary + streaming replicas per environment. Read replica for analytics. PgBouncer connection pooling. Per-environment isolated DB instance. |
| ObjectStorage | storage | S3-compatible object storage for: file uploads, DB backups, container image layers, log archives, deployment artifacts. Bucket-per-environment isolation. |
| CacheLayer | storage | Redis cluster per environment. Used for: session storage, rate limiting, job queues, frequently-read data. |
| SecretsStore | storage | HashiCorp Vault (or cloud-native equivalent). Short-lived dynamic credentials. Automatic rotation. Audit log on every access. |
| BackupScheduler | service | Daily automated DB snapshots. Retained for: 7 daily, 4 weekly, 12 monthly. Encrypted at rest and in transit. Restoration tested automatically monthly. |
| MigrationTracker | service | Tracks applied DB migrations per environment. Prevents duplicate runs. Enables safe rollback by tracking reversible migrations. |

---

### 🌐 Network & Security
`VPC` `WAF` `TLS` `zero-trust`

Enforces zero-trust internally. Every service must authenticate to every other service. No implicit trust based on being "inside the cluster." A compromised adapter cannot pivot to the platform core — lateral movement is blocked at the network level.

| Sub-component | Type | Description |
|---|---|---|
| IngressController | service | Nginx/Traefik ingress. TLS termination with auto-renewed certs (Let's Encrypt or custom CA). Routes traffic to correct environment by subdomain or path. |
| WAF | service | Web Application Firewall. Rules for: OWASP Top 10, rate limiting per IP, geo-blocking (optional), bot detection. Blocks before traffic reaches app. |
| NetworkPolicies | config | Kubernetes NetworkPolicy rules. Default: deny-all. Explicitly allow: app → DB, app → cache, app → external APIs. No lateral movement between environments. |
| mTLSLayer | service | Service mesh enforces mTLS for all east-west traffic. No unencrypted traffic inside the cluster. |
| DDoSProtection | service | Layer 3/4 volumetric attack mitigation at cloud edge. Layer 7 application-level rate limiting within ingress. Auto-mitigates; alerts human on sustained attack. |
| VulnScanner | service | Continuous CVE scanning of: container images (on push), OS packages (weekly), application dependencies (on deploy). Blocks deploy if critical CVE found. |
| AuditNetworkLog | service | Captures all inbound and outbound network flows. Stored in Observability Engine. Used for: security incidents, compliance, forensics. |

---

### Autoscaling State Machine

```
Steady → Pressure → ScaleOut → Validating → Stable
                                           ↓
                              ScaleIn  ←──┘
                                           
           CostAlert (exception) · Degraded (exception)
```

| State | Description | Trigger |
|---|---|---|
| Steady | All services within resource bounds. DevOps Agent monitoring passively. | All metrics within thresholds |
| Pressure | One or more metrics approaching threshold (>70% utilization for 2min). | Metric crosses warning threshold |
| ScaleOut | DevOps Agent determined: add more pod replicas. HPA scale-up executing. | Scale-out decision made |
| Validating | New pods running. DevOps Agent monitoring recovery. 3-minute validation window. | Pods scheduled and starting |
| Stable | Metrics recovered. New pods healthy and serving traffic. | Metrics return to normal |
| ScaleIn | Traffic reduced for >15min below threshold. Gracefully terminating excess pods. | Low utilization sustained |
| CostAlert | Monthly projected spend exceeding configured budget. DevOps Agent pauses further scale-outs. Human notified. | Cost projection threshold crossed |
| Degraded | Scaling did not resolve pressure. Multiple pods failing health checks. Human P1 alert raised. | Validation fails after scale-out |

---

### Cloud Provider Compatibility

| Component | AWS | GCP | Azure | Self-hosted |
|---|---|---|---|---|
| Container Runtime | EKS | GKE | AKS | k3s / k8s |
| Relational DB | RDS Postgres | Cloud SQL | Azure Postgres | Postgres + PgBouncer |
| Object Storage | S3 | Cloud Storage | Blob Storage | MinIO |
| Cache | ElastiCache | Memorystore | Azure Cache | Redis OSS |
| Secrets | Secrets Manager | Secret Manager | Key Vault | HashiCorp Vault |
| Container Registry | ECR | Artifact Registry | ACR | Harbor |
| Load Balancer | ALB | Cloud LB | App Gateway | Nginx / Traefik |
| DNS / TLS | Route53 / ACM | Cloud DNS | Azure DNS | CoreDNS / cert-manager |

---

## Architectural Invariants

These rules hold at every layer, always, without exception:

1. **Agents never call each other directly.** All inter-agent communication is via Event Bus events.
2. **No code reaches production without passing every pipeline gate.** Gates cannot be bypassed by anyone, including admins.
3. **Production deploys require explicit human authorization.** The `pipeline.deploy_authorized` event can only originate from an authenticated human session.
4. **Agents operate with minimum-privilege credentials** scoped to their role and environment, issued fresh per session by the Secrets Vault.
5. **The Adapter Layer is the only product-specific code.** Everything above and below it is application-agnostic.
6. **Every action is append-only logged.** Agent actions, human actions, and infrastructure mutations all produce immutable audit records.
7. **No shared state exists between environments.** DEV, STAGING, and PROD are fully isolated at network, storage, and credentials level.
