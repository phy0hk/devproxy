"use strict";

// Keep ROW_HEIGHT in sync with --row-height in app.css.
const ROW_HEIGHT = 22;
const OVERSCAN = 8;
const LIMITS = { log: 5000, lifecycle: 1000, proxy: 2000 };
const AUTOSCROLL_THRESHOLD = 48;
const PROCESS_POLL_MS = 5000;
const HIDDEN_DRAIN_MS = 500;
const STORAGE_THEME = "devproxy-theme";

const VIEWS = ["stdout", "stderr", "lifecycle", "proxy"];

const STATE_BADGE = {
    running: "badge badge-success badge-sm",
    stopping: "badge badge-info badge-sm",
    exited: "badge badge-warning badge-sm",
    failed: "badge badge-error badge-sm",
    configured: "badge badge-ghost badge-sm",
};

const state = {
    processes: [],
    routes: [],
    routeCounts: new Map(),
    selected: null,
    view: "stdout",
    runs: [],
    activeRun: null,
    selectedRun: null,
    followCurrent: true,
    logs: { stdout: [], stderr: [], lifecycle: [], proxy: [] },
    rev: { stdout: 0, stderr: 0, lifecycle: 0, proxy: 0 },
    search: "",
    follow: true,
    pending: [],
    pendingDrain: false,
    rafId: 0,
    hiddenTimer: 0,
    dirty: {
        stats: true,
        processes: true,
        routes: true,
        toolbar: true,
        terminal: true,
    },
    cards: new Map(),
    processSignature: "",
    history: {
        token: 0,
        loadedKey: "",
        loadingKey: "",
    },
    filterCache: { key: "", lines: [] },
    rowCache: { key: "", entries: [] },
    lastEventId: 0,
    runOptionsSignature: "",
    routeTimer: 0,
    refreshTimer: 0,
};

const $ = (id) => document.getElementById(id);

const ANSI = window.DevProxyAnsi;
const MAX_PENDING = 20000;
const ansiCache = new WeakMap();

function rawTextForLine(line) {
    return displayText(line);
}

function parsedForLine(line) {
    const raw = rawTextForLine(line) ?? "";
    const cached = ansiCache.get(line);
    if (cached && cached.raw === raw) return cached;

    const parsed = ANSI
        ? ANSI.parse(raw)
        : { text: raw, segments: [{ text: raw, style: {} }], clear: false };
    parsed.raw = raw;
    ansiCache.set(line, parsed);
    return parsed;
}

function visibleText(line) {
    return parsedForLine(line).text;
}

function ansiStyle(element, style) {
    if (!style) return;

    if (style.bold) element.style.fontWeight = "700";
    if (style.dim) element.style.opacity = "0.72";
    if (style.italic) element.style.fontStyle = "italic";
    if (style.underline) element.style.textDecoration = "underline";

    let fg = style.fg;
    let bg = style.bg;
    if (style.inverse) {
        const swap = fg;
        fg = bg;
        bg = swap;
        if (!fg && !bg) {
            element.style.backgroundColor = "currentColor";
            element.style.color = "var(--color-base-100)";
            return;
        }
    }

    if (fg) element.style.color = ANSI.cssColor(fg);
    if (bg) element.style.backgroundColor = ANSI.cssColor(bg);
}

function timeLabel(timestamp) {
    const date = timestamp ? new Date(timestamp) : new Date();
    return Number.isNaN(date.getTime())
        ? ""
        : date.toLocaleTimeString([], {
              hour: "2-digit",
              minute: "2-digit",
              second: "2-digit",
          });
}

function numberLabel(value) {
    return Number(value || 0).toLocaleString();
}

function bump(view) {
    state.rev[view] = (state.rev[view] || 0) + 1;
}

function pushCapped(list, item, cap) {
    list.push(item);
    if (list.length > cap) {
        list.splice(0, list.length - cap);
    }
}

function markDirty(...components) {
    for (const component of components) {
        state.dirty[component] = true;
    }
}

/* Theme ------------------------------------------------------------------ */

function applyTheme(mode) {
    const theme = mode === "light" || mode === "dark" ? mode : "system";
    if (theme === "system") {
        document.documentElement.removeAttribute("data-theme");
    } else {
        document.documentElement.setAttribute("data-theme", theme);
    }
    try {
        localStorage.setItem(STORAGE_THEME, theme);
    } catch (error) {
        /* Storage can be unavailable in private modes. */
    }
    const select = $("theme");
    if (select) select.value = theme;
}

/* Status ----------------------------------------------------------------- */

