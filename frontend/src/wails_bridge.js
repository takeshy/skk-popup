// Compatibility adapter between the existing frontend and Wails v3's
// module-based runtime. Keeping the small surface here avoids coupling the
// SKK input engine to generated numeric binding IDs.
import { Call, Events } from "/wails/runtime.js";

const call = (method, ...args) =>
  Call.ByName(`github.com/takeshy/skk-popup.App.${method}`, ...args);

globalThis.window.go = {
  main: {
    App: {
      CopyToClipboard: (text) => call("CopyToClipboard", text),
      HidePopup: () => call("HidePopup"),
      IsVisible: () => call("IsVisible"),
      LoadHistory: () => call("LoadHistory"),
      LoadUserDict: () => call("LoadUserDict"),
      NotifyReady: () => call("NotifyReady"),
      SaveHistory: (data) => call("SaveHistory", data),
      SaveUserDict: (data) => call("SaveUserDict", data),
      ShowPopup: () => call("ShowPopup"),
      TogglePopup: () => call("TogglePopup")
    }
  }
};

globalThis.window.runtime = {
  EventsOn: (name, callback) => Events.On(name, (event) => callback(event.data))
};
