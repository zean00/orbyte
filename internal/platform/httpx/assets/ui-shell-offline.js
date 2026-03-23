// Offline runtime, cached API helpers, preferences, and sync queue.
    async function registerServiceWorker() {
      if (!('serviceWorker' in navigator)) return;
      try {
        await navigator.serviceWorker.register('/ui/sw.js', {scope: '/ui/'});
      } catch (_) {}
    }

    function openOfflineDB() {
      return new Promise((resolve, reject) => {
        const request = indexedDB.open(offlineDBName, offlineDBVersion);
        request.onupgradeneeded = () => {
          const db = request.result;
          ['app_meta', 'contracts', 'reference_packages', 'projection_packages', 'drafts', 'sync_queue', 'sync_results', 'records'].forEach((storeName) => {
            if (!db.objectStoreNames.contains(storeName)) db.createObjectStore(storeName);
          });
        };
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error || new Error('indexeddb unavailable'));
      });
    }

    async function idbPut(storeName, key, value) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readwrite');
        tx.objectStore(storeName).put(value, key);
        tx.oncomplete = () => { db.close(); resolve(); };
        tx.onerror = () => { db.close(); reject(tx.error || new Error('indexeddb write failed')); };
      });
    }

    async function idbGet(storeName, key) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readonly');
        const req = tx.objectStore(storeName).get(key);
        req.onsuccess = () => { db.close(); resolve(req.result || null); };
        req.onerror = () => { db.close(); reject(req.error || new Error('indexeddb read failed')); };
      });
    }

    async function idbDelete(storeName, key) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readwrite');
        tx.objectStore(storeName).delete(key);
        tx.oncomplete = () => { db.close(); resolve(); };
        tx.onerror = () => { db.close(); reject(tx.error || new Error('indexeddb delete failed')); };
      });
    }

    async function idbClear(storeName) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readwrite');
        tx.objectStore(storeName).clear();
        tx.oncomplete = () => { db.close(); resolve(); };
        tx.onerror = () => { db.close(); reject(tx.error || new Error('indexeddb clear failed')); };
      });
    }

    async function idbEntries(storeName) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readonly');
        const req = tx.objectStore(storeName).getAll();
        req.onsuccess = () => { db.close(); resolve(req.result || []); };
        req.onerror = () => { db.close(); reject(req.error || new Error('indexeddb list failed')); };
      });
    }

    function cachedResponseKey(path) {
      return 'response:' + path;
    }

    function shouldCacheResponse(path, options) {
      const method = ((options && options.method) || 'GET').toUpperCase();
      if (method !== 'GET') return false;
      return path === '/auth/options' ||
        path === '/ui/bootstrap' ||
        path.indexOf('/ui/routes/resolve') === 0 ||
        path.indexOf('/ui/views/') === 0 ||
        path.indexOf('/ui/data/') === 0;
    }

    function isNetworkError(err) {
      const message = String((err && err.message) || err || '').toLowerCase();
      return !message || message.indexOf('failed to fetch') >= 0 || message.indexOf('network') >= 0 || message.indexOf('load failed') >= 0;
    }

    async function rememberResponse(path, payload, kind) {
      await idbPut('contracts', cachedResponseKey(path), {
        payload,
        kind,
        updated_at: new Date().toISOString()
      });
      state.cacheWarm = true;
      refreshOfflineStatus();
    }

    async function loadCachedResponse(path) {
      const cached = await idbGet('contracts', cachedResponseKey(path));
      return cached ? cached.payload : null;
    }

    async function api(path, options) {
      const requestOptions = Object.assign({credentials: 'same-origin'}, options || {});
      const method = String(requestOptions.method || 'GET').toUpperCase();
      if (method !== 'GET' && method !== 'HEAD') {
        const headers = Object.assign({}, requestOptions.headers || {});
        if (!headers['X-CSRF-Token']) {
          const csrf = readCookie('orbyte_csrf');
          if (csrf) headers['X-CSRF-Token'] = csrf;
        }
        requestOptions.headers = headers;
      }
      try {
        const response = await fetch(path, requestOptions);
        if (!response.ok) {
          let message = response.statusText;
          try {
            const payload = await response.json();
            message = payload.error && payload.error.message ? payload.error.message : message;
          } catch (_) {}
          throw new Error(message);
        }
        const contentType = response.headers.get('content-type') || '';
        const payload = contentType.includes('application/json') ? await response.json() : await response.text();
        if (shouldCacheResponse(path, requestOptions)) {
          await rememberResponse(path, payload, contentType.includes('application/json') ? 'json' : 'text');
        }
        return payload;
      } catch (err) {
        if (shouldCacheResponse(path, requestOptions) && (isNetworkError(err) || !navigator.onLine)) {
          const cached = await loadCachedResponse(path);
          if (cached != null) {
            setStatus(t('using_cached_data') + ' ' + path + '.');
            return cached;
          }
        }
        throw err;
      }
    }

    function currentPath() {
      const raw = window.location.hash.replace(/^#/, '');
      if (!raw) return '';
      const qIndex = raw.indexOf('?');
      return qIndex >= 0 ? raw.slice(0, qIndex) : raw;
    }

    function currentParams() {
      const raw = window.location.hash.replace(/^#/, '');
      const qIndex = raw.indexOf('?');
      return new URLSearchParams(qIndex >= 0 ? raw.slice(qIndex + 1) : '');
    }

    function currentSurface() {
      const params = new URLSearchParams(window.location.search);
      const value = String(params.get('surface') || state.surface || 'backoffice').trim().toLowerCase();
      if (value === 'worklist' || value === 'self_service' || value === 'pos' || value === 'mobile' || value === 'admin') return value;
      return 'backoffice';
    }

    function bootstrapURL() {
      return '/ui/bootstrap?surface=' + encodeURIComponent(currentSurface());
    }

    function preferenceStorageKey(pathname) {
      return 'orbyte:ui-prefs:' + currentSurface() + ':' + pathname;
    }

    function worklistFilterStorageKey(pathname, source) {
      return 'orbyte:worklist-filter:' + currentSurface() + ':' + pathname + ':' + source;
    }

    function readLocalUIPreferences(pathname) {
      try {
        const raw = window.localStorage.getItem(preferenceStorageKey(pathname));
        if (!raw) return null;
        const parsed = JSON.parse(raw);
        return parsed && typeof parsed === 'object' ? parsed : null;
      } catch (_) {
        return null;
      }
    }

    function writeLocalUIPreferences(pathname, prefs) {
      try {
        const payload = Object.assign({surface: currentSurface(), route_path: pathname}, prefs || {});
        const hasColumns = Array.isArray(payload.columns) && payload.columns.length > 0;
        const hasColumnOrder = Array.isArray(payload.column_order) && payload.column_order.length > 0;
        if ((!payload.filters || !Object.keys(payload.filters).length) && !hasColumns && !hasColumnOrder && !payload.density) {
          window.localStorage.removeItem(preferenceStorageKey(pathname));
          return;
        }
        window.localStorage.setItem(preferenceStorageKey(pathname), JSON.stringify(payload));
      } catch (_) {}
    }

    function readSavedWorklistFilter(pathname, source) {
      const prefs = readLocalUIPreferences(pathname);
      const filters = prefs && prefs.filters && prefs.filters[source];
      return filters && typeof filters === 'object' ? filters : null;
    }

    async function loadUIPreferences(pathname, viewKey) {
      const routePath = pathname || currentPath();
      if (!routePath) return {surface: currentSurface(), route_path: routePath, filters: {}, columns: [], column_order: [], density: 'comfortable'};
      try {
        const payload = await api('/me/preferences/ui?surface=' + encodeURIComponent(currentSurface()) + '&route_path=' + encodeURIComponent(routePath));
        const merged = Object.assign({filters: {}, columns: [], column_order: [], density: 'comfortable'}, payload || {});
        if (viewKey && !merged.view_key) merged.view_key = viewKey;
        writeLocalUIPreferences(routePath, merged);
        return merged;
      } catch (_) {
        const local = readLocalUIPreferences(routePath);
        return Object.assign({surface: currentSurface(), route_path: routePath, view_key: viewKey || '', filters: {}, columns: [], column_order: [], density: 'comfortable'}, local || {});
      }
    }

    async function saveUIPreferences(pathname, nextPrefs) {
      const routePath = pathname || currentPath();
      const payload = Object.assign({surface: currentSurface(), route_path: routePath, filters: {}, columns: [], column_order: [], density: 'comfortable'}, nextPrefs || {});
      writeLocalUIPreferences(routePath, payload);
      try {
        await api('/me/preferences/ui', {
          method: 'PUT',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify(payload)
        });
      } catch (_) {}
      setStatus(t('preferences_saved'));
      return payload;
    }

    async function saveWorklistFilterPreference(pathname, source, params, viewKey) {
      const current = await loadUIPreferences(pathname, viewKey);
      const filters = Object.assign({}, current.filters || {});
      const next = {};
      Object.keys(params || {}).forEach((key) => {
        if (params[key]) next[key] = params[key];
      });
      if (Object.keys(next).length) filters[source] = next; else delete filters[source];
      return saveUIPreferences(pathname, Object.assign({}, current, {view_key: viewKey || current.view_key || '', filters}));
    }

    function paramsFromObject(values) {
      const params = new URLSearchParams();
      Object.keys(values || {}).forEach((key) => {
        if (values[key]) params.set(key, values[key]);
      });
      return params;
    }

    function routeForWorkItem(item, fallbackDocumentType) {
      if (!item) return '';
      if (item.target_type === 'document') {
        const detailRoute = routeForDocument(item.document_type || fallbackDocumentType || 'generic_request', 'detail');
        if (!detailRoute || !item.target_id) return '';
        const params = new URLSearchParams();
        params.set('id', item.target_id);
        if (item.id) params.set('work_item_id', item.id);
        params.set('work_item_kind', item.stage_key ? 'approval' : 'task');
        return detailRoute + '?' + params.toString();
      }
      return '';
    }

    function setStatus(text) {
      document.getElementById('route-status').textContent = text;
    }

    function floorplanForRoute(route) {
      if (!route) return 'workspace';
      if (route.render_mode === 'flow') return 'editor';
      const view = route.view || {};
      if (view.kind === 'list' || view.kind === 'queue') return 'worklist';
      if (view.kind === 'detail') return 'object';
      if (view.kind === 'form') return 'editor';
      if (view.kind === 'dashboard') return 'dashboard';
      if (route.render_mode === 'custom') return 'dashboard';
      return 'workspace';
    }

    function renderFilterChips(params, keys) {
      const chips = keys.filter((key) => params.get(key)).map((key) => '<button type="button" class="filter-chip" data-clear-filter="' + escapeHTML(key) + '">' + escapeHTML(key.replace(/_/g, ' ')) + ': ' + escapeHTML(params.get(key)) + ' ×</button>');
      return chips.length ? '<div class="filter-chip-row">' + chips.join('') + '</div>' : '';
    }

    function updateRoutePanel(route) {
      const floorplanNode = document.getElementById('route-floorplan');
      const moduleNode = document.getElementById('route-module');
      if (floorplanNode) floorplanNode.textContent = floorplanForRoute(route);
      if (moduleNode) moduleNode.textContent = (route && route.module_key) || ((state.bootstrap && state.bootstrap.shell_kind) || 'workspace');
    }

    function setRouteState(kind, focusSelector) {
      state.routeState = kind;
      state.routeFocusSelector = focusSelector || '';
    }

    function focusRecoveryTarget(selector) {
      window.requestAnimationFrame(() => {
        const node = document.querySelector(selector || state.routeFocusSelector || '[data-route-focus]');
        if (!node) return;
        if (!node.hasAttribute('tabindex')) node.setAttribute('tabindex', '-1');
        node.focus();
      });
    }

    function routeFallbackPath(surface) {
      const targetSurface = surface || currentSurface();
      const paths = (state.bootstrap && state.bootstrap.fallback_paths) || {};
      return paths[targetSurface] || (targetSurface === state.surface ? state.bootstrap && state.bootstrap.default_path : '');
    }

    async function navigateToSurface(surface, preferredPath) {
      const params = new URLSearchParams(window.location.search);
      params.set('surface', surface || 'backoffice');
      const targetPath = preferredPath || ((state.bootstrap && state.bootstrap.fallback_paths && state.bootstrap.fallback_paths[surface]) || '');
      const target = '/ui?' + params.toString() + (targetPath ? '#' + targetPath : '');
      window.location.assign(target);
    }

    async function resolveCurrentRoute() {
      const path = currentPath() || (state.bootstrap && state.bootstrap.default_path) || '';
      if (!path) {
        return {status: 'not_found', requested_path: '', surface: currentSurface(), fallback_path: routeFallbackPath(currentSurface()), message: t('no_routes')};
      }
      return api('/ui/routes/resolve?path=' + encodeURIComponent(path) + '&surface=' + encodeURIComponent(state.surface || currentSurface()));
    }

    function refreshOfflineStatus() {
      const networkNode = document.getElementById('network-status');
      const syncNode = document.getElementById('sync-status');
      const cacheNode = document.getElementById('cache-status');
      if (networkNode) networkNode.textContent = navigator.onLine ? t('online') : t('offline');
      if (syncNode) syncNode.textContent = state.syncStats.pending + ' ' + t('sync_pending') + ' / ' + state.syncStats.conflict + ' ' + t('sync_conflict') + ' / ' + state.syncStats.failed + ' ' + t('sync_failed');
      if (cacheNode) cacheNode.textContent = state.cacheWarm ? t('cache_warm') : t('cache_cold');
    }

    function renderRecoveryPanel(title, message, actions) {
      const root = document.getElementById('view-root');
      root.innerHTML = '<section class="page-panel" aria-live="polite"><div class="page-header"><div><h3 data-route-focus>' + escapeHTML(title) + '</h3><p class="status">' + escapeHTML(message) + '</p></div></div><div class="page-actions" id="route-recovery-actions"></div></section>';
      const zone = document.getElementById('route-recovery-actions');
      (actions || []).forEach((action, index) => {
        const button = document.createElement('button');
        button.type = 'button';
        if (action.secondary) button.className = 'secondary';
        button.textContent = action.label;
        button.onclick = action.onClick;
        if (index === 0) button.dataset.routeFocus = '1';
        zone.appendChild(button);
      });
      setRouteState('recovery', '[data-route-focus]');
      focusRecoveryTarget('[data-route-focus]');
    }

    function bindGlobalShortcuts() {
      document.addEventListener('keydown', (event) => {
        if ((event.target && /input|textarea|select/i.test(event.target.tagName)) || event.defaultPrevented) return;
        if (event.key === '/') {
          const target = document.querySelector('[data-primary-filter]');
          if (target) {
            event.preventDefault();
            target.focus();
          }
          return;
        }
        if (event.key === 'g' && !event.metaKey && !event.ctrlKey && !event.altKey) {
          event.preventDefault();
          const fallback = routeFallbackPath(currentSurface());
          if (fallback) window.location.hash = '#' + fallback;
        }
      });
    }

    function authErrorFromQuery() {
      const params = new URLSearchParams(window.location.search);
      return params.get('auth_error') || '';
    }

	function loginTitle() {
		if (state.authOptions && state.authOptions['login_title_' + state.locale]) return state.authOptions['login_title_' + state.locale];
		if (state.authOptions && state.authOptions.login_title && !(state.locale !== 'en' && state.authOptions.login_title === 'Platform Access')) return state.authOptions.login_title;
		return t('login_title');
	}

	function loginSubtitle() {
		if (state.authOptions && state.authOptions['login_subtitle_' + state.locale]) return state.authOptions['login_subtitle_' + state.locale];
		if (state.authOptions && state.authOptions.login_subtitle && !(state.locale !== 'en' && state.authOptions.login_subtitle === 'Sign in to continue.')) return state.authOptions.login_subtitle;
		return t('login_subtitle');
	}

	function googleButtonLabel() {
		if (state.authOptions && state.authOptions['google_button_label_' + state.locale]) return state.authOptions['google_button_label_' + state.locale];
		if (state.authOptions && state.authOptions.google_button_label && !(state.locale !== 'en' && state.authOptions.google_button_label === 'Continue with Google')) return state.authOptions.google_button_label;
		return t('google_button');
	}

    function requestedUIRoute() {
      return currentPath();
    }

    function requestedNextPath() {
      const query = new URLSearchParams(window.location.search);
      const next = String(query.get('next') || '').trim();
      if (!next || next[0] !== '/') return '';
      if (next.startsWith('//')) return '';
      return next;
    }

    function requestedUIHref() {
      const next = requestedNextPath();
      if (next) return next;
      const path = requestedUIRoute();
      const query = new URLSearchParams(window.location.search);
      query.delete('next');
      query.set('surface', currentSurface());
      const base = '/ui' + (query.toString() ? '?' + query.toString() : '');
      if (!path) return base;
      const params = currentParams().toString();
      return base + '#' + path + (params ? '?' + params : '');
    }

    function offlineDocumentCapability(documentType) {
      return (state.offlineBootstrap && state.offlineBootstrap.documents || []).find((item) => item.type === documentType) || null;
    }

    function offlineModelCapability(modelKey) {
      return (state.offlineBootstrap && state.offlineBootstrap.models || []).find((item) => item.model_key === modelKey) || null;
    }

    function offlineProjectionCapabilityForView(view) {
      if (!state.offlineBootstrap || !state.offlineBootstrap.projections) return null;
      if (view.projection_key) {
        return state.offlineBootstrap.projections.find((item) => item.index_key.indexOf('documents.') === 0) || null;
      }
      if (view.model_key) {
        return state.offlineBootstrap.projections.find((item) => item.index_key.indexOf(view.model_key) >= 0 || item.title.toLowerCase().indexOf(view.model_key) >= 0) || null;
      }
      return null;
    }

    function draftKey(kind, targetKey, targetID) {
      return [kind, targetKey, targetID || 'new'].join(':');
    }

    async function loadDraft(kind, targetKey, targetID) {
      return idbGet('drafts', draftKey(kind, targetKey, targetID));
    }

    async function saveDraft(kind, targetKey, targetID, draft) {
      const key = draftKey(kind, targetKey, targetID);
      await idbPut('drafts', key, Object.assign({draft_key: key, updated_at: new Date().toISOString()}, draft));
      return key;
    }

    function queueKey(idempotencyKey) {
      return 'queue:' + idempotencyKey;
    }

    async function offlineDeviceID() {
      let deviceID = '';
      try {
        deviceID = window.localStorage.getItem('orbyte.offline.device_id') || '';
      } catch (_) {}
      if (!deviceID) {
        deviceID = 'offline-device-' + Math.random().toString(36).slice(2) + Date.now().toString(36);
        try {
          window.localStorage.setItem('orbyte.offline.device_id', deviceID);
        } catch (_) {}
      }
      return deviceID;
    }

    async function queueSyncItem(item) {
      const idempotencyKey = item.idempotency_key || ('sync-' + Date.now() + '-' + Math.random().toString(36).slice(2));
      item.idempotency_key = idempotencyKey;
      await idbPut('sync_queue', queueKey(idempotencyKey), Object.assign({
        queued_at: new Date().toISOString(),
        attempt_count: Number(item.attempt_count || 0),
        next_retry_at: item.next_retry_at || ''
      }, item));
      await refreshSyncStats();
      return item;
    }

    async function refreshSyncStats() {
      const queued = await idbEntries('sync_queue');
      const drafts = await idbEntries('drafts');
      state.syncStats.pending = queued.length;
      state.syncStats.conflict = drafts.filter((item) => item && item.status === 'conflict').length;
      state.syncStats.failed = drafts.filter((item) => item && (item.status === 'failed' || item.status === 'failed_retryable' || item.status === 'failed_terminal' || item.status === 'forbidden')).length;
      refreshOfflineStatus();
    }

    async function rememberProjectionPackage(pkg) {
      await idbPut('projection_packages', pkg.package_key, pkg);
    }

    async function rememberReferencePackage(pkg) {
      await idbPut('reference_packages', pkg.package_key, pkg);
    }

    async function prefetchOfflinePackages() {
      if (!state.offlineBootstrap) return;
      for (const item of (state.offlineBootstrap.references || [])) {
        try {
          const pkg = await api('/offline/packages/references', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({type_key: item.type_key})
          });
          await rememberReferencePackage(pkg);
        } catch (_) {}
      }
      for (const item of (state.offlineBootstrap.projections || [])) {
        try {
          const pkg = await api('/offline/packages/projections', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({index_key: item.index_key, query: {page: 1, page_size: 100, include_fields: item.default_include_fields || []}})
          });
          await rememberProjectionPackage(pkg);
        } catch (_) {}
      }
    }

    async function loadOfflineBootstrap() {
      const cached = await idbGet('app_meta', 'offline_bootstrap');
      try {
        state.offlineBootstrap = await api('/offline/bootstrap');
        if (cached && (cached.cache_token !== state.offlineBootstrap.cache_token || cached.schema_version !== state.offlineBootstrap.schema_version)) {
          await Promise.all([
            idbClear('reference_packages'),
            idbClear('projection_packages')
          ]);
        }
        await idbPut('app_meta', 'offline_bootstrap', state.offlineBootstrap);
        await prefetchOfflinePackages();
      } catch (err) {
        state.offlineBootstrap = cached;
      }
      return state.offlineBootstrap;
    }

    async function projectionFallback(view) {
      const capability = offlineProjectionCapabilityForView(view);
      if (!capability) return null;
      const pkg = await idbGet('projection_packages', 'projection:' + capability.index_key);
      if (!pkg || !pkg.result || !Array.isArray(pkg.result.hits)) return null;
      if (view.model_key) {
        return {
          items: pkg.result.hits.map((hit) => ({
            id: hit.source_id,
            model_key: view.model_key,
            values: Object.assign({}, hit.fields || {})
          })),
          total: pkg.result.total || pkg.result.hits.length
        };
      }
      return {
        items: pkg.result.hits.map((hit) => ({
          header: {
            id: hit.fields && (hit.fields.document_id || hit.source_id) || hit.source_id,
            type: hit.fields && hit.fields.document_type || view.document_type || '',
            status: hit.fields && hit.fields.status || '',
            updated_at: hit.fields && hit.fields.updated_at || '',
            etag: hit.fields && hit.fields.etag || '',
            version: hit.fields && hit.fields.version || 0
          },
          body: {payload: hit.fields || {}}
        })),
        total: pkg.result.total || pkg.result.hits.length
      };
    }

    async function processSyncQueue() {
      if (!navigator.onLine) {
        await refreshSyncStats();
        return;
      }
      const now = Date.now();
      const queued = (await idbEntries('sync_queue')).filter((item) => {
        if (!item) return false;
        if (!item.next_retry_at) return true;
        const retryAt = Date.parse(item.next_retry_at);
        return Number.isNaN(retryAt) || retryAt <= now;
      });
      if (!queued.length) {
        await refreshSyncStats();
        return;
      }
      try {
        const deviceID = await offlineDeviceID();
        const payload = await api('/offline/sync', {
          method: 'POST',
          headers: {'Content-Type': 'application/json', 'X-Offline-Device-ID': deviceID},
          body: JSON.stringify({items: queued})
        });
        const results = payload.items || [];
        for (const result of results) {
          const key = queueKey(result.idempotency_key);
          const queueItem = await idbGet('sync_queue', key);
          if (!queueItem) continue;
          const targetKey = queueItem.kind === 'model' ? queueItem.model_key : queueItem.document_type;
          const draft = await loadDraft(queueItem.kind, targetKey, queueItem.target_id);
          if (result.status === 'accepted') {
            if (draft) {
              draft.status = 'accepted';
              draft.target_id = result.target_id || draft.target_id || '';
              draft.version = result.version || draft.version || 0;
              draft.etag = result.etag || draft.etag || '';
              draft.last_error = '';
              draft.conflict = null;
              await saveDraft(queueItem.kind, targetKey, draft.target_id || queueItem.target_id, draft);
            }
            await idbDelete('sync_queue', key);
          } else if (result.status === 'conflict') {
            if (draft) {
              draft.status = 'conflict';
              draft.conflict = result.conflict || {};
              draft.last_error = result.error || '';
              await saveDraft(queueItem.kind, targetKey, queueItem.target_id, draft);
            }
            await idbDelete('sync_queue', key);
          } else if (result.status === 'failed_retryable') {
            queueItem.attempt_count = Number(result.attempt_count || queueItem.attempt_count || 0);
            queueItem.next_retry_at = result.retry_after || '';
            queueItem.last_error = result.error || 'sync failed';
            queueItem.last_result_status = result.status;
            await idbPut('sync_queue', key, queueItem);
            if (draft) {
              draft.status = 'failed_retryable';
              draft.last_error = result.error || 'sync failed';
              draft.retry_after = result.retry_after || '';
              await saveDraft(queueItem.kind, targetKey, queueItem.target_id, draft);
            }
          } else {
            if (draft) {
              draft.status = result.status || 'failed_terminal';
              draft.last_error = result.error || 'sync failed';
              draft.retry_after = result.retry_after || '';
              await saveDraft(queueItem.kind, targetKey, queueItem.target_id, draft);
            }
            await idbDelete('sync_queue', key);
          }
          await idbPut('sync_results', result.idempotency_key, result);
        }
      } catch (_) {}
      await refreshSyncStats();
    }

    async function performLogout() {
      const csrf = readCookie('orbyte_csrf');
      try {
        await fetch('/auth/logout', {
          method: 'POST',
          credentials: 'same-origin',
          headers: csrf ? {'X-CSRF-Token': csrf} : {}
        });
      } catch (_) {}
      state.bootstrap = null;
      state.route = null;
      state.offlineBootstrap = null;
      resetAgentState();
      if (document.getElementById('agent-toggle-button')) document.getElementById('agent-toggle-button').hidden = true;
      window.location.hash = '';
      applyShellLayout(false);
      renderLogin(loginSubtitle());
    }

