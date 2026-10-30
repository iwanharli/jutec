export default defineAppConfig({
  ui: {
    primary: 'indigo',
    gray: 'slate',
    card: {
      background: 'bg-slate-900/60',
      ring: 'ring-1 ring-slate-800',
      rounded: 'rounded-2xl',
      shadow: 'shadow-none',
      divide: 'divide-y divide-slate-800',
      header: { padding: 'px-6 py-5' },
      body: { padding: 'px-6 py-5' },
      footer: { padding: 'px-6 py-4' },
    },
    input: { default: { size: 'lg' } },
    select: { default: { size: 'lg' } },
    selectMenu: { default: { size: 'lg' } },
    button: {
      default: { size: 'md' },
      variant: {
        solid: 'shadow-sm text-white dark:text-white bg-{color}-600 dark:bg-{color}-600 hover:bg-{color}-500 dark:hover:bg-{color}-500 disabled:bg-{color}-600 dark:disabled:bg-{color}-600 aria-disabled:bg-{color}-600 dark:aria-disabled:bg-{color}-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-{color}-500',
      },
      // Varian abu-abu ditulis eksplisit (bukan template {color}) agar class-nya pasti di-generate Tailwind
      color: {
        gray: {
          solid: 'shadow-sm ring-1 ring-inset ring-slate-700 text-slate-200 bg-slate-800 hover:bg-slate-700 disabled:bg-slate-800 focus-visible:ring-2 focus-visible:ring-primary-400',
          soft:  'text-slate-300 bg-slate-800/70 hover:bg-slate-700/80 hover:text-white disabled:bg-slate-800/70 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-slate-400',
          ghost: 'text-slate-400 hover:text-white hover:bg-slate-800/80 disabled:bg-transparent focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-400',
          link:  'text-slate-400 hover:text-slate-200 underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-400',
        },
      },
    },
    table: {
      th: { base: 'text-left rtl:text-right whitespace-nowrap', padding: 'px-5 py-3.5', color: 'text-slate-400', font: 'font-medium', size: 'text-xs uppercase tracking-wider' },
      td: { padding: 'px-5 py-4', color: 'text-slate-300', size: 'text-sm' },
      divide: 'divide-y divide-slate-800',
      tbody: 'divide-y divide-slate-800/70',
    },
  },
})
