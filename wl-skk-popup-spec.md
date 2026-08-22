# wl-skk-popup 仕様書

Wails ベースの常駐型 SKK ポップアップ入力窓。Hyprland のキーバインドから呼び出し、変換結果をクリップボードへ送る。

---

## 1. 背景と目的

### 背景

`chrome-skk-lite` の「クリップボード入力窓」(`Ctrl+Shift+K`)を、ブラウザ外でも日本語入力するための簡易 IME として利用していた。しかし Omarchy が Chromium を `--ozone-platform-hint=wayland`(ネイティブ Wayland)で起動するため、Chrome 拡張の Global スコープショートカットが機能しなくなった。

Chrome 拡張の Global ショートカットは Linux 上では実質 X11 の `XGrabKey` に依存しており、ネイティブ Wayland では代替経路(`hyprland-global-shortcuts-v1` ポータル)が Chromium 側で未対応。この構造的な問題は Chromium のバージョン次第で今後も再発しうる。

### 目的

SKK 変換エンジンと入力窓 UI を Chrome 拡張から切り離し、Hyprland ネイティブのキーバインドで起動する独立アプリケーションにする。ブラウザの起動モードやバージョンに一切依存しない構成にすることで、恒久的に安定させる。

### 非目標

- OS レベルの IME(fcitx5/ibus プロトコル対応)にはしない。あくまで「入力窓 → クリップボード」方式を踏襲する
- `chrome-skk-lite` のページ内入力機能(content script 方式)の置き換えは行わない。両者は併存する
- マルチプラットフォーム対応(当面 Linux/Wayland のみ。Wails なので将来的な移植余地は残る)

---

## 2. 前提環境

| 項目 | 値 |
|---|---|
| OS | Arch Linux (Omarchy) |
| コンポジタ | Hyprland |
| 表示サーバ | Wayland (ネイティブ) |
| フレームワーク | Wails |
| Go | 1.22 以降 |
| Node.js | 22 以降(既存のビルドスクリプト踏襲) |
| WebView | WebKit2GTK 4.1 (Linux 版 Wails の依存) |
| クリップボード | wl-clipboard (`wl-copy`) |
| 任意 | `wtype`(自動貼り付け機能を使う場合) |

### Wails のバージョン選定

着手前に v3 の安定版リリース状況を確認すること。v3 が安定していればウィンドウ制御 API が整理されているため v3 を推奨。未安定なら v2 系で問題なく実装できる(本仕様は v2 の API 名で記述するが、設計自体はどちらでも成立する)。

---

## 3. アーキテクチャ

### 3.1 全体構成

```
┌─────────────────────────────────────────────┐
│ Hyprland                                    │
│   bind = CTRL SHIFT, K, exec, wl-skk toggle │
└──────────────┬──────────────────────────────┘
               │ プロセス起動(CLI モード)
               ▼
    ┌──────────────────────┐
    │ wl-skk (CLI モード)  │
    │  Unix socket へ送信  │
    └──────────┬───────────┘
               │ $XDG_RUNTIME_DIR/wl-skk.sock
               ▼
┌─────────────────────────────────────────────┐
│ wl-skk (デーモンモード / 常駐)              │
│                                             │
│  ┌───────────────┐    ┌──────────────────┐  │
│  │ Go backend    │◄──►│ Frontend         │  │
│  │ - IPC server  │    │ (WebKit2GTK)     │  │
│  │ - AssetServer │    │ - skk_engine.js  │  │
│  │ - ユーザ辞書  │    │ - 入力窓 UI      │  │
│  │ - クリップボード│   │ - 辞書 fetch     │  │
│  │ - ウィンドウ制御│   │                  │  │
│  └───────────────┘    └──────────────────┘  │
│         AssetServer 経由で /dictionary.json  │
└─────────────────────────────────────────────┘
```

### 3.2 設計上の要点

**単一常駐プロセス方式を採る。** 毎回プロセスを起動する方式は却下する。理由は辞書(`dictionary.json`、`SKK-JISYO.L` ベースで数十 MB 規模)のロードコストで、押してから窓が出るまでの体感が実用に耐えないため。常駐させて起動時に一度だけロードし、以降は表示/非表示のトグルのみを行う。

