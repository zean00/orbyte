// Flow runtime, record editors, and shell bootstrap orchestration.
    function flowDraftStateKey(flowKey, targetID) {
      return flowKey + '::' + (targetID || 'new');
    }

    function flowDraftData(flow, targetID) {
      const current = state.flowTabs[flowDraftStateKey(flow.key, targetID)] || {};
      return {
        documents: Object.assign({}, current.documents || {}),
        step_index: current.step_index || 0,
        active_document_key: current.active_document_key || ''
      };
    }

    async function loadFlowDraft(flowKey, targetID) {
      const saved = await loadDraft('document_flow', flowKey, targetID || '');
      if (saved) state.flowTabs[flowDraftStateKey(flowKey, targetID)] = saved;
      return flowDraftData({key: flowKey}, targetID);
    }

    async function saveFlowDraft(flowKey, targetID, draft) {
      state.flowTabs[flowDraftStateKey(flowKey, targetID)] = draft;
      await saveDraft('document_flow', flowKey, targetID || '', draft);
    }

    function flowContext(draft) {
      const documents = {};
      Object.keys(draft.documents || {}).forEach((key) => {
        documents[key] = {payload: (draft.documents[key] && draft.documents[key].payload) || {}};
      });
      return {documents};
    }

    function flowPathValue(payload, path) {
      return String(path || '').split('.').reduce((current, key) => current && current[key] != null ? current[key] : '', payload);
    }

    function flowRuleMatches(rule, draft) {
      const value = flowPathValue(flowContext(draft), rule.path);
      if (rule.truthy) return !!value && String(value).toLowerCase() !== 'false';
      if (rule.equals !== undefined && rule.equals !== '') return String(value) === String(rule.equals);
      if (Array.isArray(rule.in) && rule.in.length) return rule.in.map(String).includes(String(value));
      return false;
    }

    function resolveFlowSequence(flow, draft) {
      const steps = flow.steps || [];
      if (!steps.length) return [];
      const stepMap = {};
      steps.forEach((step) => { stepMap[step.key] = step; });
      const sequence = [];
      const seen = {};
      let current = steps[0];
      while (current && !seen[current.key]) {
        seen[current.key] = true;
        sequence.push(current);
        let next = '';
        for (const rule of (current.next_rules || [])) {
          if (flowRuleMatches(rule, draft)) {
            next = rule.next_step_key;
            break;
          }
        }
        if (!next) next = current.next_step_key || '';
        current = next ? stepMap[next] : null;
      }
      return sequence;
    }

    function flowDocumentRecord(docDef, draft) {
      return {
        header: {type: docDef.document_type, status: 'draft'},
        body: {payload: ((draft.documents || {})[docDef.key] && draft.documents[docDef.key].payload) || {}}
      };
    }

    function flowDocumentFields(docDef) {
      const fields = [];
      (docDef.fields || []).forEach((field) => fields.push(field));
      (docDef.sections || []).forEach((section) => {
        (section.fields || []).forEach((field) => fields.push(field));
      });
      (docDef.tabs || []).forEach((tab) => {
        (tab.sections || []).forEach((section) => {
          (section.fields || []).forEach((field) => fields.push(field));
        });
      });
      return fields;
    }

    function applyFlowDraftDefaults(flow, draft) {
      const next = JSON.parse(JSON.stringify(draft || {documents: {}}));
      let changed = false;
      (flow.steps || []).forEach((step) => {
        (step.documents || []).forEach((docDef) => {
          next.documents[docDef.key] = next.documents[docDef.key] || {payload: {}};
          const payload = next.documents[docDef.key].payload || {};
          flowDocumentFields(docDef).forEach((field) => {
            const normalizedPath = String(field.path || '').replace(/^body\.payload\./, '');
            if (!normalizedPath) return;
            const current = resolvePath(payload, normalizedPath);
            if ((current === '' || current == null) && (field.options || []).length > 0) {
              assignPath(payload, normalizedPath, field.options[0]);
              changed = true;
            }
          });
          next.documents[docDef.key].payload = payload;
        });
      });
      return {draft: next, changed};
    }

    function renderFlowDocumentDefinition(docDef, draft) {
      const record = flowDocumentRecord(docDef, draft);
      if ((docDef.tabs || []).length > 0) {
        return (docDef.tabs || []).map((tab) => {
          const sections = (tab.sections || []).map((section) => renderFormSection(section, record)).join('');
          return '<section class="panel"><h3>' + escapeHTML(pickText(tab, 'title')) + '</h3>' + sections + '</section>';
        }).join('');
      }
      if ((docDef.sections || []).length > 0) {
        return (docDef.sections || []).map((section) => renderFormSection(section, record)).join('');
      }
      return '<div class="form-grid">' + (docDef.fields || []).map((field) => renderEditableField(field, record)).join('') + '</div>';
    }

    function collectFlowDraft(root, draft) {
      const next = JSON.parse(JSON.stringify(draft || {documents: {}}));
      root.querySelectorAll('[data-flow-doc-key]').forEach((section) => {
        const docKey = section.dataset.flowDocKey;
        next.documents[docKey] = next.documents[docKey] || {payload: {}};
        const payload = {};
        section.querySelectorAll('[data-path]').forEach((input) => {
          assignPath(payload, input.dataset.path.replace(/^body\.payload\./, ''), readFieldValue(input));
        });
        next.documents[docKey].payload = payload;
      });
      return next;
    }

    function flowDraftFromInstance(instance) {
      const documents = {};
      (instance.items || []).forEach((item) => {
        documents[item.definition.key] = {
          payload: (((item.record || {}).body || {}).payload) || {}
        };
      });
      return {
        documents,
        step_index: 0,
        active_document_key: instance.active_document_key || ''
      };
    }

    function flowStepIndexForDocumentKey(steps, documentKey) {
      if (!documentKey) return 0;
      for (let index = 0; index < (steps || []).length; index += 1) {
        if (((steps[index].documents || []).some((item) => item.key === documentKey))) return index;
      }
      return 0;
    }

    async function renderFlow(route) {
      const root = document.getElementById('view-root');
      const flow = route.flow;
      if (!flow) {
        renderJSONCard(t('view_unavailable'), route);
        return;
      }
      const flowDocumentID = currentParams().get('id') || '';
      const activeDocumentParam = currentParams().get('document_key') || '';
      let draft = await loadFlowDraft(flow.key, flowDocumentID);
      if (flowDocumentID) {
        try {
          const payload = await api('/ui/data/documents/' + encodeURIComponent(flowDocumentID));
          if (payload.flow_instance && payload.flow_instance.flow && payload.flow_instance.flow.key === flow.key) {
            draft = flowDraftFromInstance(payload.flow_instance);
            if (activeDocumentParam) draft.active_document_key = activeDocumentParam;
          }
        } catch (_) {}
      }
      const seeded = applyFlowDraftDefaults(flow, draft);
      draft = seeded.draft;
      if (seeded.changed) {
        await saveFlowDraft(flow.key, flowDocumentID, draft);
      }
      const steps = resolveFlowSequence(flow, draft);
      if (!steps.length) {
        root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(flow, 'title')) + '</h3><p class="status">' + escapeHTML(t('view_unavailable')) + '</p></div></div></section>';
        return;
      }
      let stepIndex = Math.min((activeDocumentParam ? flowStepIndexForDocumentKey(steps, activeDocumentParam) : (draft.step_index || 0)), steps.length - 1);
      const step = steps[stepIndex];
      const activeDocKey = draft.active_document_key && (step.documents || []).some((item) => item.key === draft.active_document_key)
        ? draft.active_document_key
        : (activeDocumentParam && (step.documents || []).some((item) => item.key === activeDocumentParam)
          ? activeDocumentParam
          : (((step.documents || [])[0] && step.documents[0].key) || ''));
      const stepRail = steps.map((item, index) => '<button type="button" class="' + (index === stepIndex ? '' : 'secondary') + '" data-flow-step="' + index + '">' + escapeHTML(pickText(item, 'title')) + '</button>').join('');
      const tabBar = (step.documents || []).length > 1
        ? '<div class="toolbar-row">' + (step.documents || []).map((item) => '<button type="button" class="' + (item.key === activeDocKey ? '' : 'secondary') + '" data-flow-tab="' + escapeHTML(item.key) + '">' + escapeHTML(pickText(item, 'title')) + '</button>').join('') + '</div>'
        : '';
      const panels = (step.documents || []).map((docDef) => {
        const hidden = docDef.key === activeDocKey ? '' : ' hidden';
        return '<section class="section-block' + hidden + '" data-flow-doc-key="' + escapeHTML(docDef.key) + '"><div class="section-head"><h3>' + escapeHTML(pickText(docDef, 'title')) + '</h3></div><div class="section-body">' + renderFlowDocumentDefinition(docDef, draft) + '</div></section>';
      }).join('');
      const isLast = stepIndex >= steps.length - 1;
      root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(flow, 'title')) + '</h3><p class="status">' + escapeHTML(pickText(step, 'title')) + '</p></div></div><div class="page-body"><div class="toolbar-row">' + stepRail + '</div>' + tabBar + '<div class="section-stack">' + panels + '</div><p class="status" id="flow-status"></p></div><div class="page-actions">' +
        '<button type="button" id="flow-back" class="secondary"' + (stepIndex === 0 ? ' disabled' : '') + '>' + escapeHTML(t('previous')) + '</button>' +
        '<button type="button" id="flow-save-local" class="secondary">' + escapeHTML(t('save_local')) + '</button>' +
        '<button type="button" id="flow-next">' + escapeHTML(isLast ? (flowDocumentID ? t('save') : t('create')) : t('next')) + '</button>' +
        '</div></section>';

      root.querySelectorAll('[data-flow-step]').forEach((button) => {
        button.addEventListener('click', async () => {
          draft = collectFlowDraft(root, draft);
          draft.step_index = parseInt(button.dataset.flowStep || '0', 10);
          draft.active_document_key = activeDocKey;
          await saveFlowDraft(flow.key, flowDocumentID, draft);
          await renderFlow(route);
        });
      });
      root.querySelectorAll('[data-flow-tab]').forEach((button) => {
        button.addEventListener('click', async () => {
          draft = collectFlowDraft(root, draft);
          draft.step_index = stepIndex;
          draft.active_document_key = button.dataset.flowTab || '';
          await saveFlowDraft(flow.key, flowDocumentID, draft);
          await renderFlow(route);
        });
      });
      root.querySelectorAll('[data-flow-doc-key] [data-path]').forEach((input) => {
        input.addEventListener('change', async () => {
          const updated = collectFlowDraft(root, draft);
          updated.step_index = stepIndex;
          updated.active_document_key = activeDocKey;
          draft = updated;
          await saveFlowDraft(flow.key, flowDocumentID, updated);
        });
      });
      const back = root.querySelector('#flow-back');
      if (back) {
        back.addEventListener('click', async () => {
          draft = collectFlowDraft(root, draft);
          draft.step_index = Math.max(0, stepIndex - 1);
          draft.active_document_key = activeDocKey;
          await saveFlowDraft(flow.key, flowDocumentID, draft);
          await renderFlow(route);
        });
      }
      const saveLocal = root.querySelector('#flow-save-local');
      if (saveLocal) {
        saveLocal.addEventListener('click', async () => {
          draft = collectFlowDraft(root, draft);
          draft.step_index = stepIndex;
          draft.active_document_key = activeDocKey;
          await saveFlowDraft(flow.key, flowDocumentID, draft);
          document.getElementById('flow-status').textContent = t('draft_saved_local');
          setStatus(t('draft_saved_local'));
        });
      }
      const next = root.querySelector('#flow-next');
      if (next) {
        next.addEventListener('click', async () => {
          draft = collectFlowDraft(root, draft);
          draft.step_index = stepIndex;
          draft.active_document_key = activeDocKey;
          await saveFlowDraft(flow.key, flowDocumentID, draft);
          const updatedSteps = resolveFlowSequence(flow, draft);
          const hasNext = stepIndex < updatedSteps.length - 1;
          if (hasNext) {
            draft.step_index = stepIndex + 1;
            await saveFlowDraft(flow.key, flowDocumentID, draft);
            await renderFlow(route);
            return;
          }
          try {
            const payload = await api('/document-flows/' + encodeURIComponent(flow.key) + '/commit', {
              method: 'POST',
              headers: {'Content-Type': 'application/json', 'X-CSRF-Token': readCookie('orbyte_csrf')},
              body: JSON.stringify({
                organization_id: 'org_default',
                primary_document_id: flowDocumentID || '',
                documents: Object.keys(draft.documents || {}).reduce((result, key) => {
                  result[key] = (draft.documents[key] && draft.documents[key].payload) || {};
                  return result;
                }, {})
              })
            });
            await idbDelete('drafts', draftKey('document_flow', flow.key, flowDocumentID || ''));
            delete state.flowTabs[flowDraftStateKey(flow.key, flowDocumentID)];
            const detailRoute = routeForDocument(payload.primary_document_type, 'detail');
            if (detailRoute && payload.primary_document_id) {
              window.location.hash = '#' + detailRoute + '?id=' + encodeURIComponent(payload.primary_document_id);
              return;
            }
            document.getElementById('flow-status').textContent = t('record_created');
            setStatus(t('record_created'));
          } catch (err) {
            document.getElementById('flow-status').textContent = err.message;
            setStatus(err.message);
          }
        });
      }
    }

    async function renderCustom(route) {
      const root = document.getElementById('view-root');
      root.innerHTML = '<section class="panel"><h3>' + escapeHTML(t('custom_loading')) + '</h3></section>';
      const entry = route.custom_entry;
      const printSupport = await resolveCustomPrintSupport(entry);
      const bundle = await loadBundle(entry.bundle_key);
      const renderFn = bundle[entry.component_export];
      if (typeof renderFn !== 'function') throw new Error('module component export not found');
      const mount = document.getElementById('view-root');
      mount.innerHTML = '';
      await renderFn({
        mount,
        route,
        api,
        params: Object.fromEntries(currentParams().entries()),
        t,
        locale: state.locale,
        print: {
          resolved: !!printSupport.resolved,
          definition: printSupport.definition || null,
          version: printSupport.version || null,
          preview: async function(extra) {
            if (!printSupport.resolved) return;
            await previewTemplateOutput(Object.assign({
              target_kind: entry.print_target_kind,
              target_key: entry.print_target_key,
              purpose: entry.print_purpose || '',
              channel: entry.print_channel || 'print',
              sample: false
            }, extra || {}));
          },
          open: async function(extra) {
            if (!printSupport.resolved) return;
            const payload = await api('/outputs/render', {
              method: 'POST',
              headers: {'Content-Type': 'application/json', 'X-CSRF-Token': readCookie('orbyte_csrf')},
              body: JSON.stringify(Object.assign({
                target_kind: entry.print_target_kind,
                target_key: entry.print_target_key,
                purpose: entry.print_purpose || '',
                channel: entry.print_channel || 'print',
                format: 'html'
              }, extra || {}))
            });
            openPrintWindow(payload.output);
          },
          downloadPDF: async function(extra) {
            if (!printSupport.resolved) return;
            await downloadTemplatePDF(Object.assign({
              target_kind: entry.print_target_kind,
              target_key: entry.print_target_key,
              purpose: entry.print_purpose || '',
              channel: entry.print_channel || 'print'
            }, extra || {}));
          }
        }
      });
    }

    function activeAgentSession() {
      return (state.agent.sessions || []).find((item) => item.id === state.agent.currentSessionId) || null;
    }

    function resetAgentState() {
      stopAgentStream();
      state.agent.open = false;
      state.agent.sessions = [];
      state.agent.currentSessionId = '';
      state.agent.stream = null;
    }

    function buildAgentContextBlocks() {
      return [{
        key: 'current_page',
        label: 'Current Page',
        kind: 'route',
        selected: !!state.agent.attachContext,
        value: {
          shell: state.shellKind || 'workspace',
          route_path: currentPath() || (state.route && state.route.requested_path) || '',
          route_title: document.querySelector('#route-title h2') ? document.querySelector('#route-title h2').textContent : '',
          route_status: document.getElementById('route-status') ? document.getElementById('route-status').textContent : '',
          surface: currentSurface(),
          actor_user_id: state.bootstrap && state.bootstrap.auth_context ? state.bootstrap.auth_context.actor_user_id : ''
        }
      }];
    }

    async function loadAgentSessions() {
      try {
        const payload = await api('/agent/api/sessions');
        state.agent.sessions = payload.items || [];
        if (!state.agent.currentSessionId && state.agent.sessions.length) state.agent.currentSessionId = state.agent.sessions[0].id;
      } catch (_) {
        state.agent.sessions = [];
      }
    }

    function stopAgentStream() {
      if (state.agent.stream) {
        state.agent.stream.close();
        state.agent.stream = null;
      }
    }

    async function refreshAgentSession(sessionID) {
      if (!sessionID) return;
      try {
        const session = await api('/agent/api/sessions/' + encodeURIComponent(sessionID));
        const idx = state.agent.sessions.findIndex((item) => item.id === session.id);
        if (idx >= 0) state.agent.sessions[idx] = session; else state.agent.sessions.unshift(session);
      } catch (_) {}
    }

    function ensureAgentStream(sessionID) {
      if (!sessionID) return;
      if (state.agent.stream && state.agent.stream.url.indexOf(encodeURIComponent(sessionID)) >= 0) return;
      stopAgentStream();
      const stream = new EventSource('/agent/api/sessions/' + encodeURIComponent(sessionID) + '/events');
      stream.addEventListener('session_update', () => { void refreshAgentSession(sessionID).then(renderAgentPanel); });
      stream.addEventListener('turn_completed', () => { void refreshAgentSession(sessionID).then(renderAgentPanel); });
      stream.addEventListener('turn_failed', () => { void refreshAgentSession(sessionID).then(renderAgentPanel); });
      stream.addEventListener('session_started', () => { void refreshAgentSession(sessionID).then(renderAgentPanel); });
      state.agent.stream = stream;
    }

    async function startAgentSession() {
      const providers = (((state.bootstrap || {}).acp || {}).providers || []).filter((item) => item.available);
      if (!providers.length) return null;
      const session = await api('/agent/api/sessions', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          provider_key: providers[0].key,
          shell: 'workspace',
          route_path: currentPath() || ((state.route && state.route.requested_path) || ''),
          title: document.querySelector('#route-title h2') ? document.querySelector('#route-title h2').textContent : 'Workspace Agent',
          context_blocks: buildAgentContextBlocks()
        })
      });
      state.agent.currentSessionId = session.id;
      await loadAgentSessions();
      ensureAgentStream(session.id);
      return session;
    }

    async function sendAgentPrompt() {
      const composer = document.getElementById('agent-composer-input');
      if (!composer) return;
      const content = String(composer.value || '').trim();
      if (!content) return;
      let sessionID = state.agent.currentSessionId;
      if (!sessionID) {
        const created = await startAgentSession();
        sessionID = created && created.id;
      }
      if (!sessionID) return;
      composer.value = '';
      await api('/agent/api/sessions/' + encodeURIComponent(sessionID) + '/prompt', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({content, context_blocks: buildAgentContextBlocks()})
      });
      await refreshAgentSession(sessionID);
      renderAgentPanel();
    }

    function renderAgentWorkspace(root) {
      const session = activeAgentSession();
      const sessionsMarkup = (state.agent.sessions || []).map((item) => {
        const classes = item.id === state.agent.currentSessionId ? 'agent-session-item active' : 'agent-session-item';
        return '<button type="button" class="' + classes + '" data-agent-session="' + escapeHTML(item.id) + '"><div class="meta">' + escapeHTML(item.provider_name || item.provider_key) + '</div><strong>' + escapeHTML(item.title || item.route_path || item.id) + '</strong><div class="status">' + escapeHTML(item.status || 'ready') + '</div></button>';
      }).join('') || '<p class="status">No agent sessions yet.</p>';
      const threadMarkup = session && session.messages && session.messages.length
        ? session.messages.map((item) => '<article class="agent-bubble ' + escapeHTML(item.role) + '"><div class="meta">' + escapeHTML(item.role) + '</div><div>' + escapeHTML(item.content || '') + '</div></article>').join('')
        : '<p class="status">Start a session to interact with an ACP agent.</p>';
      root.innerHTML = '<section class="agent-shell-page"><div class="page-header"><div><h3>Agent Workspace</h3><p class="status">Review ACP sessions, prompts, and streamed responses.</p></div><div class="actions"><button type="button" id="agent-panel-open" class="secondary">Open Panel</button></div></div><div class="page-body"><div class="agent-workspace-grid"><section class="panel"><h3>Sessions</h3><div class="agent-session-list">' + sessionsMarkup + '</div></section><section class="panel"><h3>Conversation</h3><div class="agent-thread">' + threadMarkup + '</div></section></div></div></section>';
      enhanceControlAccessibility(root);
      document.querySelectorAll('[data-agent-session]').forEach((node) => {
        node.addEventListener('click', () => {
          state.agent.currentSessionId = node.dataset.agentSession;
          ensureAgentStream(state.agent.currentSessionId);
          void refreshAgentSession(state.agent.currentSessionId).then(() => renderAgentWorkspace(root));
        });
      });
      const panelOpen = document.getElementById('agent-panel-open');
      if (panelOpen) panelOpen.onclick = () => { state.agent.open = true; renderAgentPanel(); };
    }

    function renderAgentPanel() {
      const panel = document.getElementById('agent-panel');
      if (!panel) return;
      const acpInfo = (state.bootstrap && state.bootstrap.acp) || {enabled: false, providers: []};
      if (!state.agent.open) {
        panel.classList.add('hidden');
        return;
      }
      panel.classList.remove('hidden');
      const session = activeAgentSession();
      const contextBlocks = buildAgentContextBlocks();
      const providers = (acpInfo.providers || []).filter((item) => item.available);
      const sessionMarkup = (state.agent.sessions || []).slice(0, 5).map((item) => {
        const classes = item.id === state.agent.currentSessionId ? 'agent-session-item active' : 'agent-session-item';
        return '<button type="button" class="' + classes + '" data-agent-session="' + escapeHTML(item.id) + '">' + escapeHTML(item.title || item.route_path || item.id) + '</button>';
      }).join('');
      const threadMarkup = session && session.messages && session.messages.length
        ? session.messages.map((item) => '<article class="agent-bubble ' + escapeHTML(item.role) + '"><div class="meta">' + escapeHTML(item.role) + '</div><div>' + escapeHTML(item.content || '') + '</div></article>').join('')
        : '<p class="status">' + (providers.length ? 'No conversation yet.' : 'ACP is not configured for this deployment.') + '</p>';
      panel.innerHTML = '<div class="agent-panel-header"><div class="agent-panel-title"><strong>Agent</strong><span class="status">' + escapeHTML((session && session.provider_name) || (providers[0] && providers[0].name) || 'Unavailable') + '</span></div><div class="actions"><button type="button" id="agent-open-workspace" class="secondary">Workspace</button><button type="button" id="agent-close-panel" class="secondary">Close</button></div></div><div class="agent-context-list">' + contextBlocks.map((item) => '<label class="agent-context-item"><input type="checkbox" ' + (item.selected ? 'checked' : '') + ' id="agent-context-toggle"> <strong>' + escapeHTML(item.label) + '</strong><div class="status">' + escapeHTML(item.value.route_path || '') + '</div></label>').join('') + '</div><div class="agent-panel-body"><div class="agent-session-list">' + sessionMarkup + '</div><div class="agent-thread">' + threadMarkup + '</div></div><div class="agent-composer"><textarea id="agent-composer-input" placeholder="Ask the agent about this page or current task."></textarea><div class="agent-panel-actions"><button type="button" id="agent-send-button"' + (providers.length ? '' : ' disabled') + '>Send</button></div></div>';
      enhanceControlAccessibility(panel);
      const closeButton = document.getElementById('agent-close-panel');
      if (closeButton) closeButton.onclick = () => { state.agent.open = false; renderAgentPanel(); };
      const workspaceButton = document.getElementById('agent-open-workspace');
      if (workspaceButton) workspaceButton.onclick = () => { window.location.hash = '#/agent/workspace'; };
      const sendButton = document.getElementById('agent-send-button');
      if (sendButton) sendButton.onclick = () => { void sendAgentPrompt(); };
      const contextToggle = document.getElementById('agent-context-toggle');
      if (contextToggle) contextToggle.onchange = () => { state.agent.attachContext = !!contextToggle.checked; };
      document.querySelectorAll('[data-agent-session]').forEach((node) => {
        node.addEventListener('click', () => {
          state.agent.currentSessionId = node.dataset.agentSession;
          ensureAgentStream(state.agent.currentSessionId);
          void refreshAgentSession(state.agent.currentSessionId).then(renderAgentPanel);
        });
      });
    }

    async function bootstrapAgent() {
      state.agent.providers = ((((state.bootstrap || {}).acp || {}).providers) || []).filter((item) => item.available);
      await loadAgentSessions();
      const toggle = document.getElementById('agent-toggle-button');
      if (toggle) {
        toggle.hidden = !((state.bootstrap && state.bootstrap.acp && state.bootstrap.acp.enabled) || false);
        toggle.onclick = () => {
          state.agent.open = !state.agent.open;
          renderAgentPanel();
        };
      }
      if (state.agent.currentSessionId) ensureAgentStream(state.agent.currentSessionId);
      renderAgentPanel();
    }

    async function renderRoute() {
      if (!state.bootstrap) {
        applyShellLayout(false);
        return;
      }
      renderMenus();
      renderAgentPanel();
      if (currentPath() === '/notifications') {
        document.getElementById('route-title').innerHTML = '<h2>' + escapeHTML(t('notifications')) + '</h2>';
        setStatus(t('notifications_status'));
        updateRoutePanel({module_key: 'notification', render_mode: 'custom', view: {kind: 'worklist'}});
        await renderNotificationsWorkspace(document.getElementById('view-root'));
        return;
      }
      if (currentPath() === '/agent/workspace') {
        document.getElementById('route-title').innerHTML = '<h2>Agent Workspace</h2>';
        setStatus('ACP session history and contextual agent workspace.');
        updateRoutePanel({module_key: 'acp', render_mode: 'custom', view: {kind: 'dashboard'}});
        renderAgentWorkspace(document.getElementById('view-root'));
        renderAgentPanel();
        return;
      }
      const route = await resolveCurrentRoute();
      state.route = route;
      const path = route.requested_path || currentPath() || state.bootstrap.default_path;
      if (!path && route.status !== 'ok') {
        setStatus(t('no_routes'));
        document.getElementById('view-root').innerHTML = '';
        return;
      }
      if (route.status !== 'ok') {
        const recoveryActions = [];
        if (route.status === 'surface_mismatch') {
          recoveryActions.push({
            label: t('recovery_switch_surface'),
            onClick: async () => {
              const targetSurface = route.suggested_surface || currentSurface();
              const fallback = route.fallback_path || routeFallbackPath(targetSurface);
              await navigateToSurface(targetSurface, fallback);
            }
          });
        }
        if (route.fallback_path) {
          recoveryActions.push({
            label: t('recovery_go_default'),
            secondary: route.status === 'surface_mismatch',
            onClick: () => { window.location.hash = '#' + route.fallback_path; }
          });
        }
        if (navigator.onLine) {
          recoveryActions.push({
            label: t('recovery_retry'),
            secondary: true,
            onClick: () => { void renderRoute(); }
          });
        }
        if (!navigator.onLine) {
          recoveryActions.push({
            label: t('recovery_offline'),
            secondary: true,
            onClick: () => { void renderRoute(); }
          });
        }
        const titleKey = route.status === 'forbidden' ? 'route_forbidden' : (route.status === 'surface_mismatch' ? 'surface_mismatch' : 'route_not_found');
        document.getElementById('route-title').innerHTML = '<h2>' + escapeHTML(t(titleKey)) + '</h2>';
        setStatus(route.message || t(titleKey));
        updateRoutePanel(route);
        renderRecoveryPanel(t(titleKey), route.message || t(titleKey), recoveryActions);
        return;
      }
      document.getElementById('route-title').innerHTML = '<h2>' + escapeHTML(pickText(route.action, 'label') || route.path) + '</h2>';
      setStatus(t('resolved_from_module') + ' ' + route.module_key + ' ' + t('using_rendering') + ' ' + route.render_mode + ' rendering.');
      updateRoutePanel(route);
      setRouteState('route_ready', '#route-title h2');
      if (route.render_mode === 'custom') {
        await renderCustom(route);
        enhanceControlAccessibility(document.getElementById('view-root'));
        focusRecoveryTarget('#route-title h2');
        return;
      }
      if (route.render_mode === 'flow') {
        await renderFlow(route);
        renderMenus();
        enhanceControlAccessibility(document.getElementById('view-root'));
        focusRecoveryTarget('#route-title h2');
        return;
      }
      await renderGeneric(route);
      renderMenus();
      enhanceControlAccessibility(document.getElementById('view-root'));
      focusRecoveryTarget('#route-title h2');
    }

    async function renderNotificationsWorkspace(root) {
      const payload = await api('/ui/data/notifications');
      const items = payload.items || [];
      root.innerHTML = '<section class="page-panel floorplan-worklist"><div class="page-header"><div><div class="page-eyebrow">Inbox</div><h3>' + escapeHTML(t('notifications')) + '</h3><p class="status">' + escapeHTML(t('notifications_status')) + '</p></div><div class="page-header-actions"><span class="badge badge-subtle">items ' + escapeHTML(String(items.length)) + '</span></div></div><div class="page-body"><div class="table-shell"><table class="data-table"><thead><tr><th>Message</th><th>Status</th><th>Created</th><th></th></tr></thead><tbody>' + (items.map(function (item) {
        const target = (item.document && (item.document.number || item.document.id)) || item.target_id || '';
        return '<tr><td><div class="row-primary">' + escapeHTML(item.title || target || item.id) + '</div><div class="row-secondary">' + escapeHTML((item.body || '') + (target ? ' · ' + target : '')) + '</div></td><td>' + escapeHTML(displayValue(item.status || 'unread')) + '</td><td>' + escapeHTML(item.created_at || '') + '</td><td><div class="toolbar-row"><button type="button" class="secondary" data-notification-open="' + escapeHTML(item.id) + '">' + escapeHTML(t('open_link')) + '</button><button type="button" class="secondary" data-notification-read="' + escapeHTML(item.id) + '">' + escapeHTML(t('mark_read')) + '</button><button type="button" class="secondary" data-notification-dismiss="' + escapeHTML(item.id) + '">' + escapeHTML(t('dismiss')) + '</button></div></td></tr>';
      }).join('') || '<tr><td colspan="4"><div class="empty-state-inline">' + escapeHTML(t('notifications_empty')) + '</div></td></tr>') + '</tbody></table></div></div></section>';
      root.querySelectorAll('[data-notification-read]').forEach(function (button) {
        button.onclick = async function () {
          await api('/ui/data/notifications/' + encodeURIComponent(button.dataset.notificationRead || '') + '/actions/read', {method: 'POST', headers: {'X-CSRF-Token': readCookie('orbyte_csrf')}});
          await renderNotificationsWorkspace(root);
        };
      });
      root.querySelectorAll('[data-notification-dismiss]').forEach(function (button) {
        button.onclick = async function () {
          await api('/ui/data/notifications/' + encodeURIComponent(button.dataset.notificationDismiss || '') + '/actions/dismiss', {method: 'POST', headers: {'X-CSRF-Token': readCookie('orbyte_csrf')}});
          await renderNotificationsWorkspace(root);
        };
      });
      root.querySelectorAll('[data-notification-open]').forEach(function (button) {
        button.onclick = async function () {
          const item = items.find(function (entry) { return entry.id === (button.dataset.notificationOpen || ''); });
          if (!item) return;
          await api('/ui/data/notifications/' + encodeURIComponent(item.id) + '/actions/read', {method: 'POST', headers: {'X-CSRF-Token': readCookie('orbyte_csrf')}});
          const target = item.action_link_path || item.deep_link_path || '';
          if (!target) {
            await renderNotificationsWorkspace(root);
            return;
          }
          if (target.indexOf('/ui#') === 0 || target.indexOf('#/') === 0) {
            window.location.assign(target);
            return;
          }
          window.location.assign(target);
        };
      });
    }

    function readCookie(name) {
      return orbyteGetCookie(name);
    }

    function resolvePath(payload, path) {
      if (!path) return '';
      return path.split('.').reduce((current, key) => current && current[key] != null ? current[key] : '', payload);
    }

    function assignPath(target, path, value) {
      const parts = path.split('.');
      let current = target;
      while (parts.length > 1) {
        const key = parts.shift();
        current[key] = current[key] || {};
        current = current[key];
      }
      current[parts[0]] = value;
    }

    function readFieldValue(input) {
      if (input.type === 'checkbox') return !!input.checked;
      return input.value;
    }

    function renderFieldInput(field, value) {
      const readonly = field.read_only ? ' readonly disabled' : '';
      const current = value == null ? '' : value;
      const name = ' name="' + escapeHTML(String(field.path || field.key || 'field').replace(/[^a-zA-Z0-9_.-]+/g, '_')) + '"';
      if (field.widget === 'textarea') {
        return '<textarea data-path="' + field.path + '"' + name + readonly + ' placeholder="' + escapeHTML(pickText(field, 'placeholder')) + '">' + escapeHTML(String(current)) + '</textarea>';
      }
      if (field.widget === 'select' || (field.options || []).length > 0) {
        const options = (field.options || []).map((option) => '<option value="' + option + '"' + (String(current) === option ? ' selected' : '') + '>' + escapeHTML(displayValue(option)) + '</option>').join('');
        return '<select data-path="' + field.path + '"' + name + readonly + '>' + options + '</select>';
      }
      if (field.type === 'bool') {
        return '<input type="checkbox" data-path="' + field.path + '"' + name + (current ? ' checked' : '') + readonly + '>';
      }
      if (field.type === 'int' || field.type === 'number') {
        return '<input type="number" data-path="' + field.path + '"' + name + ' value="' + escapeHTML(String(current)) + '"' + readonly + ' placeholder="' + escapeHTML(pickText(field, 'placeholder')) + '">';
      }
      return '<input data-path="' + field.path + '"' + name + ' value="' + escapeHTML(String(current)) + '"' + readonly + ' placeholder="' + escapeHTML(pickText(field, 'placeholder')) + '">';
    }

    function renderRelatedView(def, payload, view) {
      const items = payload[def.source] || [];
      const relatedDef = payload.related_definitions ? payload.related_definitions[def.source] : null;
      const relation = (payload.definition && payload.definition.relations || []).find((item) => item.key === def.source);
      const createForm = relatedDef && relation ? renderRelatedCreateForm(def.source, relatedDef, relation) : '';
      const content = items.length ? '<div class="list">' + items.map((item) => {
        if (typeof item !== 'object' || item == null) {
          return '<article class="detail-item"><strong>' + escapeHTML(String(item)) + '</strong></article>';
        }
        const values = (item.record && item.record.values) || item.values || item;
        const entries = Object.keys(values).sort().slice(0, 6).map((key) => '<div><span class="meta">' + key + '</span><strong>' + escapeHTML(displayValue(values[key])) + '</strong></div>').join('');
        return '<article class="detail-item"><div class="kv">' + entries + '</div></article>';
      }).join('') + '</div>' : '<p class="status">' + escapeHTML(pickText(def, 'empty_state') || t('no_related_items')) + '</p>';
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(pickText(def, 'title')) + '</h3></div><div class="section-body">' + content + createForm + '</div></section>';
    }

    function renderSection(section, record) {
      const fields = (section.fields || []).map((field) => {
        return '<article class="detail-item"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span><strong>' + escapeHTML(displayValue(resolvePath(record, field.path))) + '</strong></article>';
      }).join('');
      const extensionModule = section.extension_slot_key || '';
      let extensionFields = '';
      if (extensionModule && record.body && record.body.payload && record.body.payload.extensions && record.body.payload.extensions[extensionModule]) {
        const ext = record.body.payload.extensions[extensionModule];
        extensionFields = Object.keys(ext).sort().map((key) => {
          return '<article class="detail-item"><span class="meta">' + extensionModule + '.' + key + '</span><strong>' + escapeHTML(displayValue(ext[key])) + '</strong></article>';
        }).join('');
      }
      return '<section class="section-block"><div class="section-head"><h4>' + escapeHTML(pickText(section, 'title')) + '</h4></div><div class="section-body"><div class="detail-grid">' + fields + extensionFields + '</div></div></section>';
    }

    function renderFlowReadonlyDocumentDefinition(docDef, record) {
      if ((docDef.tabs || []).length > 0) {
        return (docDef.tabs || []).map((tab) => {
          const sections = (tab.sections || []).map((section) => renderSection(section, record)).join('');
          return '<section class="panel"><h3>' + escapeHTML(pickText(tab, 'title')) + '</h3>' + sections + '</section>';
        }).join('');
      }
      if ((docDef.sections || []).length > 0) {
        return (docDef.sections || []).map((section) => renderSection(section, record)).join('');
      }
      const fields = (docDef.fields || []).map((field) => {
        return '<article class="detail-item"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span><strong>' + escapeHTML(displayValue(resolvePath(record, field.path))) + '</strong></article>';
      }).join('');
      return '<section class="section-block"><div class="section-head"><h4>' + escapeHTML(pickText(docDef, 'title')) + '</h4></div><div class="section-body"><div class="detail-grid">' + fields + '</div></div></section>';
    }

    function renderModelSection(section, record) {
      const fields = (section.fields || []).map((field) => {
        return '<article class="detail-item"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span><strong>' + escapeHTML(displayValue(resolvePath(record, field.path))) + '</strong></article>';
      }).join('');
      return '<section class="section-block"><div class="section-head"><h4>' + escapeHTML(pickText(section, 'title')) + '</h4></div><div class="section-body"><div class="detail-grid">' + fields + '</div></div></section>';
    }

    function renderEditableField(field, record) {
      const value = resolvePath(record, field.path);
      const helpText = pickText(field, 'help_text');
      return '<label class="form-field' + (((field.widget === 'textarea') || (field.type === 'json') || (field.type === 'text')) ? ' wide' : '') + '"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput(field, value) + (helpText ? '<span class="status">' + escapeHTML(helpText) + '</span>' : '') + '</label>';
    }

    function renderEditableModelField(field, record) {
      const value = resolvePath(record, field.path);
      const helpText = pickText(field, 'help_text');
      return '<label class="form-field' + (((field.widget === 'textarea') || (field.type === 'json') || (field.type === 'text')) ? ' wide' : '') + '"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput(field, value) + (helpText ? '<span class="status">' + escapeHTML(helpText) + '</span>' : '') + '</label>';
    }

    function renderFormSection(section, record) {
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(pickText(section, 'title')) + '</h3></div><div class="section-body"><div class="form-grid">' + (section.fields || []).map((field) => renderEditableField(field, record)).join('') + '</div></div></section>';
    }

    function renderModelFormSection(section, record) {
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(pickText(section, 'title')) + '</h3></div><div class="section-body"><div class="form-grid">' + (section.fields || []).map((field) => renderEditableModelField(field, record)).join('') + '</div></div></section>';
    }

    function renderRelationEditor(def, payload) {
      const relatedDef = payload.related_definitions ? payload.related_definitions[def.source] : null;
      const relation = (payload.definition && payload.definition.relations || []).find((item) => item.key === def.source);
      if (!relatedDef || !relation) return '';
      const rows = (payload[def.source] || []).map((item) => renderRelationRow(def.source, relatedDef, relation, item, payload.model_definitions || {})).join('');
      return '<section class="section-block" data-relation-editor="' + def.source + '" data-parent-model-key="' + escapeHTML(payload.definition.key || '') + '" data-target-model-key="' + escapeHTML(relatedDef.key || '') + '"><div class="section-head"><h3>' + escapeHTML(pickText(def, 'title')) + '</h3></div><div class="section-body"><div class="list" data-relation-list="' + def.source + '">' + (rows || '<p class="status">' + t('no_related_items_yet') + '</p>') + '</div><div class="actions"><button type="button" class="secondary" data-relation-add="' + def.source + '">' + t('add_row') + '</button></div></div></section>';
    }

    function deriveRelatedViews(definition) {
      const relations = definition && definition.relations ? definition.relations : [];
      return relations.map((relation) => ({
        key: relation.key,
        title: relation.key.replace(/_/g, ' '),
        source: relation.key,
        empty_state: t('no_related_items_yet')
      }));
    }

    function renderRelationRow(relationKey, relatedDef, relation, item, modelDefinitions) {
      const graphNode = item && item.record ? item : null;
      const record = graphNode ? graphNode.record : (item || {id: '', version: 0, values: {}});
      const values = record.values || {};
      const fields = (relatedDef.fields || []).filter((field) => field.key !== relation.foreign_key && !field.read_only).map((field) => {
        const enriched = {path: 'values.' + field.key, type: field.type, widget: field.widget, options: field.options || [], placeholder: pickText(field, 'placeholder') || '', help_text: pickText(field, 'help_text') || ''};
        return '<label class="form-field"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput(enriched, values[field.key]) + '</label>';
      }).join('');
      const nested = renderNestedRelationEditors(graphNode, relatedDef, modelDefinitions);
      return '<article class="detail-item" data-relation-row="' + relationKey + '" data-record-id="' + escapeHTML(record.id || '') + '" data-record-version="' + escapeHTML(String(record.version || 0)) + '" data-record-op="upsert"><div class="form-grid">' + fields + '</div>' + nested + '<div class="actions"><button type="button" class="secondary" data-relation-remove="' + relationKey + '">' + t('remove') + '</button></div></article>';
    }

    function renderNestedRelationEditors(graphNode, relatedDef, modelDefinitions) {
      const nestedRelations = relatedDef.relations || [];
      if (!nestedRelations.length) return '';
      const relatedMap = graphNode && graphNode.related ? graphNode.related : {};
      return nestedRelations.map((relation) => {
        const targetDef = modelDefinitions[relation.target_model_key];
        if (!targetDef) return '';
        const rows = (relatedMap[relation.key] || []).map((item) => renderRelationRow(relation.key, targetDef, relation, item, modelDefinitions)).join('');
        return '<section class="section-block" data-relation-editor="' + relation.key + '" data-parent-model-key="' + escapeHTML(relatedDef.key || '') + '" data-target-model-key="' + escapeHTML(targetDef.key || '') + '"><div class="section-head"><h4>' + relation.key.replace(/_/g, ' ') + '</h4></div><div class="section-body"><div class="list" data-relation-list="' + relation.key + '">' + (rows || '<p class="status">' + t('no_related_items_yet') + '</p>') + '</div><div class="actions"><button type="button" class="secondary" data-relation-add="' + relation.key + '">' + t('add_row') + '</button></div></div></section>';
      }).join('');
    }

    function appendRelationRow(relationKey, payload) {
      const editor = document.querySelector('[data-relation-editor="' + relationKey + '"]');
      const list = editor && editor.querySelector('[data-relation-list="' + relationKey + '"]');
      if (!editor || !list) return;
      const modelDefinitions = payload.model_definitions || {};
      const relatedDef = resolveRelatedDefinition(editor, relationKey, payload);
      const relation = resolveRelationDefinition(editor, relationKey, payload);
      if (!relatedDef || !relation) return;
      if (list.querySelector('.status')) list.innerHTML = '';
      list.insertAdjacentHTML('beforeend', renderRelationRow(relationKey, relatedDef, relation, null, modelDefinitions));
      bindRelationRemove(editor);
    }

    function resolveRelatedDefinition(editor, relationKey, payload) {
      const modelDefinitions = payload.model_definitions || {};
      const targetModelKey = editor && editor.dataset ? editor.dataset.targetModelKey : '';
      if (targetModelKey && modelDefinitions[targetModelKey]) return modelDefinitions[targetModelKey];
      if (payload.related_definitions && payload.related_definitions[relationKey]) return payload.related_definitions[relationKey];
      return modelDefinitions[relationKey] || null;
    }

    function resolveRelationDefinition(editor, relationKey, payload) {
      const modelDefinitions = payload.model_definitions || {};
      const parentModelKey = editor && editor.dataset ? editor.dataset.parentModelKey : '';
      const parentDefinition = (parentModelKey && modelDefinitions[parentModelKey]) || payload.definition || null;
      return findRelationDefinition(parentDefinition, relationKey, modelDefinitions);
    }

    function findRelationDefinition(definition, relationKey, modelDefinitions) {
      if (!definition) return null;
      const direct = (definition.relations || []).find((item) => item.key === relationKey);
      if (direct) return direct;
      for (const relation of (definition.relations || [])) {
        const nestedDef = modelDefinitions[relation.target_model_key];
        const nested = findRelationDefinition(nestedDef, relationKey, modelDefinitions);
        if (nested) return nested;
      }
      return null;
    }

    function directRelationEditors(root) {
      return Array.from(root.children || []).filter((child) => child.matches && child.matches('[data-relation-editor]'));
    }

    function bindRelationRemove(root) {
      root.querySelectorAll('[data-relation-remove]').forEach((button) => {
        button.onclick = () => {
          const row = button.closest('[data-relation-row]');
          if (row && row.dataset.recordId) {
            row.dataset.recordOp = 'delete';
            row.style.display = 'none';
          } else if (row) {
            row.remove();
          }
          const editor = button.closest('[data-relation-editor]');
          const relationKey = editor ? editor.dataset.relationEditor : '';
          const list = editor && editor.querySelector('[data-relation-list="' + relationKey + '"]');
          if (list && !Array.from(list.querySelectorAll('[data-relation-row]')).some((item) => item.dataset.recordOp !== 'delete')) {
            list.innerHTML = '<p class="status">' + t('no_related_items_yet') + '</p>';
          }
        };
      });
    }

    function collectRelationMutations(root) {
      const relations = {};
      directRelationEditors(root).forEach((editor) => {
        const relationKey = editor.dataset.relationEditor;
        const rows = [];
        Array.from(editor.querySelectorAll(':scope > [data-relation-list] > [data-relation-row]')).forEach((row) => {
          const op = row.dataset.recordOp || 'upsert';
          const values = {};
          row.querySelectorAll(':scope > label [data-path], :scope > .card > label [data-path]').forEach((input) => assignPath(values, input.dataset.path.replace(/^values\./, ''), readFieldValue(input)));
          const nested = collectRelationMutations(row);
          rows.push({
            operation: op,
            id: row.dataset.recordId || '',
            expected_version: parseInt(row.dataset.recordVersion || '0', 10) || 0,
            values: values,
            relations: nested
          });
        });
        if (rows.length > 0 || editor.querySelector('[data-relation-list]')) {
          relations[relationKey] = rows;
        }
      });
      return relations;
    }

    function renderRelatedCreateForm(sourceKey, relatedDef, relation) {
      const editableFields = (relatedDef.fields || []).filter((field) => field.key !== relation.foreign_key && !field.read_only);
      if (!editableFields.length) return '';
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(t('add')) + ' ' + escapeHTML(pickText(relatedDef, 'display_name') || relatedDef.key) + '</h3></div><div class="section-body"><div class="form-grid">' +
        editableFields.map((field) => '<label class="form-field"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput({path: 'values.' + field.key, type: field.type, widget: field.widget, options: field.options || [], placeholder: pickText(field, 'placeholder') || ''}, '') + '</label>').join('') +
        '</div><p class="status" data-related-status="' + sourceKey + '"></p><div class="actions"><button type="button" data-related-save="' + sourceKey + '">' + t('add') + '</button></div></div></section>';
    }

    function renderActionZones(view) {
      const placements = view.action_placements || [];
      const zones = {};
      placements.forEach((placement) => { zones[placement.zone] = true; });
      if (!zones.primary) zones.primary = true;
      if (!zones.secondary) zones.secondary = true;
      return Object.keys(zones).map((zone) => '<div class="actions" data-zone="' + zone + '"></div>').join('');
    }

    function resolveActionPlacement(view, actionKey, policyDecision) {
      const fromPolicy = policyDecision && policyDecision.output && policyDecision.output.placement;
      if (fromPolicy) return fromPolicy;
      const placement = (view.action_placements || []).find((item) => item.action_key === actionKey);
      return placement && placement.zone ? placement.zone : 'secondary';
    }

    async function bootstrap() {
      state.locale = detectPreferredLocale();
      try {
        const localePayload = await api('/locale');
        state.supportedLocales = localePayload.supported_locales || defaultSupportedLocales;
        state.locale = normalizeLocale(localePayload.locale || state.locale);
      } catch (_) {}
      renderLocaleSwitcher();
      applyLocale();
      renderRouteJumpOptions();
      refreshOfflineStatus();
      await registerServiceWorker();
      await refreshSyncStats();
      try {
        state.authOptions = await api('/auth/options');
        const session = await api('/auth/session');
        if (!session || !session.authenticated) {
          renderLocaleSwitcher();
          applyLocale();
          renderLogin(session && session.auth_error === 'session expired' ? t('session_expired') : loginSubtitle());
          return;
        }
        state.surface = currentSurface();
        state.bootstrap = await api(bootstrapURL());
        state.shellKind = state.bootstrap.shell_kind || 'workspace';
        state.surface = state.bootstrap.surface || state.surface;
        state.supportedLocales = state.bootstrap.supported_locales || defaultSupportedLocales;
        if (state.bootstrap.locale) {
          state.locale = normalizeLocale(state.bootstrap.locale);
        }
        renderLocaleSwitcher();
        applyLocale();
        renderRouteJumpOptions();
        await bootstrapAgent();
        await loadOfflineBootstrap();
        await processSyncQueue();
        if (!window.location.hash && state.bootstrap.default_path) {
          window.location.hash = '#' + state.bootstrap.default_path;
        }
        await renderRoute();
      } catch (err) {
        if (err.message === 'authentication required' || err.message === 'session not found' || err.message === 'session not active' || err.message === 'session revoked' || err.message === 'session expired') {
          renderLocaleSwitcher();
          applyLocale();
          renderLogin(t('session_expired'));
          return;
        }
        await loadOfflineBootstrap();
        renderRecoveryPanel(t('ui_bootstrap_failed'), err.message, [{label: t('recovery_retry'), onClick: () => { void bootstrap(); }}]);
        setStatus(t('ui_bootstrap_failed_status'));
      }
    }

    loadShellPrefs();
    bindWorkspaceShellControls();
    window.addEventListener('online', () => { refreshOfflineStatus(); void processSyncQueue(); });
    window.addEventListener('offline', () => { refreshOfflineStatus(); });
    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) void processSyncQueue();
    });
    window.addEventListener('hashchange', () => { void renderRoute(); });
    document.getElementById('logout-button').addEventListener('click', () => { void performLogout(); });
    bindGlobalShortcuts();
    void bootstrap();
