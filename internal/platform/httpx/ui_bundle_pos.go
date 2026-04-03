package httpx

func POSTerminalBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["pos-terminal"] = {
    render: async function(ctx) {
      const text = function(en, id) { return ctx.locale === "id" ? id : en; };
      const storageKey = "orbyte:pos-terminal";
      const mount = ctx.mount;
      const params = ctx.params || {};
      const initialCustomer = params.party_id ? { party_id: String(params.party_id), customer_name: String(params.party_name || params.party_id) } : null;
      const state = {
        loading: true,
        bootstrapping: false,
        lookupBusy: false,
        customerBusy: false,
        busy: false,
        message: "",
        bootstrap: null,
        storeCode: params.store_code || "",
        registerCode: params.register_code || "",
        shiftId: "",
        searchQuery: "",
        searchResults: [],
        cart: [],
        tenders: [{ tender_type_code: "", amount: 0, reference: "", notes: "" }],
        heldSales: [],
        transactions: [],
        lookupQuery: "",
        customerQuery: "",
        customerResults: [],
        customer: initialCustomer,
        promotionCodes: "",
        tenderInsights: {},
        catalogOpen: false,
        heldExpanded: false,
        transactionsExpanded: false,
      };

      const escapeHTML = function(value) {
        return String(value == null ? "" : value).replace(/[&<>"]/g, function(char) {
          return { "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;" }[char];
        });
      };
      const number = function(value) {
        const parsed = Number(value || 0);
        return Number.isFinite(parsed) ? parsed : 0;
      };
      const money = function(value) {
        return new Intl.NumberFormat(ctx.locale === "id" ? "id-ID" : "en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(number(value));
      };
      const clone = function(value) {
        return JSON.parse(JSON.stringify(value));
      };
      const readCookie = function(name) {
        const match = document.cookie.match(new RegExp("(?:^|; )" + name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "=([^;]*)"));
        return match ? decodeURIComponent(match[1]) : "";
      };
      const emitHardwareEvent = function(name, detail) {
        window.dispatchEvent(new CustomEvent(name, { detail: detail || {} }));
      };
      const notify = function(message, tone) {
        state.message = message || "";
        render();
        window.clearTimeout(notify._timer);
        notify._timer = window.setTimeout(function() {
          state.message = "";
          render();
        }, tone === "error" ? 5200 : 2800);
      };
      const persist = function() {
        try {
          localStorage.setItem(storageKey, JSON.stringify({
            storeCode: state.storeCode,
            registerCode: state.registerCode,
            shiftId: state.shiftId,
            cart: state.cart,
            tenders: state.tenders,
            promotionCodes: state.promotionCodes,
            customer: state.customer,
          }));
        } catch (_) {}
      };
      const restore = function() {
        try {
          const raw = localStorage.getItem(storageKey);
          if (!raw) return;
          const payload = JSON.parse(raw);
          if (payload && typeof payload === "object") {
            state.storeCode = state.storeCode || String(payload.storeCode || "");
            state.registerCode = state.registerCode || String(payload.registerCode || "");
            state.shiftId = state.shiftId || String(payload.shiftId || "");
            state.cart = Array.isArray(payload.cart) ? payload.cart : state.cart;
            state.tenders = Array.isArray(payload.tenders) && payload.tenders.length ? payload.tenders : state.tenders;
            state.promotionCodes = String(payload.promotionCodes || "");
            if (!state.customer && payload.customer && typeof payload.customer === "object") {
              state.customer = payload.customer;
            }
          }
        } catch (_) {}
      };

      const selectedStore = function() {
        const stores = ((state.bootstrap || {}).stores || []);
        return stores.find(function(item) { return String((item.values || {}).code || "") === state.storeCode; }) || null;
      };
      const selectedRegister = function() {
        const registers = ((state.bootstrap || {}).registers || []);
        return registers.find(function(item) { return String((item.values || {}).code || "") === state.registerCode; }) || null;
      };
      const tenderTypes = function() {
        return ((state.bootstrap || {}).tender_types || []).filter(function(item) {
          return String(((item.values || {}).status || "active")).toLowerCase() === "active";
        });
      };
      const tenderTypeByCode = function(code) {
        return tenderTypes().find(function(item) { return String((item.values || {}).code || "") === String(code || ""); }) || null;
      };
      const tenderKind = function(code) {
        const item = tenderTypeByCode(code);
        return item ? String((item.values || {}).kind || "") : "";
      };
      const tenderRequiresReference = function(code) {
        const item = tenderTypeByCode(code);
        return item ? String(((item.values || {}).requires_reference || "false")) === "true" : false;
      };
      const tenderRequiresParty = function(code) {
        const item = tenderTypeByCode(code);
        return item ? String(((item.values || {}).requires_party || "false")) === "true" : false;
      };
      const totals = function() {
        const subtotal = state.cart.reduce(function(sum, line) { return sum + number(line.line_subtotal || line.unit_price * line.quantity); }, 0);
        const tax = state.cart.reduce(function(sum, line) { return sum + number(line.tax_amount); }, 0);
        const total = state.cart.reduce(function(sum, line) { return sum + number(line.line_total || (line.line_subtotal + line.tax_amount)); }, 0);
        const tendered = state.tenders.reduce(function(sum, line) { return sum + number(line.amount); }, 0);
        return {
          subtotal: subtotal,
          tax: tax,
          total: total,
          tendered: tendered,
          change: Math.max(tendered - total, 0),
          due: Math.max(total - tendered, 0),
        };
      };

      const ensureStyles = function() {
        if (document.getElementById("pos-terminal-styles")) return;
        const style = document.createElement("style");
        style.id = "pos-terminal-styles";
        style.textContent = ""
          + ".pos-terminal { height:min(calc(100vh - 11rem), 84rem); min-height:46rem; display:grid; grid-template-rows:auto auto minmax(0,1fr); gap:1rem; color:var(--color-body); }"
          + ".pos-terminal__topbar, .pos-terminal__customerbar, .pos-terminal__panel { border:1px solid color-mix(in srgb, var(--color-line) 88%, #101317 12%); background:color-mix(in srgb, var(--color-surface) 96%, #f7f2ec 4%); box-shadow:var(--shadow-panel); }"
          + ".pos-terminal__topbar { display:grid; grid-template-columns:minmax(0,1.4fr) repeat(3,minmax(12rem,1fr)); gap:0.75rem; border-radius:1.2rem; padding:1rem 1.1rem; }"
          + ".pos-terminal__brand h2 { margin:0; font-size:1.45rem; line-height:1.1; letter-spacing:-0.02em; }"
          + ".pos-terminal__brand p { margin:0.3rem 0 0; color:var(--color-muted); max-width:40rem; }"
          + ".pos-terminal__chiprow { display:flex; flex-wrap:wrap; gap:0.45rem; margin-top:0.8rem; }"
          + ".pos-terminal__chip { display:inline-flex; align-items:center; gap:0.35rem; border-radius:999px; padding:0.38rem 0.7rem; background:#efe4d3; color:#6d371c; font-size:0.74rem; font-weight:800; letter-spacing:0.08em; text-transform:uppercase; }"
          + ".pos-terminal__chip--ok { background:#dae9dc; color:#1e5632; }"
          + ".pos-terminal__chip--warn { background:#f4e4d6; color:#9a4b17; }"
          + ".pos-terminal__chip--danger { background:#f3d8d8; color:#8a2121; }"
          + ".pos-terminal__meta { display:grid; gap:0.5rem; padding:0.95rem 1rem; border-radius:1rem; background:linear-gradient(180deg, color-mix(in srgb, var(--color-shell) 55%, var(--color-surface) 45%), var(--color-surface)); }"
          + ".pos-terminal__meta-label { font-size:0.72rem; font-weight:800; letter-spacing:0.12em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__meta-value { font-size:1rem; font-weight:800; }"
          + ".pos-terminal__meta-sub { color:var(--color-muted); font-size:0.84rem; }"
          + ".pos-terminal__customerbar { display:grid; gap:0.9rem; border-radius:1.2rem; padding:0.9rem 1rem; }"
          + ".pos-terminal__customer-top { display:grid; grid-template-columns:minmax(0,1.2fr) minmax(18rem,0.8fr); gap:0.9rem; align-items:start; }"
          + ".pos-terminal__row { display:flex; flex-wrap:wrap; gap:0.7rem; align-items:end; }"
          + ".pos-terminal__field { display:flex; flex-direction:column; gap:0.35rem; min-width:10rem; flex:1; }"
          + ".pos-terminal__field span { font-size:0.72rem; font-weight:800; letter-spacing:0.12em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__field input, .pos-terminal__field select, .pos-terminal__field textarea { width:100%; min-width:0; min-height:2.75rem; border:1px solid color-mix(in srgb, var(--color-line) 84%, #15191d 16%); border-radius:0.9rem; background:var(--color-surface); color:var(--color-body); padding:0.7rem 0.85rem; font:inherit; }"
          + ".pos-terminal__field textarea { min-height:5rem; resize:vertical; }"
          + ".pos-terminal__buttons { display:flex; flex-wrap:wrap; gap:0.55rem; }"
          + ".pos-terminal__button { appearance:none; border:1px solid color-mix(in srgb, var(--color-line) 88%, #1a1d21 12%); border-radius:0.9rem; background:var(--color-surface); color:var(--color-body); padding:0.8rem 1rem; font:inherit; font-weight:800; cursor:pointer; transition:background 120ms ease, border-color 120ms ease, color 120ms ease, transform 120ms ease; }"
          + ".pos-terminal__button:hover { border-color:var(--color-accent); color:var(--color-accent-dark); }"
          + ".pos-terminal__button:disabled { opacity:0.55; cursor:not-allowed; transform:none; }"
          + ".pos-terminal__button--primary { background:#a04d22; border-color:#a04d22; color:#fff; }"
          + ".pos-terminal__button--primary:hover { background:#8d4119; border-color:#8d4119; color:#fff; }"
          + ".pos-terminal__button--warn { border-color:#d45555; color:#a12d2d; }"
          + ".pos-terminal__button--soft { background:color-mix(in srgb, var(--color-accent-soft) 62%, var(--color-surface) 38%); color:#7b3a1d; border-color:color-mix(in srgb, var(--color-accent) 35%, var(--color-line) 65%); }"
          + ".pos-terminal__customer-results, .pos-terminal__promo-list { display:grid; gap:0.55rem; max-height:11rem; overflow:auto; }"
          + ".pos-terminal__customer-card, .pos-terminal__promo-chip { border:1px solid color-mix(in srgb, var(--color-line) 88%, #171b20 12%); border-radius:0.95rem; background:color-mix(in srgb, var(--color-shell) 42%, var(--color-surface)); padding:0.75rem 0.85rem; }"
          + ".pos-terminal__customer-card { display:flex; justify-content:space-between; gap:0.8rem; align-items:flex-start; }"
          + ".pos-terminal__customer-name { font-weight:800; }"
          + ".pos-terminal__customer-meta { color:var(--color-muted); font-size:0.83rem; margin-top:0.2rem; }"
          + ".pos-terminal__workspace { min-height:0; display:grid; grid-template-columns:minmax(0,1.7fr) minmax(22rem,25rem); gap:1rem; }"
          + ".pos-terminal__left { min-height:0; display:grid; grid-template-rows:minmax(0,1fr) auto; gap:1rem; }"
          + ".pos-terminal__aux { min-height:0; display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1fr); gap:1rem; align-content:start; }"
          + ".pos-terminal__rail { min-height:0; display:grid; grid-template-rows:auto minmax(0,1fr) auto; gap:1rem; }"
          + ".pos-terminal__panel { display:grid; grid-template-rows:auto minmax(0,1fr); min-height:0; border-radius:1.1rem; overflow:hidden; }"
          + ".pos-terminal__panel-head { display:flex; justify-content:space-between; gap:0.75rem; align-items:flex-start; padding:0.95rem 1rem; border-bottom:1px solid color-mix(in srgb, var(--color-line) 86%, #12161b 14%); }"
          + ".pos-terminal__panel-title { margin:0; font-size:0.92rem; font-weight:900; letter-spacing:0.1em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__panel-sub { color:var(--color-muted); font-size:0.83rem; margin-top:0.25rem; }"
          + ".pos-terminal__panel-body { min-height:0; padding:0.95rem 1rem; }"
          + ".pos-terminal__scroll { min-height:0; overflow:auto; }"
          + ".pos-terminal__result-list, .pos-terminal__held-list, .pos-terminal__txn-list, .pos-terminal__tender-list { display:grid; gap:0.7rem; }"
          + ".pos-terminal__result, .pos-terminal__held, .pos-terminal__txn, .pos-terminal__tender { border:1px solid color-mix(in srgb, var(--color-line) 85%, #161a1e 15%); border-radius:1rem; background:color-mix(in srgb, var(--color-shell) 34%, var(--color-surface)); padding:0.9rem; }"
          + ".pos-terminal__result-head, .pos-terminal__sale-head { display:flex; justify-content:space-between; gap:0.75rem; align-items:flex-start; }"
          + ".pos-terminal__title { font-weight:800; }"
          + ".pos-terminal__muted { color:var(--color-muted); font-size:0.84rem; }"
          + ".pos-terminal__statgrid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:0.6rem; }"
          + ".pos-terminal__stat { border-radius:0.95rem; padding:0.8rem; background:color-mix(in srgb, var(--color-shell) 55%, var(--color-surface)); border:1px solid color-mix(in srgb, var(--color-line) 84%, #161a1e 16%); }"
          + ".pos-terminal__stat label { display:block; font-size:0.7rem; font-weight:800; letter-spacing:0.1em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__stat strong { display:block; margin-top:0.2rem; font-size:1.2rem; }"
          + ".pos-terminal__cart-table { width:100%; border-collapse:collapse; table-layout:fixed; }"
          + ".pos-terminal__cart-table th, .pos-terminal__cart-table td { padding:0.72rem 0.45rem; border-top:1px solid color-mix(in srgb, var(--color-line) 86%, #171b20 14%); vertical-align:top; text-align:left; }"
          + ".pos-terminal__cart-table th { font-size:0.72rem; font-weight:900; letter-spacing:0.1em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__cart-table td:last-child, .pos-terminal__cart-table th:last-child { text-align:right; }"
          + ".pos-terminal__cart-table input { width:100%; min-width:0; min-height:2.35rem; border:1px solid color-mix(in srgb, var(--color-line) 84%, #171b20 16%); border-radius:0.8rem; background:var(--color-surface); color:var(--color-body); padding:0 0.65rem; font:inherit; }"
          + ".pos-terminal__summary { display:grid; gap:0.6rem; }"
          + ".pos-terminal__summary-row { display:flex; justify-content:space-between; gap:1rem; color:var(--color-body); }"
          + ".pos-terminal__summary-row strong { font-size:1.08rem; }"
          + ".pos-terminal__due { border-radius:1rem; padding:1rem; background:#f4e5d7; color:#6d3418; }"
          + ".pos-terminal__due strong { display:block; font-size:2rem; line-height:1; margin-top:0.25rem; }"
          + ".pos-terminal__due small { display:block; margin-top:0.25rem; color:#8b4c22; }"
          + ".pos-terminal__empty, .pos-terminal__setup { display:grid; place-items:center; min-height:100%; padding:1.2rem; text-align:center; border:1px dashed color-mix(in srgb, var(--color-line) 80%, #191d22 20%); border-radius:1rem; color:var(--color-muted); }"
          + ".pos-terminal__setup { background:linear-gradient(180deg, color-mix(in srgb, var(--color-accent-soft) 58%, var(--color-surface) 42%), var(--color-surface)); }"
          + ".pos-terminal__notice { padding:0.85rem 1rem; border-radius:1rem; background:color-mix(in srgb, var(--color-accent-soft) 58%, var(--color-surface) 42%); color:var(--color-body); border:1px solid color-mix(in srgb, var(--color-accent) 24%, var(--color-line) 76%); }"
          + ".pos-terminal__stack { display:grid; gap:0.75rem; }"
          + ".pos-terminal__toggle { appearance:none; border:0; background:transparent; color:var(--color-accent-dark); font:inherit; font-weight:800; cursor:pointer; padding:0.2rem 0; }"
          + ".pos-terminal__collapsed { display:grid; place-items:center; min-height:7rem; padding:1rem; text-align:center; color:var(--color-muted); }"
          + ".pos-terminal__catalog-trigger { display:flex; justify-content:space-between; gap:0.9rem; align-items:center; padding:1rem 1.05rem; border:1px dashed color-mix(in srgb, var(--color-line) 78%, #171b20 22%); border-radius:1rem; background:color-mix(in srgb, var(--color-shell) 45%, var(--color-surface)); }"
          + ".pos-terminal__catalog-meta { display:grid; gap:0.22rem; }"
          + ".pos-terminal__overlay { position:fixed; inset:0; z-index:80; display:grid; place-items:center; background:rgba(9, 12, 16, 0.46); padding:1.5rem; backdrop-filter:blur(5px); }"
          + ".pos-terminal__modal { width:min(72rem, calc(100vw - 3rem)); height:min(48rem, calc(100vh - 3rem)); display:grid; grid-template-rows:auto minmax(0,1fr); border:1px solid color-mix(in srgb, var(--color-line) 84%, #101317 16%); border-radius:1.3rem; background:color-mix(in srgb, var(--color-surface) 97%, #faf6f1 3%); box-shadow:0 2rem 5rem rgba(12, 16, 22, 0.24); overflow:hidden; }"
          + ".pos-terminal__modal-head { display:flex; justify-content:space-between; gap:0.9rem; align-items:flex-start; padding:1rem 1.1rem; border-bottom:1px solid color-mix(in srgb, var(--color-line) 86%, #12161b 14%); }"
          + ".pos-terminal__modal-body { min-height:0; display:grid; grid-template-rows:auto minmax(0,1fr); gap:0.95rem; padding:1rem 1.1rem 1.1rem; }"
          + ".pos-terminal__modal-body .pos-terminal__scroll { padding-right:0.25rem; }"
          + "@media (max-width: 1200px) { .pos-terminal { height:auto; min-height:0; } .pos-terminal__workspace { grid-template-columns:1fr; } .pos-terminal__rail { grid-template-rows:auto auto auto; } .pos-terminal__left { grid-template-rows:auto auto auto; } .pos-terminal__aux { grid-template-columns:1fr; } }"
          + "@media (max-width: 820px) { .pos-terminal__topbar, .pos-terminal__customer-top, .pos-terminal__statgrid { grid-template-columns:1fr; } .pos-terminal__overlay { padding:0.75rem; } .pos-terminal__modal { width:calc(100vw - 1.5rem); height:calc(100vh - 1.5rem); } }";
        document.head.appendChild(style);
      };

      async function api(path, options) {
        const request = options ? Object.assign({}, options) : {};
        const method = String(request.method || "GET").toUpperCase();
        const headers = Object.assign({}, request.headers || {});
        if (method !== "GET" && method !== "HEAD" && !headers["X-CSRF-Token"]) {
          headers["X-CSRF-Token"] = readCookie("orbyte_csrf");
        }
        request.headers = headers;
        return ctx.api(path, request);
      }

      function recalcLine(line) {
        line.quantity = Math.max(number(line.quantity), 0);
        line.unit_price = number(line.unit_price);
        line.discount_amount = Math.max(number(line.discount_amount), 0);
        line.tax_rate = number(line.tax_rate);
        const gross = Math.max(line.quantity * line.unit_price - line.discount_amount, 0);
        const taxMode = String(line.tax_mode || "exclusive").toLowerCase();
        if (taxMode === "inclusive" && line.tax_rate > 0) {
          line.line_subtotal = Math.round((gross / (1 + line.tax_rate / 100)) * 100) / 100;
          line.tax_amount = Math.round((gross - line.line_subtotal) * 100) / 100;
          line.line_total = Math.round(gross * 100) / 100;
        } else if (taxMode === "exempt") {
          line.line_subtotal = Math.round(gross * 100) / 100;
          line.tax_amount = 0;
          line.line_total = line.line_subtotal;
        } else {
          line.line_subtotal = Math.round(gross * 100) / 100;
          line.tax_amount = Math.round((line.line_subtotal * line.tax_rate / 100) * 100) / 100;
          line.line_total = Math.round((line.line_subtotal + line.tax_amount) * 100) / 100;
        }
        return line;
      }

      function payloadLines() {
        return state.cart.filter(function(line) { return number(line.quantity) > 0; }).map(function(line) {
          return {
            product_code: line.product_code || "",
            variant_signature: line.variant_signature || "",
            item_code: line.item_code || "",
            description: line.description || "",
            quantity: number(line.quantity),
            discount_amount: number(line.discount_amount),
            note: line.note || "",
          };
        });
      }

      function payloadPromotionCodes() {
        return String(state.promotionCodes || "").split(/[\n,;]+/).map(function(item) {
          return String(item || "").trim();
        }).filter(Boolean);
      }

      function payloadTenders() {
        return state.tenders.filter(function(tender) { return String(tender.tender_type_code || "").trim() !== "" && number(tender.amount) > 0; }).map(function(tender) {
          return {
            tender_type_code: tender.tender_type_code,
            amount: number(tender.amount),
            reference: tender.reference || "",
            notes: tender.notes || "",
          };
        });
      }

      function addCatalogItem(item) {
        const existing = state.cart.find(function(line) {
          return String(line.item_code || "") === String(item.item_code || "") && String(line.variant_signature || "") === String(item.variant_signature || "");
        });
        if (existing) {
          existing.quantity = number(existing.quantity) + 1;
          recalcLine(existing);
        } else {
          state.cart.push(recalcLine({
            product_code: item.product_code || "",
            variant_signature: item.variant_signature || "",
            item_code: item.item_code || "",
            description: item.description || item.name || item.item_code || "",
            quantity: 1,
            unit_price: number(item.unit_price),
            tax_code: item.tax_code || "",
            tax_rate: number(item.tax_rate),
            tax_mode: item.tax_mode || "exclusive",
            discount_amount: 0,
            inventory_enabled: !!item.inventory_enabled,
            available_quantity: number(item.available_quantity),
            line_subtotal: 0,
            tax_amount: 0,
            line_total: 0,
          }));
        }
        persist();
        render();
      }

      async function loadBootstrap() {
        state.bootstrapping = true;
        render();
        try {
          const query = new URLSearchParams();
          if (state.storeCode) query.set("store_code", state.storeCode);
          if (state.registerCode) query.set("register_code", state.registerCode);
          state.bootstrap = await api("/ui/data/pos/bootstrap" + (query.toString() ? "?" + query.toString() : ""));
          if (!state.storeCode && state.bootstrap.current_store && state.bootstrap.current_store.values) {
            state.storeCode = String(state.bootstrap.current_store.values.code || "");
          }
          if (!state.shiftId && state.bootstrap.open_shift && state.bootstrap.open_shift.id) {
            state.shiftId = String(state.bootstrap.open_shift.id || "");
          }
        } finally {
          state.bootstrapping = false;
        }
      }

      async function searchCatalog() {
        if (!state.storeCode) {
          notify(text("Select a store first.", "Pilih toko terlebih dahulu."), "error");
          return;
        }
        const query = new URLSearchParams();
        query.set("store_code", state.storeCode);
        query.set("q", state.searchQuery);
        const payload = await api("/ui/data/pos/catalog/search?" + query.toString());
        state.searchResults = payload.items || [];
        emitHardwareEvent("orbyte:pos-scanner-input", { query: state.searchQuery, matches: state.searchResults });
        render();
      }

      function openCatalog() {
        state.catalogOpen = true;
        render();
        window.requestAnimationFrame(function() {
          mount.querySelector("#pos-search")?.focus();
        });
      }

      function closeCatalog() {
        state.catalogOpen = false;
        render();
      }

      async function searchCustomers() {
        state.customerBusy = true;
        render();
        try {
          const query = new URLSearchParams();
          if (state.customerQuery) query.set("q", state.customerQuery);
          const payload = await api("/ui/data/pos/customers/search?" + query.toString());
          state.customerResults = payload.items || [];
        } catch (error) {
          notify(error instanceof Error ? error.message : text("Customer lookup failed.", "Lookup pelanggan gagal."), "error");
        } finally {
          state.customerBusy = false;
          render();
        }
      }

      function attachCustomer(item) {
        state.customer = item ? {
          party_id: String(item.party_id || ""),
          customer_name: String(item.customer_name || item.party_id || ""),
          member_status: String(item.member_status || ""),
          member_tier: String(item.member_tier || ""),
          member_valid_to: String(item.member_valid_to || ""),
          customer_type: String(item.customer_type || ""),
        } : null;
        state.customerResults = [];
        state.customerQuery = state.customer ? state.customer.customer_name : "";
        persist();
        maybeRefreshStoreCreditInsights();
        render();
      }

      async function lookupStoredValue(index) {
        const tender = state.tenders[index];
        if (!tender) return;
        const kind = tenderKind(tender.tender_type_code);
        if (kind !== "gift_card" && kind !== "store_credit") return;
        state.tenderInsights[String(index)] = { loading: true };
        render();
        try {
          const query = new URLSearchParams();
          query.set("kind", kind);
          if (kind === "gift_card") {
            query.set("reference", String(tender.reference || ""));
          } else {
            query.set("party_id", String((state.customer || {}).party_id || ""));
          }
          const payload = await api("/ui/data/pos/stored-value/lookup?" + query.toString());
          state.tenderInsights[String(index)] = payload.item || {};
        } catch (error) {
          state.tenderInsights[String(index)] = { error: error instanceof Error ? error.message : "Lookup failed" };
        }
        render();
      }

      function maybeRefreshStoreCreditInsights() {
        state.tenders.forEach(function(tender, index) {
          if (tenderKind(tender.tender_type_code) === "store_credit" && state.customer && state.customer.party_id) {
            lookupStoredValue(index).catch(function() {});
          }
        });
      }

      async function openShift() {
        if (!state.storeCode || !state.registerCode) {
          notify(text("Store and register are required.", "Toko dan register wajib diisi."), "error");
          return;
        }
        const opening = Number(window.prompt(text("Opening cash amount", "Jumlah kas awal"), "0") || "0");
        const payload = await api("/ui/data/pos/shifts/open", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            store_code: state.storeCode,
            register_code: state.registerCode,
            opening_cash_amount: Number.isFinite(opening) ? opening : 0,
          }),
        });
        state.shiftId = String(((payload || {}).record || {}).id || "");
        await loadBootstrap();
        persist();
        notify(text("Shift opened.", "Shift dibuka."));
        render();
      }

      async function closeShift() {
        if (!state.shiftId) {
          notify(text("No open shift.", "Tidak ada shift terbuka."), "error");
          return;
        }
        const actual = Number(window.prompt(text("Counted cash amount", "Jumlah kas aktual"), String(totals().tendered || 0)) || "0");
        await api("/ui/data/pos/shifts/" + encodeURIComponent(state.shiftId) + "/close", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ actual_cash_amount: Number.isFinite(actual) ? actual : 0 }),
        });
        state.shiftId = "";
        await loadBootstrap();
        persist();
        notify(text("Shift closed.", "Shift ditutup."));
        render();
      }

      async function loadHeldSales() {
        if (!state.registerCode || !state.shiftId) {
          state.heldSales = [];
          render();
          return;
        }
        const query = new URLSearchParams({ register_code: state.registerCode, shift_id: state.shiftId });
        const payload = await api("/ui/data/pos/sales/held?" + query.toString());
        state.heldSales = payload.items || [];
        render();
      }

      async function holdSale() {
        if (!state.shiftId) {
          notify(text("Open a shift first.", "Buka shift terlebih dahulu."), "error");
          return;
        }
        const payload = await api("/ui/data/pos/sales/hold", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            store_code: state.storeCode,
            register_code: state.registerCode,
            shift_id: state.shiftId,
            party_id: state.customer ? state.customer.party_id : "",
            party_name: state.customer ? state.customer.customer_name : "",
            lines: payloadLines(),
            tenders: payloadTenders(),
            promotion_codes: payloadPromotionCodes(),
            offline_cached: !navigator.onLine,
          }),
        });
        await loadHeldSales();
        state.cart = [];
        state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
        state.promotionCodes = "";
        state.tenderInsights = {};
        persist();
        notify(text("Sale held.", "Penjualan disimpan."));
        render();
        return payload;
      }

      function resumeHeldSale(record) {
        try {
          state.cart = JSON.parse(String(((record || {}).values || {}).lines_json || "[]"));
          state.tenders = JSON.parse(String(((record || {}).values || {}).tenders_json || "[{\"tender_type_code\":\"\",\"amount\":0}]"));
          state.promotionCodes = JSON.parse(String(((record || {}).values || {}).promotion_codes_json || "[]")).join(", ");
          if (!Array.isArray(state.tenders) || !state.tenders.length) {
            state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
          }
          state.customer = {
            party_id: String(((record || {}).values || {}).party_id || ""),
            customer_name: String(((record || {}).values || {}).party_name || text("Walk-in customer", "Pelanggan umum")),
          };
          state.cart = state.cart.map(recalcLine);
          state.tenderInsights = {};
          persist();
          maybeRefreshStoreCreditInsights();
          notify(text("Held sale loaded.", "Penjualan tertahan dimuat."));
          render();
        } catch (_) {
          notify(text("Failed to load held sale.", "Gagal memuat penjualan tertahan."), "error");
        }
      }

      async function checkout() {
        if (!navigator.onLine) {
          notify(text("Checkout requires a live connection.", "Checkout membutuhkan koneksi aktif."), "error");
          return;
        }
        if (!state.shiftId) {
          notify(text("Open a shift before checkout.", "Buka shift sebelum checkout."), "error");
          return;
        }
        if (!payloadLines().length) {
          notify(text("Add at least one line.", "Tambahkan minimal satu baris."), "error");
          return;
        }
        state.busy = true;
        render();
        try {
          const result = await api("/ui/data/pos/checkout", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              store_code: state.storeCode,
              register_code: state.registerCode,
              shift_id: state.shiftId,
              party_id: state.customer ? state.customer.party_id : "",
              party_name: state.customer ? state.customer.customer_name : "",
              lines: payloadLines(),
              tenders: payloadTenders(),
              promotion_codes: payloadPromotionCodes(),
              offline_cached: false,
            }),
          });
          emitHardwareEvent("orbyte:pos-receipt-print", result);
          emitHardwareEvent("orbyte:pos-cash-drawer-open", { total: totals().total, change: totals().change });
          state.cart = [];
          state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
          state.promotionCodes = "";
          state.tenderInsights = {};
          persist();
          await loadHeldSales();
          notify(text("Checkout completed.", "Checkout selesai."));
          render();
          if (window.confirm(text("Print receipt now?", "Cetak struk sekarang?"))) {
            window.print();
          }
        } catch (error) {
          notify(error instanceof Error ? error.message : text("Checkout failed.", "Checkout gagal."), "error");
        } finally {
          state.busy = false;
          render();
        }
      }

      async function lookupTransactions() {
        state.lookupBusy = true;
        render();
        try {
          const query = new URLSearchParams();
          if (state.lookupQuery) query.set("q", state.lookupQuery);
          if (state.storeCode) query.set("store_code", state.storeCode);
          if (state.registerCode) query.set("register_code", state.registerCode);
          const payload = await api("/ui/data/pos/transactions/search?" + query.toString());
          state.transactions = payload.items || [];
        } finally {
          state.lookupBusy = false;
          render();
        }
      }

      async function transactAction(saleID, action) {
        const payload = await api("/ui/data/pos/transactions/" + encodeURIComponent(saleID) + "/" + action, { method: "POST" });
        const label = action === "refund-store-credit"
          ? text("Store credit refund documents created.", "Dokumen refund store credit dibuat.")
          : action === "refund"
            ? text("Refund documents created.", "Dokumen refund dibuat.")
            : text("Exchange documents created.", "Dokumen tukar dibuat.");
        notify(label);
        emitHardwareEvent("orbyte:pos-payment-terminal-request", { action: action, payload: payload });
        await lookupTransactions();
      }

      function renderStoreOptions(items, valueKey, labelKey, current) {
        return ['<option value="">' + escapeHTML(text("Select", "Pilih")) + '</option>'].concat((items || []).map(function(item) {
          const values = item.values || {};
          const value = String(values[valueKey] || "");
          const selected = value === current ? " selected" : "";
          return '<option value="' + escapeHTML(value) + '"' + selected + ">" + escapeHTML(String(values[labelKey] || value)) + "</option>";
        })).join("");
      }

      function renderSetupState(bootstrap) {
        const missing = [];
        if (!(bootstrap.stores || []).length) missing.push(text("POS stores", "toko POS"));
        if (!(bootstrap.registers || []).length) missing.push(text("registers", "register"));
        if (!(bootstrap.tender_types || []).length) missing.push(text("tender types", "jenis tender"));
        return ''
          + '<section class="pos-terminal__setup">'
          +   '<div class="pos-terminal__stack">'
          +     '<strong>' + escapeHTML(text("Terminal setup is incomplete.", "Setup terminal belum lengkap.")) + '</strong>'
          +     '<div>' + escapeHTML(text("Create POS stores, registers, and tender types before the cashier can start selling.", "Buat toko POS, register, dan jenis tender sebelum kasir mulai berjualan.")) + '</div>'
          +     '<div class="pos-terminal__muted">' + escapeHTML(text("Missing: ", "Belum ada: ")) + escapeHTML(missing.join(", ")) + '</div>'
          +     '<div class="pos-terminal__buttons" style="justify-content:center">'
          +       '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-nav="/ui/pos/stores">' + escapeHTML(text("Open POS setup", "Buka setup POS")) + '</button>'
          +       '<button type="button" class="pos-terminal__button" data-nav="/ui/backoffice">' + escapeHTML(text("Back to backoffice", "Kembali ke backoffice")) + '</button>'
          +     '</div>'
          +   '</div>'
          + '</section>';
      }

      function renderCustomerSummary() {
        if (!state.customer) {
          return '<div class="pos-terminal__empty">' + escapeHTML(text("Attach a customer or member for loyalty, voucher, and store-credit flows.", "Pasang pelanggan atau member untuk flow loyalti, voucher, dan store credit.")) + '</div>';
        }
        return ''
          + '<div class="pos-terminal__customer-card">'
          +   '<div>'
          +     '<div class="pos-terminal__customer-name">' + escapeHTML(state.customer.customer_name || state.customer.party_id || "") + '</div>'
          +     '<div class="pos-terminal__customer-meta">' + escapeHTML(state.customer.party_id || "") + '</div>'
          +     '<div class="pos-terminal__chiprow">'
          +       (state.customer.member_status ? '<span class="pos-terminal__chip pos-terminal__chip--ok">' + escapeHTML(state.customer.member_status) + '</span>' : '')
          +       (state.customer.member_tier ? '<span class="pos-terminal__chip">' + escapeHTML(state.customer.member_tier) + '</span>' : '')
          +       (state.customer.customer_type ? '<span class="pos-terminal__chip">' + escapeHTML(state.customer.customer_type) + '</span>' : '')
          +     '</div>'
          +   '</div>'
          +   '<div class="pos-terminal__buttons">'
          +     '<button type="button" class="pos-terminal__button" data-action="clear-customer">' + escapeHTML(text("Clear", "Lepas")) + '</button>'
          +   '</div>'
          + '</div>';
      }

      function renderCustomerResults() {
        if (!state.customerResults.length) {
          return '<div class="pos-terminal__empty">' + escapeHTML(state.customerBusy ? text("Searching customers…", "Mencari pelanggan…") : text("Search by member ID, customer name, or party ID.", "Cari dengan ID member, nama pelanggan, atau ID party.")) + '</div>';
        }
        return '<div class="pos-terminal__customer-results">' + state.customerResults.map(function(item, index) {
          return ''
            + '<article class="pos-terminal__customer-card">'
            +   '<div>'
            +     '<div class="pos-terminal__customer-name">' + escapeHTML(item.customer_name || item.party_id || "") + '</div>'
            +     '<div class="pos-terminal__customer-meta">' + escapeHTML(String(item.party_id || "")) + '</div>'
            +     '<div class="pos-terminal__chiprow">'
            +       (item.member_status ? '<span class="pos-terminal__chip pos-terminal__chip--ok">' + escapeHTML(String(item.member_status || "")) + '</span>' : '')
            +       (item.member_tier ? '<span class="pos-terminal__chip">' + escapeHTML(String(item.member_tier || "")) + '</span>' : '')
            +     '</div>'
            +   '</div>'
            +   '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-attach-customer="' + String(index) + '">' + escapeHTML(text("Attach", "Pakai")) + '</button></div>'
            + '</article>';
        }).join("") + '</div>';
      }

      function renderCatalogModal() {
        if (!state.catalogOpen) return "";
        return ''
          + '<div class="pos-terminal__overlay" data-action="close-catalog-overlay">'
          +   '<section class="pos-terminal__modal" role="dialog" aria-modal="true" aria-label="' + escapeHTML(text("Catalog search", "Pencarian katalog")) + '">'
          +     '<div class="pos-terminal__modal-head">'
          +       '<div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Catalog search", "Pencarian katalog")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Search by barcode first, then add items directly into the basket.", "Cari barcode lebih dulu, lalu tambah item langsung ke basket.")) + '</div></div>'
          +       '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="close-catalog">' + escapeHTML(text("Close", "Tutup")) + '</button></div>'
          +     '</div>'
          +     '<div class="pos-terminal__modal-body">'
          +     '<div class="pos-terminal__row">'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Barcode or item", "Barcode atau item")) + '</span><input id="pos-search" name="pos_search" placeholder="' + escapeHTML(text("Scan barcode or type item name", "Scan barcode atau ketik nama barang")) + '" value="' + escapeHTML(state.searchQuery) + '"></div>'
          +       '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="search">' + escapeHTML(text("Search", "Cari")) + '</button></div>'
          +     '</div>'
          +     '<div class="pos-terminal__scroll">' + (state.searchResults.length ? '<div class="pos-terminal__result-list">' + state.searchResults.map(function(item, index) {
                return ''
                  + '<article class="pos-terminal__result">'
                  +   '<div class="pos-terminal__result-head">'
                  +     '<div><div class="pos-terminal__title">' + escapeHTML(item.name || item.item_code) + '</div><div class="pos-terminal__muted">' + escapeHTML((item.item_code || "") + (item.variant_label ? " • " + item.variant_label : "")) + '</div></div>'
                  +     '<div><strong>' + escapeHTML(money(item.unit_price)) + '</strong></div>'
                  +   '</div>'
                  +   '<div class="pos-terminal__muted" style="margin-top:0.55rem">' + escapeHTML(text("Available", "Tersedia")) + ': ' + escapeHTML(String(number(item.available_quantity))) + '</div>'
                  +   '<div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-add-result="' + String(index) + '">' + escapeHTML(text("Add to cart", "Tambah ke keranjang")) + '</button></div>'
                  + '</article>';
              }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("Search results appear here.", "Hasil pencarian muncul di sini.")) + '</div>') + '</div>'
          +     '</div>'
          +   '</section>'
          + '</div>';
      }

      function renderCartPanel() {
        return ''
          + '<article class="pos-terminal__panel">'
          +   '<div class="pos-terminal__panel-head">'
          +     '<div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Cart", "Keranjang")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Keep the basket clean before tendering.", "Pastikan keranjang rapi sebelum tender.")) + '</div></div>'
          +     '<div class="pos-terminal__buttons">' + (state.cart.length ? '<span class="pos-terminal__chip">' + escapeHTML(String(state.cart.length)) + " " + escapeHTML(text("lines", "baris")) + '</span>' : "") + '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="open-catalog">' + escapeHTML(text("Find items", "Cari item")) + '</button></div>'
          +   '</div>'
          +   '<div class="pos-terminal__panel-body" style="display:grid;grid-template-rows:auto minmax(0,1fr);gap:0.9rem">'
          +     '<div class="pos-terminal__catalog-trigger"><div class="pos-terminal__catalog-meta"><strong>' + escapeHTML(text("Need another item?", "Butuh item lain?")) + '</strong><div class="pos-terminal__muted">' + escapeHTML(text("Open catalog search in a full overlay so the basket stays maximized.", "Buka pencarian katalog dalam overlay penuh supaya keranjang tetap dominan.")) + '</div></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="open-catalog">' + escapeHTML(text("Open catalog", "Buka katalog")) + '</button></div></div>'
          +     '<div class="pos-terminal__scroll">' + (state.cart.length ? '<table class="pos-terminal__cart-table"><thead><tr><th style="width:34%">' + escapeHTML(text("Item", "Item")) + '</th><th style="width:13%">' + escapeHTML(text("Qty", "Qty")) + '</th><th style="width:16%">' + escapeHTML(text("Price", "Harga")) + '</th><th style="width:16%">' + escapeHTML(text("Discount", "Diskon")) + '</th><th style="width:13%">' + escapeHTML(text("Total", "Total")) + '</th><th style="width:8%"></th></tr></thead><tbody>' + state.cart.map(function(line, index) {
                return '<tr><td><div class="pos-terminal__title">' + escapeHTML(line.description || line.item_code) + '</div><div class="pos-terminal__muted">' + escapeHTML(String(line.item_code || "")) + '</div></td><td><input id="pos-line-qty-' + String(index) + '" name="pos_line_qty_' + String(index) + '" type="number" min="0" step="1" data-line-qty="' + String(index) + '" value="' + escapeHTML(String(line.quantity || 0)) + '"></td><td><input id="pos-line-price-' + String(index) + '" name="pos_line_price_' + String(index) + '" type="number" min="0" step="0.01" data-line-price="' + String(index) + '" value="' + escapeHTML(String(line.unit_price || 0)) + '"></td><td><input id="pos-line-discount-' + String(index) + '" name="pos_line_discount_' + String(index) + '" type="number" min="0" step="0.01" data-line-discount="' + String(index) + '" value="' + escapeHTML(String(line.discount_amount || 0)) + '"></td><td><strong>' + escapeHTML(money(line.line_total || 0)) + '</strong></td><td><button type="button" class="pos-terminal__button pos-terminal__button--warn" data-remove-line="' + String(index) + '">' + escapeHTML(text("Remove", "Hapus")) + '</button></td></tr>';
              }).join("") + '</tbody></table>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No items in cart yet.", "Belum ada item di keranjang.")) + '</div>') + '</div>'
          +   '</div>'
          + '</article>';
      }

      function renderAuxPanel() {
        return ''
          + '<div class="pos-terminal__aux">'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Held sales", "Penjualan tertahan")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Resume suspended baskets without leaving the terminal.", "Lanjutkan basket tertahan tanpa keluar dari terminal.")) + '</div></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__toggle" data-action="toggle-held">' + escapeHTML(state.heldExpanded ? text("Collapse", "Tutup") : text("Expand", "Buka")) + '</button></div></div>'
          +     (state.heldExpanded
                ? '<div class="pos-terminal__panel-body pos-terminal__scroll">' + (state.heldSales.length ? '<div class="pos-terminal__held-list">' + state.heldSales.map(function(item, index) {
                const values = item.values || {};
                return '<article class="pos-terminal__held"><div class="pos-terminal__sale-head"><div><div class="pos-terminal__title">' + escapeHTML(String(values.sale_number || item.id || "")) + '</div><div class="pos-terminal__muted">' + escapeHTML(String(values.party_name || text("Walk-in customer", "Pelanggan umum"))) + '</div></div><strong>' + escapeHTML(money(values.total_amount || 0)) + '</strong></div><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-resume-held="' + String(index) + '">' + escapeHTML(text("Resume", "Lanjutkan")) + '</button></div></article>';
              }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No held sales for this register.", "Tidak ada penjualan tertahan untuk register ini.")) + '</div>') + '</div>'
                : '<div class="pos-terminal__collapsed">' + escapeHTML(text("Held sales stay tucked away until the cashier needs to resume a basket.", "Penjualan tertahan disimpan rapat sampai kasir perlu melanjutkan basket.")) + '</div>')
          +   '</article>'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Transaction lookup", "Pencarian transaksi")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Refund, exchange, or convert to store credit from here.", "Refund, tukar, atau ubah ke store credit dari sini.")) + '</div></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__toggle" data-action="toggle-transactions">' + escapeHTML(state.transactionsExpanded ? text("Collapse", "Tutup") : text("Expand", "Buka")) + '</button></div></div>'
          +     (state.transactionsExpanded
                ? '<div class="pos-terminal__panel-body" style="display:grid;grid-template-rows:auto minmax(0,1fr);gap:0.9rem">'
          +       '<div class="pos-terminal__row"><div class="pos-terminal__field"><span>' + escapeHTML(text("Lookup", "Cari")) + '</span><input id="pos-lookup" name="pos_lookup" value="' + escapeHTML(state.lookupQuery) + '" placeholder="' + escapeHTML(text("Sale number, customer, invoice", "Nomor jual, pelanggan, invoice")) + '"></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="lookup-transactions">' + escapeHTML(state.lookupBusy ? text("Loading…", "Memuat…") : text("Lookup", "Cari")) + '</button></div></div>'
          +       '<div class="pos-terminal__scroll">' + (state.transactions.length ? '<div class="pos-terminal__txn-list">' + state.transactions.map(function(item, index) {
                const values = item.values || {};
                return '<article class="pos-terminal__txn"><div class="pos-terminal__sale-head"><div><div class="pos-terminal__title">' + escapeHTML(String(values.sale_number || "")) + '</div><div class="pos-terminal__muted">' + escapeHTML(String(values.invoice_number || values.order_number || "")) + (values.party_name ? " • " + escapeHTML(String(values.party_name || "")) : "") + '</div></div><strong>' + escapeHTML(money(values.total_amount || 0)) + '</strong></div><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-refund-sale="' + String(index) + '">' + escapeHTML(text("Refund", "Refund")) + '</button><button type="button" class="pos-terminal__button" data-exchange-sale="' + String(index) + '">' + escapeHTML(text("Exchange", "Tukar")) + '</button><button type="button" class="pos-terminal__button pos-terminal__button--soft" data-refund-store-credit="' + String(index) + '">' + escapeHTML(text("Store credit", "Store credit")) + '</button></div></article>';
              }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No transactions loaded yet.", "Belum ada transaksi dimuat.")) + '</div>') + '</div>'
          +     '</div>'
                : '<div class="pos-terminal__collapsed">' + escapeHTML(text("Transaction history stays collapsed until the cashier needs refunds or exchanges.", "Riwayat transaksi disimpan tertutup sampai kasir perlu refund atau tukar.")) + '</div>')
          +   '</article>'
          + '</div>';
      }

      function renderTenderCard(line, index) {
        const kind = tenderKind(line.tender_type_code);
        const insight = state.tenderInsights[String(index)] || null;
        const statusBlock = kind === "gift_card" || kind === "store_credit"
          ? '<div class="pos-terminal__muted" style="margin-top:0.65rem">'
              + (insight && insight.loading ? escapeHTML(text("Checking balance…", "Memeriksa saldo…"))
                : insight && insight.error ? escapeHTML(String(insight.error || ""))
                : insight && kind === "gift_card" ? escapeHTML(text("Balance", "Saldo")) + ": " + escapeHTML(money(insight.remaining_balance || 0)) + " • " + escapeHTML(String(insight.status || ""))
                : insight && kind === "store_credit" ? escapeHTML(text("Available", "Tersedia")) + ": " + escapeHTML(money(insight.balance_amount || 0)) + " • " + escapeHTML(String(insight.status || ""))
                : kind === "gift_card" ? escapeHTML(text("Enter the gift card code, then check its balance.", "Masukkan kode gift card, lalu cek saldonya."))
                : escapeHTML(state.customer && state.customer.party_id ? text("Load the attached customer's balance before checkout.", "Muat saldo pelanggan terpasang sebelum checkout.") : text("Attach a customer before using store credit.", "Pasang pelanggan sebelum memakai store credit.")))
            + '</div>'
          : kind === "voucher"
            ? '<div class="pos-terminal__muted" style="margin-top:0.65rem">' + escapeHTML(text("Use the promo / voucher field to capture voucher codes. Reference can store the issuer slip or approval number.", "Gunakan field promo / voucher untuk memasukkan kode voucher. Referensi bisa menyimpan nomor slip atau approval.")) + '</div>'
            : '';
        return ''
          + '<article class="pos-terminal__tender">'
          +   '<div class="pos-terminal__sale-head"><div><div class="pos-terminal__title">' + escapeHTML(text("Tender", "Tender")) + " " + escapeHTML(String(index + 1)) + '</div><div class="pos-terminal__muted">' + escapeHTML(kind || text("Select a tender type", "Pilih jenis tender")) + '</div></div><button type="button" class="pos-terminal__button" data-remove-tender="' + String(index) + '">' + escapeHTML(text("Remove", "Hapus")) + '</button></div>'
          +   '<div class="pos-terminal__row" style="margin-top:0.8rem">'
          +     '<div class="pos-terminal__field"><span>' + escapeHTML(text("Tender type", "Jenis tender")) + '</span><select id="pos-tender-type-' + String(index) + '" name="pos_tender_type_' + String(index) + '" data-tender-type="' + String(index) + '">' + ['<option value="">' + escapeHTML(text("Select", "Pilih")) + '</option>'].concat(tenderTypes().map(function(item) {
                  const code = String((item.values || {}).code || "");
                  const selected = code === String(line.tender_type_code || "") ? " selected" : "";
                  return '<option value="' + escapeHTML(code) + '"' + selected + ">" + escapeHTML(String((item.values || {}).name || code)) + "</option>";
                })).join("") + '</select></div>'
          +     '<div class="pos-terminal__field"><span>' + escapeHTML(text("Amount", "Jumlah")) + '</span><input id="pos-tender-amount-' + String(index) + '" name="pos_tender_amount_' + String(index) + '" type="number" min="0" step="0.01" data-tender-amount="' + String(index) + '" value="' + escapeHTML(String(line.amount || 0)) + '"></div>'
          +   '</div>'
          +   '<div class="pos-terminal__row" style="margin-top:0.7rem">'
          +     '<div class="pos-terminal__field"><span>' + escapeHTML(tenderRequiresReference(line.tender_type_code) ? text("Reference (required)", "Referensi (wajib)") : text("Reference", "Referensi")) + '</span><input id="pos-tender-reference-' + String(index) + '" name="pos_tender_reference_' + String(index) + '" type="text" data-tender-reference="' + String(index) + '" value="' + escapeHTML(String(line.reference || "")) + '" placeholder="' + escapeHTML(kind === "gift_card" ? text("Gift card code", "Kode gift card") : kind === "voucher" ? text("Voucher reference", "Referensi voucher") : text("Optional reference", "Referensi opsional")) + '"></div>'
          +   '</div>'
          +   statusBlock
          +   ((kind === "gift_card") || (kind === "store_credit" && state.customer && state.customer.party_id) ? '<div class="pos-terminal__buttons" style="margin-top:0.7rem"><button type="button" class="pos-terminal__button" data-check-tender="' + String(index) + '">' + escapeHTML(kind === "gift_card" ? text("Check balance", "Cek saldo") : text("Load balance", "Muat saldo")) + '</button></div>' : '')
          + '</article>';
      }

      function renderRail(totalsPayload) {
        const activeStore = selectedStore();
        const activeRegister = selectedRegister();
        return ''
          + '<aside class="pos-terminal__rail">'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Checkout summary", "Ringkasan checkout")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(state.shiftId ? text("Shift is open and ready to settle.", "Shift terbuka dan siap settle.") : text("Open a shift before accepting payments.", "Buka shift sebelum menerima pembayaran.")) + '</div></div></div>'
          +     '<div class="pos-terminal__panel-body">'
          +       '<div class="pos-terminal__due"><span class="pos-terminal__meta-label">' + escapeHTML(text("Amount due", "Jumlah kurang")) + '</span><strong>' + escapeHTML(money(totalsPayload.due > 0 ? totalsPayload.due : totalsPayload.total)) + '</strong><small>' + escapeHTML(totalsPayload.change > 0 ? text("Change due: ", "Kembalian: ") + money(totalsPayload.change) : text("Tender until due becomes zero.", "Tender hingga nilai kurang menjadi nol.")) + '</small></div>'
          +       '<div class="pos-terminal__summary" style="margin-top:0.95rem">'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Store", "Toko")) + '</span><span>' + escapeHTML(String((activeStore && activeStore.values && activeStore.values.name) || state.storeCode || "—")) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Register", "Register")) + '</span><span>' + escapeHTML(String((activeRegister && activeRegister.values && activeRegister.values.name) || state.registerCode || "—")) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Subtotal", "Subtotal")) + '</span><span>' + escapeHTML(money(totalsPayload.subtotal)) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Tax", "Pajak")) + '</span><span>' + escapeHTML(money(totalsPayload.tax)) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><strong>' + escapeHTML(text("Total", "Total")) + '</strong><strong>' + escapeHTML(money(totalsPayload.total)) + '</strong></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Tendered", "Dibayar")) + '</span><span>' + escapeHTML(money(totalsPayload.tendered)) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Change", "Kembalian")) + '</span><span>' + escapeHTML(money(totalsPayload.change)) + '</span></div>'
          +       '</div>'
          +     '</div>'
          +   '</article>'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Tenders", "Tender")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Keep cash, voucher, gift card, and store credit flows explicit.", "Buat flow kas, voucher, gift card, dan store credit tetap jelas.")) + '</div></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="add-tender">' + escapeHTML(text("Add tender", "Tambah tender")) + '</button></div></div>'
          +     '<div class="pos-terminal__panel-body pos-terminal__scroll">' + '<div class="pos-terminal__tender-list">' + state.tenders.map(renderTenderCard).join("") + '</div></div>'
          +   '</article>'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-body">'
          +       '<div class="pos-terminal__buttons" style="justify-content:space-between">'
          +         '<button type="button" class="pos-terminal__button" data-action="hold">' + escapeHTML(text("Hold sale", "Tahan penjualan")) + '</button>'
          +         '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="checkout"' + (state.busy ? " disabled" : "") + '>' + escapeHTML(state.busy ? text("Processing…", "Memproses…") : text("Complete sale", "Selesaikan penjualan")) + '</button>'
          +       '</div>'
          +     '</div>'
          +   '</article>'
          + '</aside>';
      }

      function renderTopBar() {
        const bootstrap = state.bootstrap || { stores: [], registers: [], tender_types: [] };
        return ''
          + '<section class="pos-terminal__topbar">'
          +   '<div class="pos-terminal__brand">'
          +     '<h2>' + escapeHTML(text("Counter terminal", "Terminal counter")) + '</h2>'
          +     '<p>' + escapeHTML(text("Designed for fast cashier work: search, basket control, tenders, and post-sale actions stay visible without page-length scrolling.", "Dirancang untuk kerja kasir cepat: pencarian, kontrol basket, tender, dan aksi pasca-penjualan tetap terlihat tanpa scroll panjang.")) + '</p>'
          +     '<div class="pos-terminal__chiprow">'
          +       '<span class="pos-terminal__chip ' + (navigator.onLine ? 'pos-terminal__chip--ok' : 'pos-terminal__chip--danger') + '">' + escapeHTML(navigator.onLine ? text("Online", "Online") : text("Offline", "Offline")) + '</span>'
          +       '<span class="pos-terminal__chip ' + (state.shiftId ? 'pos-terminal__chip--ok' : 'pos-terminal__chip--warn') + '">' + escapeHTML(state.shiftId ? text("Shift open", "Shift terbuka") : text("Shift closed", "Shift tutup")) + '</span>'
          +       '<span class="pos-terminal__chip">' + escapeHTML(text("Stores", "Toko")) + ': ' + escapeHTML(String((bootstrap.stores || []).length)) + '</span>'
          +       '<span class="pos-terminal__chip">' + escapeHTML(text("Tenders", "Tender")) + ': ' + escapeHTML(String((bootstrap.tender_types || []).length)) + '</span>'
          +       (state.bootstrapping ? '<span class="pos-terminal__chip pos-terminal__chip--warn">' + escapeHTML(text("Refreshing terminal…", "Memuat ulang terminal…")) + '</span>' : '')
          +     '</div>'
          +   '</div>'
          +   '<article class="pos-terminal__meta"><span class="pos-terminal__meta-label">' + escapeHTML(text("Store", "Toko")) + '</span><div class="pos-terminal__meta-value"><select id="pos-store" name="pos_store">' + renderStoreOptions(bootstrap.stores, "code", "name", state.storeCode) + '</select></div><div class="pos-terminal__meta-sub">' + escapeHTML(text("Route items and stock to the correct outlet.", "Arahkan item dan stok ke outlet yang tepat.")) + '</div></article>'
          +   '<article class="pos-terminal__meta"><span class="pos-terminal__meta-label">' + escapeHTML(text("Register", "Register")) + '</span><div class="pos-terminal__meta-value"><select id="pos-register" name="pos_register">' + renderStoreOptions((bootstrap.registers || []).filter(function(item) { return !state.storeCode || String((item.values || {}).store_code || "") === state.storeCode; }), "code", "name", state.registerCode) + '</select></div><div class="pos-terminal__meta-sub">' + escapeHTML(text("Use the assigned drawer and settlement profile.", "Gunakan laci dan profil settlement yang ditetapkan.")) + '</div></article>'
          +   '<article class="pos-terminal__meta"><span class="pos-terminal__meta-label">' + escapeHTML(text("Shift control", "Kontrol shift")) + '</span><div class="pos-terminal__buttons">' + (state.shiftId ? '<button type="button" class="pos-terminal__button pos-terminal__button--warn" data-action="close-shift">' + escapeHTML(text("Close shift", "Tutup shift")) + '</button>' : '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="open-shift">' + escapeHTML(text("Open shift", "Buka shift")) + '</button>') + '<button type="button" class="pos-terminal__button" data-action="refresh-terminal">' + escapeHTML(text("Refresh", "Muat ulang")) + '</button></div><div class="pos-terminal__meta-sub">' + escapeHTML(text("Keep cashier context current before tendering.", "Pastikan konteks kasir terbaru sebelum tender.")) + '</div></article>'
          + '</section>';
      }

      function renderCustomerBar() {
        return ''
          + '<section class="pos-terminal__customerbar">'
          +   '<div class="pos-terminal__customer-top">'
          +     '<div class="pos-terminal__stack">'
          +       '<div class="pos-terminal__row">'
          +         '<div class="pos-terminal__field"><span>' + escapeHTML(text("Customer or member", "Pelanggan atau member")) + '</span><input id="pos-customer-search" name="pos_customer_search" placeholder="' + escapeHTML(text("Member ID, customer name, or party ID", "ID member, nama pelanggan, atau ID party")) + '" value="' + escapeHTML(state.customerQuery) + '"></div>'
          +         '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="search-customers">' + escapeHTML(state.customerBusy ? text("Searching…", "Mencari…") : text("Search", "Cari")) + '</button></div>'
          +       '</div>'
          +       renderCustomerResults()
          +     '</div>'
          +     '<div class="pos-terminal__stack">'
          +       renderCustomerSummary()
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Promo / voucher codes", "Kode promo / voucher")) + '</span><textarea id="pos-voucher-codes" name="pos_voucher_codes" placeholder="' + escapeHTML(text("Each line or comma-separated code", "Satu baris atau dipisah koma")) + '">' + escapeHTML(state.promotionCodes) + '</textarea></div>'
          +       '<div class="pos-terminal__muted">' + escapeHTML(text("Voucher redemption is captured through promo / voucher codes, while gift card and store credit stay in the tender rail with balance lookup.", "Redeem voucher dicatat melalui kode promo / voucher, sementara gift card dan store credit tetap di rail tender dengan pengecekan saldo.")) + '</div>'
          +     '</div>'
          +   '</div>'
          + '</section>';
      }

      function bindEvents() {
        mount.querySelectorAll("[data-nav]").forEach(function(node) {
          node.addEventListener("click", function() {
            window.location.href = String(node.getAttribute("data-nav") || "/ui/backoffice");
          });
        });
        mount.querySelector("#pos-store")?.addEventListener("change", function(event) {
          state.storeCode = String(event.target.value || "");
          state.searchResults = [];
          persist();
          loadBootstrap().then(function() { return loadHeldSales(); }).then(render).catch(function(error) {
            notify(error instanceof Error ? error.message : "Failed to load store context", "error");
          });
        });
        mount.querySelector("#pos-register")?.addEventListener("change", function(event) {
          state.registerCode = String(event.target.value || "");
          persist();
          loadBootstrap().then(function() { return loadHeldSales(); }).then(render).catch(function(error) {
            notify(error instanceof Error ? error.message : "Failed to load register context", "error");
          });
        });
        mount.querySelector("#pos-search")?.addEventListener("input", function(event) {
          state.searchQuery = String(event.target.value || "");
        });
        mount.querySelector("#pos-search")?.addEventListener("keydown", function(event) {
          if (event.key === "Enter") {
            event.preventDefault();
            searchCatalog().catch(function(error) {
              notify(error instanceof Error ? error.message : "Search failed", "error");
            });
          }
        });
        mount.querySelector("#pos-customer-search")?.addEventListener("input", function(event) {
          state.customerQuery = String(event.target.value || "");
        });
        mount.querySelector("#pos-customer-search")?.addEventListener("keydown", function(event) {
          if (event.key === "Enter") {
            event.preventDefault();
            searchCustomers().catch(function(error) {
              notify(error instanceof Error ? error.message : "Customer lookup failed", "error");
            });
          }
        });
        mount.querySelector("#pos-lookup")?.addEventListener("input", function(event) {
          state.lookupQuery = String(event.target.value || "");
        });
        mount.querySelector("#pos-voucher-codes")?.addEventListener("input", function(event) {
          state.promotionCodes = String(event.target.value || "");
          persist();
        });
        mount.querySelector("#pos-promotion-codes")?.addEventListener("input", function(event) {
          state.promotionCodes = String(event.target.value || "");
          persist();
        });
        mount.querySelectorAll("[data-attach-customer]").forEach(function(node) {
          node.addEventListener("click", function() {
            const item = state.customerResults[number(node.getAttribute("data-attach-customer"))];
            if (item) attachCustomer(item);
          });
        });
        mount.querySelectorAll("[data-add-result]").forEach(function(node) {
          node.addEventListener("click", function() {
            const item = state.searchResults[number(node.getAttribute("data-add-result"))];
            if (item) addCatalogItem(item);
          });
        });
        mount.querySelectorAll("[data-resume-held]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.heldSales[number(node.getAttribute("data-resume-held"))];
            if (record) resumeHeldSale(record);
          });
        });
        mount.querySelectorAll("[data-refund-sale]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.transactions[number(node.getAttribute("data-refund-sale"))];
            if (record && window.confirm(text("Create refund documents for this sale?", "Buat dokumen refund untuk penjualan ini?"))) {
              transactAction(String(record.id || ""), "refund").catch(function(error) {
                notify(error instanceof Error ? error.message : "Refund failed", "error");
              });
            }
          });
        });
        mount.querySelectorAll("[data-refund-store-credit]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.transactions[number(node.getAttribute("data-refund-store-credit"))];
            if (record && window.confirm(text("Convert this refund to store credit?", "Ubah refund ini menjadi store credit?"))) {
              transactAction(String(record.id || ""), "refund-store-credit").catch(function(error) {
                notify(error instanceof Error ? error.message : "Store credit refund failed", "error");
              });
            }
          });
        });
        mount.querySelectorAll("[data-exchange-sale]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.transactions[number(node.getAttribute("data-exchange-sale"))];
            if (record && window.confirm(text("Create exchange documents for this sale?", "Buat dokumen tukar untuk penjualan ini?"))) {
              transactAction(String(record.id || ""), "exchange").catch(function(error) {
                notify(error instanceof Error ? error.message : "Exchange failed", "error");
              });
            }
          });
        });
        mount.querySelectorAll("[data-line-qty]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.cart[number(node.getAttribute("data-line-qty"))];
            if (!line) return;
            line.quantity = number(node.value);
            recalcLine(line);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-line-price]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.cart[number(node.getAttribute("data-line-price"))];
            if (!line) return;
            line.unit_price = number(node.value);
            recalcLine(line);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-line-discount]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.cart[number(node.getAttribute("data-line-discount"))];
            if (!line) return;
            line.discount_amount = number(node.value);
            recalcLine(line);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-remove-line]").forEach(function(node) {
          node.addEventListener("click", function() {
            state.cart.splice(number(node.getAttribute("data-remove-line")), 1);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-tender-type]").forEach(function(node) {
          node.addEventListener("change", function() {
            const line = state.tenders[number(node.getAttribute("data-tender-type"))];
            if (!line) return;
            line.tender_type_code = String(node.value || "");
            if (tenderRequiresReference(line.tender_type_code) && !line.reference) {
              line.reference = "";
            }
            if (tenderRequiresParty(line.tender_type_code) && (!state.customer || !state.customer.party_id)) {
              notify(text("Attach a customer for this tender type.", "Pasang pelanggan untuk jenis tender ini."), "error");
            }
            state.tenderInsights[String(node.getAttribute("data-tender-type"))] = null;
            persist();
            maybeRefreshStoreCreditInsights();
            render();
          });
        });
        mount.querySelectorAll("[data-tender-amount]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.tenders[number(node.getAttribute("data-tender-amount"))];
            if (!line) return;
            line.amount = number(node.value);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-tender-reference]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.tenders[number(node.getAttribute("data-tender-reference"))];
            if (!line) return;
            line.reference = String(node.value || "");
            persist();
          });
        });
        mount.querySelectorAll("[data-check-tender]").forEach(function(node) {
          node.addEventListener("click", function() {
            lookupStoredValue(number(node.getAttribute("data-check-tender"))).catch(function(error) {
              notify(error instanceof Error ? error.message : "Lookup failed", "error");
            });
          });
        });
        mount.querySelectorAll("[data-remove-tender]").forEach(function(node) {
          node.addEventListener("click", function() {
            state.tenders.splice(number(node.getAttribute("data-remove-tender")), 1);
            if (!state.tenders.length) state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
            state.tenderInsights = {};
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-action]").forEach(function(node) {
          node.addEventListener("click", function() {
            const action = String(node.getAttribute("data-action") || "");
            if (action === "search") {
              searchCatalog().catch(function(error) { notify(error instanceof Error ? error.message : "Search failed", "error"); });
              return;
            }
            if (action === "open-catalog") {
              openCatalog();
              return;
            }
            if (action === "close-catalog") {
              closeCatalog();
              return;
            }
            if (action === "search-customers") {
              searchCustomers().catch(function(error) { notify(error instanceof Error ? error.message : "Customer lookup failed", "error"); });
              return;
            }
            if (action === "open-shift") {
              openShift().catch(function(error) { notify(error instanceof Error ? error.message : "Open shift failed", "error"); });
              return;
            }
            if (action === "close-shift") {
              closeShift().catch(function(error) { notify(error instanceof Error ? error.message : "Close shift failed", "error"); });
              return;
            }
            if (action === "hold") {
              holdSale().catch(function(error) { notify(error instanceof Error ? error.message : "Hold sale failed", "error"); });
              return;
            }
            if (action === "checkout") {
              checkout();
              return;
            }
            if (action === "refresh-terminal") {
              loadBootstrap().then(function() { return loadHeldSales(); }).then(render).catch(function(error) { notify(error instanceof Error ? error.message : "Refresh failed", "error"); });
              return;
            }
            if (action === "lookup-transactions") {
              lookupTransactions().catch(function(error) { notify(error instanceof Error ? error.message : "Lookup failed", "error"); });
              return;
            }
            if (action === "toggle-held") {
              state.heldExpanded = !state.heldExpanded;
              render();
              return;
            }
            if (action === "toggle-transactions") {
              state.transactionsExpanded = !state.transactionsExpanded;
              render();
              return;
            }
            if (action === "add-tender") {
              state.tenders.push({ tender_type_code: "", amount: 0, reference: "", notes: "" });
              persist();
              render();
              return;
            }
            if (action === "clear-customer") {
              attachCustomer(null);
            }
          });
        });
        mount.querySelectorAll("[data-action='close-catalog-overlay']").forEach(function(node) {
          node.addEventListener("click", function(event) {
            if (event.target === node) closeCatalog();
          });
        });
      }

      function render() {
        ensureStyles();
        const bootstrap = state.bootstrap || { stores: [], registers: [], tender_types: [] };
        const totalsPayload = totals();
        if (state.loading) {
          mount.innerHTML = '<section class="pos-terminal"><section class="pos-terminal__setup"><div class="pos-terminal__stack"><strong>' + escapeHTML(text("Loading terminal…", "Memuat terminal…")) + '</strong><div>' + escapeHTML(text("Loading store context, registers, and active tender methods.", "Memuat konteks toko, register, dan metode tender aktif.")) + '</div></div></section></section>';
          return;
        }

        mount.innerHTML = ''
          + '<section class="pos-terminal">'
          +   renderTopBar()
          +   renderCustomerBar()
          +   (state.message ? '<div class="pos-terminal__notice">' + escapeHTML(state.message) + '</div>' : '')
          +   (!(bootstrap.stores || []).length || !(bootstrap.registers || []).length || !(bootstrap.tender_types || []).length
                ? renderSetupState(bootstrap)
                : '<section class="pos-terminal__workspace"><div class="pos-terminal__left">' + renderCartPanel() + renderAuxPanel() + '</div>' + renderRail(totalsPayload) + '</section>' + renderCatalogModal())
          + '</section>';
        bindEvents();
      }

      restore();
      render();
      try {
        await loadBootstrap();
        await loadHeldSales();
        state.loading = false;
        render();
      } catch (error) {
        state.loading = false;
        state.message = error instanceof Error ? error.message : text("Failed to load terminal.", "Gagal memuat terminal.");
        render();
      }
    }
  };
})();`
}