**キー捕捉はアプリ側で一切行わない。** グローバルホットキーの登録は Hyprland の `bind` に完全に委譲する。これが今回の問題の再発を防ぐ設計上の核心。

**IPC は Unix domain socket。** D-Bus でもよいが、依存とデバッグコストが増える。単一ユーザー・単一マシン前提なので socket で十分。

**辞書はフロントエンドのアセットとして配置し、WebView から `fetch()` で直接読む。** Go 側から JS へ巨大文字列を返す設計は採らない。JS↔ネイティブのブリッジは呼び出しごとに JSON シリアライズを挟むため、数十 MB の文字列を渡すと初期化に無視できない時間がかかり、その間のメモリピークも二重に発生する。AssetServer が返す HTTP レスポンスを WebView が直接ストリームで受ける方が速く、`Response.json()` により WebKit 側のネイティブパーサをそのまま使えるため、JS で `JSON.parse()` を呼ぶより有利になる。この判断は Wails でも Tauri でも同様に成立する。

---

## 4. リポジトリ構成

```
wl-skk-popup/
├── main.go                  # エントリポイント(CLI/デーモンの分岐)
├── app.go                   # Wails App 構造体、フロントエンドへの bind メソッド
├── internal/
│   ├── ipc/
│   │   ├── server.go        # socket サーバ(デーモン側)
│   │   └── client.go        # socket クライアント(CLI 側)
│   ├── dict/
│   │   └── userdict.go      # ユーザー辞書・学習履歴の永続化
│   ├── assetserver/
│   │   └── handler.go       # 外部辞書ファイル用のフォールバックハンドラ
│   ├── clipboard/
│   │   └── clipboard.go     # wl-copy 呼び出し / Wails runtime フォールバック
│   └── config/
│       └── config.go        # 設定ファイル読み込み
├── frontend/
│   ├── index.html           # skk_clipboard.html を移植
│   ├── public/
│   │   └── dictionary.json  # ★ ビルド時に生成。dist へコピーされ /dictionary.json で配信
│   ├── src/
│   │   ├── main.js          # skk_clipboard.js を移植
│   │   ├── skk_engine.js    # ★ chrome-skk-lite からほぼそのまま流用
│   │   └── style.css
│   ├── dist/                # ビルド成果物。go:embed の対象
│   └── package.json
├── scripts/
│   └── build_dictionary.js  # chrome-skk-lite の build_extension.js を流用
├── wails.json
└── README.md
```

---

## 5. 移植対象と改変点

`chrome-skk-lite` からの流用と、Chrome API 依存の除去が作業の中心になる。

| 元ファイル | 移植先 | 改変内容 |
|---|---|---|
| `skk_engine.js` | `frontend/src/skk_engine.js` | **無改変を目標**。Chrome API 非依存であることを確認し、依存があれば注入可能な形に切り出す |
| `skk_clipboard.html` | `frontend/index.html` | Wails のスクリプト読み込みに合わせて調整 |
| `skk_clipboard.js` | `frontend/src/main.js` | Chrome API 呼び出しを Go bindings に差し替え(下記) |
| `dictionary_sources.json` | ルート直下 | 無改変 |
| `build_extension.js` | `scripts/build_dictionary.js` | 拡張ディレクトリ生成部を削り、`frontend/public/dictionary.json` 出力のみに簡略化 |

### 5.1 Chrome API の差し替え表

| Chrome API | 用途 | 差し替え先 |
|---|---|---|
| `chrome.storage.local.get/set` | ユーザー辞書、学習履歴、最終確定候補 | Go 側 `LoadUserDict()` / `SaveUserDict(json)` |
| `chrome.runtime.getURL` | 辞書ファイルの取得 | `fetch('/dictionary.json')`(AssetServer から配信) |
| `chrome.commands` | ショートカット登録 | 削除(Hyprland bind に置換) |
| `navigator.clipboard.writeText` | 確定文字列のコピー | Go 側 `CopyToClipboard(text)` |
| `window.close()` | 入力窓を閉じる | Go 側 `HidePopup()` |