function setStatus(kind) {
    const badge = $("status");
    if (badge) {
        badge.dataset.state = kind;
        const text = $("status-text");
        if (text) {
            text.textContent =
                kind === "connected"
                    ? "connected"
                    : kind === "connecting"
                      ? "connecting"
                      : "reconnecting";
        }
    }

    const statusBar = $("log-stats")?.parentElement;
    if (statusBar) statusBar.dataset.stream = kind;

    const streamState = $("stream-state");
    if (streamState) {
        streamState.textContent =
            kind === "connected"
                ? "streaming"
                : kind === "connecting"
                  ? "connecting"
                  : "reconnecting";
    }
}

function toast(message) {
    const element = $("toast");
    if (!element) return;
    element.textContent = message;
    element.hidden = false;
    clearTimeout(toast._timer);
    toast._timer = setTimeout(() => {
        element.hidden = true;
    }, 2200);
}

/* Processes -------------------------------------------------------------- */

function processByName(name) {
    return state.processes.find((process) => process.name === name) ?? null;
}

async function loadProcesses() {
    try {
        const response = await fetch("/api/processes", {
            headers: { Accept: "application/json" },
        });
        if (!response.ok) throw new Error(`process list: ${response.status}`);
        const processes = await response.json();
        if (!Array.isArray(processes)) throw new Error("invalid process list");

        state.processes = processes;
        if (!state.selected || !processByName(state.selected)) {
            const next = processes[0]?.name ?? null;
            selectProcess(next, { initial: true });
        }

        markDirty("processes", "toolbar", "stats");
        scheduleFrame();
    } catch (error) {
        setStatus("disconnected");
    }
}

function createCard() {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "instance-item";

    const top = document.createElement("span");
    top.className = "instance-top";

    const name = document.createElement("span");
    name.className = "instance-name";

    const badge = document.createElement("span");
    badge.className = `instance-state ${STATE_BADGE.configured}`;

    top.append(name, badge);

    const command = document.createElement("span");
    command.className = "instance-command";

    const meta = document.createElement("span");
    meta.className = "instance-meta";

    button.append(top, command, meta);
    return button;
}

function updateCard(card, process) {
    const selected = process.name === state.selected;
    card.classList.toggle("is-selected", selected);
    card.setAttribute("aria-current", selected ? "true" : "false");
    card.dataset.name = process.name;

    card.querySelector(".instance-name").textContent = process.name;

    const badge = card.querySelector(".instance-state");
    badge.textContent = process.state;
    badge.className = `instance-state ${STATE_BADGE[process.state] ?? STATE_BADGE.configured}`;

    card.querySelector(".instance-command").textContent = process.command ?? "";

    const bits = [];
    if (process.pid) bits.push(`pid ${process.pid}`);
    if (process.run_id) bits.push(process.run_id);
    if (process.exit_code !== undefined && process.exit_code !== null) {
        bits.push(`exit ${process.exit_code}`);
    }
    if (process.state === "failed" && process.last_error) {
        bits.push(process.last_error);
    }

    const meta = card.querySelector(".instance-meta");
    meta.textContent = bits.join(" · ");
    meta.classList.toggle(
        "is-error",
        process.state === "failed" && Boolean(process.last_error),
    );
    card.title = [process.name, process.command, bits.join(" · ")]
        .filter(Boolean)
        .join("\n");
}

function renderProcesses() {
    state.dirty.processes = false;
    const container = $("processes");
    const signature = state.processes.map((process) => process.name).join("\u0000");

    if (signature !== state.processSignature) {
        state.processSignature = signature;
        container.replaceChildren();
        state.cards.clear();

        if (!state.processes.length) {
            const empty = document.createElement("p");
            empty.className = "empty-note";
            empty.textContent = "No managed processes in config.yaml.";
            container.appendChild(empty);
        } else {
            for (const process of state.processes) {
                const card = createCard();
                card.addEventListener("click", () => selectProcess(process.name));
                state.cards.set(process.name, card);
                container.appendChild(card);
            }
        }
    }

    for (const process of state.processes) {
        const card = state.cards.get(process.name);
        if (card) updateCard(card, process);
    }

    const badge = $("instances-badge");
    if (badge) badge.textContent = String(state.processes.length);
}

/* Routes ----------------------------------------------------------------- */

async function loadRoutes() {
    try {
        const response = await fetch("/api/routes", {
            headers: { Accept: "application/json" },
        });
        if (!response.ok) throw new Error(`routes: ${response.status}`);
        const routes = await response.json();
        state.routes = Array.isArray(routes) ? routes : [];
    } catch (error) {
        state.routes = [];
    }
    markDirty("routes", "stats");
    scheduleFrame();
}

