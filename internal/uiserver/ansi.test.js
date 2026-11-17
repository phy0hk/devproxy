import { describe, expect, test } from "bun:test";
import ansi from "./static/ansi.js";

const BUG_REPORT = "\u001b[2J\u001b[3J\u001b[H\u001b[90m3:40:16 PM\u001b[0m] Starting compilation in watch mode..";

function styled(result) {
    return result.segments.map((segment) => ({
        text: segment.text,
        fg: segment.style.fg,
        bold: segment.style.bold,
        dim: segment.style.dim,
        underline: segment.style.underline,
    }));
}

describe("ANSI parser", () => {
    test("matches the bug report: clears vanish, timestamp is gray", () => {
        const result = ansi.parse(BUG_REPORT);

        expect(result.clear).toBe(true);
        expect(result.text).toBe("3:40:16 PM] Starting compilation in watch mode..");
        expect(styled(result)).toEqual([
            { text: "3:40:16 PM", fg: { type: "index", value: 8 }, bold: false, dim: false, underline: false },
            { text: "] Starting compilation in watch mode..", fg: null, bold: false, dim: false, underline: false },
        ]);
    });

    test("carries parser state across chunk boundaries", () => {
        const parser = ansi.createParser();
        const first = parser.feed("\u001b[38;5;2");
        const second = parser.feed("08mhello\u001b[0m world");

        expect(first.text).toBe("");
        expect(second.text).toBe("hello world");
        expect(second.segments[0].style.fg).toEqual({ type: "index", value: 208 });
        expect(second.segments[1].style.fg).toBe(null);
    });

    test("supports bold, dim, underline, 256-color and truecolor", () => {
        const result = ansi.parse(
            "\u001b[1;2;4;31mred\u001b[38;5;196mbright\u001b[38;2;12;34;56mtrue\u001b[0mplain",
        );

        expect(result.segments[0].text).toBe("red");
        expect(result.segments[0].style.bold).toBe(true);
        expect(result.segments[0].style.dim).toBe(true);
        expect(result.segments[0].style.underline).toBe(true);
        expect(result.segments[0].style.fg).toEqual({ type: "index", value: 1 });
        expect(result.segments[1].style.fg).toEqual({ type: "index", value: 196 });
        expect(result.segments[2].style.fg).toEqual({ type: "rgb", value: [12, 34, 56] });
        expect(result.segments[3].text).toBe("plain");
        expect(result.segments[3].style.fg).toBe(null);
    });

    test("keeps only the final carriage-return segment", () => {
        const result = ansi.parse("progress 10%\rprogress 55%\rprogress 100%\n");

        expect(result.text).toBe("progress 100%");
        expect(result.segments.map((segment) => segment.text).join("")).toBe("progress 100%");
    });

    test("treats a trailing CR as a line terminator", () => {
        expect(ansi.strip("windows line\r\n")).toBe("windows line");
    });

    test("strips OSC sequences terminated by BEL and ST", () => {
        const result = ansi.parse(
            "\u001b]0;window title\u0007ok \u001b]8;;https://example.com\u001b\\link\u001b]8;;\u001b\\",
        );

        expect(result.text).toBe("ok link");
    });

    test("ignores cursor movement, erase-line and unknown sequences", () => {
        const result = ansi.parse(
            "a\u001b[1Ab\u001b[2Kc\u001b[5;7Hd\u001b[?25le\u001b[2Jf",
        );

        expect(result.text).toBe("abcdef");
        expect(result.clear).toBe(true);
    });

    test("strips hostile HTML and incomplete sequences without rendering them", () => {
        const result = ansi.parse("<script>alert(1)</script>\u001b[31m<img src=x onerror=alert(1)>\u001b[0m\u001b[38;5");

        expect(result.text).toBe("<script>alert(1)</script><img src=x onerror=alert(1)>");
        expect(result.segments.some((segment) => segment.text.includes("\u001b"))).toBe(false);
    });

    test("does not display an incomplete escape sequence", () => {
        expect(ansi.strip("ok\u001b[")).toBe("ok");
        expect(ansi.strip("ok\u001b]0;title")).toBe("ok");
    });
});