**クリップボードを Go 側に寄せる理由:** WebKitGTK 環境で `navigator.clipboard` はセキュアコンテキストとフォーカス状態の制約を受けやすく、確実性に欠ける。`wl-copy` を叩く実装なら Wayland 上で確実に動作する。

**辞書だけはブリッジを通さない理由:** 3.2 の通り。ユーザー辞書は数 KB〜数百 KB 程度に収まるためブリッジ経由で問題ないが、システム辞書は桁が二つ違うため HTTP 経由にする。

---

## 6. Go バックエンド仕様

### 6.1 起動モード

`main.go` は引数によって二つのモードに分岐する。

```
wl-skk                    # デーモンモード(常駐)。socket が既に存在する場合は起動を拒否
wl-skk toggle             # 表示/非表示をトグル
wl-skk show               # 表示
wl-skk hide               # 非表示
wl-skk quit               # デーモン終了
```

CLI モードは socket に 1 行のコマンド文字列を送って即座に終了する。デーモンが起動していない場合は標準エラーにメッセージを出して exit 1。

### 6.2 IPC 仕様

- **socket パス**: `$XDG_RUNTIME_DIR/wl-skk.sock`(未設定時は `/tmp/wl-skk-$UID.sock`)
- **プロトコル**: 改行区切りのプレーンテキスト。リクエストは `toggle` / `show` / `hide` / `quit` のいずれか
- **レスポンス**: `ok` または `error: <message>`
- **多重起動防止**: デーモン起動時に socket への接続を試み、成功したら既存プロセスありとして終了。失敗したら stale socket として削除してから listen

### 6.3 フロントエンドへ公開するメソッド

```go
// ユーザー辞書(サイズが小さいためブリッジ経由で問題ない)
func (a *App) LoadUserDict() (string, error)       // ユーザー辞書 JSON
func (a *App) SaveUserDict(data string) error      // ユーザー辞書 JSON を保存

// クリップボード
func (a *App) CopyToClipboard(text string) error

// ウィンドウ制御
func (a *App) HidePopup()                          // 窓を隠す(プロセスは常駐継続)
func (a *App) NotifyReady()                        // フロント初期化完了通知
```

システム辞書を返すメソッドは意図的に置かない。フロントエンドは `fetch('/dictionary.json')` で直接取得する。

### 6.4 AssetServer と辞書の配信

`frontend/dist` を `go:embed` でバイナリに埋め込み、Wails の AssetServer がそのまま配信する。`frontend/public/dictionary.json` はビルドツールによって `dist/dictionary.json` へコピーされるため、追加の配線なしに `/dictionary.json` で参照できる。

外部辞書ファイルへのフォールバックが必要な場合(9.2 参照)は、AssetServer の `Handler` オプションにミドルウェアを挿す。

```go
AssetServer: &assetserver.Options{
    Assets: assets,
    Middleware: func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 外部辞書が存在する場合のみ、埋め込みアセットより優先して返す
            if r.URL.Path == "/dictionary.json" {
                if p := externalDictPath(); p != "" {
                    http.ServeFile(w, r, p)
                    return
                }
            }
            next.ServeHTTP(w, r)
        })
    },
},
```

`http.ServeFile` を使うことで Range リクエストと `Content-Length` が正しく処理され、WebView 側はストリームとして受け取れる。

### 6.5 データ永続化

| ファイル | パス | 内容 |
|---|---|---|
| ユーザー辞書 | `$XDG_DATA_HOME/wl-skk/userdict.json` | 手動登録した単語 |
| 学習履歴 | `$XDG_DATA_HOME/wl-skk/history.json` | 最終確定候補、変換履歴 |
| 設定 | `$XDG_CONFIG_HOME/wl-skk/config.toml` | 下記 |

