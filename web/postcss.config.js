// Tailwind v4 moved its PostCSS plugin to its own package, and does its own vendor
// prefixing, so autoprefixer is gone rather than kept alongside it.
export default {
  plugins: {
    '@tailwindcss/postcss': {},
  },
}
