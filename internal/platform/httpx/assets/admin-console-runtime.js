// Shared admin runtime helpers, agent tooling, and auth settings.
    async function getJSON(url, options) {
      return orbyteGetJSON(url, options);
    }
    async function optionalJSON(url, options) {
      return orbyteOptionalJSON(url, options);
    }
    function activeAdminAgentSession() {
      return (adminState.agent.sessions || []).find((item) => item.id === adminState.agent.currentSessionId) || null;
    }
    function buildAdminAgentContextBlocks() {
      return [{
        key: 'current_page',
        label: 'Current Page',
        kind: 'route',
        selected: !!adminState.agent.attachContext,
        value: {
          shell: 'admin',
          route_path: adminCurrentPath(),
          route_title: document.querySelector('[data-admin-route="' + adminCurrentPath() + '"] h2, [data-admin-route="' + adminCurrentPath() + '"] h3') ? document.querySelector('[data-admin-route="' + adminCurrentPath() + '"] h2, [data-admin-route="' + adminCurrentPath() + '"] h3').textContent : '',
          actor_user_id: (adminState.bootstrap && adminState.bootstrap.current_user_id) || '',
          organization_id: (adminState.bootstrap && adminState.bootstrap.organization && adminState.bootstrap.organization.id) || ''
        }
      }];
    }
    function stopAdminAgentStream() {
      if (adminState.agent.stream) {
        adminState.agent.stream.close();
        adminState.agent.stream = null;
      }
    }
    function resetAdminAgentState() {
      stopAdminAgentStream();
      adminState.agent.open = false;
      adminState.agent.sessions = [];
      adminState.agent.currentSessionId = '';
      adminState.agent.stream = null;
    }
    async function loadAdminAgentSessions() {
      try {
        const payload = await getJSON('/agent/api/sessions');
        adminState.agent.sessions = payload.items || [];
        if (!adminState.agent.currentSessionId && adminState.agent.sessions.length) adminState.agent.currentSessionId = adminState.agent.sessions[0].id;
      } catch (_) {
        adminState.agent.sessions = [];
      }
    }
    async function refreshAdminAgentSession(sessionID) {
      if (!sessionID) return;
      try {
        const session = await getJSON('/agent/api/sessions/' + encodeURIComponent(sessionID));
        const idx = adminState.agent.sessions.findIndex((item) => item.id === session.id);
        if (idx >= 0) adminState.agent.sessions[idx] = session; else adminState.agent.sessions.unshift(session);
      } catch (_) {}
    }
    function ensureAdminAgentStream(sessionID) {
      if (!sessionID) return;
      if (adminState.agent.stream && adminState.agent.stream.url && adminState.agent.stream.url.indexOf(encodeURIComponent(sessionID)) >= 0) return;
      stopAdminAgentStream();
      const stream = new EventSource('/agent/api/sessions/' + encodeURIComponent(sessionID) + '/events');
      ['session_update', 'turn_completed', 'turn_failed', 'session_started'].forEach((kind) => {
        stream.addEventListener(kind, () => {
          void refreshAdminAgentSession(sessionID).then(() => {
            renderAdminAgentPanel();
            if (adminCurrentPath() === '/admin/agent') renderAdminAgentWorkspace();
          });
        });
      });
      adminState.agent.stream = stream;
    }
    async function startAdminAgentSession() {
      const providers = (((adminState.bootstrap || {}).acp || {}).providers || []).filter((item) => item.available);
      if (!providers.length) return null;
      const session = await getJSON('/agent/api/sessions', {
        method: 'POST',
        headers: {'Content-Type':'application/json', 'X-CSRF-Token': getCookie('orbyte_csrf')},
        body: JSON.stringify({
          provider_key: providers[0].key,
          shell: 'admin',
          route_path: adminCurrentPath(),
          title: 'Admin Agent',
          context_blocks: buildAdminAgentContextBlocks()
        })
      });
      adminState.agent.currentSessionId = session.id;
      await loadAdminAgentSessions();
      ensureAdminAgentStream(session.id);
      return session;
    }
    function adminAgentComposerNode() {
      return document.getElementById('admin-agent-panel-composer-input') || document.getElementById('admin-agent-workspace-composer-input');
    }
    async function sendAdminAgentPrompt() {
      const composer = adminAgentComposerNode();
      if (!composer) return;
      const content = String(composer.value || '').trim();
      if (!content) return;
      let sessionID = adminState.agent.currentSessionId;
      if (!sessionID) {
        const created = await startAdminAgentSession();
        sessionID = created && created.id;
      }
      if (!sessionID) return;
      composer.value = '';
      await getJSON('/agent/api/sessions/' + encodeURIComponent(sessionID) + '/prompt', {
        method: 'POST',
        headers: {'Content-Type':'application/json', 'X-CSRF-Token': getCookie('orbyte_csrf')},
        body: JSON.stringify({content: content, context_blocks: buildAdminAgentContextBlocks()})
      });
      await refreshAdminAgentSession(sessionID);
      renderAdminAgentPanel();
      if (adminCurrentPath() === '/admin/agent') renderAdminAgentWorkspace();
    }
    async function resolveAdminAgentApproval(sessionID, approvalID, action) {
      await getJSON('/agent/api/sessions/' + encodeURIComponent(sessionID) + '/approvals/' + encodeURIComponent(approvalID) + '/' + action, {
        method: 'POST',
        headers: {'X-CSRF-Token': getCookie('orbyte_csrf')}
      });
      await refreshAdminAgentSession(sessionID);
      renderAdminAgentPanel();
      if (adminCurrentPath() === '/admin/agent') renderAdminAgentWorkspace();
    }
    function adminAgentThreadMarkup(session) {
      const messages = (session && session.messages) || [];
      const approvals = (session && session.approvals) || [];
      const messageMarkup = messages.map((item) => '<article class="agent-bubble ' + escapeHTML(item.role) + '"><div class="meta">' + escapeHTML(item.role) + '</div><div>' + escapeHTML(item.content || '') + '</div></article>').join('');
      const approvalMarkup = approvals.length ? '<div class="agent-approval-list">' + approvals.map((item) => '<article class="agent-approval-item"><strong>' + escapeHTML(item.title || item.method || item.id) + '</strong><div class="status">' + escapeHTML(item.status || 'pending') + '</div><div class="actions">' + (item.status === 'pending' ? '<button type="button" data-agent-approval="' + escapeHTML(item.id) + '" data-agent-approval-action="approve">Approve</button><button type="button" class="secondary" data-agent-approval="' + escapeHTML(item.id) + '" data-agent-approval-action="reject">Reject</button>' : '') + '</div></article>').join('') + '</div>' : '';
      return messageMarkup || approvalMarkup ? messageMarkup + approvalMarkup : '<p class="status">Start a session to interact with an ACP agent.</p>';
    }
    function bindAdminAgentCommon(root) {
      if (!root) return;
      root.querySelectorAll('[data-agent-session]').forEach((node) => {
        node.addEventListener('click', () => {
          adminState.agent.currentSessionId = node.dataset.agentSession || '';
          ensureAdminAgentStream(adminState.agent.currentSessionId);
          void refreshAdminAgentSession(adminState.agent.currentSessionId).then(() => {
            renderAdminAgentPanel();
            if (adminCurrentPath() === '/admin/agent') renderAdminAgentWorkspace();
          });
        });
      });
      root.querySelectorAll('[data-agent-approval]').forEach((node) => {
        node.addEventListener('click', () => {
          const session = activeAdminAgentSession();
          if (!session) return;
          void resolveAdminAgentApproval(session.id, node.getAttribute('data-agent-approval') || '', node.getAttribute('data-agent-approval-action') || 'approve');
        });
      });
    }
    function renderAdminAgentWorkspace() {
      const root = document.getElementById('admin-agent-workspace');
      if (!root) return;
      const session = activeAdminAgentSession();
      const sessionsMarkup = (adminState.agent.sessions || []).map((item) => {
        const classes = item.id === adminState.agent.currentSessionId ? 'agent-session-item active' : 'agent-session-item';
        return '<button type="button" class="' + classes + '" data-agent-session="' + escapeHTML(item.id) + '"><div class="meta">' + escapeHTML(item.provider_name || item.provider_key) + '</div><strong>' + escapeHTML(item.title || item.route_path || item.id) + '</strong><div class="status">' + escapeHTML(item.status || 'ready') + '</div></button>';
      }).join('') || '<p class="status">No agent sessions yet.</p>';
      root.innerHTML = '<div class="agent-workspace-grid"><section class="panel"><h3>Sessions</h3><div class="agent-session-list">' + sessionsMarkup + '</div></section><section class="panel"><h3>Conversation</h3><div class="agent-thread">' + adminAgentThreadMarkup(session) + '</div><div class="agent-composer"><textarea id="admin-agent-workspace-composer-input" placeholder="Ask the agent about this admin context."></textarea><div class="agent-panel-actions"><button type="button" id="admin-agent-workspace-send-button"' + ((((adminState.bootstrap || {}).acp || {}).enabled ? '' : ' disabled')) + '>Send</button></div></div></section></div>';
      enhanceAdminControlAccessibility(root);
      const openPanel = document.getElementById('admin-agent-open-panel');
      if (openPanel) openPanel.onclick = () => { adminState.agent.open = true; renderAdminAgentPanel(); };
      const sendButton = document.getElementById('admin-agent-workspace-send-button');
      if (sendButton) sendButton.onclick = () => { void sendAdminAgentPrompt(); };
      bindAdminAgentCommon(root);
    }
    function renderAdminAgentPanel() {
      const panel = document.getElementById('admin-agent-panel');
      if (!panel) return;
      if (!adminState.agent.open) {
        panel.classList.add('hidden');
        return;
      }
      panel.classList.remove('hidden');
      const session = activeAdminAgentSession();
      const providers = ((((adminState.bootstrap || {}).acp || {}).providers) || []).filter((item) => item.available);
      const contextBlocks = buildAdminAgentContextBlocks();
      const sessionMarkup = (adminState.agent.sessions || []).slice(0, 6).map((item) => {
        const classes = item.id === adminState.agent.currentSessionId ? 'agent-session-item active' : 'agent-session-item';
        return '<button type="button" class="' + classes + '" data-agent-session="' + escapeHTML(item.id) + '">' + escapeHTML(item.title || item.route_path || item.id) + '</button>';
      }).join('');
      panel.innerHTML = '<div class="agent-panel-header"><div class="agent-panel-title"><strong>Agent</strong><span class="status">' + escapeHTML((session && session.provider_name) || (providers[0] && providers[0].name) || 'Unavailable') + '</span></div><div class="actions"><button type="button" id="admin-agent-go-workspace" class="secondary">Workspace</button><button type="button" id="admin-agent-close-panel" class="secondary">Close</button></div></div><div class="agent-context-list">' + contextBlocks.map((item) => '<label class="agent-context-item"><input type="checkbox" ' + (item.selected ? 'checked' : '') + ' id="admin-agent-context-toggle"> <strong>' + escapeHTML(item.label) + '</strong><div class="status">' + escapeHTML(item.value.route_path || '') + '</div></label>').join('') + '</div><div class="agent-panel-body"><div class="agent-session-list">' + sessionMarkup + '</div><div class="agent-thread">' + adminAgentThreadMarkup(session) + '</div></div><div class="agent-composer"><textarea id="admin-agent-panel-composer-input" placeholder="Ask the agent about this admin context."></textarea><div class="agent-panel-actions"><button type="button" id="admin-agent-panel-send-button"' + (providers.length ? '' : ' disabled') + '>Send</button></div></div>';
      enhanceAdminControlAccessibility(panel);
      const closeButton = document.getElementById('admin-agent-close-panel');
      if (closeButton) closeButton.onclick = () => { adminState.agent.open = false; renderAdminAgentPanel(); };
      const workspaceButton = document.getElementById('admin-agent-go-workspace');
      if (workspaceButton) workspaceButton.onclick = () => { window.location.hash = '#/admin/agent'; };
      const sendButton = document.getElementById('admin-agent-panel-send-button');
      if (sendButton) sendButton.onclick = () => { void sendAdminAgentPrompt(); };
      const contextToggle = document.getElementById('admin-agent-context-toggle');
      if (contextToggle) contextToggle.onchange = () => { adminState.agent.attachContext = !!contextToggle.checked; };
      bindAdminAgentCommon(panel);
    }
    async function bootstrapAdminAgent() {
      adminState.agent.providers = ((((adminState.bootstrap || {}).acp || {}).providers) || []).filter((item) => item.available);
      await loadAdminAgentSessions();
      const toggle = document.getElementById('admin-agent-toggle-button');
      if (toggle) {
        toggle.hidden = !((adminState.bootstrap && adminState.bootstrap.acp && adminState.bootstrap.acp.enabled) || false);
        toggle.onclick = () => {
          adminState.agent.open = !adminState.agent.open;
          renderAdminAgentPanel();
        };
      }
      if (adminState.agent.currentSessionId) ensureAdminAgentStream(adminState.agent.currentSessionId);
      renderAdminAgentPanel();
    }
    async function boot() {
      if (!adminState.bootstrap) {
        adminState.locale = normalizeLocale(navigator.language || 'en');
      }
      const [bootstrap, modules, definitions, roleTemplates, policyHooks, observability, authSettings, users, bindings, reportingLines, workflows, templateDefinitions, templateBindings] = await Promise.all([
        getJSON('/admin/api/bootstrap'),
        getJSON('/admin/api/modules'),
        getJSON('/admin/api/config/definitions'),
        getJSON('/admin/api/security/role-templates'),
        getJSON('/admin/api/security/policy-hooks'),
        getJSON('/admin/api/observability/contracts'),
        getJSON('/admin/api/auth/settings'),
        optionalJSON('/users'),
        optionalJSON('/role-bindings'),
        optionalJSON('/admin/api/reporting-lines'),
        optionalJSON('/admin/api/workflows'),
        optionalJSON('/admin/api/templates/definitions'),
        optionalJSON('/admin/api/template-bindings')
      ]);
      adminState.bootstrap = bootstrap;
      adminState.supportedLocales = bootstrap.supported_locales || ['en', 'id'];
      adminState.users = (users && users.items) || [];
      adminState.bindings = (bindings && bindings.items) || [];
      adminState.reportingLines = (reportingLines && reportingLines.items) || [];
      adminState.workflows = (workflows && workflows.items) || [];
      adminState.navigationManageAllowed = !!(users && bindings);
      adminState.templateDefinitions = (templateDefinitions && templateDefinitions.items) || [];
      adminState.templateBindings = (templateBindings && templateBindings.items) || [];
      if (bootstrap.locale) {
        adminState.locale = normalizeLocale(bootstrap.locale);
      }
      const uiLink = document.getElementById('admin-ui-link');
      if (uiLink) {
        uiLink.hidden = !bootstrap.ui_access;
        uiLink.href = bootstrap.ui_path || '/ui';
      }
      await bootstrapAdminAgent();
      renderLocaleSwitcher();
      renderAdminChrome();
      renderAdminRouteOptions();
      renderAdminMenus();
      applyAdminShellState();
      enhanceAdminControlAccessibility(document);
      document.getElementById('organization-id').innerHTML = '<option value="">' + t('default_option') + '</option><option value="' + bootstrap.organization.id + '">' + bootstrap.organization.name + '</option>';
      document.getElementById('location-id').innerHTML = '<option value="">' + t('default_option') + '</option>' + bootstrap.locations.map(loc => '<option value="' + loc.id + '">' + loc.name + '</option>').join('');
      renderModules(modules.items);
      renderDefinitions(definitions.items);
      renderHierarchyAdmin();
      void renderWorkflowAdmin();
      renderTemplates();
      renderAuthSettings(authSettings.entry.value);
      renderNavigationSettings();
      renderRoleTemplates(roleTemplates.items);
      renderPolicyHooks(policyHooks.items);
      renderObservability(observability);
      applyAdminRoute();
    }
    function boolValue(id) {
      return document.getElementById(id).value === 'true';
    }
    function csvValue(id) {
      return (document.getElementById(id).value || '').split(',').map(item => item.trim()).filter(Boolean);
    }
    function selectedScopeID(scope) {
      if (scope === 'deployment') return '';
      if (scope === 'organization') return document.getElementById('organization-id').value;
      return document.getElementById('location-id').value;
    }
    function renderProvisionScopeOptions(scopeType, selectedValue) {
      const target = document.getElementById('auth-google-auto-provision-scope-id');
      if (scopeType === 'deployment') {
        target.innerHTML = '<option value="">' + t('deployment_default') + '</option>';
        target.value = '';
        target.disabled = true;
        return;
      }
      if (scopeType === 'organization') {
        const org = adminState.bootstrap && adminState.bootstrap.organization;
        target.innerHTML = '<option value="">' + t('select_organization') + '</option>' + (org ? '<option value="' + org.id + '">' + org.name + ' (' + org.id + ')</option>' : '');
        target.disabled = false;
        target.value = selectedValue || '';
        return;
      }
      const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
      target.innerHTML = '<option value="">' + t('select_location') + '</option>' + locations.map(loc => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
      target.disabled = false;
      target.value = selectedValue || '';
    }
    function setDisabled(ids, disabled) {
      ids.forEach((id) => {
        const el = document.getElementById(id);
        if (el) el.disabled = disabled;
      });
    }
    function syncAuthSettingsState() {
      const googleEnabled = boolValue('auth-google-enabled');
      const autoProvisionEnabled = googleEnabled && boolValue('auth-google-auto-provision-enabled');
      setDisabled([
        'auth-google-button-label',
        'auth-google-client-id',
        'auth-google-client-secret',
        'auth-google-redirect-url',
        'auth-google-hosted-domain',
        'auth-google-auth-url',
        'auth-google-token-url',
        'auth-google-jwks-url',
        'auth-google-issuer',
        'auth-google-timeout-seconds',
        'auth-google-auto-provision-enabled'
      ], !googleEnabled);
      setDisabled([
        'auth-google-auto-provision-role-id',
        'auth-google-auto-provision-default-location-id',
        'auth-google-auto-provision-scope-type',
        'auth-google-auto-provision-allowed-domains'
      ], !autoProvisionEnabled);
      if (!autoProvisionEnabled) {
        document.getElementById('auth-google-auto-provision-scope-id').disabled = true;
      } else {
        renderProvisionScopeOptions(document.getElementById('auth-google-auto-provision-scope-type').value, document.getElementById('auth-google-auto-provision-scope-id').value);
      }
    }
    async function loadAuthSettingsValidation() {
      const orgID = document.getElementById('organization-id').value;
      const locationID = document.getElementById('location-id').value;
      const payload = await getJSON('/admin/api/config/validate?organization_id=' + encodeURIComponent(orgID) + '&location_id=' + encodeURIComponent(locationID));
      const issues = (payload.issues || []).filter((issue) => issue.key === 'identity.auth');
      document.getElementById('auth-settings-validation').textContent = issues.length
        ? JSON.stringify(issues, null, 2)
        : t('auth_validation_clear');
    }
    function renderAuthSettings(value) {
      value = value || {};
      const roles = (adminState.bootstrap && adminState.bootstrap.roles) || [];
      const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
      document.getElementById('auth-google-auto-provision-role-id').innerHTML = '<option value="">' + t('select_role') + '</option>' + roles.map(role => '<option value="' + role.id + '">' + role.name + ' (' + role.id + ')</option>').join('');
      document.getElementById('auth-google-auto-provision-default-location-id').innerHTML = '<option value="">' + t('default_location') + '</option>' + locations.map(loc => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
      document.getElementById('auth-password-enabled').value = String(value.password_enabled !== false);
      document.getElementById('auth-google-enabled').value = String(!!value.google_enabled);
      document.getElementById('auth-login-title').value = value.login_title || '';
      document.getElementById('auth-login-subtitle').value = value.login_subtitle || '';
      document.getElementById('auth-google-button-label').value = value.google_button_label || '';
      document.getElementById('auth-google-client-id').value = value.google_client_id || '';
      document.getElementById('auth-google-client-secret').value = value.google_client_secret || '';
      document.getElementById('auth-google-redirect-url').value = value.google_redirect_url || '';
      document.getElementById('auth-google-hosted-domain').value = value.google_hosted_domain || '';
      document.getElementById('auth-google-auth-url').value = value.google_auth_url || '';
      document.getElementById('auth-google-token-url').value = value.google_token_url || '';
      document.getElementById('auth-google-jwks-url').value = value.google_jwks_url || '';
      document.getElementById('auth-google-issuer').value = value.google_issuer || '';
      document.getElementById('auth-google-timeout-seconds').value = value.google_timeout_seconds || 5;
      document.getElementById('auth-google-auto-provision-enabled').value = String(!!value.google_auto_provision_enabled);
      document.getElementById('auth-google-auto-provision-role-id').value = value.google_auto_provision_role_id || '';
      document.getElementById('auth-google-auto-provision-default-location-id').value = value.google_auto_provision_default_location_id || '';
      document.getElementById('auth-google-auto-provision-scope-type').value = value.google_auto_provision_scope_type || 'deployment';
      renderProvisionScopeOptions(value.google_auto_provision_scope_type || 'deployment', value.google_auto_provision_scope_id || '');
      document.getElementById('auth-google-auto-provision-allowed-domains').value = (value.google_auto_provision_allowed_domains || []).join(', ');
      document.getElementById('load-auth-settings').onclick = loadAuthSettings;
      document.getElementById('save-auth-settings').onclick = saveAuthSettings;
      document.getElementById('auth-google-enabled').onchange = syncAuthSettingsState;
      document.getElementById('auth-google-auto-provision-enabled').onchange = syncAuthSettingsState;
      document.getElementById('auth-google-auto-provision-scope-type').onchange = () => {
        renderProvisionScopeOptions(document.getElementById('auth-google-auto-provision-scope-type').value, '');
        syncAuthSettingsState();
      };
      syncAuthSettingsState();
      void loadAuthSettingsValidation();
    }
    async function loadAuthSettings() {
      const orgID = document.getElementById('organization-id').value;
      const locationID = document.getElementById('location-id').value;
      const payload = await getJSON('/admin/api/auth/settings?organization_id=' + encodeURIComponent(orgID) + '&location_id=' + encodeURIComponent(locationID));
      renderAuthSettings(payload.entry.value);
      document.getElementById('auth-settings-status').textContent = t('loaded_auth_settings') + ' ' + payload.entry.source_scope + (payload.entry.source_scope_id ? ':' + payload.entry.source_scope_id : '');
      await loadAuthSettingsValidation();
    }
    async function saveAuthSettings() {
      const scope = document.getElementById('config-scope').value;
      const scopeID = selectedScopeID(scope);
      const value = {
        password_enabled: boolValue('auth-password-enabled'),
        login_title: document.getElementById('auth-login-title').value,
        login_subtitle: document.getElementById('auth-login-subtitle').value,
        google_button_label: document.getElementById('auth-google-button-label').value,
        google_enabled: boolValue('auth-google-enabled'),
        google_auto_provision_enabled: boolValue('auth-google-auto-provision-enabled'),
        google_auto_provision_allowed_domains: csvValue('auth-google-auto-provision-allowed-domains'),
        google_auto_provision_role_id: document.getElementById('auth-google-auto-provision-role-id').value,
        google_auto_provision_scope_type: document.getElementById('auth-google-auto-provision-scope-type').value,
        google_auto_provision_scope_id: document.getElementById('auth-google-auto-provision-scope-id').value,
        google_auto_provision_default_location_id: document.getElementById('auth-google-auto-provision-default-location-id').value,
        google_client_id: document.getElementById('auth-google-client-id').value,
        google_client_secret: document.getElementById('auth-google-client-secret').value,
        google_redirect_url: document.getElementById('auth-google-redirect-url').value,
        google_auth_url: document.getElementById('auth-google-auth-url').value,
        google_token_url: document.getElementById('auth-google-token-url').value,
        google_jwks_url: document.getElementById('auth-google-jwks-url').value,
        google_issuer: document.getElementById('auth-google-issuer').value,
        google_hosted_domain: document.getElementById('auth-google-hosted-domain').value,
        google_timeout_seconds: parseInt(document.getElementById('auth-google-timeout-seconds').value || '5', 10)
      };
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/admin/api/auth/settings', {
        method:'PUT',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({scope: scope, scope_id: scopeID, value: value})
      });
      renderAuthSettings(payload.entry.value);
      document.getElementById('auth-settings-status').textContent = t('saved_auth_settings') + ' ' + scope + (scopeID ? ':' + scopeID : '');
      await loadAuthSettingsValidation();
    }