function renderRoutes() {
    state.dirty.routes = false;
    const container = $("routes");
    if (!container) return;

    container.replaceChildren();
    if (!state.routes.length) {
        const empty = document.createElement("p");
        empty.className = "empty-note";
        empty.textContent = "No proxy targets configured.";
        container.appendChild(empty);
        return;
    }

    for (const route of state.routes) {
        const item = document.createElement("div");
        item.className = "route-item";

        const name = document.createElement("span");
        name.className = "route-name";
        name.textContent = route.name;

        const count = document.createElement("span");
        count.className = "route-count";
        const hits = state.routeCounts.get(route.name) ?? 0;
        count.textContent = hits ? `${numberLabel(hits)} hits` : "0 hits";

        const path = document.createElement("span");
        path.className = "route-path";
        path.textContent = route.path;

        const upstream = document.createElement("span");
        upstream.className = "route-upstream";
        upstream.textContent = `${route.upstream}${route.rewrite ? " · rewrite" : ""}`;
        upstream.title = upstream.textContent;

        item.append(name, count, path, upstream);
        container.appendChild(item);
    }
}

function noteRouteHit(route) {
    if (!route) return;
    state.routeCounts.set(route, (state.routeCounts.get(route) ?? 0) + 1);

    if (state.routeTimer) return;
    state.routeTimer = setTimeout(() => {
        state.routeTimer = 0;
        markDirty("routes");
        scheduleFrame();
    }, 500);
}

/* Selection and runs ----------------------------------------------------- */

function selectProcess(name, options = {}) {
    if (!options.initial && name === state.selected) return;

    state.selected = name;
    state.followCurrent = true;
    state.selectedRun = null;
    state.activeRun = null;
    state.runs = [];
    state.logs.stdout = [];
    state.logs.stderr = [];
    bump("stdout");
    bump("stderr");
    state.history.loadedKey = "";
    state.history.loadingKey = "";
    state.follow = true;

    markDirty("processes", "toolbar", "terminal", "stats");
    scheduleFrame();

    if (name) loadHistory();
}

function historyKey() {
    const run = state.followCurrent ? "current" : state.selectedRun || "current";
    return `${state.selected}|${run}`;
}

async function loadHistory() {
    if (!state.selected) return;

    const key = historyKey();
    if (key === state.history.loadedKey || key === state.history.loadingKey) {
        return;
    }

    const token = ++state.history.token;
    const name = state.selected;
    const run = state.followCurrent ? "current" : state.selectedRun || "current";
    state.history.loadingKey = key;

    const fetchRun = async (runName) => {
        const params = new URLSearchParams({ run: runName, limit: String(LIMITS.log) });
        const response = await fetch(
            `/api/processes/${encodeURIComponent(name)}/logs?${params}`,
            { headers: { Accept: "application/json" } },
        );
        if (!response.ok) throw new Error(`history: ${response.status}`);
        return response.json();
    };

    try {
        const current = await fetchRun(run);
        let lines = Array.isArray(current.lines) ? current.lines : [];
        const allowedRuns = [current.run];

        // Live mode keeps the previous run as a visible boundary after a
        // restart instead of discarding it on reload.
        if (state.followCurrent) {
            try {
                const previous = await fetchRun("previous");
                if (Array.isArray(previous.lines) && previous.lines.length) {
                    lines = [...previous.lines, ...lines];
                    allowedRuns.push(previous.run);
                }
            } catch (error) {
                /* No previous run retained. */
            }
        }

        if (token !== state.history.token || state.selected !== name) return;

        state.runs = Array.isArray(current.runs) ? current.runs : [];
        state.activeRun = current.run || null;
        if (state.followCurrent) state.selectedRun = current.run || null;

        installHistory(
            "stdout",
            lines.filter((line) => line.stream !== "stderr"),
            allowedRuns,
        );
        installHistory(
            "stderr",
            lines.filter((line) => line.stream === "stderr"),
            allowedRuns,
        );

        state.history.loadedKey = key;

        markDirty("toolbar", "terminal", "stats");
        scheduleFrame();
    } catch (error) {
        if (token === state.history.token) {
            state.history.loadedKey = "";
            markDirty("toolbar", "terminal");
            scheduleFrame();
        }
    } finally {
        if (token === state.history.token) state.history.loadingKey = "";
    }
}

