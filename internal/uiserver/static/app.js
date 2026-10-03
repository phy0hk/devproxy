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
                "&": "&amp;",
                "<": "&lt;",
                ">": "&gt;",
                '"': "&quot;",
                "'": "&#39;",
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
        disconnected: "disconnected, retrying…",
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

    container.querySelectorAll(".card").forEach((card) => {
        card.addEventListener("click", () => {
            state.selected = card.dataset.name;
            state.view = "stdout";
            renderProcesses();
            renderOutput();
        });
        card.querySelectorAll("[data-action]").forEach((button) => {
            button.addEventListener("click", async (event) => {
                event.stopPropagation();
                button.disabled = true;
                try {
                    const response = await fetch(
                        `/api/processes/${encodeURIComponent(card.dataset.name)}/${button.dataset.action}`,
                        { method: "POST" },
                    );
                    if (!response.ok) {
                        const detail = (await response.text()).trim();
                        pushLifecycle(
                            card.dataset.name,
                            `process.${button.dataset.action}.error`,
                            detail,
                        );
                    }
                } catch (error) {
                    pushLifecycle(
                        card.dataset.name,
                        `process.${button.dataset.action}.error`,
                        error.message,
                    );
                } finally {
                    button.disabled = false;
                    await loadProcesses();
                }
            });
        });
    });
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
    $("tabs")
        .querySelectorAll("[data-view]")
        .forEach((tab) => {
            tab.addEventListener("click", () => {
                state.view = tab.dataset.view;
                renderOutput();
            });
        });
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
        const summary = `${line.method} ${line.path} → ${line.upstream} <span class="${statusClass}">${line.status}</span> ${line.duration_ms}ms`;
        return `<div class="terminal-line proxy">${time}${summary}</div>`;
    }

    if (state.view === "lifecycle") {
        const detail = line.error || line.message || "";
        return `<div class="terminal-line lifecycle">${time}${escapeHTML(line.type)}${detail ? ` — ${escapeHTML(detail)}` : ""}</div>`;
    }

    return `<div class="terminal-line ${line.stream}">${time}${escapeHTML(line.message)}</div>`;
}

function renderOutput() {
    renderTabs();
    const selected =
        state.processes.find((p) => p.name === state.selected) ?? null;
    $("terminal-title").textContent = selected
        ? `${selected.name} · ${state.view}`
        : "Instance output";

    const lines = visibleLines();
    const terminal = $("events");
    terminal.innerHTML = lines.length
        ? lines.map(lineHTML).join("")
        : '<span class="terminal-empty">No output in this view yet.</span>';
    terminal.scrollTop = terminal.scrollHeight;
}

function connectEvents() {
    const source = new EventSource("/events");
    source.onopen = () => setStatus("connected");
    source.onerror = () => setStatus("disconnected");

    source.addEventListener("devproxy", (event) => {
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
setInterval(loadProcesses, 5000);
