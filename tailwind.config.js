module.exports = {
  content: [
    "./internal/platform/httpx/*.go"
  ],
  theme: {
    extend: {
      colors: {
        ink: "#1f2937",
        muted: "#667085",
        line: "#d9dee8",
        accent: "#3b5ccc",
        "accent-soft": "#e8eeff",
        warn: "#c2412d",
        surface: "#ffffff",
        shell: "#f5f6fa"
      },
      fontFamily: {
        display: ["IBM Plex Sans", "Segoe UI", "Helvetica Neue", "Arial", "sans-serif"],
        body: ["IBM Plex Sans", "Segoe UI", "Helvetica Neue", "Arial", "sans-serif"],
        mono: ["ui-monospace", "SFMono-Regular", "monospace"]
      },
      boxShadow: {
        panel: "0 12px 30px rgba(15, 23, 42, .08)"
      }
    }
  },
  plugins: []
};