function installHistory(stream, historical, allowedRuns) {
    const allowed = new Set((allowedRuns || []).filter(Boolean));
    const merged = new Map();

    for (const line of historical) {
        if (line.sequence) merged.set(line.sequence, line);
    }

    for (const line of state.logs[stream]) {
        if (line.process !== state.selected) continue;
        if (allowed.size && line.run_id && !allowed.has(line.run_id)) continue;
        if (!merged.has(line.sequence)) merged.set(line.sequence, line);
    }

    const lines = [...merged.values()]
        .sort((left, right) => (left.sequence ?? 0) - (right.sequence ?? 0))
        .slice(-LIMITS.log);

    state.logs[stream] = lines;
    bump(stream);
}

function scheduleHistoryReload(delay = 200) {
    clearTimeout(scheduleHistoryReload._timer);
    scheduleHistoryReload._timer = setTimeout(() => {
        state.history.loadedKey = "";
        state.history.loadingKey = "";
        loadHistory();
    }, delay);
}

function loadRunMetadata() {
    if (!state.selected) return;
    fetch(
        `/api/processes/${encodeURIComponent(state.selected)}/logs?run=current&limit=1`,
        { headers: { Accept: "application/json" } },
    )
        .then((response) => (response.ok ? response.json() : null))
        .then((data) => {
            if (!data || !Array.isArray(data.runs)) return;
            state.runs = data.runs;
            markDirty("toolbar");
            scheduleFrame();
        })
        .catch(() => {});
}

/* Incoming events -------------------------------------------------------- */

function applyEvent(item) {
    if (item.sequence) {
        state.lastEventId = Math.max(state.lastEventId, item.sequence);
    }

    const type = item.type || "";

    if (type === "process.output") {
        const stream = item.stream === "stderr" ? "stderr" : "stdout";
        if (item.process === state.selected && acceptsOutput(item)) {
            pushCapped(state.logs[stream], item, LIMITS.log);
            bump(stream);
            if (state.view === stream) markDirty("terminal");
        }
        return;
    }

    if (type.startsWith("process.")) {
        pushCapped(state.logs.lifecycle, item, LIMITS.lifecycle);
        bump("lifecycle");

        if (item.process === state.selected) {
            if (state.view === "lifecycle") markDirty("terminal");
            markDirty("toolbar");
        }

        if (type === "process.started" && item.process === state.selected) {
            if (state.followCurrent) {
                scheduleHistoryReload(250);
            } else {
                loadRunMetadata();
            }
        }

        scheduleProcessRefresh();
        return;
    }

    if (type === "proxy.request" || type === "proxy.error") {
        pushCapped(state.logs.proxy, item, LIMITS.proxy);
        bump("proxy");
        if (state.view === "proxy") markDirty("terminal");
        noteRouteHit(item.route);
    }
}

function acceptsOutput(item) {
    if (state.followCurrent) {
        if (!state.activeRun || !item.run_id) return true;
        return item.run_id === state.activeRun;
    }
    return item.run_id === state.selectedRun;
}

function scheduleProcessRefresh() {
    if (state.refreshTimer) return;
    state.refreshTimer = setTimeout(() => {
        state.refreshTimer = 0;
        loadProcesses();
    }, 250);
}

/* Frame scheduling ------------------------------------------------------- */

function scheduleFrame(drain = false) {
    if (drain) state.pendingDrain = true;

    if (document.hidden) {
        if (!state.hiddenTimer) {
            state.hiddenTimer = setTimeout(() => {
                state.hiddenTimer = 0;
                if (state.pendingDrain) {
                    state.pendingDrain = false;
                    drainPending();
                }
            }, HIDDEN_DRAIN_MS);
        }
        return;
    }

    if (state.rafId) return;
    state.rafId = requestAnimationFrame(() => {
        state.rafId = 0;
        if (state.pendingDrain) {
            state.pendingDrain = false;
            drainPending();
        }
        render();
    });
}

function drainPending() {
    if (!state.pending.length) return;
    const pending = state.pending;
    state.pending = [];
    for (const item of pending) applyEvent(item);
}

/* Rendering -------------------------------------------------------------- */

function render() {
    if (document.hidden) return;

    if (state.dirty.stats) renderStats();
    if (state.dirty.processes) renderProcesses();
    if (state.dirty.routes) renderRoutes();
    if (state.dirty.toolbar) renderToolbar();
    if (state.dirty.terminal) renderTerminal();

    renderStreamMeta();
}

function renderStats() {
    state.dirty.stats = false;

    const running = state.processes.filter(
        (process) => process.state === "running",
    ).length;
    const failing = state.processes.filter(
        (process) => process.state === "failed",
    ).length;

    $("process-count").textContent = numberLabel(state.processes.length);
    $("running-count").textContent = numberLabel(running);
    $("error-count").textContent = numberLabel(failing);
    $("route-count").textContent = numberLabel(state.routes.length);

    document.title = failing
        ? `DevProxy · ${failing} failing`
        : `DevProxy · ${running} running`;
}

