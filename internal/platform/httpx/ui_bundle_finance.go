package httpx

func FinanceReportsBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["finance-reports"] = {
    render: async function(ctx) {
      const text = function(en, id) { return ctx.locale === "id" ? id : en; };
      const path = window.location.pathname || "";
      const reportKey = path.includes("trial-balance") ? "trial-balance"
        : path.includes("profit-and-loss") ? "profit-and-loss"
        : path.includes("balance-sheet") ? "balance-sheet"
        : path.includes("tax-summary") ? "tax-summary"
        : path.includes("ar-aging") ? "ar-aging"
        : path.includes("ap-aging") ? "ap-aging"
        : path.includes("ar-reconciliation") ? "ar-reconciliation"
        : path.includes("ap-reconciliation") ? "ap-reconciliation"
        : "journal-ledger";
      const title = reportKey === "trial-balance" ? text("Trial Balance", "Neraca Saldo")
        : reportKey === "profit-and-loss" ? text("Profit and Loss", "Laba Rugi")
        : reportKey === "balance-sheet" ? text("Balance Sheet", "Neraca")
        : reportKey === "tax-summary" ? text("Tax Summary", "Ringkasan Pajak")
        : reportKey === "ar-aging" ? text("AR Aging", "Umur Piutang")
        : reportKey === "ap-aging" ? text("AP Aging", "Umur Utang")
        : reportKey === "ar-reconciliation" ? text("AR Reconciliation", "Rekonsiliasi Piutang")
        : reportKey === "ap-reconciliation" ? text("AP Reconciliation", "Rekonsiliasi Utang")
        : text("Journal Ledger", "Buku Jurnal");
      const mount = ctx.mount;
      const params = new URLSearchParams(window.location.search);
      const filters = {
        from_date: params.get("from_date") || "",
        to_date: params.get("to_date") || "",
        as_of_date: params.get("as_of_date") || "",
        party_id: params.get("party_id") || "",
        vendor_id: params.get("vendor_id") || "",
        account_code: params.get("account_code") || "",
        aging_bucket: params.get("aging_bucket") || "",
      };
      const usesAsOf = reportKey === "balance-sheet" || reportKey === "ar-aging" || reportKey === "ap-aging" || reportKey === "ar-reconciliation" || reportKey === "ap-reconciliation";
      function escapeHTML(value) {
        return String(value == null ? "" : value).replace(/[&<>"]/g, function(char) {
          return { "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;" }[char];
        });
      }
      function money(value) {
        return new Intl.NumberFormat(ctx.locale === "id" ? "id-ID" : "en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number(value || 0));
      }
      function ensureStyles() {
        if (document.getElementById("finance-reports-styles")) return;
        const style = document.createElement("style");
        style.id = "finance-reports-styles";
        style.textContent = ""
          + ".finance-report { display:grid; gap:1rem; }"
          + ".finance-report__hero{display:flex;justify-content:space-between;gap:1rem;flex-wrap:wrap;padding:1rem 1.2rem;border:1px solid var(--color-line);border-radius:1rem;background:linear-gradient(135deg,color-mix(in srgb,var(--color-accent-soft) 70%,var(--color-surface) 30%),var(--color-surface));box-shadow:var(--shadow-panel);}"
          + ".finance-report__hero h2{margin:0;color:var(--color-body);font-size:1.5rem;}"
          + ".finance-report__hero p{margin:0.4rem 0 0;color:var(--color-muted);}"
          + ".finance-report__panel{border:1px solid var(--color-line);border-radius:1rem;background:var(--color-surface);box-shadow:var(--shadow-panel);padding:1rem;}"
          + ".finance-report__filters{display:flex;gap:0.75rem;flex-wrap:wrap;align-items:end;}"
          + ".finance-report__field{display:flex;flex-direction:column;gap:0.35rem;min-width:12rem;}"
          + ".finance-report__field span{font-size:0.74rem;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:var(--color-muted);}"
          + ".finance-report__field input{height:2.5rem;border:1px solid var(--color-line);border-radius:0.75rem;background:var(--color-surface);color:var(--color-body);padding:0 0.8rem;font:inherit;}"
          + ".finance-report__button{appearance:none;border:1px solid var(--color-line);border-radius:0.8rem;background:var(--color-surface);color:var(--color-body);padding:0.8rem 1rem;font:inherit;font-weight:700;cursor:pointer;}"
          + ".finance-report__button--primary{background:var(--color-accent);border-color:var(--color-accent);color:#fff;}"
          + ".finance-report__nav{display:flex;gap:0.5rem;flex-wrap:wrap;}"
          + ".finance-report__nav a{display:inline-flex;padding:0.55rem 0.8rem;border-radius:999px;border:1px solid var(--color-line);color:var(--color-body);text-decoration:none;font-weight:700;}"
          + ".finance-report__nav a.is-active{background:var(--color-accent);border-color:var(--color-accent);color:#fff;}"
          + ".finance-report__table-wrap{overflow:auto;border:1px solid var(--color-line);border-radius:0.9rem;}"
          + ".finance-report__table{width:100%;border-collapse:collapse;min-width:48rem;}"
          + ".finance-report__table th,.finance-report__table td{padding:0.8rem 0.7rem;border-top:1px solid var(--color-line);text-align:left;vertical-align:top;}"
          + ".finance-report__table th{font-size:0.74rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;color:var(--color-muted);}"
          + ".finance-report__table td:last-child,.finance-report__table th:last-child{text-align:right;}"
          + ".finance-report__table td:nth-last-child(2),.finance-report__table th:nth-last-child(2){text-align:right;}"
          + ".finance-report__cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(13rem,1fr));gap:0.75rem;}"
          + ".finance-report__card{border:1px solid var(--color-line);border-radius:0.9rem;padding:0.9rem;background:color-mix(in srgb,var(--color-shell) 35%,var(--color-surface));}"
          + ".finance-report__card span{display:block;color:var(--color-muted);font-size:0.74rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;}"
          + ".finance-report__card strong{display:block;margin-top:0.45rem;color:var(--color-body);font-size:1.3rem;}"
          + "@media (max-width: 900px){.finance-report__table{min-width:36rem;}}";
        document.head.appendChild(style);
      }
      async function loadReport() {
        const query = new URLSearchParams();
        if (usesAsOf) {
          if (filters.as_of_date) query.set("as_of_date", filters.as_of_date);
        } else {
          if (filters.from_date) query.set("from_date", filters.from_date);
          if (filters.to_date) query.set("to_date", filters.to_date);
        }
        if (filters.party_id && (reportKey === "ar-aging" || reportKey === "ar-reconciliation")) query.set("party_id", filters.party_id);
        if (filters.vendor_id && (reportKey === "ap-aging" || reportKey === "ap-reconciliation")) query.set("vendor_id", filters.vendor_id);
        if (filters.account_code && (reportKey === "ar-reconciliation" || reportKey === "ap-reconciliation")) query.set("account_code", filters.account_code);
        if (filters.aging_bucket && (reportKey === "ar-aging" || reportKey === "ap-aging")) query.set("aging_bucket", filters.aging_bucket);
        return ctx.api("/ui/data/finance/" + reportKey + (query.toString() ? "?" + query.toString() : ""));
      }
      function renderRows(payload) {
        if (reportKey === "ar-aging" || reportKey === "ap-aging") {
          return "<div class='finance-report__table-wrap'><table class='finance-report__table'><thead><tr><th>" + text("Counterparty", "Mitra") + "</th><th>" + text("Document", "Dokumen") + "</th><th>" + text("Date", "Tanggal") + "</th><th>" + text("Due", "Jatuh Tempo") + "</th><th>" + text("Account", "Akun") + "</th><th>" + text("Bucket", "Kelompok") + "</th><th>" + text("Open", "Terbuka") + "</th></tr></thead><tbody>" + (payload.items || []).map(function(row) {
            return "<tr><td>" + escapeHTML(row.counterparty_name) + "</td><td>" + escapeHTML(row.document_number) + "</td><td>" + escapeHTML(row.document_date) + "</td><td>" + escapeHTML(row.due_date) + "</td><td>" + escapeHTML(row.account_code) + "</td><td>" + escapeHTML(row.aging_bucket) + "</td><td>" + money(row.open_amount) + "</td></tr>";
          }).join("") + "</tbody></table></div>";
        }
        if (reportKey === "ar-reconciliation" || reportKey === "ap-reconciliation") {
          const accountRows = (payload.accounts || []).map(function(row) {
            return "<tr><td>" + escapeHTML(row.account_code) + "</td><td>" + escapeHTML(row.account_name) + "</td><td>" + money(row.subledger_amount) + "</td><td>" + money(row.gl_amount) + "</td><td>" + money(row.difference) + "</td></tr>";
          }).join("");
          const mismatchRows = (payload.mismatches || []).map(function(row) {
            return "<tr><td>" + escapeHTML(row.account_code || row.document_number || "") + "</td><td>" + escapeHTML(row.reason || "") + "</td><td>" + money(row.subledger_amount) + "</td><td>" + money(row.gl_amount) + "</td><td>" + money(row.difference) + "</td></tr>";
          }).join("");
          return "<section class='finance-report__panel'><h3>" + escapeHTML(text("Account Comparison", "Perbandingan Akun")) + "</h3><div class='finance-report__table-wrap'><table class='finance-report__table'><thead><tr><th>" + text("Account", "Akun") + "</th><th>" + text("Name", "Nama") + "</th><th>" + text("Subledger", "Subledger") + "</th><th>" + text("GL", "GL") + "</th><th>" + text("Difference", "Selisih") + "</th></tr></thead><tbody>" + accountRows + "</tbody></table></div></section>"
            + "<section class='finance-report__panel'><h3>" + escapeHTML(text("Mismatches", "Selisih")) + "</h3><div class='finance-report__table-wrap'><table class='finance-report__table'><thead><tr><th>" + text("Reference", "Referensi") + "</th><th>" + text("Reason", "Alasan") + "</th><th>" + text("Subledger", "Subledger") + "</th><th>" + text("GL", "GL") + "</th><th>" + text("Difference", "Selisih") + "</th></tr></thead><tbody>" + mismatchRows + "</tbody></table></div></section>";
        }
        if (reportKey === "profit-and-loss" || reportKey === "balance-sheet") {
          const sections = payload.sections || [];
          return sections.map(function(section) {
            const rows = (section.rows || []).map(function(row) {
              return "<tr><td>" + escapeHTML(row.account_code) + "</td><td>" + escapeHTML(row.account_name) + "</td><td>" + money(row.amount) + "</td></tr>";
            }).join("");
            return "<section class='finance-report__panel'><h3>" + escapeHTML(section.label) + "</h3><div class='finance-report__table-wrap'><table class='finance-report__table'><thead><tr><th>" + text("Account", "Akun") + "</th><th>" + text("Name", "Nama") + "</th><th>" + text("Amount", "Jumlah") + "</th></tr></thead><tbody>" + rows + "<tr><td colspan='2'><strong>" + text("Section Total", "Total Bagian") + "</strong></td><td><strong>" + money(section.amount) + "</strong></td></tr></tbody></table></div></section>";
          }).join("");
        }
        const rows = payload.rows || [];
        if (reportKey === "tax-summary") {
          return "<div class='finance-report__table-wrap'><table class='finance-report__table'><thead><tr><th>" + text("Account", "Akun") + "</th><th>" + text("Name", "Nama") + "</th><th>" + text("Bucket", "Kelompok") + "</th><th>" + text("Debit", "Debit") + "</th><th>" + text("Credit", "Kredit") + "</th><th>" + text("Net", "Bersih") + "</th></tr></thead><tbody>" + rows.map(function(row) {
            return "<tr><td>" + escapeHTML(row.account_code) + "</td><td>" + escapeHTML(row.account_name) + "</td><td>" + escapeHTML(row.tax_bucket) + "</td><td>" + money(row.debit) + "</td><td>" + money(row.credit) + "</td><td>" + money(row.net_amount) + "</td></tr>";
          }).join("") + "</tbody></table></div>";
        }
        if (reportKey === "journal-ledger") {
          return "<div class='finance-report__table-wrap'><table class='finance-report__table'><thead><tr><th>" + text("Date", "Tanggal") + "</th><th>" + text("Posting", "Posting") + "</th><th>" + text("Source", "Sumber") + "</th><th>" + text("Account", "Akun") + "</th><th>" + text("Description", "Deskripsi") + "</th><th>" + text("Debit", "Debit") + "</th><th>" + text("Credit", "Kredit") + "</th></tr></thead><tbody>" + rows.map(function(row) {
            return "<tr><td>" + escapeHTML(row.posting_date) + "</td><td>" + escapeHTML(row.posting_number) + "</td><td>" + escapeHTML((row.source_document_type || "") + " " + (row.source_document_id || "")) + "</td><td>" + escapeHTML(row.account_code) + "</td><td>" + escapeHTML(row.description) + "</td><td>" + money(row.debit) + "</td><td>" + money(row.credit) + "</td></tr>";
          }).join("") + "</tbody></table></div>";
        }
        return "<div class='finance-report__table-wrap'><table class='finance-report__table'><thead><tr><th>" + text("Account", "Akun") + "</th><th>" + text("Name", "Nama") + "</th><th>" + text("Type", "Tipe") + "</th><th>" + text("Opening", "Awal") + "</th><th>" + text("Debit", "Debit") + "</th><th>" + text("Credit", "Kredit") + "</th><th>" + text("Ending", "Akhir") + "</th></tr></thead><tbody>" + rows.map(function(row) {
          return "<tr><td>" + escapeHTML(row.account_code) + "</td><td>" + escapeHTML(row.account_name) + "</td><td>" + escapeHTML(row.account_type) + "</td><td>" + money(row.opening) + "</td><td>" + money(row.debit) + "</td><td>" + money(row.credit) + "</td><td>" + money(row.ending) + "</td></tr>";
        }).join("") + "</tbody></table></div>";
      }
      function summaryCards(payload) {
        if (reportKey === "profit-and-loss") {
          return "<section class='finance-report__cards'><article class='finance-report__card'><span>" + text("Gross Profit", "Laba Kotor") + "</span><strong>" + money(payload.gross_profit) + "</strong></article><article class='finance-report__card'><span>" + text("Net Profit", "Laba Bersih") + "</span><strong>" + money(payload.net_profit) + "</strong></article></section>";
        }
        if (reportKey === "balance-sheet") {
          return "<section class='finance-report__cards'><article class='finance-report__card'><span>" + text("Retained Earnings", "Laba Ditahan") + "</span><strong>" + money(payload.retained_earnings) + "</strong></article></section>";
        }
        if (reportKey === "ar-reconciliation" || reportKey === "ap-reconciliation") {
          return "<section class='finance-report__cards'><article class='finance-report__card'><span>" + text("Subledger", "Subledger") + "</span><strong>" + money(payload.subledger_total) + "</strong></article><article class='finance-report__card'><span>" + text("GL", "GL") + "</span><strong>" + money(payload.gl_total) + "</strong></article><article class='finance-report__card'><span>" + text("Difference", "Selisih") + "</span><strong>" + money(payload.difference) + "</strong></article></section>";
        }
        const totals = payload.totals || {};
        const keys = Object.keys(totals);
        if (!keys.length) return "";
        return "<section class='finance-report__cards'>" + keys.map(function(key) {
          return "<article class='finance-report__card'><span>" + escapeHTML(key.replace(/_/g, " ")) + "</span><strong>" + money(totals[key]) + "</strong></article>";
        }).join("") + "</section>";
      }
      ensureStyles();
      const payload = await loadReport();
      const navItems = [
        { key: "trial-balance", label: text("Trial Balance", "Neraca Saldo"), path: "/ui/finance/trial-balance" },
        { key: "profit-and-loss", label: text("Profit and Loss", "Laba Rugi"), path: "/ui/finance/profit-and-loss" },
        { key: "balance-sheet", label: text("Balance Sheet", "Neraca"), path: "/ui/finance/balance-sheet" },
        { key: "tax-summary", label: text("Tax Summary", "Ringkasan Pajak"), path: "/ui/finance/tax-summary" },
        { key: "ar-aging", label: text("AR Aging", "Umur Piutang"), path: "/ui/finance/ar-aging" },
        { key: "ap-aging", label: text("AP Aging", "Umur Utang"), path: "/ui/finance/ap-aging" },
        { key: "ar-reconciliation", label: text("AR Reconciliation", "Rekonsiliasi Piutang"), path: "/ui/finance/ar-reconciliation" },
        { key: "ap-reconciliation", label: text("AP Reconciliation", "Rekonsiliasi Utang"), path: "/ui/finance/ap-reconciliation" },
        { key: "journal-ledger", label: text("Journal Ledger", "Buku Jurnal"), path: "/ui/finance/journal-ledger" }
      ];
      mount.innerHTML = ""
        + "<section class='finance-report'>"
        +   "<section class='finance-report__hero'><div><h2>" + escapeHTML(title) + "</h2><p>" + escapeHTML(text("Finance statements and tax visibility from posted journals.", "Laporan keuangan dan visibilitas pajak dari jurnal yang sudah diposting.")) + "</p></div><nav class='finance-report__nav'>" + navItems.map(function(item) {
              return "<a href='" + item.path + "' class='" + (item.key === reportKey ? "is-active" : "") + "'>" + escapeHTML(item.label) + "</a>";
            }).join("") + "</nav></section>"
        +   "<section class='finance-report__panel'><div class='finance-report__filters'>"
        +     (usesAsOf
                ? "<label class='finance-report__field'><span>" + text("As Of", "Per Tanggal") + "</span><input data-filter='as_of_date' type='date' value='" + escapeHTML(filters.as_of_date) + "' /></label>"
                : "<label class='finance-report__field'><span>" + text("From", "Dari") + "</span><input data-filter='from_date' type='date' value='" + escapeHTML(filters.from_date) + "' /></label><label class='finance-report__field'><span>" + text("To", "Sampai") + "</span><input data-filter='to_date' type='date' value='" + escapeHTML(filters.to_date) + "' /></label>")
        +     ((reportKey === "ar-aging" || reportKey === "ar-reconciliation") ? "<label class='finance-report__field'><span>" + text("Party", "Pihak") + "</span><input data-filter='party_id' value='" + escapeHTML(filters.party_id) + "' /></label>" : "")
        +     ((reportKey === "ap-aging" || reportKey === "ap-reconciliation") ? "<label class='finance-report__field'><span>" + text("Vendor", "Vendor") + "</span><input data-filter='vendor_id' value='" + escapeHTML(filters.vendor_id) + "' /></label>" : "")
        +     ((reportKey === "ar-reconciliation" || reportKey === "ap-reconciliation") ? "<label class='finance-report__field'><span>" + text("Account", "Akun") + "</span><input data-filter='account_code' value='" + escapeHTML(filters.account_code) + "' /></label>" : "")
        +     ((reportKey === "ar-aging" || reportKey === "ap-aging") ? "<label class='finance-report__field'><span>" + text("Bucket", "Kelompok") + "</span><input data-filter='aging_bucket' value='" + escapeHTML(filters.aging_bucket) + "' /></label>" : "")
        +     "<button class='finance-report__button finance-report__button--primary' data-apply>" + escapeHTML(text("Apply", "Terapkan")) + "</button>"
        +   "</div></section>"
        +   summaryCards(payload)
        +   renderRows(payload)
        + "</section>";
      const apply = mount.querySelector("[data-apply]");
      if (apply) {
        apply.addEventListener("click", function() {
          mount.querySelectorAll("[data-filter]").forEach(function(node) {
            filters[node.getAttribute("data-filter")] = node.value || "";
          });
          const next = new URL(window.location.href);
          if (usesAsOf) {
            if (filters.as_of_date) next.searchParams.set("as_of_date", filters.as_of_date); else next.searchParams.delete("as_of_date");
            next.searchParams.delete("from_date");
            next.searchParams.delete("to_date");
          } else {
            if (filters.from_date) next.searchParams.set("from_date", filters.from_date); else next.searchParams.delete("from_date");
            if (filters.to_date) next.searchParams.set("to_date", filters.to_date); else next.searchParams.delete("to_date");
            next.searchParams.delete("as_of_date");
          }
          if (filters.party_id) next.searchParams.set("party_id", filters.party_id); else next.searchParams.delete("party_id");
          if (filters.vendor_id) next.searchParams.set("vendor_id", filters.vendor_id); else next.searchParams.delete("vendor_id");
          if (filters.account_code) next.searchParams.set("account_code", filters.account_code); else next.searchParams.delete("account_code");
          if (filters.aging_bucket) next.searchParams.set("aging_bucket", filters.aging_bucket); else next.searchParams.delete("aging_bucket");
          window.location.assign(next.toString());
        });
      }
    }
  };
})();`
}