書き込みは一時ファイル + `rename` によるアトミック書き込みとする。SKK は入力のたびに学習履歴が更新されるため、書き込み頻度を抑えるべく **デバウンス(最終更新から 2 秒後にフラッシュ)** を入れる。また窓を閉じるタイミングで必ずフラッシュする。

### 6.6 設定ファイル

```toml
[window]
width = 600
height = 200
# 閉じたあとに直前のウィンドウへフォーカスを戻す
restore_focus = true

[clipboard]
# "wl-copy" | "wails"
backend = "wl-copy"
# コピー後に自動で Ctrl+V を送出(wtype が必要)
auto_paste = false
# 自動貼り付け時、フォーカス復帰から送出までの待ち時間(ミリ秒)
auto_paste_delay_ms = 80

[dictionary]
# 外部辞書ファイルのパス。指定するとバイナリ埋め込みより優先される(省略可)
external_path = ""
```

---

## 7. ウィンドウ挙動

### 7.1 Wails のウィンドウ設定

```go
wails.Run(&options.App{
    Title:         "wl-skk",
    Width:         600,
    Height:        200,
    Frameless:     true,           // タイトルバーなしのポップアップ外観
    AlwaysOnTop:   true,
    StartHidden:   true,           // 起動時は非表示で常駐
    HideWindowOnClose: true,       // ×相当の操作でも終了させない
    BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
})
```

`StartHidden` と `HideWindowOnClose` の組み合わせが常駐化の要。

### 7.2 表示/非表示

- `show`: `runtime.WindowShow()` → フロントエンドへ `popup:shown` イベントを送出 → フロント側で入力バッファをクリアし、入力欄にフォーカス
- `hide`: フロント側の状態をフラッシュしてから `runtime.WindowHide()`
- `toggle`: 現在の可視状態を Go 側で保持し、反転させる

### 7.3 Hyprland 側のウィンドウルール

Wayland ではアプリが自力でフォーカスを奪えないため、コンポジタ側のルールで補う必要がある。

```
# ~/.config/hypr/hyprland.conf

windowrulev2 = float, class:^(wl-skk)$
windowrulev2 = center, class:^(wl-skk)$
windowrulev2 = pin, class:^(wl-skk)$
windowrulev2 = stayfocused, class:^(wl-skk)$
windowrulev2 = noborder, class:^(wl-skk)$
windowrulev2 = noanim, class:^(wl-skk)$

bind = CTRL SHIFT, K, exec, wl-skk toggle

exec-once = uwsm app -- wl-skk
```

`stayfocused` が特に重要。これがないと入力窓が表示されてもキーボードフォーカスが移らず、そもそも入力できない。

**注意点として、実際の `class` 名を必ず `hyprctl clients` で確認すること。** Wails が設定する app_id は必ずしもバイナリ名と一致しない。

### 7.4 フォーカス復帰

窓を閉じたあと、直前に使っていたウィンドウへフォーカスを戻す。Go 側から実行する。

```
hyprctl dispatch focuscurrentorlast
```

`config.toml` の `restore_focus = true` のときのみ実行する。

### 7.5 自動貼り付け(オプション機能)

Chrome 拡張では実現できなかったが、こちらの構成なら可能になる追加価値。`auto_paste = true` のとき、コピー → 窓を隠す → フォーカス復帰 → `auto_paste_delay_ms` 待機 → `wtype -M ctrl -k v -m ctrl` を実行する。

ただしフォーカス復帰のタイミングはコンポジタ依存で不安定になりうるため、**デフォルトは無効**とし、実機で安定を確認できた場合のみ有効化する位置づけとする。

---

## 8. フロントエンド仕様

### 8.1 初期化フロー

1. `fetch('/dictionary.json')` を即座に発火(Wails ランタイムの ready を待たない)
2. 並行して Wails ランタイムの ready を待ち、`LoadUserDict()` でユーザー辞書・学習履歴を復元
3. `Response.json()` で辞書をパース
4. 両者が揃ったら `skk_engine` を初期化
5. `NotifyReady()` を呼び、Go 側にロード完了を通知

