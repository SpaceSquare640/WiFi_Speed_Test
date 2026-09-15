/*
 * The desktop application, minus its window.
 *
 * Three pages show this same application inside three different window frames,
 * because a Tauri build runs in the system WebView and the frame around it is
 * whatever the platform draws. The frames are genuinely different designs; the
 * application inside them is one implementation, mounted here.
 *
 * Splitting it this way is what keeps "three separate pages" from meaning
 * "three copies to keep in step".
 */

const DesktopShell = (() => {
  const PHASES = [
    { key: 'phasePing', metric: 'latency', ms: 900 },
    { key: 'phaseDownload', metric: 'download', ms: 1700 },
    { key: 'phaseUpload', metric: 'upload', ms: 1400 },
    { key: 'phaseLayers', metric: null, ms: 1000 },
  ];

  const ARC = 2 * Math.PI * 100 * 0.75;
  const HISTORY_LIMIT = 8;

  const LAYER_KEYS = {
    gateway: 'layerGateway',
    regional: 'layerRegional',
    international: 'layerInternational',
  };

  /** Builds the application body. Returns the elements the simulation drives. */
  function markup(mount) {
    mount.innerHTML = `
      <div class="pane pane--left">
        <div class="dial">
          <svg viewBox="0 0 232 232" aria-hidden="true">
            <circle class="dial__track" cx="116" cy="116" r="100" stroke-dasharray="471 628" />
            <circle class="dial__arc" data-el="arc" cx="116" cy="116" r="100" />
          </svg>
          <div class="dial__readout">
            <span class="dial__value numeric" data-el="value">--</span>
            <span class="dial__unit" data-el="unit"></span>
            <span class="dial__phase" data-el="phase"></span>
          </div>
        </div>

        <button type="button" class="button" data-el="start" data-i18n="start">Start test</button>

        <div class="control">
          <span class="control__label" data-i18n="endpointLabel">Endpoint</span>
          <select class="select" data-el="endpoint" data-i18n-label="endpointLabel">
            <option data-i18n="endpointAuto">Automatic (3 servers)</option>
            <option>https://example.invalid/100mb</option>
            <option>https://example.invalid/upload</option>
          </select>
          <span class="control__note" data-i18n="endpointNote"></span>
        </div>

        <div class="control">
          <label class="switch">
            <input type="checkbox" data-el="watch" />
            <span data-i18n="watchLabel">Keep testing</span>
          </label>
          <span class="control__note" data-i18n="watchNote"></span>
        </div>
      </div>

      <div class="pane">
        <div data-el="results" hidden>
          <section>
            <h2 class="card__title" data-i18n="summaryTitle">This run</h2>
            <div class="summary metrics">
              <div class="metric metric--download">
                <div class="metric__label" data-i18n="download">Download</div>
                <div class="metric__value numeric">
                  <span data-el="sumDownload">--</span><span class="metric__unit">Mbps</span>
                </div>
              </div>
              <div class="metric metric--upload">
                <div class="metric__label" data-i18n="upload">Upload</div>
                <div class="metric__value numeric">
                  <span data-el="sumUpload">--</span><span class="metric__unit">Mbps</span>
                </div>
              </div>
              <div class="metric metric--latency">
                <div class="metric__label" data-i18n="latency">Ping</div>
                <div class="metric__value numeric">
                  <span data-el="sumLatency">--</span><span class="metric__unit">ms</span>
                </div>
              </div>
              <div class="metric">
                <div class="metric__label" data-i18n="grade">Grade</div>
                <div class="metric__value" data-el="sumGrade">--</div>
              </div>
            </div>
          </section>

          <section class="card" style="margin-top: var(--space-4)">
            <h2 class="card__title" data-i18n="layered">Layered diagnostics</h2>
            <ul class="layers" data-el="layers"></ul>
            <div class="verdict" data-el="verdict">
              <span class="verdict__dot" aria-hidden="true"></span>
              <span class="verdict__text">
                <strong data-el="verdictTitle"></strong>
                <span data-el="verdictBody"></span>
              </span>
            </div>
          </section>
        </div>

        <section>
          <h2 class="card__title" data-i18n="historyTitle">History</h2>
          <table class="history">
            <thead>
              <tr>
                <th scope="col" data-i18n="colTime">Time</th>
                <th scope="col" data-i18n="colDown">Down</th>
                <th scope="col" data-i18n="colUp">Up</th>
                <th scope="col" data-i18n="colPing">Ping</th>
              </tr>
            </thead>
            <tbody data-el="historyBody"></tbody>
          </table>
          <p class="history-note" data-el="historyEmpty" data-i18n="historyEmpty"></p>
          <p class="history-note" style="margin-top: var(--space-2)" data-i18n="localOnly"></p>
        </section>
      </div>
    `;

    const el = {};
    mount.querySelectorAll('[data-el]').forEach((node) => {
      el[node.dataset.el] = node;
    });
    return el;
  }

  /** Mounts the application into a container and wires the simulation to it. */
  function mount(container, { scenarioChips, liveRegion } = {}) {
    const el = markup(container);
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    let running = false;
    let phaseKey = 'idle';
    let forced = null;
    let last = null;
    let history = [];
    let watchTimer = null;

    const setArc = (fraction, variant) => {
      el.arc.style.strokeDasharray = `${ARC} ${ARC}`;
      el.arc.style.strokeDashoffset = String(ARC * (1 - Math.max(0, Math.min(1, fraction))));
      el.arc.className.baseVal = `dial__arc${variant ? ` dial__arc--${variant}` : ''}`;
    };

    const setReadout = (value, unit) => {
      el.value.textContent = value;
      el.unit.textContent = unit;
    };

    // The animation frame gives the sweep; the timer guarantees the phase ends
    // even when the tab is hidden and frames stop arriving.
    const animate = (phase, target, unit, scale) =>
      new Promise((resolve) => {
        phaseKey = phase.key;
        el.phase.textContent = I18n.t(phaseKey);

        let settled = false;
        const finish = () => {
          if (settled) return;
          settled = true;
          setReadout(target.toFixed(unit === 'ms' ? 0 : 1), unit);
          setArc(target / scale, phase.metric);
          resolve();
        };

        if (reduceMotion) return void finish();

        const guard = setTimeout(finish, phase.ms + 400);
        const start = performance.now();
        const tick = (now) => {
          if (settled) return;
          const progress = Math.min(1, (now - start) / phase.ms);
          const shown = target * (1 - (1 - progress) ** 3);
          setReadout(shown.toFixed(unit === 'ms' ? 0 : 1), unit);
          setArc(shown / scale, phase.metric);
          if (progress < 1) requestAnimationFrame(tick);
          else {
            clearTimeout(guard);
            finish();
          }
        };
        requestAnimationFrame(tick);
      });

    const wait = (ms) => new Promise((r) => setTimeout(r, reduceMotion ? 0 : ms));

    function renderLayers(report) {
      el.layers.replaceChildren();
      report.layers.forEach((layer) => {
        const row = document.createElement('li');
        row.className = `layer layer--${layer.status}`;

        const name = document.createElement('span');
        name.className = 'layer__name';
        name.textContent = I18n.t(LAYER_KEYS[layer.kind]);

        const target = document.createElement('span');
        target.className = 'layer__target numeric';
        target.textContent = layer.target;

        const value = document.createElement('span');
        value.className = 'layer__value numeric';
        value.textContent = `${layer.latency.toFixed(2)} ms`;

        const detail = document.createElement('span');
        detail.className = 'layer__detail';
        detail.textContent =
          `${I18n.t('jitter')} ${layer.jitter.toFixed(2)} ms · ${I18n.t('loss')} ${layer.loss}%`;

        row.append(name, target, value, detail);
        el.layers.append(row);
      });
    }

    function renderHistory() {
      el.historyBody.replaceChildren();
      el.historyEmpty.hidden = history.length > 0;

      [...history].reverse().forEach((entry, index) => {
        const row = document.createElement('tr');
        if (index === 0) row.dataset.latest = 'true';
        [
          [entry.time, ''],
          [entry.download.toFixed(1), 'down numeric'],
          [entry.upload.toFixed(1), 'up numeric'],
          [entry.latency.toFixed(0), 'ping numeric'],
        ].forEach(([text, cls]) => {
          const td = document.createElement('td');
          td.className = cls;
          td.textContent = text;
          row.append(td);
        });
        el.historyBody.append(row);
      });
    }

    function render(report) {
      el.sumDownload.textContent = report.download.toFixed(1);
      el.sumUpload.textContent = report.upload.toFixed(1);
      el.sumLatency.textContent = report.layers[2].latency.toFixed(0);

      const grade = FakeData.grade(report.download);
      el.sumGrade.textContent = I18n.t(`grade${grade[0].toUpperCase()}${grade.slice(1)}`);

      renderLayers(report);
      el.verdict.className = `verdict verdict--${report.severity}`;
      el.verdictTitle.textContent = I18n.t(report.verdictKey);
      el.verdictBody.textContent = I18n.t(`${report.verdictKey}Body`);
      el.results.hidden = false;

      if (liveRegion) {
        liveRegion.textContent =
          `${I18n.t(report.verdictKey)}. ${I18n.t('download')} ${report.download.toFixed(1)} Mbps.`;
      }
    }

    function record(report) {
      const now = new Date();
      const pad = (n) => String(n).padStart(2, '0');
      history.push({
        time: `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`,
        download: report.download,
        upload: report.upload,
        latency: report.layers[2].latency,
      });
      if (history.length > HISTORY_LIMIT) history = history.slice(-HISTORY_LIMIT);
      renderHistory();
    }

    async function run() {
      if (running) return;
      running = true;

      el.start.disabled = true;
      el.start.textContent = I18n.t('running');
      el.results.hidden = true;

      const report = FakeData.report(forced);
      last = report;

      await animate(PHASES[0], report.layers[2].latency, 'ms', 300);
      await wait(150);
      await animate(PHASES[1], report.download, 'Mbps', 150);
      await wait(150);
      await animate(PHASES[2], report.upload, 'Mbps', 60);
      await wait(150);

      phaseKey = PHASES[3].key;
      el.phase.textContent = I18n.t(phaseKey);
      await wait(PHASES[3].ms);

      setReadout(report.download.toFixed(1), 'Mbps');
      setArc(report.download / 150, 'download');
      phaseKey = 'phaseDone';
      el.phase.textContent = I18n.t(phaseKey);

      render(report);
      record(report);

      el.start.disabled = false;
      el.start.textContent = I18n.t('again');
      running = false;

      if (el.watch.checked) {
        clearTimeout(watchTimer);
        watchTimer = setTimeout(run, reduceMotion ? 1500 : 6000);
      }
    }

    el.start.addEventListener('click', run);

    el.watch.addEventListener('change', () => {
      if (!el.watch.checked) {
        clearTimeout(watchTimer);
        watchTimer = null;
      } else if (!running) {
        watchTimer = setTimeout(run, 1200);
      }
    });

    (scenarioChips || []).forEach((chip) => {
      chip.addEventListener('click', () => {
        forced = chip.dataset.scenario === 'random' ? null : chip.dataset.scenario;
        scenarioChips.forEach((other) =>
          other.setAttribute('aria-pressed', String(other === chip)),
        );
      });
    });

    document.addEventListener('languagechange', () => {
      if (!running) el.start.textContent = I18n.t(last ? 'again' : 'start');
      el.phase.textContent = I18n.t(phaseKey);
      if (last) render(last);
    });

    setArc(0, null);
    setReadout('--', '');
    el.phase.textContent = I18n.t('idle');
    renderHistory();
    I18n.apply(container);

    return { run };
  }

  return { mount };
})();