function renderToolbar() {
    state.dirty.toolbar = false;

    for (const tab of document.querySelectorAll("[data-view]")) {
        const active = tab.dataset.view === state.view;
        tab.setAttribute("aria-selected", active ? "true" : "false");
        tab.tabIndex = active ? 0 : -1;
    }

    const process = processByName(state.selected);
    const title = $("terminal-title");
    const stateBadge = $("process-state");
    const meta = $("process-meta");

    title.textContent = process ? process.name : "Instance output";

    if (!process) {
        stateBadge.textContent = "idle";
        stateBadge.className = STATE_BADGE.configured;
        meta.textContent = "No instance selected";
        meta.title = "";
    } else {
        stateBadge.textContent = process.state;
        stateBadge.className =
            STATE_BADGE[process.state] ?? STATE_BADGE.configured;

        const bits = [process.command];
        if (process.pid) bits.push(`pid ${process.pid}`);
        if (process.run_id) bits.push(process.run_id);
        if (process.started_at) bits.push(`started ${timeLabel(process.started_at)}`);
        if (process.last_error) bits.push(process.last_error);

        meta.textContent = bits.filter(Boolean).join(" · ");
        meta.title = meta.textContent;
    }

    const logView = state.view === "stdout" || state.view === "stderr";
    const runSelect = $("run-select");
    runSelect.disabled = !process || !logView;
    runSelect.title = logView
        ? "Choose the process run shown in this view"
        : "Runs apply to stdout and stderr views";

    const signature = [
        state.runs.map((run) => run.id).join(","),
        state.followCurrent ? "live" : "pinned",
        state.selectedRun ?? "",
    ].join("|");

    if (signature !== state.runOptionsSignature) {
        state.runOptionsSignature = signature;
        runSelect.replaceChildren();

        const live = document.createElement("option");
        live.value = "live";
        live.textContent = "Live (current run)";
        runSelect.appendChild(live);

        state.runs.forEach((run, index) => {
            const option = document.createElement("option");
            option.value = run.id;
            const tag = index === 0 ? "current" : index === 1 ? "previous" : "older";
            const clock = timeLabel(run.started_at);
            option.textContent = `${run.id} · ${tag}${clock ? ` · ${clock}` : ""}${run.ended_at ? "" : " · live"}`;
            runSelect.appendChild(option);
        });
    }

    runSelect.value = state.followCurrent
        ? "live"
        : state.selectedRun && state.runs.some((run) => run.id === state.selectedRun)
          ? state.selectedRun
          : "live";

    const running = process?.state === "running";
    const stopping = process?.state === "stopping";
    $("action-start").disabled = !process || running || stopping;
    $("action-restart").disabled = !process;
    $("action-stop").disabled = !process || !running;
}

function displayText(line) {
    switch (state.view) {
        case "proxy": {
            const parts = [
                line.method,
                line.path,
                line.status,
                `${line.duration_ms ?? 0}ms`,
            ];
            if (line.route) parts.push(`route=${line.route}`);
            if (line.upstream) parts.push(`upstream=${line.upstream}`);
            return parts.filter(Boolean).join(" ");
        }
        case "lifecycle": {
            const detail = line.error || line.message || "";
            return `${line.type}${detail ? ` — ${detail}` : ""}`;
        }
        default:
            return line.message ?? "";
    }
}

function plainLine(line) {
    const time = timeLabel(line.timestamp);
    return `${time}  ${visibleText(line)}`;
}

function statusSnapshot(process) {
    const bits = [`state=${process.state}`];
    if (process.run_id) bits.push(process.run_id);
    if (process.exit_code !== undefined && process.exit_code !== null) {
        bits.push(`exit=${process.exit_code}`);
    }
    if (process.last_error) bits.push(process.last_error);

    return {
        type: "process.status",
        process: process.name,
        message: bits.join(" · "),
        timestamp:
            process.exited_at ||
            process.started_at ||
            new Date().toISOString(),
        run_id: process.run_id,
    };
}

function viewLines() {
    const lines = state.logs[state.view] ?? [];
    if (state.view === "lifecycle") {
        const events = lines.filter((line) => line.process === state.selected);
        if (events.length) return events;

        // Lifecycle events are not persisted server-side. Show the current
        // status as a derived row so a reload under heavy output is not blank.
        const process = processByName(state.selected);
        return process ? [statusSnapshot(process)] : [];
    }
    return lines;
}

