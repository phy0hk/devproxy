const statusElement = document.getElementById('status');
const processesElement = document.getElementById('processes');
const eventsElement = document.getElementById('events');
const tabs = Array.from(document.querySelectorAll('.tab'));
const source = new EventSource('/events');

let activeTab = 'all';
let allEvents = [];

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

for (const tab of tabs) {
  tab.addEventListener('click', () => {
    activeTab = tab.dataset.tab;

    for (const item of tabs) {
      item.classList.toggle('active', item === tab);
    }

    renderEvents();
  });
}

async function loadProcesses() {
  try {
    const response = await fetch('/api/processes');
    if (!response.ok) {
      processesElement.innerHTML = '<div class="muted">failed to load processes</div>';
      return;
    }

    renderProcesses(await response.json());
  } catch (error) {
    processesElement.innerHTML = '<div class="muted">failed to load processes: ' + error.message + '</div>';
  }
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

    const name = document.createElement('div');
    name.className = 'process-name';
    name.textContent = process.name;

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
      appendLocalError('process.' + action, '[' + name + '] ' + error.trim());
      return;
    }

    renderProcesses(await response.json());
  } catch (error) {
    appendLocalError('process.' + action, '[' + name + '] ' + error.message);
  }
}

function appendLocalError(type, message) {
  allEvents.unshift({
    type,
    timestamp: new Date().toISOString(),
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

  const visibleEvents = allEvents.filter(matchesActiveTab);
  if (!visibleEvents.length) {
    eventsElement.innerHTML = '<div class="muted">no events in this tab yet</div>';
    return;
  }

  for (const event of visibleEvents.slice(0, 300)) {
    eventsElement.append(eventElement(event));
  }
}

function matchesActiveTab(event) {
  if (activeTab === 'all') {
    return true;
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

function eventElement(event) {
  const item = document.createElement('article');
  item.className = 'event';

  const header = document.createElement('div');
  header.className = 'event-header';

  const type = document.createElement('span');
  type.className = 'type';
  type.textContent = event.type || 'event';

  const timestamp = document.createElement('span');
  timestamp.textContent = event.timestamp ? new Date(event.timestamp).toLocaleTimeString() : '';

  const body = document.createElement('pre');
  body.textContent = formatEvent(event);

  header.append(type, timestamp);
  item.append(header, body);
  return item;
}

function formatEvent(event) {
  if (event.type === 'process.output') {
    return '[' + event.process + ':' + event.stream + '] ' + event.message;
  }

  if (event.type === 'process.started') {
    return '[' + event.process + '] started: ' + event.message;
  }

  if (event.type === 'process.stopping') {
    return '[' + event.process + '] stopping';
  }

  if (event.type === 'process.exited') {
    return '[' + event.process + '] exited with code ' + event.exit_code + (event.error ? ': ' + event.error : '');
  }

  if (event.type === 'process.failed') {
    return '[' + event.process + '] failed: ' + event.error;
  }

  if (event.type === 'proxy.request') {
    const route = event.route ? ' route=' + event.route : ' no-route';
    const upstream = event.upstream ? ' upstream=' + event.upstream : '';
    return event.method + ' ' + event.path + ' -> ' + event.status + route + upstream + ' (' + event.duration_ms + 'ms)';
  }

  if (event.error) {
    return event.error;
  }

  return JSON.stringify(event, null, 2);
}
