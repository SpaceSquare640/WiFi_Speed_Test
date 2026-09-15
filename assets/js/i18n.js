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
