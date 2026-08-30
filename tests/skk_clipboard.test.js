const assert = require("node:assert/strict");

const DICT = {
  "かんじ": ["感じ", "漢字"],
  "ちょう>": ["超"],
  "もt": ["持"],
  ">てき": ["的"]
};

class FakeElement {
  constructor(id) {
    this.id = id;
    this._value = "";
    this.textContent = "";
    this.dataset = {};
    this.listeners = {};
    this.selectionStart = 0;
    this.selectionEnd = 0;
    this.rangeTextUpdates = 0;
    this.scrollTop = 0;
    this.scrollHeight = 1000;
    this.focusCount = 0;
  }

  get value() {
    return this._value;
  }

  set value(value) {
    // textarea's API value uses LF even when clipboard text uses CRLF.
    this._value = String(value).replace(/\r\n?/g, "\n");
  }

  addEventListener(type, listener) {
    this.listeners[type] = listener;
  }

  focus() {
    this.focusCount += 1;
  }

  select() {
    this.selectionStart = 0;
    this.selectionEnd = this.value.length;
  }

  setRangeText(text, start, end) {
    this.value = this.value.slice(0, start) + text + this.value.slice(end);
    this.rangeTextUpdates += 1;
  }
}

const elements = {
  input: new FakeElement("input"),
  mode: new FakeElement("mode"),
  candidate: new FakeElement("candidate"),
  status: new FakeElement("status"),
  copy: new FakeElement("copy"),
  close: new FakeElement("close"),
  "register-overlay": new FakeElement("register-overlay"),
  "register-reading": new FakeElement("register-reading"),
  "register-mode": new FakeElement("register-mode"),
  "register-input": new FakeElement("register-input"),
  "register-candidate": new FakeElement("register-candidate"),
  "register-error": new FakeElement("register-error"),
  "register-save": new FakeElement("register-save"),
  "register-cancel": new FakeElement("register-cancel")
};

let copiedText = "";
let clipboardText = "";
let hidden = false;
let notifyReadyCount = 0;
let savedUserDict = {};
let savedHistory = {};
let savedInputHistory = [];
let popupShownListeners = [];
let popupFocusInputListeners = [];
const persistenceWarnings = [];
console.warn = (...args) => persistenceWarnings.push(args);

globalThis.document = {
  getElementById(id) {
    return elements[id] || null;
  }
};

const fakeApp = {
  async LoadUserDict() {
    return "{}";
  },
  async LoadHistory() {
    // A damaged optional history file must not prevent the system dictionary
    // and the rest of the popup from loading.
    return "{damaged json";
  },
  async SaveUserDict(data) {
    savedUserDict = JSON.parse(data);
  },
  async SaveHistory(data) {
    savedHistory = JSON.parse(data);
  },
  async LoadInputHistory() {
    return "[]";
  },
  async SaveInputHistory(data) {
    savedInputHistory = JSON.parse(data);
  },
  async ReadClipboard() {
    return clipboardText;
  },
  async CopyToClipboard(text) {
    copiedText = text;
    clipboardText = text;
  },
  HidePopup() {
    hidden = true;
  },
  NotifyReady() {
    notifyReadyCount += 1;
  }
};

globalThis.window = {
  close() {
    throw new Error("window.close must not be used; call HidePopup");
  },
  go: { main: { App: fakeApp } },
  runtime: {
    EventsOn(event, listener) {
      if (event === "popup:shown") popupShownListeners.push(listener);
      if (event === "popup:focus-input") popupFocusInputListeners.push(listener);
    }
  }
};

globalThis.navigator = {};

globalThis.fetch = async (url) => {
  if (url === "/dictionary.json") {
    return { ok: true, status: 200, json: async () => DICT };
  }
  return { ok: false, status: 404, json: async () => ({}) };
};

require("../frontend/src/skk_engine.js");
require("../frontend/src/main.js");

const input = elements.input;
const keydown = input.listeners.keydown;
const paste = input.listeners.paste;

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function press(key, opts = {}) {
  const event = {
    key,
    ctrlKey: !!opts.ctrl,
    altKey: false,
    metaKey: false,
    shiftKey: !!opts.shift,
    keyCode: opts.keyCode ?? (key.length === 1 ? key.toUpperCase().charCodeAt(0) : 0),
    defaultPrevented: false,
    preventDefault() {
      this.defaultPrevented = true;
    },
    stopImmediatePropagation() {}
  };
  keydown(event);
  await flush();
  return event;
}

async function pressRegister(key, opts = {}) {
  const event = {
    key,
    ctrlKey: !!opts.ctrl,
    altKey: false,
    metaKey: false,
    shiftKey: !!opts.shift,
    keyCode: opts.keyCode ?? (key.length === 1 ? key.toUpperCase().charCodeAt(0) : 0),
    isComposing: false,
    defaultPrevented: false,
    preventDefault() {
      this.defaultPrevented = true;
    },
    stopImmediatePropagation() {}
  };
  elements["register-input"].listeners.keydown(event);
  await flush();
  return event;
}