function filteredLines() {
    const key = [
        state.view,
        state.selected ?? "",
        state.activeRun ?? "",
        state.followCurrent ? "live" : "pinned",
        state.rev[state.view] ?? 0,
        state.search,
    ].join("|");

    if (key === state.filterCache.key) return state.filterCache.lines;

    let lines = viewLines();
    if (state.search) {
        const needle = state.search.toLowerCase();
        lines = lines.filter((line) =>
            visibleText(line).toLowerCase().includes(needle),
        );
    }

    state.filterCache = { key, lines };
    return lines;
}

function runLabel(runID) {
    const run = state.runs.find((item) => item.id === runID);
    const clock = run ? timeLabel(run.started_at) : "";
    return `run ${runID}${clock ? ` · ${clock}` : ""}`;
}

function displayEntries() {
    const key = [
        state.view,
        state.selected ?? "",
        state.activeRun ?? "",
        state.followCurrent ? "live" : "pinned",
        state.rev[state.view] ?? 0,
        state.search,
        state.runs.map((run) => run.id).join(","),
    ].join("|");

    if (key === state.rowCache.key) return state.rowCache.entries;

    const lines = filteredLines();
    const entries = [];
    let lastRun = null;

    for (const line of lines) {
        const parsed = parsedForLine(line);
        if (parsed.clear) {
            entries.push({ kind: "clear" });
        }

        if (line.run_id && line.run_id !== lastRun) {
            entries.push({ kind: "run", runID: line.run_id });
            lastRun = line.run_id;
        }

        if (parsed.text) {
            entries.push({ kind: "line", line });
        }
    }

    state.rowCache = { key, entries };
    return entries;
}

function rowClass(line) {
    if (state.view !== "proxy") return `log-row ${state.view}`;

    const status = Number(line.status || 0);
    const tone =
        status >= 500
            ? "status-error"
            : status >= 400
              ? "status-warn"
              : "status-ok";
    return `log-row proxy ${tone}`;
}

function appendHighlighted(parent, text, needle) {
    if (!needle) {
        parent.textContent = text;
        return;
    }

    const haystack = text.toLowerCase();
    const query = needle.toLowerCase();
    let index = 0;

    while (index < text.length) {
        const found = haystack.indexOf(query, index);
        if (found < 0) {
            parent.appendChild(document.createTextNode(text.slice(index)));
            return;
        }
        if (found > index) {
            parent.appendChild(document.createTextNode(text.slice(index, found)));
        }
        const mark = document.createElement("mark");
        mark.textContent = text.slice(found, found + query.length);
        parent.appendChild(mark);
        index = found + query.length;
    }
}

function appendStyledText(content, line, needle) {
    const parsed = parsedForLine(line);
    const segments = parsed.segments.length
        ? parsed.segments
        : [{ text: parsed.text, style: {} }];

    for (const segment of segments) {
        const span = document.createElement("span");
        ansiStyle(span, segment.style);
        appendHighlighted(span, segment.text, needle);
        content.appendChild(span);
    }
}

function buildRow(entry, needle) {
    const row = document.createElement("div");
    row.style.height = `${ROW_HEIGHT}px`;

    if (entry.kind !== "line") {
        row.className = `log-row log-separator ${entry.kind === "clear" ? "log-clear" : "log-run"}`;

        const label = document.createElement("span");
        label.className = "log-separator-label";
        label.textContent =
            entry.kind === "clear"
                ? "── screen cleared · server history kept ──"
                : `── ${runLabel(entry.runID)} ──`;

        row.appendChild(label);
        return row;
    }

    const line = entry.line;
    row.className = rowClass(line);

    const time = document.createElement("span");
    time.className = "log-time";
    time.textContent = timeLabel(line.timestamp);

    const content = document.createElement("span");
    content.className = "log-content";
    appendStyledText(content, line, needle);

    row.append(time, content);
    return row;
}

