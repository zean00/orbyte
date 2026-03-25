// Authentication, menu routing, view rendering, and agent workspace.
    function renderLogin(message) {
      const root = document.getElementById('view-root');
      const passwordEnabled = !state.authOptions || !!state.authOptions.password_enabled;
      const googleEnabled = !!(state.authOptions && state.authOptions.google_enabled);
      const statusMessage = message || (authErrorFromQuery() === 'google_login_failed' ? 'Google sign-in failed. Try again or use a local account.' : loginSubtitle());
      applyShellLayout(false);
      document.getElementById('route-title').innerHTML = '<h2>' + escapeHTML(loginTitle()) + '</h2>';
      setStatus(statusMessage);
      document.getElementById('menu').innerHTML = '';
      document.getElementById('admin-link-button').hidden = true;
      document.getElementById('logout-button').hidden = true;
      if (document.getElementById('agent-toggle-button')) document.getElementById('agent-toggle-button').hidden = true;
      resetAgentState();
      renderAgentPanel();
      const passwordForm = passwordEnabled
        ? '<form id="login-form"><label class="field"><span class="meta">' + escapeHTML(t('username')) + '</span><input id="login-username" name="username" autocomplete="username"></label><label class="field"><span class="meta">' + escapeHTML(t('password')) + '</span><input id="login-password" name="password" type="password" autocomplete="current-password"></label><div class="actions"><button type="submit">' + escapeHTML(t('sign_in')) + '</button></div></form>'
        : '';
      const emptyState = !passwordEnabled && !googleEnabled
        ? '<p class="status">' + escapeHTML(t('sign_in_unavailable')) + '</p>'
        : '';
      const divider = passwordEnabled && googleEnabled ? '<div class="divider">' + escapeHTML(t('or')) + '</div>' : '';
      const googleAction = googleEnabled ? '<div class="actions"><button type="button" id="google-login" class="google">' + escapeHTML(googleButtonLabel()) + '</button></div>' : '';
      root.innerHTML = '<section class="login-shell"><div class="panel"><h3>' + escapeHTML(loginTitle()) + '</h3><p class="status">' + escapeHTML(statusMessage) + '</p>' + passwordForm + divider + googleAction + emptyState + '</div></section>';
      enhanceControlAccessibility(root);
      const form = document.getElementById('login-form');
      if (form) {
        form.addEventListener('submit', async (event) => {
          event.preventDefault();
          const username = document.getElementById('login-username').value.trim();
          const password = document.getElementById('login-password').value;
          try {
            await api('/auth/login', {
              method: 'POST',
              headers: {'Content-Type': 'application/json'},
              body: JSON.stringify({username, password})
            });
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
            const nextPath = requestedNextPath();
            if (nextPath) {
              window.location.assign(nextPath);
              return;
            }
            if (!window.location.hash && state.bootstrap.default_path) {
              window.location.hash = '#' + state.bootstrap.default_path;
            }
            await renderRoute();
          } catch (err) {
            renderLogin(err.message);
          }
        });
      }
      const googleButton = document.getElementById('google-login');
      if (googleButton) {
        googleButton.addEventListener('click', () => {
          const target = requestedUIHref();
          window.location.assign('/auth/google/start?next=' + encodeURIComponent(target));
        });
      }
    }

    function renderMenus() {
      const container = document.getElementById('menu');
      container.innerHTML = '';
      renderSurfaceSwitcher();
      renderRouteJumpOptions();
      if (!state.bootstrap) {
        document.getElementById('admin-link-button').hidden = true;
        document.getElementById('logout-button').hidden = true;
        return;
      }
      applyShellLayout(true);
      document.getElementById('admin-link-button').hidden = !state.bootstrap || !state.bootstrap.admin_access;
      document.getElementById('admin-link-button').href = state.bootstrap && state.bootstrap.admin_path ? state.bootstrap.admin_path : '/admin';
      const notificationsButton = document.getElementById('notifications-button');
      if (notificationsButton) {
        notificationsButton.hidden = !state.bootstrap;
        notificationsButton.textContent = t('notifications');
        notificationsButton.onclick = () => { window.location.hash = '#/notifications'; };
      }
      document.getElementById('logout-button').hidden = !state.bootstrap;
      for (const menu of state.bootstrap.menus) {
        const action = state.bootstrap.actions.find((item) => item.key === menu.action_key);
        if (!action) continue;
        const link = document.createElement('a');
        link.className = 'menu-link' + (currentPath() === action.route_path ? ' active' : '');
        link.href = '#' + action.route_path;
        link.textContent = pickText(menu, 'label');
        link.addEventListener('click', () => {
          if (window.innerWidth < 1024) {
            state.navCollapsed = true;
            persistShellPrefs();
            applyWorkspaceNavState();
          }
        });
        container.appendChild(link);
      }
      enhanceControlAccessibility(document.getElementById('shell-bar'));
    }

    function renderSurfaceSwitcher() {
      const container = document.getElementById('surface-switcher');
      if (!container) return;
      const items = (state.bootstrap && state.bootstrap.available_surfaces) || [];
      if (!items.length) {
        container.hidden = true;
        container.innerHTML = '';
        return;
      }
      container.hidden = false;
      container.innerHTML = items.map((surface) => '<button type="button" class="' + (state.surface === surface ? '' : 'secondary') + '" data-surface="' + escapeHTML(surface) + '">' + escapeHTML(t('surface_' + surface)) + '</button>').join('');
      container.querySelectorAll('[data-surface]').forEach((button) => {
        button.addEventListener('click', async () => {
          await navigateToSurface(button.dataset.surface || 'backoffice');
        });
      });
    }

    function findActionByView(predicate) {
      return (state.bootstrap.actions || []).find((action) => {
        if (!action.view_key) return false;
        const view = (state.bootstrap.views || []).find((item) => item.key === action.view_key);
        return !!view && predicate(view);
      }) || null;
    }

    function routeForModel(modelKey, kind) {
      const action = findActionByView((view) => view.model_key === modelKey && view.kind === kind);
      return action ? action.route_path : '';
    }

    function detailViewForDocumentType(documentType) {
      return ((state.bootstrap && state.bootstrap.views) || []).find((view) => view.document_type === documentType && view.kind === 'detail') || null;
    }

    function routeForDocument(documentType, kind) {
      const action = findActionByView((view) => view.document_type === documentType && view.kind === kind);
      return action ? action.route_path : '';
    }

    function routeForFlow(flowKey) {
      const action = (state.bootstrap.actions || []).find((item) => item.render_mode === 'flow' && item.flow_key === flowKey);
      return action ? action.route_path : '';
    }

    function flowForKey(flowKey) {
      return ((state.bootstrap && state.bootstrap.flows) || []).find((item) => item.key === flowKey) || null;
    }

    function routeForDocumentCreate(documentType) {
      const flowAction = (state.bootstrap.actions || []).find((action) => {
        if (action.render_mode !== 'flow' || !action.flow_key) return false;
        const flow = flowForKey(action.flow_key);
        return !!flow && flow.primary_document_type === documentType;
      });
      if (flowAction) return flowAction.route_path;
      return routeForDocument(documentType, 'form');
    }

    function routeForDocumentEdit(record, flowInstance) {
      const instance = flowInstance || null;
      if (instance && instance.flow && instance.primary_document_id) {
        const flowRoute = routeForFlow(instance.flow.key);
        if (flowRoute) {
          const params = new URLSearchParams();
          params.set('id', instance.primary_document_id);
          const activeDocumentKey = currentParams().get('document_key') || ((((record || {}).header || {}).metadata || {}).flow_document_key) || instance.active_document_key || '';
          if (activeDocumentKey) params.set('document_key', activeDocumentKey);
          return flowRoute + '?' + params.toString();
        }
      }
      const formRoute = routeForDocument(record.header.type, 'form');
      if (!formRoute || !record.header.id) return '';
      return formRoute + '?id=' + encodeURIComponent(record.header.id);
    }

    async function loadBundle(bundleKey) {
      if (state.bundles[bundleKey]) return state.bundles[bundleKey];
      const script = document.createElement('script');
      script.src = '/ui/assets/modules/' + encodeURIComponent(bundleKey) + '.js';
      script.async = true;
      const loaded = await new Promise((resolve, reject) => {
        script.onload = resolve;
        script.onerror = () => reject(new Error('failed to load module bundle'));
        document.head.appendChild(script);
      });
      void loaded;
      const bundles = window.ClinicModuleBundles || {};
      if (!bundles[bundleKey]) throw new Error('module bundle not registered');
      state.bundles[bundleKey] = bundles[bundleKey];
      return bundles[bundleKey];
    }

    function renderJSONCard(title, payload) {
      const root = document.getElementById('view-root');
      root.innerHTML = '<section class="panel"><h3>' + escapeHTML(title) + '</h3><pre></pre></section>';
      root.querySelector('pre').textContent = JSON.stringify(payload, null, 2);
    }

    function actionButtonClass(actionKey, zone) {
      if (zone === 'primary') return '';
      if (actionKey === 'reject' || actionKey === 'cancel') return 'warn';
      return 'secondary';
    }

    async function resolveAllowedActionsForDocumentItem(item) {
      if (!item || item.target_type !== 'document' || !item.target_id || !item.document_type) return [];
      const view = detailViewForDocumentType(item.document_type);
      if (!view) return [];
      const allowed = [];
      for (const actionKey of view.allowed_actions || []) {
        try {
          const placement = await api('/ui/actions/render?action=' + encodeURIComponent(actionKey) + '&document_id=' + encodeURIComponent(item.target_id));
          if (placement && placement.allowed) {
            allowed.push({key: actionKey, zone: resolveActionPlacement(view, actionKey, placement)});
          }
        } catch (_) {}
      }
      return allowed;
    }

    async function hydrateWorkItemActionContainers(root, items, rerenderTarget) {
      const containers = Array.from(root.querySelectorAll('[data-workitem-actions]'));
      for (const container of containers) {
        const item = items.find((candidate) => candidate.id === (container.dataset.workitemActions || ''));
        if (!item) continue;
        const actions = await resolveAllowedActionsForDocumentItem(item);
        if (!actions.length) {
          container.innerHTML = '<span class="row-secondary">' + escapeHTML(t('queue_action_ready')) + '</span>';
          continue;
        }
        container.innerHTML = actions.map((action) => '<button type="button" class="' + actionButtonClass(action.key, action.zone) + '" data-workitem-action="' + escapeHTML(action.key) + '" data-workitem-id="' + escapeHTML(item.id) + '">' + escapeHTML(translateToken('action', action.key)) + '</button>').join('');
      }
      root.querySelectorAll('[data-workitem-action]').forEach((button) => {
        button.addEventListener('click', async () => {
          const item = items.find((candidate) => candidate.id === (button.dataset.workitemId || ''));
          if (!item) return;
          try {
            const documentPayload = await api('/ui/data/documents/' + encodeURIComponent(item.target_id));
            const targetRecord = documentPayload.record;
            await invokeDocumentAction(targetRecord.header.id, button.dataset.workitemAction || '', targetRecord.header.version, targetRecord.header.etag);
            if (rerenderTarget === 'detail') {
              await renderRoute();
              return;
            }
            await renderRoute();
          } catch (err) {
            setStatus(err.message);
          }
        });
      });
    }

    function renderWorkflowContextPanel(context, currentKind, currentID) {
      if (!context) return '';
      const tasks = context.tasks || [];
      const approvals = context.approvals || [];
      const history = context.history || [];
      const currentItem = currentKind === 'approval' ? context.current_approval : context.current_task;
      const currentMarkup = currentItem
        ? '<article class="metric-card"><span class="meta">' + t('workflow_opened_from_queue') + '</span><strong>' + escapeHTML(displayValue(currentItem.status || currentItem.stage_key || currentItem.task_type || '')) + '</strong><div class="row-secondary">' + escapeHTML((currentItem.workflow_key || '') + ' · ' + (currentItem.due_at || '')) + '</div><div class="toolbar-row" data-workitem-actions="' + escapeHTML(currentItem.id || '') + '"></div></article>'
        : '';
      const taskMarkup = tasks.length
        ? tasks.map((item) => '<article class="detail-item"><span class="meta">' + escapeHTML(item.task_type || item.workflow_key || item.id) + '</span><strong>' + escapeHTML(displayValue(item.status)) + '</strong><div class="row-secondary">' + escapeHTML((item.assignee_user_id || item.assignee_role_key || '') + (item.due_at ? ' · ' + item.due_at : '')) + '</div></article>').join('')
        : '<p class="status">' + t('no_records') + '</p>';
      const approvalMarkup = approvals.length
        ? approvals.map((item) => '<article class="detail-item"><span class="meta">' + escapeHTML(item.stage_key || item.workflow_key || item.id) + '</span><strong>' + escapeHTML(displayValue(item.status)) + '</strong><div class="row-secondary">' + escapeHTML((item.requested_by || '') + (item.due_at ? ' · ' + item.due_at : '')) + '</div></article>').join('')
        : '<p class="status">' + t('no_records') + '</p>';
      const historyMarkup = history.length
        ? history.slice(0, 6).map((item) => '<article class="detail-item"><span class="meta">' + escapeHTML(displayValue(item.action || '')) + '</span><strong>' + escapeHTML([item.from_state, item.to_state].filter(Boolean).join(' → ') || displayValue(item.decision_code || '')) + '</strong><div class="row-secondary">' + escapeHTML((item.actor_id || '') + (item.occurred_at ? ' · ' + item.occurred_at : '')) + '</div></article>').join('')
        : '<p class="status">' + t('no_records') + '</p>';
      return '<section class="panel"><h3>' + t('workflow_context') + '</h3><div class="metric-grid">' + currentMarkup + '</div><div class="section-stack"><section class="panel"><h3>' + t('workflow_active_tasks') + '</h3>' + taskMarkup + '</section><section class="panel"><h3>' + t('workflow_active_approvals') + '</h3>' + approvalMarkup + '</section><section class="panel"><h3>' + t('workflow_history') + '</h3>' + historyMarkup + '</section></div></section>';
    }

    async function renderGeneric(route) {
      const root = document.getElementById('view-root');
      const view = route.view;
      if (!view) {
        renderJSONCard(t('view_unavailable'), route);
        return;
      }
      if (view.kind === 'queue') {
        const source = (view.projection_key || '').indexOf('approval') >= 0 ? 'approvals' : 'tasks';
        const params = currentParams();
        const viewPrefs = await loadUIPreferences(currentPath(), view.key);
        if (![...params.keys()].length) {
          const saved = (viewPrefs.filters && viewPrefs.filters[source]) || readSavedWorklistFilter(currentPath(), source);
          if (saved && Object.keys(saved).length) {
            window.location.hash = '#' + currentPath() + '?' + paramsFromObject(saved).toString();
            setStatus(t('queue_saved_filter'));
            return;
          }
        }
        const density = viewPrefs.density || 'comfortable';
        const query = new URLSearchParams();
        if (params.get('status')) query.set('status', params.get('status'));
        if (params.get('due')) query.set('due', params.get('due'));
        if (params.get('workflow_key')) query.set('workflow_key', params.get('workflow_key'));
        if (source === 'tasks' && params.get('mine')) query.set('mine', params.get('mine'));
        if (source === 'approvals' && params.get('requested_by_me')) query.set('requested_by_me', params.get('requested_by_me'));
        const payload = await api('/ui/data/worklist/' + source + (query.toString() ? '?' + query.toString() : ''));
        const summary = await api('/ui/data/worklist/summary' + (query.toString() ? '?' + query.toString() : ''));
        const items = payload.items || [];
        const workflowOptions = Array.from(new Set(items.map((item) => item.workflow_key).filter(Boolean))).sort();
        const filterBar = '<div class="toolbar-row">'
          + '<label class="control-tile"><span class="meta">' + t('queue_status') + '</span><select data-worklist-filter="status"><option value="">' + t('all') + ' ' + t('queue_status') + '</option>'
          + (source === 'approvals'
            ? ['pending', 'approved', 'rejected'].map((status) => '<option value="' + status + '"' + (params.get('status') === status ? ' selected' : '') + '>' + escapeHTML(displayValue(status)) + '</option>').join('')
            : ['open', 'completed', 'cancelled'].map((status) => '<option value="' + status + '"' + (params.get('status') === status ? ' selected' : '') + '>' + escapeHTML(displayValue(status)) + '</option>').join(''))
          + '</select></label>'
          + '<label class="control-tile"><span class="meta">' + t('queue_due') + '</span><select data-worklist-filter="due"><option value="">' + t('queue_due_any') + '</option><option value="overdue"' + (params.get('due') === 'overdue' ? ' selected' : '') + '>' + t('queue_due_overdue') + '</option></select></label>'
          + (source === 'tasks'
            ? '<label class="control-tile"><span class="meta">' + t('queue_assignee') + '</span><select data-worklist-filter="mine"><option value="">' + t('queue_assignee_any') + '</option><option value="1"' + (params.get('mine') === '1' ? ' selected' : '') + '>' + t('queue_assignee_mine') + '</option></select></label>'
            : '<label class="control-tile"><span class="meta">' + t('queue_assignee') + '</span><select data-worklist-filter="requested_by_me"><option value="">' + t('queue_assignee_any') + '</option><option value="1"' + (params.get('requested_by_me') === '1' ? ' selected' : '') + '>' + t('queue_requested_by_me') + '</option></select></label>')
          + '<label class="control-tile"><span class="meta">' + t('queue_workflow') + '</span><select data-worklist-filter="workflow_key"><option value="">' + t('all') + ' ' + t('queue_workflow') + '</option>' + workflowOptions.map((key) => '<option value="' + escapeHTML(key) + '"' + (params.get('workflow_key') === key ? ' selected' : '') + '>' + escapeHTML(humanizeToken(key)) + '</option>').join('') + '</select></label>'
          + '<label class="control-tile"><span class="meta">' + t('density') + '</span><select data-density-mode="1"><option value="comfortable"' + (density === 'comfortable' ? ' selected' : '') + '>' + t('density_comfortable') + '</option><option value="compact"' + (density === 'compact' ? ' selected' : '') + '>' + t('density_compact') + '</option></select></label>'
          + '<button type="button" class="secondary" data-save-worklist-filter="1">' + t('queue_save_filter') + '</button>'
          + '<button type="button" class="secondary" data-reset-worklist-filter="1">' + t('queue_reset_filter') + '</button>'
          + '</div>';
        const rows = items.map((item) => {
          const primary = item.target_title || item.target_number || item.target_id || item.id;
          const secondary = [item.document_type || item.target_type, item.stage_key || item.task_type || item.workflow_key, item.target_status].filter(Boolean).join(' · ');
          const assignment = source === 'tasks' ? (item.assignee_user_id || item.assignee_role_key || item.assignment_mode || '') : (item.requested_by || item.stage_key || '');
          return '<tr><td><div class="row-primary">' + escapeHTML(primary) + '</div><div class="row-secondary">' + escapeHTML(secondary) + '</div></td><td>' + escapeHTML(displayValue(item.status)) + '</td><td>' + escapeHTML(assignment) + '<div class="toolbar-row" data-workitem-actions="' + escapeHTML(item.id || '') + '"></div></td><td>' + escapeHTML(item.due_at || '') + '</td><td><button class="secondary" data-open-workitem="' + escapeHTML(item.id || '') + '">Open</button></td></tr>';
        }).join('');
        const queueSummary = source === 'approvals' ? (summary.approvals || {}) : (summary.tasks || {});
        const summaryMarkup = source === 'approvals'
          ? '<div class="metric-grid"><article class="metric-card"><span class="meta">' + t('queue_approvals_label') + '</span><strong>' + escapeHTML(String(queueSummary.pending || 0)) + '</strong></article><article class="metric-card"><span class="meta">' + t('queue_due_overdue') + '</span><strong>' + escapeHTML(String(queueSummary.overdue || 0)) + '</strong></article><article class="metric-card"><span class="meta">' + t('queue_requested_by_me') + '</span><strong>' + escapeHTML(String(queueSummary.requested_by_me || 0)) + '</strong></article><article class="metric-card"><span class="meta">' + t('queue_workflows_label') + '</span><strong>' + escapeHTML(String(queueSummary.workflows || 0)) + '</strong></article></div>'
          : '<div class="metric-grid"><article class="metric-card"><span class="meta">' + t('queue_tasks_label') + '</span><strong>' + escapeHTML(String(queueSummary.open || 0)) + '</strong></article><article class="metric-card"><span class="meta">' + t('queue_due_overdue') + '</span><strong>' + escapeHTML(String(queueSummary.overdue || 0)) + '</strong></article><article class="metric-card"><span class="meta">' + t('queue_mine_label') + '</span><strong>' + escapeHTML(String(queueSummary.mine || 0)) + '</strong></article><article class="metric-card"><span class="meta">' + t('queue_workflows_label') + '</span><strong>' + escapeHTML(String(queueSummary.workflows || 0)) + '</strong></article></div>';
        const activeChips = renderFilterChips(params, ['status', 'due', 'mine', 'requested_by_me', 'workflow_key']);
        root.innerHTML = '<section class="page-panel floorplan-worklist" data-density="' + escapeHTML(density) + '"><div class="page-header"><div><div class="page-eyebrow">Worklist</div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(pickText(view, 'empty_state') || 'Operational queue for workflow-driven work.') + '</p></div><div class="page-header-actions"><span class="badge badge-subtle">items ' + escapeHTML(String(items.length)) + '</span></div></div><div class="page-body">' + summaryMarkup + filterBar + activeChips + '<div class="table-shell"><table class="data-table"><thead><tr><th>' + t('queue_target') + '</th><th>' + t('queue_status') + '</th><th>' + t('queue_assignment') + '</th><th>' + t('queue_due') + '</th><th></th></tr></thead><tbody>' + (rows || '<tr><td colspan="5"><div class="empty-state-inline">' + escapeHTML(t('no_records')) + '</div></td></tr>') + '</tbody></table></div></div></section>';
        root.querySelectorAll('[data-worklist-filter]').forEach((input) => {
          input.addEventListener('change', () => {
            const next = currentParams();
            if (input.value) next.set(input.dataset.worklistFilter, input.value); else next.delete(input.dataset.worklistFilter);
            window.location.hash = '#' + currentPath() + (next.toString() ? '?' + next.toString() : '');
          });
        });
        root.querySelectorAll('[data-save-worklist-filter]').forEach((button) => {
          button.addEventListener('click', async () => {
            const next = currentParams();
            await saveWorklistFilterPreference(currentPath(), source, Object.fromEntries(next.entries()), view.key);
            setStatus(t('queue_filter_saved'));
          });
        });
        root.querySelectorAll('[data-reset-worklist-filter]').forEach((button) => {
          button.addEventListener('click', async () => {
            await saveWorklistFilterPreference(currentPath(), source, {}, view.key);
            setStatus(t('queue_filter_cleared'));
            window.location.hash = '#' + currentPath();
          });
        });
        root.querySelectorAll('[data-clear-filter]').forEach((button) => {
          button.addEventListener('click', () => {
            const next = currentParams();
            next.delete(button.dataset.clearFilter);
            window.location.hash = '#' + currentPath() + (next.toString() ? '?' + next.toString() : '');
          });
        });
        root.querySelectorAll('[data-density-mode]').forEach((input) => {
          input.addEventListener('change', async () => {
            await saveUIPreferences(currentPath(), Object.assign({}, viewPrefs, {view_key: view.key, density: input.value || 'comfortable'}));
            await renderRoute();
          });
        });
        root.querySelectorAll('[data-open-workitem]').forEach((button) => {
          button.addEventListener('click', () => {
            const item = items.find((candidate) => candidate.id === (button.dataset.openWorkitem || ''));
            const targetPath = routeForWorkItem(item, view.document_type);
            if (!targetPath) return;
            window.location.hash = '#' + targetPath;
          });
        });
        await hydrateWorkItemActionContainers(root, items, 'queue');
        return;
      }
      if (view.kind === 'list') {
        const params = currentParams();
        const viewPrefs = await loadUIPreferences(currentPath(), view.key);
        const query = new URLSearchParams();
        if (view.document_type) query.set('type', view.document_type);
        if (view.model_key) query.set('model', view.model_key);
        if (params.get('status')) query.set('status', params.get('status'));
        if (params.get('name')) query.set('name', params.get('name'));
        if (params.get('sort')) query.set('sort', params.get('sort'));
        const pageSize = parseInt(params.get('page_size') || view.default_page_size || '10', 10);
        const page = parseInt(params.get('page') || '1', 10);
        query.set('page', String(page));
        query.set('page_size', String(pageSize));
        const listPath = view.model_key ? '/ui/data/models?' : (view.projection_key ? '/ui/data/projections/documents?' : '/ui/data/documents?');
        let payload;
        try {
          payload = await api(listPath + query.toString());
        } catch (err) {
          payload = await projectionFallback(view);
          if (!payload) throw err;
        }
        const pagedItems = payload.items || [];
        const newRoute = view.model_key ? routeForModel(view.model_key, 'form') : routeForDocumentCreate(view.document_type);
        const density = viewPrefs.density || 'comfortable';
        const configuredColumns = view.columns || [];
        const preferredColumns = (viewPrefs.columns || []).filter((key) => configuredColumns.some((column) => column.key === key));
        const visibleColumns = (preferredColumns.length ? preferredColumns : configuredColumns.map((column) => column.key))
          .map((key) => configuredColumns.find((column) => column.key === key))
          .filter(Boolean);
        const filterBar = '<div class="toolbar-row">' +
          ((view.filters || []).map((filter) => {
            if (filter.type !== 'enum') return '';
            const options = ['<option value="">' + t('all') + ' ' + escapeHTML(pickText(filter, 'label')) + '</option>'].concat((filter.options || []).map((option) => '<option value="' + option + '"' + (params.get(filter.key) === option ? ' selected' : '') + '>' + escapeHTML(displayValue(option)) + '</option>'));
            return '<label class="control-tile"><span class="meta">' + escapeHTML(pickText(filter, 'label')) + '</span><select data-filter="' + filter.key + '">' + options.join('') + '</select></label>';
          }).join('')) +
          (view.model_key ? '<label class="control-tile grow"><span class="meta">' + t('search') + '</span><input data-primary-filter="1" data-filter="name" value="' + escapeHTML(params.get('name') || '') + '" placeholder="' + escapeHTML(t('search')) + '"></label>' : '') +
          '<label class="control-tile"><span class="meta">' + t('sort') + '</span><select data-filter="sort"><option value="">' + t('sort_document') + '</option><option value="updated_at"' + (params.get('sort') === 'updated_at' ? ' selected' : '') + '>' + t('sort_updated') + '</option><option value="status"' + (params.get('sort') === 'status' ? ' selected' : '') + '>' + t('sort_status') + '</option><option value="name"' + (params.get('sort') === 'name' ? ' selected' : '') + '>' + t('sort_name') + '</option></select></label>' +
          '<label class="control-tile"><span class="meta">' + t('density') + '</span><select data-density-mode="1"><option value="comfortable"' + (density === 'comfortable' ? ' selected' : '') + '>' + t('density_comfortable') + '</option><option value="compact"' + (density === 'compact' ? ' selected' : '') + '>' + t('density_compact') + '</option></select></label>' +
          '</div>';
        const columnDefs = visibleColumns;
        const columnChooser = configuredColumns.length
          ? '<div class="toolbar-row">' + configuredColumns.map((column) => '<label class="control-tile"><input type="checkbox" data-column-toggle="' + escapeHTML(column.key) + '"' + (columnDefs.some((visible) => visible.key === column.key) ? ' checked' : '') + '> <span class="meta">' + escapeHTML(pickText(column, 'label')) + '</span></label>').join('') + '</div>'
          : '';
        const rows = pagedItems.map((item) => {
          const openID = item.id || (item.header && item.header.id) || '';
          const cells = columnDefs.map((column, index) => {
            const value = escapeHTML(displayValue(resolvePath(item, column.path)));
            if (index === 0) {
              return '<td><div class="row-primary">' + value + '</div><div class="row-secondary">' + escapeHTML(openID) + '</div></td>';
            }
            return '<td>' + value + '</td>';
          }).join('');
          return '<tr>' + (cells || ('<td><div class="row-primary">' + escapeHTML(openID) + '</div></td>')) + '<td><button class="secondary" data-open="' + openID + '">' + t('open') + '</button></td></tr>';
        }).join('');
        const total = payload.total || pagedItems.length;
        const tableHeader = columnDefs.map((column) => '<th>' + escapeHTML(pickText(column, 'label')) + '</th>').join('') + '<th></th>';
        const tableMarkup = rows
          ? '<div class="table-shell"><table class="data-table"><thead><tr>' + tableHeader + '</tr></thead><tbody>' + rows + '</tbody></table></div>'
          : '<div class="table-shell"><div class="empty-state-block"><h4>' + escapeHTML(pickText(view, 'title')) + '</h4><p class="status">' + escapeHTML(pickText(view, 'empty_state') || t('no_records')) + '</p>' + (newRoute ? '<button type="button" data-new="1">' + t('new') + '</button>' : '') + '</div></div>';
        const pagination = '<div class="pagination-bar"><span class="status">' + t('page') + ' ' + page + ' / ' + Math.max(1, Math.ceil(total / pageSize)) + '</span><div class="actions"><button class="secondary" data-page="' + Math.max(1, page - 1) + '"' + (page <= 1 ? ' disabled' : '') + '>' + t('previous') + '</button><button class="secondary" data-page="' + (page + 1) + '"' + (page * pageSize >= total ? ' disabled' : '') + '>' + t('next') + '</button></div></div>';
        const activeChips = renderFilterChips(params, ['status', 'name', 'sort']);
        root.innerHTML = '<section class="page-panel floorplan-worklist" data-density="' + escapeHTML(density) + '"><div class="page-header"><div><div class="page-eyebrow">Worklist</div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(pickText(view, 'empty_state') || t('standard_list')) + '</p></div><div class="page-header-actions"><span class="badge badge-subtle">items ' + escapeHTML(String(total)) + '</span>' + (newRoute ? '<button type="button" data-new="1">' + t('new') + '</button>' : '') + '</div></div><div class="page-body">' + filterBar + activeChips + '<details class="page-tools"><summary>Columns</summary>' + columnChooser + '</details>' + tableMarkup + pagination + '</div></section>';
        root.querySelectorAll('[data-filter]').forEach((input) => {
          input.addEventListener('change', () => {
            const next = currentParams();
            if (input.value) next.set(input.dataset.filter, input.value); else next.delete(input.dataset.filter);
            next.set('page', '1');
            window.location.hash = '#' + currentPath() + (next.toString() ? '?' + next.toString() : '');
          });
        });
        root.querySelectorAll('[data-page]').forEach((button) => {
          button.addEventListener('click', () => {
            const next = currentParams();
            next.set('page', button.dataset.page);
            next.set('page_size', String(pageSize));
            window.location.hash = '#' + currentPath() + '?' + next.toString();
          });
        });
        root.querySelectorAll('[data-density-mode]').forEach((input) => {
          input.addEventListener('change', async () => {
            await saveUIPreferences(currentPath(), Object.assign({}, viewPrefs, {view_key: view.key, density: input.value || 'comfortable'}));
            await renderRoute();
          });
        });
        root.querySelectorAll('[data-column-toggle]').forEach((input) => {
          input.addEventListener('change', async () => {
            const columns = Array.from(root.querySelectorAll('[data-column-toggle]')).filter((item) => item.checked).map((item) => item.dataset.columnToggle);
            await saveUIPreferences(currentPath(), Object.assign({}, viewPrefs, {view_key: view.key, columns, column_order: columns}));
            await renderRoute();
          });
        });
        root.querySelectorAll('[data-open]').forEach((button) => {
          button.addEventListener('click', () => {
            const targetPath = view.model_key ? routeForModel(view.model_key, 'detail') : routeForDocument(view.document_type, 'detail');
            if (!targetPath) return;
            window.location.hash = '#' + targetPath + '?id=' + encodeURIComponent(button.dataset.open);
          });
        });
        root.querySelectorAll('[data-new]').forEach((button) => {
          button.addEventListener('click', () => {
            if (!newRoute) return;
            window.location.hash = '#' + newRoute;
          });
        });
        root.querySelectorAll('[data-clear-filter]').forEach((button) => {
          button.addEventListener('click', () => {
            const next = currentParams();
            next.delete(button.dataset.clearFilter);
            next.set('page', '1');
            window.location.hash = '#' + currentPath() + (next.toString() ? '?' + next.toString() : '');
          });
        });
        return;
      }
      if (view.kind === 'detail') {
        const detailParams = currentParams();
        const documentID = detailParams.get('id');
        if (!documentID) {
          root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(t('select_record')) + '</p></div></div></section>';
          return;
        }
        if (view.model_key) {
          const payload = await api('/ui/data/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(documentID));
          const record = payload.record;
          const tabMarkup = (view.tabs || []).map((tab) => {
            const sections = (tab.sections || []).map((section) => renderModelSection(section, record)).join('');
            return '<section class="panel"><h3>' + escapeHTML(pickText(tab, 'title')) + '</h3>' + sections + '</section>';
          }).join('');
          const sectionMarkup = (view.sections || []).map((section) => renderModelSection(section, record)).join('');
          const relatedViews = (view.related_views || []).map((item) => renderRelatedView(item, payload, view)).join('');
          root.innerHTML = '<section class="page-panel floorplan-object"><div class="page-header"><div><div class="page-eyebrow">Object</div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(record.id + ' · v' + record.version) + '</p></div><div class="page-header-actions"><span class="badge badge-subtle">' + escapeHTML(view.model_key || 'model') + '</span></div></div><div class="page-body"><div class="section-stack">' + (tabMarkup || sectionMarkup) + '</div></div></section>' + relatedViews;
          root.querySelectorAll('[data-related-save]').forEach((button) => {
            button.addEventListener('click', async () => {
              const sourceKey = button.dataset.relatedSave;
              const section = button.closest('section');
              const values = {};
              section.querySelectorAll('[data-path]').forEach((input) => assignPath(values, input.dataset.path.replace(/^values\./, ''), readFieldValue(input)));
              const csrf = readCookie('orbyte_csrf');
              try {
                await api('/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(record.id) + '/relations/' + encodeURIComponent(sourceKey), {
                  method: 'POST',
                  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                  body: JSON.stringify({values})
                });
                const statusNode = section.querySelector('[data-related-status="' + sourceKey + '"]');
                if (statusNode) statusNode.textContent = t('related_record_created');
                await renderRoute();
              } catch (err) {
                const statusNode = section.querySelector('[data-related-status="' + sourceKey + '"]');
                if (statusNode) statusNode.textContent = err.message;
                setStatus(err.message);
              }
            });
          });
          return;
        }
        const payload = await api('/ui/data/documents/' + encodeURIComponent(documentID));
        const flowInstance = payload.flow_instance || null;
        const record = payload.record;
        const workItemKind = detailParams.get('work_item_kind') || '';
        const workItemID = detailParams.get('work_item_id') || '';
        let workflowContext = null;
        if (workItemKind || currentSurface() === 'worklist') {
          try {
            workflowContext = await api('/ui/data/worklist/context?target_type=document&target_id=' + encodeURIComponent(documentID) + (workItemKind ? '&work_item_kind=' + encodeURIComponent(workItemKind) : '') + (workItemID ? '&work_item_id=' + encodeURIComponent(workItemID) : ''));
          } catch (_) {}
        }
        let activeRecord = record;
        let detailMarkup = '';
        let relatedViews = '';
        if (flowInstance && (flowInstance.items || []).length > 0) {
          const requestedDocumentKey = currentParams().get('document_key') || flowInstance.active_document_key || '';
          const activeItem = (flowInstance.items || []).find((item) => item.definition.key === requestedDocumentKey) || flowInstance.items[0];
          activeRecord = activeItem.record;
          const flowTabs = (flowInstance.items || []).map((item) => '<button type="button" class="' + (item.definition.key === activeItem.definition.key ? '' : 'secondary') + '" data-flow-detail-tab="' + escapeHTML(item.definition.key) + '">' + escapeHTML(pickText(item.definition, 'title')) + '</button>').join('');
          detailMarkup = '<div class="toolbar-row">' + flowTabs + '</div><div class="section-stack">' + renderFlowReadonlyDocumentDefinition(activeItem.definition, activeItem.record) + '</div>';
        } else {
          const tabMarkup = (view.tabs || []).map((tab) => {
            const sections = (tab.sections || []).map((section) => renderSection(section, record)).join('');
            return '<section class="panel"><h3>' + escapeHTML(pickText(tab, 'title')) + '</h3>' + sections + '</section>';
          }).join('');
          const sectionMarkup = (view.sections || []).map((section) => renderSection(section, record)).join('');
          detailMarkup = '<div class="section-stack">' + renderWorkflowContextPanel(workflowContext, workItemKind, workItemID) + (tabMarkup || sectionMarkup || ('<pre>' + escapeHTML(JSON.stringify(record.body.payload, null, 2)) + '</pre>')) + '</div>';
          relatedViews = (view.related_views || []).map((item) => renderRelatedView(item, payload, view)).join('');
        }
        if (workflowContext && flowInstance) {
          detailMarkup = renderWorkflowContextPanel(workflowContext, workItemKind, workItemID) + detailMarkup;
        }
        const actionZones = renderActionZones(view);
        root.innerHTML = '<section class="page-panel floorplan-object"><div class="page-header"><div><div class="page-eyebrow">Object</div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(activeRecord.header.id + ' · v' + activeRecord.header.version + ' · ' + displayValue(activeRecord.header.status)) + '</p></div><div class="page-header-actions"><span class="badge badge-subtle">' + escapeHTML(activeRecord.header.type || '') + '</span><span class="badge">' + escapeHTML(displayValue(activeRecord.header.status)) + '</span></div></div><div class="page-body">' + detailMarkup + '</div><div class="page-actions">' + actionZones + '</div></section>' + relatedViews;
        root.querySelectorAll('[data-flow-detail-tab]').forEach((button) => {
          button.addEventListener('click', () => {
            const params = currentParams();
            params.set('id', flowInstance.primary_document_id || documentID);
            params.set('document_key', button.dataset.flowDetailTab || '');
            window.location.hash = '#' + route.action.route_path + '?' + params.toString();
          });
        });
        const editRoute = activeRecord.header.status === 'draft' ? routeForDocumentEdit(activeRecord, flowInstance) : '';
        if (editRoute) {
          const editButton = document.createElement('button');
          editButton.className = 'secondary';
          editButton.textContent = t('edit');
          editButton.addEventListener('click', () => {
            window.location.hash = '#' + editRoute;
          });
          (root.querySelector('[data-zone="secondary"]') || root.querySelector('.page-actions')).appendChild(editButton);
        }
        if (workItemKind && currentSurface() === 'worklist') {
          const backButton = document.createElement('button');
          backButton.className = 'secondary';
          backButton.textContent = t('workflow_back_to_queue');
          backButton.addEventListener('click', () => {
            const targetPath = workItemKind === 'approval' ? '/worklist/approvals' : '/worklist';
            window.location.hash = '#' + targetPath;
          });
          (root.querySelector('[data-zone="secondary"]') || root.querySelector('.page-actions')).appendChild(backButton);
        }
        await hydrateWorkItemActionContainers(root, []
          .concat((workflowContext && workflowContext.tasks) || [])
          .concat((workflowContext && workflowContext.approvals) || []), 'detail');
        for (const actionKey of view.allowed_actions || []) {
          const placement = await api('/ui/actions/render?action=' + encodeURIComponent(actionKey) + '&document_id=' + encodeURIComponent(activeRecord.header.id));
          if (!placement.allowed) {
            continue;
          }
          const button = document.createElement('button');
          button.textContent = translateToken('action', actionKey);
          const zone = resolveActionPlacement(view, actionKey, placement);
          if (zone === 'primary') {
            button.className = '';
          } else if (actionKey === 'reject' || actionKey === 'cancel') {
            button.className = 'warn';
          } else {
            button.className = 'secondary';
          }
          button.addEventListener('click', async () => {
            try {
              await invokeDocumentAction(activeRecord.header.id, actionKey, activeRecord.header.version, activeRecord.header.etag);
              await renderRoute();
            } catch (err) {
              setStatus(err.message);
            }
          });
          (root.querySelector('[data-zone="' + zone + '"]') || root.querySelector('[data-zone="secondary"]')).appendChild(button);
        }
        const printSupport = view.printable
          ? await resolvePrintTemplate(
              'document',
              activeRecord.header.type,
              view.print_purpose || 'official',
              view.print_channel || 'print',
              activeRecord.header.organization_id || '',
              activeRecord.header.location_id || ''
            )
          : {resolved: false};
        if (printSupport.resolved) {
          const secondaryZone = root.querySelector('[data-zone="secondary"]');
          const previewButton = document.createElement('button');
          previewButton.className = 'secondary';
          previewButton.textContent = t('print_preview');
          previewButton.addEventListener('click', async () => {
            try {
              await previewTemplateOutput({
                target_kind: 'document',
                target_key: activeRecord.header.type,
                target_id: activeRecord.header.id,
                organization_id: activeRecord.header.organization_id || '',
                location_id: activeRecord.header.location_id || '',
                purpose: view.print_purpose || 'official',
                channel: view.print_channel || 'print'
              });
            } catch (err) {
              setStatus(err.message);
            }
          });
          secondaryZone.appendChild(previewButton);
          const printButton = document.createElement('button');
          printButton.className = 'secondary';
          printButton.textContent = t('print_document');
          printButton.addEventListener('click', async () => {
            try {
              const payload = await api('/outputs/render', {
                method: 'POST',
                headers: {'Content-Type': 'application/json', 'X-CSRF-Token': readCookie('orbyte_csrf')},
                body: JSON.stringify({
                  target_kind: 'document',
                  target_key: activeRecord.header.type,
                  target_id: activeRecord.header.id,
                  organization_id: activeRecord.header.organization_id || '',
                  location_id: activeRecord.header.location_id || '',
                  purpose: view.print_purpose || 'official',
                  channel: view.print_channel || 'print',
                  format: 'html'
                })
              });
              openPrintWindow(payload.output);
            } catch (err) {
              setStatus(err.message);
            }
          });
          secondaryZone.appendChild(printButton);
          const pdfButton = document.createElement('button');
          pdfButton.className = 'secondary';
          pdfButton.textContent = t('download_pdf');
          pdfButton.addEventListener('click', async () => {
            try {
              await downloadTemplatePDF({
                target_kind: 'document',
                target_key: activeRecord.header.type,
                target_id: activeRecord.header.id,
                organization_id: activeRecord.header.organization_id || '',
                location_id: activeRecord.header.location_id || '',
                purpose: view.print_purpose || 'official',
                channel: view.print_channel || 'print'
              });
            } catch (err) {
              setStatus(err.message);
            }
          });
          secondaryZone.appendChild(pdfButton);
        }
        return;
      }
      if (view.kind === 'form') {
        const documentID = currentParams().get('id');
        if (view.model_key) {
          let payload = {record: {id: '', version: 0, values: {}}, definition: {relations: []}, related_definitions: {}};
          let record = {id: '', version: 0, values: {}};
          const localDraft = await loadDraft('model', view.model_key, documentID);
          if (localDraft && localDraft.values) {
            record = {id: documentID || '', version: localDraft.version || 0, values: localDraft.values};
          }
          try {
            if (documentID) {
              payload = await api('/ui/data/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(documentID));
              record = payload.record;
            } else {
              payload = await api('/ui/data/models?model=' + encodeURIComponent(view.model_key) + '&page_size=1');
            }
          } catch (_) {
            if (!documentID) {
              payload = {record, definition: {relations: []}, related_definitions: {}, model_definitions: {}};
            }
          }
          const formSections = (view.sections || []).length > 0
            ? (view.sections || []).map((section) => renderModelFormSection(section, record)).join('')
            : '<div class="form-grid">' + (view.fields || []).map((field) => renderEditableModelField(field, record)).join('') + '</div>';
          const relationViews = (view.related_views && view.related_views.length) ? view.related_views : deriveRelatedViews(payload.definition);
          const relationEditors = relationViews.map((item) => renderRelationEditor(item, payload)).join('');
          const offlineCapable = !!offlineModelCapability(view.model_key);
          root.innerHTML = '<section class="page-panel floorplan-editor"><div class="page-header"><div><div class="page-eyebrow">Editor</div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(documentID ? record.id + ' · v' + record.version : t('record_created')) + '</p></div><div class="page-header-actions"><span class="badge badge-subtle">' + escapeHTML(view.model_key || 'model') + '</span></div></div><div class="page-body"><div class="section-stack">' + formSections + relationEditors + '</div><p class="status" id="form-status"></p></div><div class="page-actions"><button id="save-form">' + ((offlineCapable && !navigator.onLine) ? t('queue_sync') : (documentID ? t('save') : t('create'))) + '</button><button id="save-local" class="secondary"' + (offlineCapable ? '' : ' disabled') + '>' + t('save_local') + '</button></div></section>';
          bindRelationRemove(root);
          root.querySelectorAll('[data-relation-add]').forEach((button) => {
            button.addEventListener('click', () => appendRelationRow(button.dataset.relationAdd, payload));
          });
          const saveLocalButton = root.querySelector('#save-local');
          if (saveLocalButton) {
            saveLocalButton.addEventListener('click', async () => {
              const values = {};
              root.querySelectorAll('[data-path]').forEach((input) => {
                if (input.closest('[data-relation-editor]')) return;
                assignPath(values, input.dataset.path.replace(/^values\./, ''), readFieldValue(input));
              });
              const relations = collectRelationMutations(root);
              await saveDraft('model', view.model_key, documentID, {
                kind: 'model',
                model_key: view.model_key,
                target_id: documentID || '',
                version: record.version || 0,
                values,
                relations,
                status: 'local_only'
              });
              await refreshSyncStats();
              document.getElementById('form-status').textContent = t('draft_saved_local');
              setStatus(t('draft_saved_local'));
            });
          }
          const button = root.querySelector('#save-form');
          if (button) {
            button.addEventListener('click', async () => {
              const values = {};
              root.querySelectorAll('[data-path]').forEach((input) => {
                if (input.closest('[data-relation-editor]')) return;
                assignPath(values, input.dataset.path.replace(/^values\./, ''), readFieldValue(input));
              });
              const relations = collectRelationMutations(root);
              const csrf = readCookie('orbyte_csrf');
              try {
                if (!navigator.onLine && offlineCapable) {
                  const queued = await queueSyncItem({
                    kind: 'model',
                    operation: documentID ? 'update' : 'create',
                    model_key: view.model_key,
                    target_id: documentID || '',
                    expected_version: record.version || 0,
                    values,
                    relations
                  });
                  await saveDraft('model', view.model_key, documentID, {
                    kind: 'model',
                    model_key: view.model_key,
                    target_id: documentID || '',
                    version: record.version || 0,
                    values,
                    relations,
                    status: 'queued',
                    idempotency_key: queued.idempotency_key
                  });
                  document.getElementById('form-status').textContent = t('draft_queued');
                  setStatus(t('draft_queued'));
                } else {
                  const created = await api('/models/' + encodeURIComponent(view.model_key) + (documentID ? '/' + encodeURIComponent(documentID) : ''), {
                    method: documentID ? 'PUT' : 'POST',
                    headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                    body: JSON.stringify(documentID ? {values, expected_version: record.version, relations} : {values, relations})
                  });
                  document.getElementById('form-status').textContent = documentID ? t('record_updated') : t('record_created');
                  setStatus(documentID ? t('record_updated') : t('record_created'));
                  if (!documentID) {
                    const detailRoute = routeForModel(view.model_key, 'detail');
                    const createdRecord = created && (created.record || created);
                    if (detailRoute && createdRecord && createdRecord.id) {
                      window.location.hash = '#' + detailRoute + '?id=' + encodeURIComponent(createdRecord.id);
                    }
                  }
                }
              } catch (err) {
                document.getElementById('form-status').textContent = err.message;
                setStatus(err.message);
              }
            });
          }
          return;
        }
        const offlineCapable = !!offlineDocumentCapability(view.document_type);
        let record = {header: {id: documentID || '', version: 0, etag: '', type: view.document_type || '', status: 'draft'}, body: {payload: {}}};
        const localDraft = await loadDraft('document', view.document_type, documentID);
        if (localDraft && localDraft.payload) {
          record.body.payload = localDraft.payload;
          record.header.version = localDraft.version || 0;
          record.header.etag = localDraft.etag || '';
        }
        if (documentID) {
          try {
            record = await api('/documents/' + encodeURIComponent(documentID) + '?view=expanded');
            const flowKey = (((record || {}).header || {}).metadata || {}).flow_key;
            const primaryDocumentID = ((((record || {}).header || {}).metadata || {}).flow_primary_document_id) || documentID;
            const flowDocumentKey = ((((record || {}).header || {}).metadata || {}).flow_document_key) || '';
            const flowRoute = flowKey ? routeForFlow(flowKey) : '';
            if (flowRoute) {
              const params = new URLSearchParams();
              params.set('id', primaryDocumentID);
              if (flowDocumentKey) params.set('document_key', flowDocumentKey);
              window.location.hash = '#' + flowRoute + '?' + params.toString();
              return;
            }
          } catch (_) {}
        }
        const formSections = (view.sections || []).length > 0
          ? (view.sections || []).map((section) => renderFormSection(section, record)).join('')
          : '<div class="form-grid">' + (view.fields || []).map((field) => renderEditableField(field, record)).join('') + '</div>';
        root.innerHTML = '<section class="page-panel floorplan-editor"><div class="page-header"><div><div class="page-eyebrow">Editor</div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(displayValue(record.header.status || 'draft')) + '</p></div><div class="page-header-actions"><span class="badge badge-subtle">' + escapeHTML(view.document_type || '') + '</span></div></div><div class="page-body"><div class="section-stack">' + formSections + '</div><p class="status" id="form-status"></p></div><div class="page-actions"><button id="save-form">' + ((offlineCapable && !navigator.onLine) ? t('queue_sync') : t('save_draft')) + '</button><button id="save-local" class="secondary"' + (offlineCapable ? '' : ' disabled') + '>' + t('save_local') + '</button></div></section>';
        const saveLocalButton = root.querySelector('#save-local');
        if (saveLocalButton) {
          saveLocalButton.addEventListener('click', async () => {
            const payload = {};
            root.querySelectorAll('[data-path]').forEach((input) => assignPath(payload, input.dataset.path.replace(/^body\.payload\./, ''), readFieldValue(input)));
            await saveDraft('document', view.document_type, documentID, {
              kind: 'document',
              document_type: view.document_type,
              target_id: documentID || '',
              version: record.header.version || 0,
              etag: record.header.etag || '',
              payload,
              status: 'local_only'
            });
            await refreshSyncStats();
            document.getElementById('form-status').textContent = t('draft_saved_local');
            setStatus(t('draft_saved_local'));
          });
        }
        const button = root.querySelector('#save-form');
        if (button) {
          button.addEventListener('click', async () => {
            const payload = {};
            root.querySelectorAll('[data-path]').forEach((input) => assignPath(payload, input.dataset.path.replace(/^body\.payload\./, ''), readFieldValue(input)));
            const csrf = readCookie('orbyte_csrf');
            try {
              if (offlineCapable && (!navigator.onLine || !documentID)) {
                const queued = await queueSyncItem({
                  kind: 'document',
                  operation: documentID ? 'update' : 'create',
                  document_type: view.document_type,
                  target_id: documentID || '',
                  expected_version: record.header.version || 0,
                  expected_etag: record.header.etag || '',
                  organization_id: (record.header && record.header.organization_id) || 'org_default',
                  location_id: (record.header && record.header.location_id) || '',
                  payload
                });
                await saveDraft('document', view.document_type, documentID, {
                  kind: 'document',
                  document_type: view.document_type,
                  target_id: documentID || '',
                  version: record.header.version || 0,
                  etag: record.header.etag || '',
                  payload,
                  status: 'queued',
                  idempotency_key: queued.idempotency_key
                });
                if (navigator.onLine) await processSyncQueue();
                document.getElementById('form-status').textContent = t('draft_queued');
                setStatus(t('draft_queued'));
              } else {
                await api('/documents/' + encodeURIComponent(documentID), {
                  method: 'PUT',
                  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                  body: JSON.stringify({payload})
                });
                document.getElementById('form-status').textContent = t('draft_updated');
                setStatus(t('draft_updated'));
              }
            } catch (err) {
              document.getElementById('form-status').textContent = err.message;
              setStatus(err.message);
            }
          });
        }
        return;
      }
      if (view.kind === 'dashboard') {
        const source = view.dataset_key
          ? await api('/ui/data/reporting/datasets/' + encodeURIComponent(view.dataset_key))
          : (view.projection_key === 'monitoring.summary'
          ? await api('/ui/data/monitoring/summary')
          : await api('/ui/data/analytics/snapshot'));
        const summary = (view.cards || []).map((card) => ({card, value: resolvePath(source, card.path)}));
        root.innerHTML = '<section class="page-panel floorplan-dashboard"><div class="page-header"><div><div class="page-eyebrow">Dashboard</div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3></div><div class="page-header-actions"><span class="badge badge-subtle">' + escapeHTML(view.dataset_key || view.projection_key || 'analytics') + '</span></div></div><div class="page-body"><div class="metric-grid">' + summary.map((item) => {
          if (item.card.widget === 'json') {
            return '<article class="metric-card"><span class="meta">' + escapeHTML(pickText(item.card, 'label')) + '</span><pre>' + escapeHTML(JSON.stringify(item.value, null, 2)) + '</pre></article>';
          }
          if (item.card.widget === 'table' && Array.isArray(item.value)) {
            return '<article class="metric-card" data-action="' + (item.card.action_key || '') + '"><span class="meta">' + escapeHTML(pickText(item.card, 'label')) + '</span><pre>' + escapeHTML(JSON.stringify(item.value, null, 2)) + '</pre></article>';
          }
          return '<article class="metric-card" data-action="' + (item.card.action_key || '') + '"><span class="meta">' + escapeHTML(pickText(item.card, 'label')) + '</span><strong>' + escapeHTML(displayValue(item.value)) + '</strong></article>';
        }).join('') + '</div></div><div class="page-actions" data-report-actions></div></section>';
        root.querySelectorAll('[data-action]').forEach((card) => {
          if (!card.dataset.action) return;
          card.addEventListener('click', () => {
            const action = state.bootstrap.actions.find((item) => item.key === card.dataset.action);
            if (action) window.location.hash = '#' + action.route_path;
          });
        });
        if (view.dataset_key && view.printable) {
          const printSupport = await resolvePrintTemplate('report', view.dataset_key, view.print_purpose || 'report', view.print_channel || 'print');
          if (printSupport.resolved) {
            const zone = root.querySelector('[data-report-actions]');
            const previewButton = document.createElement('button');
            previewButton.className = 'secondary';
            previewButton.textContent = t('print_preview');
            previewButton.onclick = async () => {
              try {
                await previewTemplateOutput({
                  target_kind: 'report',
                  target_key: view.dataset_key,
                  purpose: view.print_purpose || 'report',
                  channel: view.print_channel || 'print',
                  sample: false
                });
              } catch (err) {
                setStatus(err.message);
              }
            };
            zone.appendChild(previewButton);
            const printButton = document.createElement('button');
            printButton.className = 'secondary';
            printButton.textContent = t('print_document');
            printButton.onclick = async () => {
              try {
                const payload = await api('/outputs/render', {
                  method: 'POST',
                  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': readCookie('orbyte_csrf')},
                  body: JSON.stringify({
                    target_kind: 'report',
                    target_key: view.dataset_key,
                    purpose: view.print_purpose || 'report',
                    channel: view.print_channel || 'print',
                    format: 'html'
                  })
                });
                openPrintWindow(payload.output);
              } catch (err) {
                setStatus(err.message);
              }
            };
            zone.appendChild(printButton);
            const pdfButton = document.createElement('button');
            pdfButton.className = 'secondary';
            pdfButton.textContent = t('download_pdf');
            pdfButton.onclick = async () => {
              try {
                await downloadTemplatePDF({
                  target_kind: 'report',
                  target_key: view.dataset_key,
                  purpose: view.print_purpose || 'report',
                  channel: view.print_channel || 'print'
                });
              } catch (err) {
                setStatus(err.message);
              }
            };
            zone.appendChild(pdfButton);
          }
        }
        return;
      }
      renderJSONCard(pickText(view, 'title'), route);
    }

    async function invokeDocumentAction(documentID, action, expectedVersion, expectedETag) {
      const csrf = readCookie('orbyte_csrf');
      return api('/documents/' + encodeURIComponent(documentID) + '/actions', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
        body: JSON.stringify({action, expected_version: expectedVersion, expected_etag: expectedETag})
      });
    }

    async function resolvePrintTemplate(targetKind, targetKey, purpose, channel, organizationID, locationID) {
      try {
        let path = '/outputs/templates/resolve?target_kind=' + encodeURIComponent(targetKind) + '&target_key=' + encodeURIComponent(targetKey) + '&purpose=' + encodeURIComponent(purpose || '') + '&channel=' + encodeURIComponent(channel || 'print');
        if (organizationID) path += '&organization_id=' + encodeURIComponent(organizationID);
        if (locationID) path += '&location_id=' + encodeURIComponent(locationID);
        return await api(path);
      } catch (_) {
        return {resolved: false};
      }
    }

    function renderPrintPreviewShell(output) {
      const html = output && output.html ? output.html : '';
      return '<section class="preview-panel"><div class="page-header"><div><h3>' + escapeHTML(t('print_preview_title')) + '</h3><p class="status">' + escapeHTML((output && output.file_name) || '') + '</p></div></div><div class="page-body"><div class="template-preview-frame">' + html + '</div></div><div class="page-actions"><button type="button" id="preview-print-button">' + escapeHTML(t('print_document')) + '</button><button type="button" id="preview-close-button" class="secondary">' + escapeHTML(t('close_preview')) + '</button></div></section>';
    }

    function ensurePreviewOverlay() {
      let overlay = document.getElementById('preview-overlay');
      if (overlay) return overlay;
      overlay = document.createElement('div');
      overlay.id = 'preview-overlay';
      overlay.className = 'preview-overlay hidden';
      overlay.innerHTML = '<div class="preview-backdrop" id="preview-backdrop"></div><div class="preview-dialog"><div id="preview-content"></div></div>';
      document.body.appendChild(overlay);
      const backdrop = document.getElementById('preview-backdrop');
      if (backdrop) {
        backdrop.onclick = () => closePreviewOverlay();
      }
      return overlay;
    }

    function closePreviewOverlay() {
      const overlay = document.getElementById('preview-overlay');
      if (!overlay) return;
      overlay.classList.add('hidden');
      const content = document.getElementById('preview-content');
      if (content) content.innerHTML = '';
    }

    function openPrintWindow(output) {
      const win = window.open('', '_blank', 'noopener,noreferrer,width=980,height=760');
      if (!win) return;
      win.document.open();
      win.document.write('<!doctype html><html><head><meta charset="utf-8"><title>' + escapeHTML((output && output.file_name) || t('print_preview_title')) + '</title><link rel="stylesheet" href="/ui/assets/platform.css?v=' + encodeURIComponent(platformAssetVersion) + '"></head><body>' + ((output && output.html) || '') + '</body></html>');
      win.document.close();
      win.focus();
      win.print();
    }

    async function downloadTemplatePDF(renderRequest) {
      const csrf = readCookie('orbyte_csrf');
      const response = await fetch('/outputs/render', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
        body: JSON.stringify(Object.assign({}, renderRequest, {format: 'pdf'}))
      });
      if (!response.ok) {
        let message = 'render failed';
        try {
          const payload = await response.json();
          message = payload.error && payload.error.message ? payload.error.message : message;
        } catch (_) {}
        throw new Error(message);
      }
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = (response.headers.get('Content-Disposition') || '').match(/filename="([^"]+)"/)?.[1] || 'output.pdf';
      document.body.appendChild(link);
      link.click();
      link.remove();
      setTimeout(() => window.URL.revokeObjectURL(url), 1000);
    }

    async function previewTemplateOutput(renderRequest) {
      const payload = await api('/outputs/render', Object.assign({
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': readCookie('orbyte_csrf')},
        body: JSON.stringify(Object.assign({}, renderRequest, {format: 'html'}))
      }));
      const overlay = ensurePreviewOverlay();
      const content = document.getElementById('preview-content');
      if (content) content.innerHTML = renderPrintPreviewShell(payload.output);
      overlay.classList.remove('hidden');
      const printButton = document.getElementById('preview-print-button');
      const closeButton = document.getElementById('preview-close-button');
      if (printButton) {
        printButton.onclick = () => openPrintWindow(payload.output);
      }
      if (closeButton) {
        closeButton.onclick = () => closePreviewOverlay();
      }
    }

    async function resolveCustomPrintSupport(entry) {
      if (!entry || !entry.printable || !entry.print_target_kind || !entry.print_target_key) {
        return {resolved: false};
      }
      return resolvePrintTemplate(
        entry.print_target_kind,
        entry.print_target_key,
        entry.print_purpose || '',
        entry.print_channel || 'print'
      );
    }

