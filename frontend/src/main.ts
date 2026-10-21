import './app.css'
import './theme.css'
import './palettes.css'
import { mount } from 'svelte'
import ConnectivityToast from './components/ConnectivityToast.svelte'
import { initializeLocale } from './lib/i18n.svelte'
import { initializeTheme } from './lib/theme.svelte'

const view = new URLSearchParams(window.location.search).get('view');
initializeTheme();
// Load only the interface needed by this window. In particular, auxiliary
// windows do not need the workspace's rich-text editor and posting workflow.
// Separate loader entries preserve each view's stylesheet preloads in the build.
const windowViews = {
  workspace: () => import('./Workspace.svelte'),
  'posting-logs': () => import('./components/PostingLogs.svelte'),
  settings: () => import('./components/Settings.svelte'),
};
const windowView = view === 'posting-logs' || view === 'settings' ? view : 'workspace';
void Promise.all([initializeLocale(), windowViews[windowView]()]).then(([, { default: WindowView }]) => {
  mount(WindowView, { target: document.getElementById('app')! })
  if (view === 'settings' || view === 'posting-logs') mount(ConnectivityToast, { target: document.getElementById('app')! })
}).catch(() => {
  document.getElementById('app')!.textContent = 'Could not load the interface. Please restart Postika.';
})