function renderTerminal() {
    state.dirty.terminal = false;

    const scroller = $("log-scroller");
    const rows = $("log-rows");
    const spacerTop = $("log-spacer-top");
    const spacerBottom = $("log-spacer-bottom");
    const empty = $("log-empty");
    const jump = $("jump-live");

    const entries = displayEntries();
    const needle = state.search.trim();
    const total = entries.length;

    empty.hidden = total > 0;

    if (!total) {
        rows.replaceChildren();
        spacerTop.style.height = "0px";
        spacerBottom.style.height = "0px";
        $("log-stats").textContent = "0 lines";
        jump.hidden = true;
        $("copy").disabled = true;
        renderSearchStats(0, 0);
        return;
    }

    const viewport = scroller.clientHeight || 1;
    const visibleCount = Math.max(1, Math.ceil(viewport / ROW_HEIGHT));
    const maxStart = Math.max(0, total - visibleCount);
    const viewportStart = state.follow
        ? maxStart
        : Math.floor(scroller.scrollTop / ROW_HEIGHT);
    const start = Math.max(0, Math.min(maxStart, viewportStart - OVERSCAN));
    const end = state.follow
        ? total
        : Math.min(total, viewportStart + visibleCount + OVERSCAN);

    spacerTop.style.height = `${start * ROW_HEIGHT}px`;
    spacerBottom.style.height = `${Math.max(0, total - end) * ROW_HEIGHT}px`;

    const fragment = document.createDocumentFragment();
    for (let index = start; index < end; index += 1) {
        fragment.appendChild(buildRow(entries[index], needle));
    }
    rows.replaceChildren(fragment);

    if (state.follow) {
        scroller.scrollTop = scroller.scrollHeight;
    }

    jump.hidden = state.follow;
    $("copy").disabled = false;
    const lineCount = filteredLines().length;
    $("log-stats").textContent = `${numberLabel(lineCount)} lines · ${state.follow ? "live" : "paused"} · entries ${numberLabel(start + 1)}–${numberLabel(end)} of ${numberLabel(total)}`;
    renderSearchStats(lineCount, viewLines().length);
}

function renderSearchStats(matches, total) {
    const element = $("search-stats");
    if (!state.search) {
        element.classList.add("hidden");
        element.textContent = "";
        return;
    }
    element.classList.remove("hidden");
    element.textContent = `${numberLabel(matches)} of ${numberLabel(total)} matched`;
}

function renderStreamMeta() {
    const sequence = $("stream-sequence");
    const label = `seq ${numberLabel(state.lastEventId)}`;
    if (sequence && sequence.textContent !== label) {
        sequence.textContent = label;
    }
}

/* User interactions ------------------------------------------------------ */

async function performAction(name, action) {
    if (!name) return;

    const button = $(`action-${action}`);
    const previous = button?.disabled;
    if (button) button.disabled = true;

    try {
        const response = await fetch(
            `/api/processes/${encodeURIComponent(name)}/${action}`,
            { method: "POST", headers: { Accept: "application/json" } },
        );
        if (!response.ok) {
            throw new Error((await response.text()).trim() || `request failed`);
        }

        const statuses = await response.json().catch(() => null);
        if (Array.isArray(statuses)) {
            state.processes = statuses;
            markDirty("processes", "toolbar", "stats");
            scheduleFrame();
        }

        if ((action === "start" || action === "restart") && state.followCurrent) {
            scheduleHistoryReload(300);
        }
        scheduleProcessRefresh();
    } catch (error) {
        pushLocalLifecycle(name, `process.${action}.error`, error.message);
        toast(`${action} ${name}: ${error.message}`);
    } finally {
        if (button && previous !== undefined) button.disabled = previous;
        markDirty("toolbar");
        scheduleFrame();
    }
}

function pushLocalLifecycle(process, type, message) {
    pushCapped(
        state.logs.lifecycle,
        {
            type,
            process,
            message,
            timestamp: new Date().toISOString(),
            run_id: state.activeRun || undefined,
        },
        LIMITS.lifecycle,
    );
    bump("lifecycle");
    if (state.view === "lifecycle") markDirty("terminal");
    scheduleFrame();
}

function setView(view) {
    if (!VIEWS.includes(view) || state.view === view) return;
    state.view = view;
    state.follow = true;
    markDirty("toolbar", "terminal", "stats");
    scheduleFrame();
}

function setRun(value) {
    if (!state.selected) return;

    if (value === "live") {
        state.followCurrent = true;
        state.selectedRun = null;
    } else {
        state.followCurrent = false;
        state.selectedRun = value;
        state.activeRun = value;
    }

    state.logs.stdout = [];
    state.logs.stderr = [];
    bump("stdout");
    bump("stderr");
    state.history.loadedKey = "";
    state.history.loadingKey = "";
    state.follow = true;

    markDirty("toolbar", "terminal");
    scheduleFrame();
    loadHistory();
}

function clearView() {
    if (state.view === "proxy") {
        state.logs.proxy = [];
    } else if (state.view === "lifecycle") {
        state.logs.lifecycle = state.logs.lifecycle.filter(
            (line) => line.process !== state.selected,
        );
    } else {
        state.logs[state.view] = [];
    }

    bump(state.view);
    state.follow = true;
    markDirty("terminal", "stats");
    scheduleFrame();
}

