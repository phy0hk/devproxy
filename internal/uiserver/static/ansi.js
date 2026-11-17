/*
 * Small, dependency-free ANSI/terminal-sequence parser for the dashboard.
 *
 * Output is always plain data: styled text segments plus a `clear` flag. The
 * caller builds DOM nodes with createElement/textContent; this module never
 * produces HTML.
 */
(function (global) {
    "use strict";

    var GROUND = 0;
    var ESCAPE = 1;
    var CSI = 2;
    var OSC = 3;
    var CONSUME_ONE = 4;

    var STANDARD = [
        "#000000", "#cd3131", "#0dbc79", "#e5e510",
        "#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
        "#666666", "#f14c4c", "#23d18b", "#f5f543",
        "#3b8eea", "#d670d6", "#29b8db", "#ffffff",
    ];

    function defaultStyle() {
        return {
            fg: null,
            bg: null,
            bold: false,
            dim: false,
            italic: false,
            underline: false,
            inverse: false,
        };
    }

    function cloneStyle(style) {
        return {
            fg: style.fg,
            bg: style.bg,
            bold: style.bold,
            dim: style.dim,
            italic: style.italic,
            underline: style.underline,
            inverse: style.inverse,
        };
    }

    function sameStyle(left, right) {
        return (
            left.fg === right.fg &&
            left.bg === right.bg &&
            left.bold === right.bold &&
            left.dim === right.dim &&
            left.italic === right.italic &&
            left.underline === right.underline &&
            left.inverse === right.inverse
        );
    }

    function colorValue(kind, value) {
        return kind === "rgb" ? { type: "rgb", value: value } : { type: "index", value: value };
    }

    function xtermColor(index) {
        if (index < 16) return STANDARD[Math.max(0, index)] || STANDARD[0];
        if (index < 232) {
            var value = index - 16;
            var steps = [0, 95, 135, 175, 215, 255];
            var red = steps[Math.floor(value / 36) % 6];
            var green = steps[Math.floor(value / 6) % 6];
            var blue = steps[value % 6];
            return rgbHex(red, green, blue);
        }
        var shade = 8 + (Math.max(232, Math.min(255, index)) - 232) * 10;
        return rgbHex(shade, shade, shade);
    }

    function rgbHex(red, green, blue) {
        return (
            "#" +
            [red, green, blue]
                .map(function (part) {
                    var value = Math.max(0, Math.min(255, Number(part) || 0));
                    return value.toString(16).padStart(2, "0");
                })
                .join("")
        );
    }

    function cssColor(value) {
        if (!value) return null;
        if (value.type === "rgb") {
            var parts = value.value || [];
            return "rgb(" + (parts[0] || 0) + ", " + (parts[1] || 0) + ", " + (parts[2] || 0) + ")";
        }
        return xtermColor(value.value);
    }

    function applySGR(style, raw) {
        var params = raw.replace(/^\?/, "").split(";");
        if (params.length === 0) params = ["0"];

        for (var index = 0; index < params.length; index += 1) {
            var code = params[index] === "" ? 0 : parseInt(params[index], 10);
            if (!Number.isFinite(code)) code = 0;

            if (code === 0) {
                var reset = defaultStyle();
                style.fg = reset.fg;
                style.bg = reset.bg;
                style.bold = false;
                style.dim = false;
                style.italic = false;
                style.underline = false;
                style.inverse = false;
            } else if (code === 1) {
                style.bold = true;
            } else if (code === 2) {
                style.dim = true;
            } else if (code === 3) {
                style.italic = true;
            } else if (code === 4) {
                style.underline = true;
            } else if (code === 7) {
                style.inverse = true;
            } else if (code === 21 || code === 22) {
                style.bold = false;
                style.dim = false;
            } else if (code === 23) {
                style.italic = false;
            } else if (code === 24) {
                style.underline = false;
            } else if (code === 27) {
                style.inverse = false;
            } else if (code >= 30 && code <= 37) {
                style.fg = colorValue("index", code - 30);
            } else if (code === 39) {
                style.fg = null;
            } else if (code >= 40 && code <= 47) {
                style.bg = colorValue("index", code - 40);
            } else if (code === 49) {
                style.bg = null;
            } else if (code >= 90 && code <= 97) {
                style.fg = colorValue("index", code - 90 + 8);
            } else if (code >= 100 && code <= 107) {
                style.bg = colorValue("index", code - 100 + 8);
            } else if (code === 38 || code === 48) {
                var target = code === 38 ? "fg" : "bg";
                var mode = parseInt(params[index + 1], 10);
                if (mode === 5) {
                    var paletteIndex = parseInt(params[index + 2], 10);
                    if (Number.isFinite(paletteIndex)) {
                        style[target] = colorValue("index", paletteIndex);
                    }
                    index += 2;
                } else if (mode === 2) {
                    var red = parseInt(params[index + 2], 10) || 0;
                    var green = parseInt(params[index + 3], 10) || 0;
                    var blue = parseInt(params[index + 4], 10) || 0;
                    style[target] = colorValue("rgb", [red, green, blue]);
                    index += 4;
                }
            }
        }
    }

    function createParser() {
        var state = GROUND;
        var csi = "";
        var oscEscape = false;
        var style = defaultStyle();
        var segments = [];
        var plain = "";
        var clear = false;

        function push(text) {
            if (!text) return;
            plain += text;
            var last = segments.length > 0 ? segments[segments.length - 1] : null;
            if (last && sameStyle(last.style, style)) {
                last.text += text;
            } else {
                segments.push({ text: text, style: cloneStyle(style) });
            }
        }

        function finishCSI(final) {
            var params = csi.replace(/[?<>=! ]/g, "");
            if (final === "m") {
                applySGR(style, params);
            } else if (final === "J") {
                if (params === "2" || params === "3") clear = true;
            } else if (final === "H" || final === "f") {
                // Bare home is what tsc watch mode emits after clearing.
                if (params === "" || params === "1;1") clear = true;
            }
            // Cursor movement, erase-line (K), and unknown private sequences
            // are intentionally ignored and never rendered as text.
            csi = "";
            state = GROUND;
        }

        function take() {
            var result = {
                segments: segments.map(function (segment) {
                    return { text: segment.text, style: cloneStyle(segment.style) };
                }),
                text: plain,
                clear: clear,
            };
            segments = [];
            plain = "";
            clear = false;
            return result;
        }

        function feed(input) {
            var text = String(input == null ? "" : input);

            // A trailing CR is a line terminator (CRLF), not an overwrite.
            while (text.length > 0 && text.charCodeAt(text.length - 1) === 13) {
                text = text.slice(0, -1);
            }

            for (var index = 0; index < text.length; index += 1) {
                var code = text.charCodeAt(index);

                if (state === GROUND) {
                    if (code === 0x1b) {
                        state = ESCAPE;
                    } else if (code === 13) {
                        // CRLF is a line break; a bare CR overwrites the line
                        // and keeps only the final segment.
                        if (text.charCodeAt(index + 1) === 10) {
                            index += 1;
                        } else {
                            segments = [];
                            plain = "";
                        }
                    } else if (code === 9) {
                        push("\t");
                    } else if (code >= 0x20 && code !== 0x7f) {
                        push(text[index]);
                    }
                    continue;
                }

                if (state === ESCAPE) {
                    if (code === 0x5b) {
                        state = CSI;
                        csi = "";
                    } else if (code === 0x5d || code === 0x50 || code === 0x58 || code === 0x5e || code === 0x5f) {
                        state = OSC;
                        oscEscape = false;
                    } else if (
                        code === 0x28 || code === 0x29 || code === 0x2a || code === 0x2b ||
                        code === 0x23 || code === 0x25
                    ) {
                        state = CONSUME_ONE;
                    } else {
                        // Single-character escapes (save/restore cursor, keypad
                        // modes, etc.) carry no display text.
                        state = GROUND;
                    }
                    continue;
                }

                if (state === CSI) {
                    if (code >= 0x40 && code <= 0x7e) {
                        finishCSI(text[index]);
                    } else {
                        csi += text[index];
                    }
                    continue;
                }

                if (state === OSC) {
                    if (oscEscape) {
                        oscEscape = false;
                        if (code === 0x5c) {
                            state = GROUND;
                        }
                        continue;
                    }
                    if (code === 0x07) {
                        state = GROUND;
                    } else if (code === 0x1b) {
                        oscEscape = true;
                    }
                    continue;
                }

                if (state === CONSUME_ONE) {
                    state = GROUND;
                }
            }

            return take();
        }

        return { feed: feed };
    }

    function parse(text) {
        return createParser().feed(text);
    }

    function strip(text) {
        return parse(text).text;
    }

    var api = {
        createParser: createParser,
        parse: parse,
        strip: strip,
        xtermColor: xtermColor,
        cssColor: cssColor,
    };

    if (typeof module !== "undefined" && module.exports) {
        module.exports = api;
    }
    global.DevProxyAnsi = api;
})(typeof globalThis !== "undefined" ? globalThis : this);