```js
const dictPromise = fetch('/dictionary.json').then(r => r.json());
const userPromise = waitForWailsRuntime().then(() => LoadUserDict());
const [dict, user] = await Promise.all([dictPromise, userPromise]);
```

**辞書の fetch はランタイム初期化と並行させる。** 辞書のダウンロードとパースが初期化フローで最も重いため、Wails ランタイムの準備完了を待ってから始めると無駄な直列化が発生する。AssetServer は WebView がページを読み込んだ時点で既に応答可能なので、`fetch` はページスクリプトの先頭で発火できる。

辞書のパースは常駐起動時の一度きりなので、数百ミリ秒かかっても許容する。ロード完了前に `show` が来た場合に備え、Go 側は `ready` フラグが立つまで表示要求をキューイングする。

### 8.2 キー操作

`chrome-skk-lite` のクリップボード入力窓の仕様をそのまま踏襲する。README のキー操作一覧が唯一の仕様であり、本アプリで挙動を変えない。

特に以下は移植時に動作確認する。

- `Enter`(未変換状態)/ `Copy` ボタン → クリップボードへコピーして窓を閉じる
- `Shift+Enter` → 改行
- `Escape` → 変換中はキャンセル、未変換状態なら窓を閉じる(コピーしない)
- `Ctrl+J` / `l` / `L` / `q` / `Ctrl+Q` のモード切替
- `Space` 連打による候補選択と `A S D F J K L` の直接選択
- `Tab` 補完、`/` Abbrev、`>` 接頭辞/接尾辞、数値変換
- 単語登録モーダル(窓の中にさらにモーダルが出る構造)

### 8.3 状態表示

`SKK OFF` / `SKK かな` / `SKK カナ` / `SKK 半ｶﾅ` / `SKK 全英` / `SKK 略語` / `SKK 変換` / `SKK 候補` を窓内に表示する。Chrome 拡張では画面右下のトーストだったが、本アプリでは窓自体が小さいため、窓内の固定位置(下部ステータスバー)に配置する。

### 8.4 窓を開いたときの初期状態

`かな` モードで開く。Chrome 拡張では「初期状態は無効(`SKK OFF`)」だったが、この入力窓は日本語入力のために明示的に開くものなので、毎回 `Ctrl+J` を押させるのは無駄な操作になる。設定で変更可能にしておく余地は残す。

---

## 9. ビルドとインストール

### 9.1 辞書のビルド

```bash
node scripts/build_dictionary.js
```

`https://skk-dev.github.io/dict/` から `dictionary_sources.json` に指定された辞書を取得し、`frontend/public/dictionary.json` を生成する。

`wails build` の前に実行する必要がある。`frontend/package.json` の `prebuild` スクリプトに登録し、ファイルが既に存在する場合はスキップする(毎回数十 MB をダウンロードさせない)。

### 9.2 アプリのビルド

```bash
wails build
```

フロントエンドのビルドツールが `frontend/public/dictionary.json` を `frontend/dist/dictionary.json` へコピーし、`dist` 全体が `go:embed` でバイナリに埋め込まれる。これにより単一バイナリで配布可能になる(`gemihub-desktop` と同じ方針)。

**ビルドツールの設定に注意。** 辞書 JSON がバンドラの処理対象になるとビルド時間とメモリを大量に消費する。`public`(Vite の場合)のような、**バンドルせずそのままコピーされるディレクトリ**に置くこと。`src` 配下に置いて `import` すると、数十 MB の JSON が JS モジュールとしてパースされ、ビルドが破綻する。

**バイナリサイズについて:** `SKK-JISYO.L` ベースの JSON は数十 MB になり、バイナリも同等に膨らむ。許容できない場合は `config.toml` の `dictionary.external_path` に外部ファイルを指定し、6.4 のミドルウェアで差し替える。この場合はビルド時に `frontend/public/dictionary.json` を最小構成(`SKK-JISYO.S` など)に差し替えてフォールバックとして残しておくと、外部辞書がなくても最低限動作する。

### 9.3 インストール

```bash
install -Dm755 build/bin/wl-skk ~/.local/bin/wl-skk
```