async function copyView() {
    const lines = filteredLines();
    if (!lines.length) {
        toast("Nothing to copy");
        return;
    }

    const text = lines.map(plainLine).join("\n");

    try {
        if (navigator.clipboard?.writeText) {
            await navigator.clipboard.writeText(text);
        } else {
            const area = document.createElement("textarea");
            area.value = text;
            area.style.position = "fixed";
            area.style.opacity = "0";
            document.body.appendChild(area);
            area.select();
            document.execCommand("copy");
            area.remove();
        }
        toast(`Copied ${numberLabel(lines.length)} lines`);
    } catch (error) {
        toast("Clipboard unavailable");
    }
}

function jumpToLive() {
    state.follow = true;
    const scroller = $("log-scroller");
    scroller.scrollTop = scroller.scrollHeight;
    markDirty("terminal");
    scheduleFrame();
}

/* Event stream ----------------------------------------------------------- */

let eventSource = null;

function connectEvents() {
    if (eventSource) eventSource.close();
    setStatus("connecting");

    eventSource = new EventSource("/events");

    eventSource.onopen = () => setStatus("connected");
    eventSource.onerror = () => setStatus("disconnected");

    eventSource.addEventListener("devproxy", (event) => {
        let payload;
        try {
            payload = JSON.parse(event.data);
        } catch (error) {
            return;
        }

        if (event.lastEventId) {
            const sequence = Number(event.lastEventId);
            if (Number.isFinite(sequence)) {
                state.lastEventId = Math.max(state.lastEventId, sequence);
            }
        }

        const items = Array.isArray(payload) ? payload : [payload];
        for (const item of items) {
            state.pending.push(item);
        }

        if (state.pending.length > MAX_PENDING) {
            state.pending.splice(0, state.pending.length - MAX_PENDING);
        }

        scheduleFrame(true);
    });
}

/* Bindings and startup --------------------------------------------------- */

function bind() {
    for (const tab of document.querySelectorAll("[data-view]")) {
        tab.addEventListener("click", () => setView(tab.dataset.view));
    }

    $("refresh").addEventListener("click", () => {
        loadProcesses();
        loadRoutes();
    });

    $("theme").addEventListener("change", (event) => {
        applyTheme(event.target.value);
    });

    $("action-start").addEventListener("click", () =>
        performAction(state.selected, "start"),
    );
    $("action-restart").addEventListener("click", () =>
        performAction(state.selected, "restart"),
    );
    $("action-stop").addEventListener("click", () =>
        performAction(state.selected, "stop"),
    );

    $("run-select").addEventListener("change", (event) =>
        setRun(event.target.value),
    );

    $("search").addEventListener("input", (event) => {
        state.search = event.target.value.trim();
        markDirty("terminal", "stats");
        scheduleFrame();
    });

    $("copy").addEventListener("click", copyView);
    $("clear").addEventListener("click", clearView);
    $("jump-live").addEventListener("click", jumpToLive);

    const scroller = $("log-scroller");
    scroller.addEventListener("scroll", () => {
        const distance =
            scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight;
        const atBottom = distance <= AUTOSCROLL_THRESHOLD;

        if (atBottom !== state.follow) {
            state.follow = atBottom;
            markDirty("terminal");
            scheduleFrame();
        } else if (!atBottom) {
            markDirty("terminal");
            scheduleFrame();
        }
    });

    window.addEventListener("resize", () => {
        markDirty("terminal");
        scheduleFrame();
    });

    document.addEventListener("visibilitychange", () => {
        if (document.hidden) return;

        if (state.hiddenTimer) {
            clearTimeout(state.hiddenTimer);
            state.hiddenTimer = 0;
        }

        if (state.pendingDrain) {
            state.pendingDrain = false;
            drainPending();
        }

        markDirty("stats", "processes", "routes", "toolbar", "terminal");
        scheduleFrame();
        loadProcesses();
    });
}

function init() {
    let stored = "system";
    try {
        stored = localStorage.getItem(STORAGE_THEME) || "system";
    } catch (error) {
        /* Ignore unavailable storage. */
    }
    applyTheme(stored);

    bind();

    loadProcesses().then(() => {
        connectEvents();
    });
    loadRoutes();

    setInterval(() => {
        if (!document.hidden) loadProcesses();
    }, PROCESS_POLL_MS);

    markDirty("stats", "processes", "routes", "toolbar", "terminal");
    scheduleFrame();
}

init();
