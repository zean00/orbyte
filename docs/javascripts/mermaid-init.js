(function () {
  function renderMermaid() {
    if (!window.mermaid) return;

    window.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "loose",
      theme: "default",
    });

    var blocks = document.querySelectorAll("pre.mermaid code, div.mermaid");
    blocks.forEach(function (block, index) {
      var source = block.textContent || "";
      var container = block.tagName === "DIV" ? block : block.parentElement;
      if (!container || container.dataset.mermaidRendered === "true") return;

      var renderTarget = document.createElement("div");
      renderTarget.className = "mermaid";
      renderTarget.textContent = source;
      renderTarget.id = "mermaid-diagram-" + index + "-" + Date.now();

      container.replaceWith(renderTarget);
      renderTarget.dataset.mermaidRendered = "true";
    });

    window.mermaid.run({ querySelector: ".mermaid" });
  }

  if (window.document$ && typeof window.document$.subscribe === "function") {
    window.document$.subscribe(renderMermaid);
  } else {
    document.addEventListener("DOMContentLoaded", renderMermaid);
  }
})();
