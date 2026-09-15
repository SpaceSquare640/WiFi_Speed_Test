/*
 * Interface text, in one place.
 *
 * English is the primary language and Chinese the secondary one, matching the
 * command line tool. Every string the visitor reads lives here; a string
 * written into the markup is a string the other language never gets.
 */

const I18n = (() => {
  const STORAGE_KEY = 'wifitest-preview-lang';

  const DICT = {
    en: {
      appName: 'WiFi Speed Test',
      simNotice: 'Simulated preview.',
      simNoticeBody:
        'Nothing here is measured. Every figure is invented on the spot and the addresses shown are documentation examples that route nowhere.',

      start: 'Start test',
      running: 'Testing…',
      again: 'Test again',
      tapToStart: 'Tap to start',
      idle: 'Ready',

      phasePing: 'Measuring latency',
      phaseDownload: 'Measuring download',
      phaseUpload: 'Measuring upload',
      phaseLayers: 'Locating the fault',
      phaseDone: 'Done',

      download: 'Download',
      upload: 'Upload',
      latency: 'Ping',
      jitter: 'jitter',
      loss: 'loss',
      grade: 'Grade',
      dns: 'DNS',

      layered: 'Layered diagnostics',
      layerGateway: 'Local gateway',
      layerRegional: 'Regional egress',
      layerInternational: 'International',
      unreachable: 'no reply',

      verdictHealthy: 'Your connection looks healthy',
      verdictHealthyBody:
        'All three segments answered quickly and lost nothing. Any slowness you feel is more likely in the site or service you are using.',
      verdictLocal: 'The problem is inside your home network',
      verdictLocalBody:
        'Your own router is already slow to answer, and every segment beyond it inherits that delay. Moving closer to the router, changing channel, or restarting it is where to start. Your provider is not the cause.',
      verdictInternational: 'Your home and your provider are fine',
      verdictInternationalBody:
        'The first two segments are healthy; only the long-haul international link is congested. Local sites should feel normal while overseas ones feel slow. There is nothing to fix on your side.',

      gradeExcellent: 'excellent',
      gradeVeryGood: 'very good',
      gradeGood: 'good',
      gradeFair: 'fair',
      gradeSlow: 'slow',

      navTest: 'Test',
      navHistory: 'History',
      navSettings: 'Settings',
      comingSoon: 'Shown for layout only in this preview.',

      // shared: scenario switch
      scenarioLabel: 'Simulated condition',
      scenarioRandom: 'Random',
      scenarioHealthy: 'Healthy',
      scenarioLocal: 'Home network fault',
      scenarioIntl: 'International congestion',

      // desktop.html
      desktopSubtitle: 'Desktop app preview',
      windowTitle: 'WiFi Speed Test',
      endpointLabel: 'Endpoint',
      endpointAuto: 'Automatic (3 servers)',
      endpointNote: 'The built-in list ships empty until each candidate is cleared. Until then, you supply your own.',
      watchLabel: 'Keep testing',
      watchNote: 'Repeats every 30 seconds and records each pass.',
      historyTitle: 'History',
      historyEmpty: 'No runs recorded yet.',
      colTime: 'Time',
      colDown: 'Down',
      colUp: 'Up',
      colPing: 'Ping',
      trendTitle: 'Trend',
      trendRuns: 'runs',
      summaryTitle: 'This run',
      localOnly: 'Stored on this device only. Nothing is uploaded.',

      // desktop: platform variants
      desktopChooseTitle: 'Which desktop?',
      desktopChooseBody:
        'One application, three window frames. The app itself is identical; what differs is the chrome the system draws around it and the conventions that come with it.',
      osWindows: 'Windows',
      osMacos: 'macOS',
      osLinux: 'Linux',
      osWindowsNote: 'Caption buttons at the top right, square corners, a menu bar in the window.',
      osMacosNote: 'Traffic lights at the top left, a unified title bar, the menu bar belongs to the system.',
      osLinuxNote: 'A GNOME header bar: actions and close button in the bar itself, no separate menu.',
      openPreview: 'Open',
      backToChooser: 'Other desktops',
      windowMenuFile: 'File',
      windowMenuView: 'View',
      windowMenuHelp: 'Help',

      // about.html
      aboutTitle: 'About this preview',
      aboutIntro:
        'This site exists so that the tool can be understood before it is installed. It is static, it measures nothing, and it sends nothing anywhere.',
      aboutSimTitle: 'What is simulated',
      aboutSimBody:
        'Every figure on every page is generated in the browser when you press the button. There is no server behind this site and no measurement of any kind. The three layer targets are addresses the IETF reserves for documentation (RFC 5737), which by definition route nowhere, so nothing here can be mistaken for a live host.',
      aboutStackTitle: 'How the real tool is built',
      aboutStackBody:
        'One measurement engine in Go, shared by every shell. The command line tool compiles from the same source; the desktop build runs the engine as a sidecar and reads its JSON; the Android build binds it as a library. None of the shells reimplements a measurement.',
      aboutStackEngine: 'Measurement engine',
      aboutStackEngineNote: 'Go. Network I/O and concurrency, and one command cross-compiles it for every platform.',
      aboutStackCli: 'Command line',
      aboutStackCliNote: 'Go, same source as the engine. A single static binary with no runtime to install.',
      aboutStackDesktop: 'Desktop',
      aboutStackDesktopNote: 'Tauri. A Rust shell around the system WebView, with this same HTML and CSS as its front end.',
      aboutStackApp: 'Android',
      aboutStackAppNote: 'Kotlin and Jetpack Compose, with the Go engine bound as a library.',
      aboutPrivacyTitle: 'Privacy',
      aboutPrivacyBody:
        'This preview stores one thing: which language you chose, in your own browser. No analytics, no cookies, no fonts or scripts fetched from anywhere else. The page works with the network cable unplugged.',
      aboutLicenceTitle: 'Licence',
      aboutLicenceBody: 'MIT. The measurement code is public because a number you cannot audit is a number you cannot trust.',
      aboutDisclaimerTitle: 'Disclaimer',
      aboutDisclaimerBody:
        'Nothing on this site is a measurement of your connection. Even in the real tool, a result is affected by everything else your machine and network are doing at the time, and should be read as an indication rather than a verdict.',

      // terminal.html
      terminalSubtitle: 'Command line preview',
      terminalNotice:
        'The layout, columns and wording below are exactly what the real tool prints. Only the figures are invented, and the addresses shown are documentation examples that route nowhere.',
      cmdFault: 'a fault on the first hop',
      skip: 'Skip',
      replay: 'Replay',
      exampleCommands: 'Example commands',

      langLabel: 'Language',

      // index.html
      tagline: 'It does not just tell you how fast your connection is. It tells you which segment is at fault.',
      whatItMeasures: 'What it measures',
      coreLatency: 'Latency, jitter and loss',
      coreLatencyBody: 'Round-trip time, how much it varies, and how much never comes back.',
      coreDownload: 'Download',
      coreDownloadBody: 'Throughput over plain HTTP, sampled across several endpoints and averaged.',
      coreUpload: 'Upload',
      coreUploadBody: 'The same, in the other direction, which is where home connections usually hurt.',
      differenceTitle: 'The part a speed test cannot do',
      differenceBody:
        'Latency, jitter and loss are measured separately at three points along the path. A fault at the first point is inside your own home; at the second, inside your provider; at the third, on the long haul. The same three numbers that a speed test reduces to one figure are what make the difference legible.',
      diffGateway: 'A fault here is inside your own network or Wi-Fi.',
      diffRegional: 'A fault here is inside your provider.',
      diffInternational: 'A fault only here points at international capacity.',
      shellsTitle: 'Three shells, one engine',
      shellsBody:
        'The measurement engine is shared. Each shell renders the same report for a different audience.',
      exploreTerminal: 'Command line',
      exploreTerminalBody: 'The real output, column for column.',
      exploreMobile: 'Android app',
      exploreMobileBody: 'Tap through a simulated run.',
      exploreDesktop: 'Desktop app',
      exploreDesktopBody: 'Watch a run with the window at full size.',
      platformsTitle: 'Platforms',
      platformCli: 'Command line',
      platformDesktop: 'Desktop',
      platformApp: 'Native app',
      planned: 'planned',
      notApplicable: 'not applicable',
      principlesTitle: 'How it behaves',
      principleCloud: 'No cloud services',
      principleCloudBody:
        'The project operates no servers. There is no account, no database, no sync and no telemetry. Results stay on your device.',
      principleTargets: 'You choose where your traffic goes',
      principleTargetsBody:
        'Defaults exist so the tool works out of the box, but every target can be overridden. Nothing is reported back to the project.',
      principleOpen: 'Open source, MIT',
      principleOpenBody: 'The measurement code is there to be read, which is the only way to trust a number.',
      viewOnGitHub: 'Source on GitHub',
      previewFooter:
        'This site is a preview. Nothing on it measures anything, and the addresses shown are documentation examples.',
    },

    'zh-TW': {
      appName: 'WiFi 測速工具',
      simNotice: '模擬展示。',
      simNoticeBody:
        '此頁面不進行任何真實量測。所有數值皆為當場隨機產生，顯示的位址為文件範例位址，不指向任何真實主機。',

      start: '開始測速',
      running: '測速中…',
      again: '重新測速',
      tapToStart: '點擊開始',
      idle: '待機',

      phasePing: '量測延遲中',
      phaseDownload: '量測下載中',
      phaseUpload: '量測上傳中',
      phaseLayers: '定位問題網段中',
      phaseDone: '完成',

      download: '下載',
      upload: '上傳',
      latency: '延遲',
      jitter: '抖動',
      loss: '遺失',
      grade: '評級',
      dns: 'DNS 解析',

      layered: '分層診斷',
      layerGateway: '本地網關',
      layerRegional: '區域出口',
      layerInternational: '國際節點',
      unreachable: '無回應',

      verdictHealthy: '你的連線狀況良好',
      verdictHealthyBody:
        '三個網段皆回應迅速且無封包遺失。若仍感覺卡頓，原因較可能在你所使用的網站或服務本身。',
      verdictLocal: '問題出在你家中的網路',
      verdictLocalBody:
        '你自己的路由器回應就已經很慢，後面每一段都繼承了這個延遲。可先嘗試靠近路由器、更換頻道或重新啟動。這與你的網路業者無關。',
      verdictInternational: '你家中與電信業者都正常',
      verdictInternationalBody:
        '前兩段皆健康，僅國際線路壅塞。連本地網站應該順暢，連國外網站則會變慢。你這端沒有需要處理的問題。',

      gradeExcellent: '極佳',
      gradeVeryGood: '很好',
      gradeGood: '良好',
      gradeFair: '普通',
      gradeSlow: '偏慢',

      navTest: '測速',
      navHistory: '歷史',
      navSettings: '設定',
      comingSoon: '此 Preview 僅呈現版面，無實際內容。',

      // shared: scenario switch
      scenarioLabel: '模擬情境',
      scenarioRandom: '隨機',
      scenarioHealthy: '連線正常',
      scenarioLocal: '家中網路異常',
      scenarioIntl: '國際頻寬壅塞',

      // desktop.html
      desktopSubtitle: '桌面應用展示',
      windowTitle: 'WiFi 測速工具',
      endpointLabel: '測速端點',
      endpointAuto: '自動（3 台伺服器）',
      endpointNote: '內建清單在每個候選端點通過檢查前維持空白。在此之前，由你自行指定。',
      watchLabel: '持續監控',
      watchNote: '每 30 秒重複一次，並記錄每次結果。',
      historyTitle: '歷史記錄',
      historyEmpty: '尚無記錄。',
      colTime: '時間',
      colDown: '下載',
      colUp: '上傳',
      colPing: '延遲',
      trendTitle: '趨勢',
      trendRuns: '次',
      summaryTitle: '本次結果',
      localOnly: '僅儲存於本機裝置，不會上傳任何資料。',

      // desktop: platform variants
      desktopChooseTitle: '選擇桌面平台',
      desktopChooseBody:
        '同一個應用程式，三種視窗外框。應用本體完全相同，不同的是系統替它畫的外框，以及隨之而來的操作慣例。',
      osWindows: 'Windows',
      osMacos: 'macOS',
      osLinux: 'Linux',
      osWindowsNote: '控制鈕在右上角、直角邊框、選單列位於視窗內。',
      osMacosNote: '紅黃綠燈在左上角、統一標題列，選單列屬於系統而非視窗。',
      osLinuxNote: 'GNOME headerbar：操作與關閉鍵直接在標題列上，無獨立選單列。',
      openPreview: '開啟',
      backToChooser: '其他桌面平台',
      windowMenuFile: '檔案',
      windowMenuView: '檢視',
      windowMenuHelp: '說明',

      // about.html
      aboutTitle: '關於本展示',
      aboutIntro:
        '本站的存在是為了讓人在安裝之前就能理解這個工具。它是靜態網頁，不進行任何量測，也不會把任何資料送到任何地方。',
      aboutSimTitle: '哪些是模擬的',
      aboutSimBody:
        '每一頁的每一個數字，都是你按下按鈕時在瀏覽器中當場產生的。本站背後沒有伺服器，也沒有任何形式的量測。三層目標使用 IETF 保留供文件使用的位址（RFC 5737），依定義不會路由到任何地方，因此不可能被誤認為真實主機。',
      aboutStackTitle: '真實工具的架構',
      aboutStackBody:
        '一個以 Go 寫成的量測引擎，由所有外殼共用。終端機版與引擎同源編譯；桌面版將引擎作為 sidecar 執行並讀取其 JSON；Android 版則以函式庫形式綁定。沒有任何一個外殼重新實作量測邏輯。',
      aboutStackEngine: '量測引擎',
      aboutStackEngineNote: 'Go。網路 I/O 與併發是它的本命領域，且一行指令即可交叉編譯全平台。',
      aboutStackCli: '終端機',
      aboutStackCliNote: 'Go，與引擎同源。單一靜態執行檔，無需安裝任何執行環境。',
      aboutStackDesktop: '桌面',
      aboutStackDesktopNote: 'Tauri。以 Rust 外殼包住系統 WebView，前端即為這一套 HTML 與 CSS。',
      aboutStackApp: 'Android',
      aboutStackAppNote: 'Kotlin 與 Jetpack Compose，並以函式庫形式綁定 Go 引擎。',
      aboutPrivacyTitle: '隱私',
      aboutPrivacyBody:
        '本展示只儲存一件事：你選擇的語言，存在你自己的瀏覽器裡。無分析工具、無 cookie、不從任何外部來源載入字型或腳本。拔掉網路線這個頁面照樣運作。',
      aboutLicenceTitle: '授權',
      aboutLicenceBody: 'MIT。量測程式碼公開，因為一個你無法查核的數字，就是一個你無法信任的數字。',
      aboutDisclaimerTitle: '免責聲明',
      aboutDisclaimerBody:
        '本站的任何內容都不是對你連線品質的量測。即使是真實工具，結果也會受到你的裝置與網路當下所有其他活動影響，應視為參考指標而非定論。',

      // terminal.html
      terminalSubtitle: '終端機展示',
      terminalNotice:
        '以下的排版、欄位與用詞，與真實工具實際輸出完全一致。僅有數值為虛構，顯示的位址為文件範例位址，不指向任何真實主機。',
      cmdFault: '第一跳故障情境',
      skip: '跳過',
      replay: '重播',
      exampleCommands: '指令範例',

      langLabel: '語言',

      // index.html
      tagline: '它不只告訴你網速多快，它告訴你問題出在哪一段。',
      whatItMeasures: '量測項目',
      coreLatency: '延遲、抖動與遺失',
      coreLatencyBody: '往返時間、它的變動幅度，以及有多少封包再也沒回來。',
      coreDownload: '下載',
      coreDownloadBody: '以一般 HTTP 量測吞吐量，對多個端點取樣後平均。',
      coreUpload: '上傳',
      coreUploadBody: '反方向的同一件事——而家用網路通常是這個方向出問題。',
      differenceTitle: '一般測速工具做不到的部分',
      differenceBody:
        '延遲、抖動與遺失在路徑上的三個位置分別量測。第一個位置出問題，代表問題在你家裡；第二個，在你的電信業者；第三個，在國際線路。一般測速工具把這些壓縮成單一數字，而分層量測正是讓差異可被讀懂的關鍵。',
      diffGateway: '此層異常代表問題在你自己的網路或 Wi-Fi。',
      diffRegional: '此層異常代表問題在你的電信業者內部。',
      diffInternational: '僅此層異常，代表問題在國際頻寬。',
      shellsTitle: '三種外殼，同一個引擎',
      shellsBody: '量測引擎完全共用。每種外殼只是把同一份報告呈現給不同的使用者。',
      exploreTerminal: '終端機',
      exploreTerminalBody: '真實輸出，逐欄一致。',
      exploreMobile: 'Android App',
      exploreMobileBody: '實際點過一次模擬測速。',
      exploreDesktop: '桌面應用',
      exploreDesktopBody: '在完整視窗中看一次測速流程。',
      platformsTitle: '支援平台',
      platformCli: '終端機',
      platformDesktop: '桌面',
      platformApp: '原生 App',
      planned: '規劃中',
      notApplicable: '不適用',
      principlesTitle: '運作原則',
      principleCloud: '不提供任何雲端服務',
      principleCloudBody:
        '本專案不營運任何伺服器。無帳號、無資料庫、無同步、無遙測。結果留在你自己的裝置上。',
      principleTargets: '流量去哪裡由你決定',
      principleTargetsBody:
        '預設值的存在只是為了開箱即用，但每一個目標都可以覆寫。不會有任何資料回報給本專案。',
      principleOpen: '開放原始碼，MIT',
      principleOpenBody: '量測程式碼公開供人閱讀——那是信任一個數字的唯一途徑。',
      viewOnGitHub: '在 GitHub 查看原始碼',
      previewFooter: '本站為展示頁面，不進行任何量測，所顯示的位址皆為文件範例位址。',
    },
  };

  let current = 'en';

  function t(key) {
    const table = DICT[current] || DICT.en;
    // A missing translation shows as its key rather than as a blank space, so
    // the gap is obvious while building instead of invisible afterwards.
    return table[key] ?? DICT.en[key] ?? key;
  }

  /** Rewrites every element carrying a data-i18n attribute. */
  function apply(root = document) {
    root.querySelectorAll('[data-i18n]').forEach((el) => {
      el.textContent = t(el.dataset.i18n);
    });
    root.querySelectorAll('[data-i18n-label]').forEach((el) => {
      el.setAttribute('aria-label', t(el.dataset.i18nLabel));
    });
    document.documentElement.lang = current === 'zh-TW' ? 'zh-Hant' : 'en';
  }

  function set(lang) {
    current = DICT[lang] ? lang : 'en';
    try {
      localStorage.setItem(STORAGE_KEY, current);
    } catch (e) {
      // A browser that refuses storage still gets a working page; the choice
      // simply does not survive a reload.
    }
    apply();
    document.dispatchEvent(new CustomEvent('languagechange', { detail: current }));
  }

  function init() {
    let saved = null;
    try {
      saved = localStorage.getItem(STORAGE_KEY);
    } catch (e) {
      saved = null;
    }
    current = DICT[saved] ? saved : 'en';
    apply();
    return current;
  }

  return { t, set, init, apply, get current() { return current; } };
})();
