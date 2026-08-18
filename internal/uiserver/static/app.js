const statusElement = document.getElementById('status');
const processesElement = document.getElementById('processes');
const eventsElement = document.getElementById('events');
const tabsElement = document.getElementById('tabs');
const terminalTitleElement = document.getElementById('terminal-title');
const source = new EventSource('/events');

const baseTabs = [
  { id: 'all', label: 'All' },
  { id: 'proxy', label: 'Proxy' },
  { id: 'process', label: 'Process' },
  { id: 'stdout', label: 'Stdout' },
  { id: 'stderr', label: 'Stderr' },
  { id: 'errors', label: 'Errors' },
];

let activeTab = 'all';
let processNames = [];
let allEvents = [];

renderTabs();
loadProcesses();

source.onopen = () => {
  statusElement.textContent = 'connected';
};

source.onerror = () => {
  statusElement.textContent = 'disconnected, retrying…';
};

source.addEventListener('devproxy', (message) => {
  const event = JSON.parse(message.data);
  allEvents.unshift(event);
  allEvents = allEvents.slice(0, 500);

  renderEvents();

  if (event.type && event.type.startsWith('process.')) {
    loadProcesses();
  }
});

async function loadProcesses() {
  try {
    const response = await fetch('/api/processes');
    if (!response.ok) {
      processesElement.innerHTML = '<div class="muted">failed to load processes</div>';
      return;
    }

    const processes = await response.json();
    processNames = processes.map((process) => process.name);

    if (activeTab.startsWith('instance:') && !processNames.includes(activeTab.slice('instance:'.length))) {
      activeTab = 'all';
    }

    renderProcesses(processes);
    renderTabs();
    renderEvents();
  } catch (error) {
    processesElement.innerHTML = '<div class="muted">failed to load processes: ' + error.message + '</div>';
  }
}

function renderTabs() {
  tabsElement.innerHTML = '';

  for (const tab of baseTabs) {
    tabsElement.append(tabButton(tab.id, tab.label, false));
  }

  for (const name of processNames) {
    tabsElement.append(tabButton('instance:' + name, name, true));
  }

  updateTerminalTitle();
}

function tabButton(id, label, isProcessTab) {
  const button = document.createElement('button');
  button.className = 'tab' + (isProcessTab ? ' process-tab' : '');
  button.dataset.tab = id;
  button.textContent = label;
  button.classList.toggle('active', activeTab === id);
  button.addEventListener('click', () => {
    activeTab = id;
    renderTabs();
    renderEvents();
  });

  return button;
}

function updateTerminalTitle() {
  if (activeTab.startsWith('instance:')) {
    terminalTitleElement.textContent = activeTab.slice('instance:'.length) + ' events';
    return;
  }

  const tab = baseTabs.find((item) => item.id === activeTab);
  terminalTitleElement.textContent = (tab ? tab.label.toLowerCase() : activeTab) + ' events';
}

function renderProcesses(items) {
  processesElement.innerHTML = '';

  if (!items.length) {
    processesElement.innerHTML = '<div class="muted">no processes configured</div>';
    return;
  }

  for (const process of items) {
    const card = document.createElement('article');
    card.className = 'process-card';

    const header = document.createElement('div');
    header.className = 'process-card-header';

    const name = document.createElement('button');
    name.className = 'process-name';
    name.textContent = process.name;
    name.title = 'Show only ' + process.name + ' events';
    name.addEventListener('click', () => {
      activeTab = 'instance:' + process.name;
      renderTabs();
      renderEvents();
    });

    const state = document.createElement('div');
    state.className = 'state ' + process.state;
    state.textContent = process.state;

    const meta = document.createElement('div');
    meta.className = 'process-meta';
    meta.textContent = processMeta(process);

    const command = document.createElement('div');
    command.className = 'process-command';
    command.textContent = process.command;

    const actions = document.createElement('div');
    actions.className = 'process-actions';
    actions.append(
      actionButton('Start', process.name, 'start', process.state === 'running' || process.state === 'stopping'),
      actionButton('Stop', process.name, 'stop', process.state !== 'running'),
      actionButton('Restart', process.name, 'restart', process.state === 'stopping'),
    );

    header.append(name, state);
    card.append(header, meta, command, actions);
    processesElement.append(card);
  }
}

function actionButton(label, name, action, disabled) {
  const button = document.createElement('button');
  button.textContent = label;
  button.disabled = disabled;
  button.addEventListener('click', async () => {
    button.disabled = true;
    await runProcessAction(name, action);
  });

  return button;
}

async function runProcessAction(name, action) {
  try {
    const response = await fetch('/api/processes/' + encodeURIComponent(name) + '/' + action, {
      method: 'POST',
    });

    if (!response.ok) {
      const error = await response.text();
      appendLocalError('process.' + action, '[' + name + '] ' + error.trim(), name);
      return;
    }

    renderProcesses(await response.json());
  } catch (error) {
    appendLocalError('process.' + action, '[' + name + '] ' + error.message, name);
  }
}

