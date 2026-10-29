import './app.css'
import './theme.css'
import './palettes.css'
import './motion.css'
import { mount } from 'svelte'
import ConnectivityToast from './components/ConnectivityToast.svelte'
import NotificationHost from './components/NotificationHost.svelte'
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
};
const windowView = view === 'posting-logs' ? view : 'workspace';
void Promise.all([initializeLocale(), windowViews[windowView]()]).then(([, { default: WindowView }]) => {
  mount(NotificationHost, { target: document.getElementById('app')! })
  mount(WindowView, { target: document.getElementById('app')! })
  if (view === 'posting-logs') mount(ConnectivityToast, { target: document.getElementById('app')! })
}).catch(() => {
  document.getElementById('app')!.textContent = 'Could not load the interface. Please restart Postika.';
})
