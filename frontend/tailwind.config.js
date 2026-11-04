/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: { extend: { colors: { ink: "#07111f", panel: "#0d1b2e", edge: "#27e2a4" }, boxShadow: { glow: "0 0 40px rgba(39,226,164,.12)" } } },
  plugins: []
};

