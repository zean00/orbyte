// Shared shell helpers reused by workspace and admin surfaces.
    function orbyteNormalizeLocale(locale) {
      const value = String(locale || '').trim().toLowerCase().replace(/_/g, '-');
      if (!value) return 'en';
      if (value === 'id' || value.indexOf('id-') === 0) return 'id';
      return 'en';
    }
    function orbyteEscapeHTML(value) {
      return String(value == null ? '' : value).replace(/[&<>"]/g, (char) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[char]));
    }
    function orbyteGetCookie(name) {
      const match = document.cookie.match(new RegExp('(^| )' + String(name || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '=([^;]+)'));
      return match ? decodeURIComponent(match[2]) : '';
    }
    async function orbyteGetJSON(url, options) {
      const resp = await fetch(url, Object.assign({credentials:'include'}, options || {}));
      if (!resp.ok) {
        const text = await resp.text();
        throw new Error(text || ('HTTP ' + resp.status));
      }
      return resp.json();
    }
    async function orbyteOptionalJSON(url, options) {
      const resp = await fetch(url, Object.assign({credentials:'include'}, options || {}));
      if (resp.status === 403 || resp.status === 404) {
        return null;
      }
      if (!resp.ok) {
        const text = await resp.text();
        throw new Error(text || ('HTTP ' + resp.status));
      }
      return resp.json();
    }
