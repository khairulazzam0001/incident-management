/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      // Palet Quizlet-reference (frontend/UI-reference/DESIGN.md).
      colors: {
        iris: "#4255ff",
        ink: "#282e3e",
        deep: "#2e3856",
        veil: "#586380",
        fog: "#939bb4",
        chalk: "#f6f7fb",
        paper: "#ffffff",
        lilac: "#edefff",
        mist: "#d9dde8",
      },
      fontFamily: {
        // Hurme Geometric Sans komersial → substitusi Inter (DESIGN.md).
        sans: [
          "Inter",
          "ui-sans-serif",
          "system-ui",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "Roboto",
          "sans-serif",
        ],
      },
      boxShadow: {
        sm: "rgba(40, 46, 62, 0.1) 0px 2px 4px 0px",
        md: "rgba(40, 46, 62, 0.1) 0px 4px 16px 0px",
      },
    },
  },
  plugins: [],
};