`~/.local/bin` が PATH に含まれていることを確認する。

### 9.4 自動起動

Hyprland の `exec-once` で `uwsm app -- wl-skk` を指定する(Omarchy は uwsm 経由で起動する構成のため)。systemd user service にする選択肢もあるが、Hyprland セッションに紐づくため `exec-once` の方が単純。

---

## 10. 実装フェーズ

| Phase | 内容 | 完了条件 |
|---|---|---|
| 1 | Wails スケルトン + ウィンドウ常駐化 | `StartHidden` で常駐し、手動で show/hide できる |
| 2 | IPC(socket サーバ/クライアント) | `wl-skk toggle` で窓が出入りする |
| 3 | Hyprland 統合 | `Ctrl+Shift+K` で窓が出てフォーカスが移る |
| 4 | 辞書配信(AssetServer + fetch)+ `skk_engine.js` 移植 | 実物の辞書でビルドが通り、窓の中で基本的なかな入力と変換ができる |
| 5 | クリップボード連携 | `Enter` でコピーされ窓が閉じ、他アプリに貼れる |
| 6 | ユーザー辞書・学習履歴の永続化 | 再起動後も学習内容が保持される |
| 7 | 全キー操作の移植確認 | README のキー操作一覧が全て動作する |
| 8 | 自動貼り付け(オプション) | `auto_paste = true` で貼り付けまで自動化される |

Phase 5 の時点で「Chrome 拡張でできていたこと」に到達する。ここが最小の実用ラインとなる。

---

## 11. 受け入れ基準

1. Chromium の Ozone 設定(`wayland` / `x11` どちらでも)に関わらず動作する
2. `Ctrl+Shift+K` を押してから入力可能になるまで 200ms 以内
3. 変換結果が Slack、ターミナル、Chrome のアドレスバーいずれにも貼り付けられる
4. `chrome-skk-lite` の README に記載された入力窓のキー操作が全て同じ挙動で動作する
5. ユーザー辞書と学習履歴が再起動をまたいで保持される
6. デーモンを二重起動しようとしても既存プロセスが壊れない
7. 窓を閉じたあと、直前のアプリにフォーカスが戻る

---

## 12. リスクと対策

| リスク | 影響 | 対策 |
|---|---|---|
| Wayland でフォーカスが移らない | 致命的(入力不能) | Hyprland の `stayfocused` ルール。それでも駄目なら `hyprctl dispatch focuswindow` を Go 側から明示的に叩く |
| `class` 名が想定と異なりルールが効かない | 高 | `hyprctl clients` で実測してから設定を書く。Phase 3 で早期に確認する |
| `skk_engine.js` が Chrome API に依存していた | 中 | Phase 4 の冒頭で `grep -n "chrome\." skk_engine.js` を実行し、依存箇所を洗い出してから着手する |
| WebKitGTK でのキーイベント挙動が Chrome と異なる | 中 | `Ctrl+J` など修飾キー系の `preventDefault` を Phase 7 で重点確認 |
| 辞書 embed でバイナリが肥大 | 低 | `dictionary.external_path` + AssetServer ミドルウェアによる外部ファイル方式を設計上用意済み |
| 辞書 JSON がバンドラに食われてビルドが破綻 | 中 | `frontend/public` に置き、`import` しない。Phase 4 の最初に空ファイルではなく実物でビルドを通して確認する |
| Wails v3 の破壊的変更 | 低 | 着手時点の安定版を選定し、途中でバージョンを変えない |

---

## 13. 将来的な拡張余地

- **`skk_engine.js` の共通パッケージ化**: `chrome-skk-lite` / `skk-lite.nvim` / 本アプリで変換ロジックが三重管理になっている。npm パッケージとして切り出せば、辞書処理・学習アルゴリズムの改善が一箇所で済む
- **fcitx5 アドオン化**: 入力窓方式ではなく本物の IME として動かす道。実装コストは大きいが、貼り付け操作が不要になる
- **辞書サーバ対応**: `skkserv` プロトコルに対応すれば、巨大辞書を embed せずに済む