function appendLocalError(type, message, processName) {
  allEvents.unshift({
    type,
    timestamp: new Date().toISOString(),
    process: processName,
    error: message,
  });
  renderEvents();
}

function processMeta(process) {
  const parts = [];

  if (process.pid) {
    parts.push('pid ' + process.pid);
  }

  if (process.exit_code !== undefined) {
    parts.push('exit ' + process.exit_code);
  }

  if (process.working_dir) {
    parts.push(process.working_dir);
  }

  if (process.last_error) {
    parts.push(process.last_error);
  }

  return parts.join(' · ') || 'waiting';
}

function renderEvents() {
  eventsElement.innerHTML = '';
  updateTerminalTitle();

  const visibleEvents = allEvents.filter(matchesActiveTab);
  if (!visibleEvents.length) {
    const empty = document.createElement('div');
    empty.className = 'terminal-empty';
    empty.textContent = 'no events in this tab yet';
    eventsElement.append(empty);
    return;
  }

  for (const event of visibleEvents.slice(0, 300).reverse()) {
    eventsElement.append(eventElement(event));
  }

  eventsElement.scrollTop = eventsElement.scrollHeight;
}

function matchesActiveTab(event) {
  if (activeTab === 'all') {
    return true;
  }

  if (activeTab.startsWith('instance:')) {
    return eventBelongsToInstance(event, activeTab.slice('instance:'.length));
  }

  if (activeTab === 'proxy') {
    return event.type === 'proxy.request' || event.type === 'proxy.error';
  }

  if (activeTab === 'process') {
    return event.type && event.type.startsWith('process.') && event.type !== 'process.output';
  }

  if (activeTab === 'stdout') {
    return event.type === 'process.output' && event.stream === 'stdout';
  }

  if (activeTab === 'stderr') {
    return event.type === 'process.output' && event.stream === 'stderr';
  }

  if (activeTab === 'errors') {
    return Boolean(event.error) || event.type === 'process.output_error' || event.type === 'proxy.error';
  }

  return true;
}

function eventBelongsToInstance(event, name) {
  if (event.process === name) {
    return true;
  }

  if (event.route === name) {
    return true;
  }

  return false;
}

function eventElement(event) {
  const item = document.createElement('div');
  item.className = 'terminal-line ' + eventClasses(event).join(' ');

  const time = document.createElement('span');
  time.className = 'terminal-time';
  time.textContent = formatTime(event.timestamp);

  const source = document.createElement('span');
  source.className = 'terminal-source';
  source.textContent = eventSource(event);

  const kind = document.createElement('span');
  kind.className = 'terminal-kind';
  kind.textContent = eventKind(event);

  const message = document.createElement('span');
  message.className = 'terminal-message';
  message.textContent = formatEvent(event);

  item.append(time, source, kind, message);
  return item;
}

function eventClasses(event) {
  const classes = [];

  if (event.type === 'proxy.request') {
    classes.push('proxy');
    if (event.status >= 500) {
      classes.push('error');
    } else if (event.status >= 400) {
      classes.push('warning');
    } else {
      classes.push('success');
    }
  }

  if (event.type && event.type.startsWith('process.')) {
    classes.push(event.type === 'process.output' ? event.stream : 'lifecycle');
  }

  if (event.error || event.type === 'process.failed' || event.type === 'process.output_error') {
    classes.push('error');
  }

  return classes;
}

function eventSource(event) {
  if (event.process) {
    return '[' + event.process + ']';
  }

  if (event.route) {
    return '[' + event.route + ']';
  }

  return '[devproxy]';
}

function eventKind(event) {
  if (event.type === 'process.output') {
    return event.stream + ' ›';
  }

  if (event.type === 'proxy.request') {
    return 'proxy ›';
  }

  return (event.type || 'event') + ' ›';
}

function formatEvent(event) {
  if (event.type === 'process.output') {
    return event.message;
  }

  if (event.type === 'process.started') {
    return 'started: ' + event.message;
  }

  if (event.type === 'process.stopping') {
    return 'stopping';
  }

  if (event.type === 'process.exited') {
    return 'exited with code ' + event.exit_code + (event.error ? ': ' + event.error : '');
  }

  if (event.type === 'process.failed') {
    return 'failed: ' + event.error;
  }

  if (event.type === 'proxy.request') {
    const route = event.route ? 'route=' + event.route : 'no-route';
    const upstream = event.upstream ? ' upstream=' + event.upstream : '';
    return event.method + ' ' + event.path + ' -> ' + event.status + ' ' + route + upstream + ' (' + event.duration_ms + 'ms)';
  }

  if (event.error) {
    return event.error;
  }

  return JSON.stringify(event);
}

function formatTime(timestamp) {
  if (!timestamp) {
    return '--:--:--';
  }

  return new Date(timestamp).toLocaleTimeString();
}
