const MAX_LINES = 400;

const state = {
    processes: [],
    selected: null,
    view: "stdout",
    output: [],
    lifecycle: [],
    proxy: [],
};

const $ = (id) => document.getElementById(id);

function escapeHTML(value) {
    return String(value ?? "").replace(
        /[&<>"']/g,
        (c) =>
            ({
                "&": "&",
                "<": "<",
                ">": ">",
                '"': """,
                "'": "'",
            })[c],
    );
}

function timeLabel(timestamp) {
    const date = timestamp ? new Date(timestamp) : new Date();
    return Number.isNaN(date.getTime()) ? "" : date.toLocaleTimeString();
}

function setStatus(kind) {
    const badge = $("status");
    const labels = {
        connected: "connected",
        disconnected: "disconnected, retrying...",
    };
    const classes = {
        connected: "badge badge-success",
        disconnected: "badge badge-error",
    };
    badge.textContent = labels[kind] ?? kind;
    badge.className = classes[kind] ?? "badge badge-outline";
}

async function loadProcesses() {
    try {
        const response = await fetch("/api/processes");
        if (!response.ok) throw new Error("request failed");
        state.processes = await response.json();

        if (
            !state.selected ||
            !state.processes.some((p) => p.name === state.selected)
        ) {
            state.selected = state.processes[0]?.name ?? null;
        }

        setStatus("connected");
        renderProcesses();
        renderOutput();
    } catch (error) {
        setStatus("disconnected");
    }
}

function stateBadge(state) {
    const map = {
        running: "badge-success",
        stopping: "badge-info",
        exited: "badge-warning",
        failed: "badge-error",
        configured: "badge-ghost",
    };
    return map[state] ?? "badge-ghost";
}

function renderProcesses() {
    const running = state.processes.filter((p) => p.state === "running").length;
    $("process-count").textContent = state.processes.length;
    $("running-count").textContent = running;

    const container = $("processes");
    if (!state.processes.length) {
        container.innerHTML =
            '<div class="text-sm text-base-content/50">No managed processes.</div>';
        return;
    }

    container.innerHTML = state.processes
        .map((p) => {
            const meta = [
                p.pid ? `pid ${p.pid}` : null,
                p.exit_code !== undefined ? `exit ${p.exit_code}` : null,
                p.last_error,
            ]
                .filter(Boolean)
                .join(" · ");
            const selected =
                p.name === state.selected ? "border border-primary" : "";
            return `
      <article class="card bg-base-200 ${selected}" data-name="${escapeHTML(p.name)}">
        <div class="card-body py-3">
          <div class="flex items-center justify-between gap-2">
            <h3 class="font-semibold">${escapeHTML(p.name)}</h3>
            <span class="${stateBadge(p.state)}">${escapeHTML(p.state)}</span>
          </div>
          <p class="truncate font-mono text-xs text-base-content/60">${escapeHTML(p.command)}</p>
          ${meta ? `<p class="truncate text-xs text-base-content/40">${escapeHTML(meta)}</p>` : ""}
          <div class="card-actions justify-end gap-2 pt-1">
            <button class="btn btn-xs btn-primary" data-action="start">Start</button>
            <button class="btn btn-xs" data-action="restart">Restart</button>
            <button class="btn btn-xs btn-error" data-action="stop">Stop</button>
          </div>
        </div>
      </article>`;
        })
        .join("");
}

function pushLifecycle(name, type, message) {
    state.lifecycle.push({
        type,
        process: name,
        message,
        timestamp: new Date().toISOString(),
    });
    state.lifecycle = state.lifecycle.slice(-MAX_LINES);
    renderOutput();
}

function renderTabs() {
    const tabs = [
        { id: "stdout", label: "stdout" },
        { id: "stderr", label: "stderr" },
        { id: "lifecycle", label: "lifecycle" },
        { id: "proxy", label: "proxy" },
    ];
    $("tabs").innerHTML = tabs
        .map(
            (tab) =>
                `<button role="tab" class="tab ${state.view === tab.id ? "tab-active" : ""}" data-view="${tab.id}">${tab.label}</button>`,
        )
        .join("");
}

function visibleLines() {
    const name = state.selected;
    switch (state.view) {
        case "stdout":
            return state.output.filter(
                (l) => l.process === name && l.stream === "stdout",
            );
        case "stderr":
            return state.output.filter(
                (l) => l.process === name && l.stream === "stderr",
            );
        case "lifecycle":
            return state.lifecycle.filter((l) => l.process === name);
        case "proxy":
            return state.proxy;
        default:
            return [];
    }
}

function lineHTML(line) {
    const time = `<span class="terminal-time">${escapeHTML(timeLabel(line.timestamp))}</span>`;

    if (state.view === "proxy") {
        const statusClass =
            line.status >= 500
                ? "text-error"
                : line.status >= 400
                  ? "text-warning"
                  : "text-success";
        const summary = `${line.method} ${line.path} \u2192 ${line.upstream} <span class="${statusClass}">${line.status}</span> ${line.duration_ms}ms`;
        return `<div class="terminal-line proxy">${time}${summary}</div>`;
    }

    if (state.view === "lifecycle") {
        const detail = line.error || line.message || "";
        return `<div class="terminal-line lifecycle">${time}${escapeHTML(line.type)}${detail ? ` \u2014 ${escapeHTML(detail)}` : ""}</div>`;
    }

    return `<div class="terminal-line ${line.stream}">${time}${escapeHTML(line.message)}</div>`;
}

function renderOutput() {
    renderTabs();
    const selected =
        state.processes.find((p) => p.name === state.selected) ?? null;
    $("terminal-title").textContent = selected
        ? `${selected.name} \u00b7 ${state.view}`
        : "Instance output";

    const lines = visibleLines();
    const terminal = $("events");
    terminal.innerHTML = lines.length
        ? lines.map(lineHTML).join("")
        : '<span class="terminal-empty">No output in this view yet.</span>';
    terminal.scrollTop = terminal.scrollHeight;
}

// Map view to event types for server-side filtering
function eventTypesForView(view) {
    switch (view) {
        case "stdout":
            return ["process.output"];
        case "stderr":
            return ["process.output"]; // We filter stream client-side
        case "lifecycle":
            return [
                "process.started",
                "process.stopping",
                "process.exited",
                "process.failed",
                "process.output_error",
            ];
        case "proxy":
            return ["proxy.request", "proxy.error"];
        default:
            return [];
    }
}

let eventSource = null;

function connectEvents() {
    // Close existing connection if any
    if (eventSource) {
        eventSource.close();
    }

    const types = eventTypesForView(state.view);
    const url = types.length > 0
        ? `/events?types=${encodeURIComponent(types.join(","))}`
        : "/events";

    eventSource = new EventSource(url);
    eventSource.onopen = () => setStatus("connected");
    eventSource.onerror = () => setStatus("disconnected");

    eventSource.addEventListener("devproxy", (event) => {
        let item;
        try {
            item = JSON.parse(event.data);
        } catch (error) {
            return;
        }

        switch (item.type) {
            case "process.output":
                state.output.push(item);
                state.output = state.output.slice(-MAX_LINES);
                break;
            case "proxy.request":
            case "proxy.error":
                state.proxy.push(item);
                state.proxy = state.proxy.slice(-MAX_LINES);
                break;
            default:
                if (item.type && item.type.startsWith("process.")) {
                    state.lifecycle.push(item);
                    state.lifecycle = state.lifecycle.slice(-MAX_LINES);
                }
        }

        if (
            item.type &&
            item.type.startsWith("process.") &&
            item.type !== "process.output"
        ) {
            loadProcesses();
        }
        renderOutput();
    });
}

function reconnectEvents() {
    connectEvents();
}

// Event delegation for process cards and action buttons
function setupProcessDelegation() {
    const container = $("processes");
    if (!container) return;

    // Use a single click handler on the container for all process cards
    container.addEventListener("click", (event) => {
        // Handle card selection (click on card but not on buttons)
        const card = event.target.closest(".card");
        if (card && !event.target.closest("[data-action]")) {
            state.selected = card.dataset.name;
            state.view = "stdout";
            renderProcesses();
            renderOutput();
            reconnectEvents();
            return;
        }

        // Handle action buttons (start, restart, stop)
        const button = event.target.closest("[data-action]");
        if (button) {
            event.stopPropagation();
            const card = button.closest(".card");
            if (!card) return;

            button.disabled = true;
            const action = button.dataset.action;
            const name = card.dataset.name;

            fetch(`/api/processes/${encodeURIComponent(name)}/${action}`, {
                method: "POST",
            })
                .then((response) => {
                    if (!response.ok) {
                        return response.text().then((detail) => {
                            throw new Error(detail);
                        });
                    }
                })
                .catch((error) => {
                    pushLifecycle(
                        name,
                        `process.${action}.error`,
                        error.message,
                    );
                })
                .finally(() => {
                    button.disabled = false;
                    loadProcesses();
                });
        }
    });
}

// Event delegation for tabs
function setupTabDelegation() {
    const tabsContainer = $("tabs");
    if (!tabsContainer) return;

    tabsContainer.addEventListener("click", (event) => {
        const tab = event.target.closest("[data-view]");
        if (tab) {
            state.view = tab.dataset.view;
            renderOutput();
            reconnectEvents();
        }
    });
}

// Initialize event delegation once at startup
function initDelegation() {
    setupProcessDelegation();
    setupTabDelegation();
}

$("refresh").addEventListener("click", loadProcesses);
$("clear").addEventListener("click", () => {
    if (state.view === "proxy") {
        state.proxy = [];
    } else if (state.view === "lifecycle") {
        state.lifecycle = state.lifecycle.filter(
            (l) => l.process !== state.selected,
        );
    } else {
        state.output = state.output.filter(
            (l) => !(l.process === state.selected && l.stream === state.view),
        );
    }
    renderOutput();
});

loadProcesses();
connectEvents();
initDelegation();
setInterval(loadProcesses, 5000);