async function typeRegister(text) {
  for (const ch of text) {
    await pressRegister(ch, { shift: ch >= "A" && ch <= "Z" });
  }
}

async function type(text) {
  for (const ch of text) {
    await press(ch, { shift: ch >= "A" && ch <= "Z" });
  }
}

function pasteText(text) {
  const event = {
    defaultPrevented: false,
    clipboardData: {
      getData(type) {
        return type === "text/plain" ? text : "";
      }
    },
    preventDefault() {
      this.defaultPrevented = true;
    }
  };
  paste(event);
  return event;
}

async function emitPopupShown() {
  for (const listener of popupShownListeners) listener();
  await flush();
}

async function emitPopupFocusInput() {
  for (const listener of popupFocusInputListeners) listener();
  await flush();
}

async function resetWindow() {
  await press("Escape");
  let guard = 0;
  while (input.value && guard++ < 100) {
    input.selectionStart = input.selectionEnd = input.value.length;
    await press("Backspace");
  }
  copiedText = "";
  hidden = false;
}

async function runTest(name, fn) {
  try {
    await fn();
    await resetWindow();
    console.log(`ok - ${name}`);
  } catch (error) {
    console.error(`not ok - ${name}`);
    throw error;
  }
}

(async () => {
  await runTest("dictionary loads through the asset server and reports readiness", async () => {
    await type("Kanji");
    await press(" ");
    assert.equal(input.value, "感じ");
    assert.ok(notifyReadyCount >= 1);
    assert.ok(persistenceWarnings.some(([message]) => message.startsWith("LoadHistory:")));
  });

  await runTest("digits type literally outside composition", async () => {
    await press("5");
    assert.equal(input.value, "5");
  });

  await runTest("ascii symbols type literally outside composition", async () => {
    for (const ch of [" ", "?", "!", "@", ":", "<"]) {
      await press(ch);
    }
    assert.equal(input.value, " ?!@:<");
  });

  await runTest("pending roman text is visible until it becomes kana", async () => {
    await press("k");
    assert.equal(input.value, "k");
    await press("a");
    assert.equal(input.value, "か");
  });

  await runTest("typing at the end scrolls the clipboard input to the caret", async () => {
    input.scrollTop = 0;
    await type("aiu");
    assert.equal(input.scrollTop, input.scrollHeight);
  });

  await runTest("editing in the middle preserves the clipboard input scroll position", async () => {
    await type("aiu");
    input.selectionStart = input.selectionEnd = 1;
    input.scrollTop = 240;
    await type("ka");
    assert.equal(input.scrollTop, 240);
  });

  await runTest("pending roman text is visible during composition", async () => {
    await press("K", { shift: true });
    assert.equal(input.value, "▽k");
    await press("a");
    assert.equal(input.value, "▽か");
  });

  await runTest("kana input preserves moved caret in the clipboard window", async () => {
    const updatesBefore = input.rangeTextUpdates;
    await type("aiu");
    input.selectionStart = input.selectionEnd = 1;
    await type("ka");
    assert.equal(input.value, "あかいう");
    assert.equal(input.selectionStart, 2);
    assert.ok(input.rangeTextUpdates > updatesBefore);
  });

  await runTest("mode changes preserve a caret moved to the end", async () => {
    await type("aiu");
    input.selectionStart = input.selectionEnd = input.value.length - 1;
    input.listeners.select();
    input.selectionStart = input.selectionEnd = input.value.length;
    input.listeners.select();

    elements.mode.listeners.click();
    await press("z");

    assert.equal(input.value, "あいうz");
    assert.equal(input.selectionStart, input.value.length);
    elements.mode.listeners.click();
  });

  await runTest("kana input replaces the selected range in the clipboard window", async () => {
    await type("aiu");
    input.selectionStart = 1;
    input.selectionEnd = 2;
    await type("ka");
    assert.equal(input.value, "あかう");
  });

  await runTest("pasted text remains when backspace and kana input follow", async () => {
    const event = pasteText("貼り付け");
    assert.equal(event.defaultPrevented, true);
    assert.equal(input.value, "貼り付け");

    await press("Backspace");
    assert.equal(input.value, "貼り付");

    await type("ka");
    assert.equal(input.value, "貼り付か");
  });

  await runTest("paste replaces the selected range", async () => {
    pasteText("abcdef");
    input.selectionStart = 2;
    input.selectionEnd = 4;
    pasteText("XY");
    assert.equal(input.value, "abXYef");
  });

  await runTest("typing after a CRLF paste uses the visual caret position", async () => {
    pasteText("ab\r\ncd");
    assert.equal(input.value, "ab\ncd");

    input.selectionStart = input.selectionEnd = 4;
    await type("ka");
    assert.equal(input.value, "ab\ncかd");
  });

  await runTest("okuri conversion auto-selects after okuri kana", async () => {
    await type("MoTi");
    assert.equal(input.value, "持ち");
  });

  await runTest("missing okuri candidates open registration", async () => {
    await type("YoutuumoTi");
    assert.equal(elements["register-overlay"].dataset.open, "true");
    assert.equal(elements["register-reading"].textContent, "ようつうもt");

    elements["register-input"].value = "腰痛持";
    elements["register-save"].listeners.click();
    await flush();
    await flush();

    assert.equal(elements["register-overlay"].dataset.open, "false");
    assert.equal(input.value, "腰痛持ち");
    assert.deepEqual(savedUserDict["ようつうもt"], ["腰痛持"]);
  });

  await runTest("new text after candidate commits current candidate first", async () => {
    await type("Kanji");
    await press(" ");
    await type("na");
    assert.equal(input.value, "感じな");
  });

  await runTest("Ctrl+G cancels candidate selection back to preedit", async () => {
    await type("Kanji");
    await press(" ");
    await press("g", { ctrl: true, keyCode: 71 });
    assert.equal(input.value, "▽かんじ");
  });

  await runTest("missing candidates can be registered from the clipboard window", async () => {
    await type("Mitei");
    await press(" ");

    assert.equal(elements["register-overlay"].dataset.open, "true");
    assert.equal(elements["register-reading"].textContent, "みてい");

    elements["register-input"].value = "未定";
    elements["register-save"].listeners.click();
    await flush();
    await flush();

    assert.equal(elements["register-overlay"].dataset.open, "false");
    assert.equal(input.value, "未定");
    assert.deepEqual(savedUserDict["みてい"], ["未定"]);
  });

  await runTest("the registration field supports kana and candidate conversion", async () => {
    await type("Mikakutei");
    await press(" ");
    assert.equal(elements["register-overlay"].dataset.open, "true");

    await typeRegister("Kanji");
    assert.equal(elements["register-input"].value, "▽かんじ");
    await pressRegister(" ");
    assert.equal(elements["register-input"].value, "感じ");
    assert.equal(elements["register-candidate"].textContent, "感じ");

    await pressRegister("Enter");
    assert.equal(elements["register-input"].value, "感じ");
    await pressRegister("Enter");
    await flush();

    assert.equal(elements["register-overlay"].dataset.open, "false");
    assert.equal(input.value, "感じ");
    assert.deepEqual(savedUserDict["みかくてい"], ["感じ"]);
  });

  await runTest("the registration field switches between kana and ascii input", async () => {
    await type("Tourokumo-do");
    await press(" ");

    await pressRegister("l");
    assert.equal(elements["register-mode"].textContent, "SKK OFF");
    await typeRegister("abc-123");
    assert.equal(elements["register-input"].value, "abc-123");

    await pressRegister("j", { ctrl: true, keyCode: 74 });
    assert.equal(elements["register-mode"].textContent, "SKK かな");
    await typeRegister("ka");
    assert.equal(elements["register-input"].value, "abc-123か");

    elements["register-cancel"].listeners.click();
  });

  await runTest("the registration mode label toggles kana and ascii input", async () => {
    await type("Mikakutei2");
    await press(" ");
    assert.equal(elements["register-overlay"].dataset.open, "true");

    elements["register-mode"].listeners.click();
    assert.equal(elements["register-mode"].textContent, "SKK OFF");
    elements["register-mode"].listeners.click();
    assert.equal(elements["register-mode"].textContent, "SKK かな");

    elements["register-cancel"].listeners.click();
  });

  await runTest("Ctrl+G cancels the registration window", async () => {
    await type("Mikakuteikyanseru");
    await press(" ");
    assert.equal(elements["register-overlay"].dataset.open, "true");

    const event = await pressRegister("g", { ctrl: true });
    assert.equal(event.defaultPrevented, true);
    assert.equal(elements["register-overlay"].dataset.open, "false");
  });

  await runTest("the registration field supports wide ascii input", async () => {
    await type("Mikakutei3");
    await press(" ");
    assert.equal(elements["register-overlay"].dataset.open, "true");

    await pressRegister("L", { shift: true });
    assert.equal(elements["register-mode"].textContent, "SKK 全英");
    await typeRegister("A 1");
    assert.equal(elements["register-input"].value, "Ａ　１");

    elements["register-cancel"].listeners.click();
  });

  await runTest("q commits katakana in the registration field", async () => {
    await type("Mikakutei4");
    await press(" ");

    await typeRegister("Katakana");
    assert.equal(elements["register-input"].value, "▽かたかな");
    await pressRegister("q");
    assert.equal(elements["register-input"].value, "カタカナ");

    elements["register-cancel"].listeners.click();
  });

  await runTest("q and Ctrl+Q toggle katakana input modes in the registration field", async () => {
    await type("Mikakutei5");
    await press(" ");

    await pressRegister("q");
    assert.equal(elements["register-mode"].textContent, "SKK カナ");
    await typeRegister("kana");
    assert.equal(elements["register-input"].value, "カナ");

    await pressRegister("q");
    await pressRegister("q", { ctrl: true, keyCode: 81 });
    assert.equal(elements["register-mode"].textContent, "SKK 半ｶﾅ");
    await typeRegister("kana");
    assert.equal(elements["register-input"].value, "カナｶﾅ");

    elements["register-cancel"].listeners.click();
  });

  await runTest("a registered clipboard candidate is available on the next conversion", async () => {
    await type("Mitei");
    await press(" ");

    assert.equal(elements["register-overlay"].dataset.open, "false");
    assert.equal(input.value, "未定");
    assert.equal(elements.candidate.textContent, "未定");
    assert.equal(elements.candidate.dataset.active, "true");
  });

  await runTest("advancing past the last candidate opens registration", async () => {
    await type("Kanji");
    await press(" ");
    await press(" ");
    await press(" ");

    assert.equal(elements["register-overlay"].dataset.open, "true");
    assert.equal(elements["register-reading"].textContent, "かんじ");

    elements["register-cancel"].listeners.click();
    assert.equal(elements["register-overlay"].dataset.open, "false");
    assert.equal(elements.candidate.textContent, "漢字");
  });

  await runTest("Ctrl+J commits conversion in the clipboard window", async () => {
    await type("Kanji");
    await press(" ");
    await press("j", { ctrl: true, keyCode: 74 });
    assert.equal(input.value, "感じ");
  });

  await runTest("Shift+Enter inserts newline", async () => {
    await type("ai");
    await press("Enter", { shift: true });
    assert.equal(input.value, "あい\n");
  });

  await runTest("> converts prefix readings", async () => {
    await type("Chou");
    await press(">", { shift: true });
    assert.equal(input.value, "超");
  });

  await runTest("z commands insert symbols", async () => {
    await type("zhzjzkzl");
    await press("z");
    await press(" ");
    assert.equal(input.value, "←↓↑→　");
  });

  await runTest("reopening without copying preserves the input", async () => {
    await type("Kanji");
    const draft = input.value;
    elements.close.listeners.click();
    assert.equal(hidden, true);

    await emitPopupShown();
    assert.equal(input.value, draft);
  });

  await runTest("reopening after Escape preserves the input", async () => {
    pasteText("draft");
    const draft = input.value;
    await press("Escape");
    assert.equal(hidden, true);

    await emitPopupShown();
    assert.equal(input.value, draft);
  });

  await runTest("copy sends committed text to the backend and hides without window.close", async () => {
    await type("Kanji");
    await press(" ");
    await press("Enter");
    await press("Enter");
    await new Promise((resolve) => setTimeout(resolve, 150));
    assert.equal(copiedText, "感じ");
    assert.equal(hidden, true);
  });

  await runTest("a successful copy clears the buffer even if popup:shown was missed", async () => {
    await type("Kanji");
    await press("l");
    assert.equal(elements.mode.textContent, "SKK OFF");
    await press("Enter");
    await new Promise((resolve) => setTimeout(resolve, 150));
    assert.equal(input.value, "");

    await emitPopupShown();
    assert.equal(input.value, "");
    assert.equal(elements.mode.textContent, "SKK かな");
  });

  await runTest("ArrowUp recalls copied input and ArrowDown restores the draft", async () => {
    pasteText("first entry");
    await press("Enter");
    await emitPopupShown();
    pasteText("current draft");

    await press("ArrowUp");
    assert.equal(input.value, "first entry");
    await press("ArrowDown");
    assert.equal(input.value, "current draft");
    assert.deepEqual(savedInputHistory.slice(-1), ["first entry"]);
  });

  await runTest("popup:shown adds an external clipboard value to input history", async () => {
    clipboardText = "copied in another app";
    await emitPopupShown();
    pasteText("replacement input");

    await press("ArrowUp");
    assert.equal(input.value, "copied in another app");
    assert.deepEqual(savedInputHistory.slice(-1), ["copied in another app"]);

    const historyLength = savedInputHistory.length;
    await emitPopupShown();
    assert.equal(savedInputHistory.length, historyLength);
  });

  await runTest("popup:focus-input focuses the clipboard input", async () => {
    const focusCount = input.focusCount;
    await emitPopupFocusInput();
    assert.equal(input.focusCount, focusCount + 1);
  });

  await press("Enter");
  assert.equal(copiedText, "");
  assert.equal(hidden, false);
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
