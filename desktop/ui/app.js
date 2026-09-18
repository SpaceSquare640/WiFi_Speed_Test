/*
 * The desktop front end.
 *
 * It measures nothing itself. Every figure on screen comes from the wifitest
 * binary running as a sidecar: the Rust side spawns it with --json, and this
 * file renders the object it printed. The schema is the same contract the
 * command line publishes, which is why the window needed no new Go code.
 *
 * The consequence worth stating: if a field is absent here, the engine did not
 * measure it. Nothing on this screen is invented to fill a gap.
 */

const invoke = window.__TAURI__.core.invoke;

const HOPS = [
  {
    kind: 'gateway',
    name: 'Your network',
    meaning:
      'The first hop out of this machine — your router or access point. A fault here is inside your own home or office, and no amount of complaining to a provider will move it.',
  },
  {
    kind: 'regional_egress',
    name: 'Your provider',
    meaning:
      "The resolver this machine is configured to use, which in practice sits inside your provider's network. A fault that starts here, with a healthy first hop, points at the provider rather than at you.",
  },
  {
    kind: 'international',
    name: 'The wider internet',
    meaning:
      'A public address reached over the long haul. A fault only here means your home and your provider are both fine, and the congestion is further out than either.',
  },
];

const el = (id) => document.getElementById(id);
const ui = {
  run: el('run'),
  method: el('method'),
  watch: el('watch'),
  phase: el('phase'),
  path: el('path'),
  verdict: el('verdict'),
  verdictTitle: el('verdict-title'),
  verdictBody: el('verdict-body'),
  download: el('download'),
  upload: el('upload'),
  dns: el('dns'),
  detailEmpty: el('detail-empty'),
  detailBody: el('detail-body'),
  raw: el('raw'),
};

let report = null;
let selected = 'gateway';
let running = false;
let watchTimer = null;

/* ---------- rendering ---------- */

function hopStatus(layer) {
  if (!layer) return '';
  if (!layer.ok) return 'bad';
  if (layer.loss_pct > 0) return 'warn';
  return 'ok';
}

function renderPath() {
  ui.path.replaceChildren();

  HOPS.forEach((hop) => {
    const layer = report?.layers?.find((l) => l.kind === hop.kind);
    const node = document.createElement('button');
    node.type = 'button';
    node.className = `hop hop--${hopStatus(layer)}`;
    node.setAttribute('role', 'tab');
    node.setAttribute('aria-selected', String(selected === hop.kind));

    const name = document.createElement('div');
    name.className = 'hop__name';
    name.textContent = hop.name;

    const value = document.createElement('div');
    value.className = 'hop__value';
    if (!layer) {
      value.classList.add('is-idle');
      value.textContent = '—';
    } else if (!layer.ok) {
      // No answer is not a slow answer, and it is certainly not zero.
      value.classList.add('is-silent');
      value.textContent = 'no reply';
    } else {
      value.textContent = `${layer.latency_ms.toFixed(1)} ms`;
    }

    const target = document.createElement('div');
    target.className = 'hop__target';
    target.textContent = layer ? layer.target : ' ';

    const bar = document.createElement('div');
    bar.className = 'hop__bar';

    node.append(name, value, target, bar);
    node.addEventListener('click', () => {
      selected = hop.kind;
      renderPath();
      renderDetail();
    });
    ui.path.append(node);
  });
}

function renderDetail() {
  const hop = HOPS.find((h) => h.kind === selected);
  const layer = report?.layers?.find((l) => l.kind === selected);

  if (!hop || !layer) {
    ui.detailEmpty.hidden = false;
    ui.detailBody.hidden = true;
    return;
  }

  ui.detailEmpty.hidden = true;
  ui.detailBody.hidden = false;

  el('detail-title').textContent = hop.name;
  el('detail-target').textContent = layer.target;

  el('d-latency').textContent = layer.ok ? `${layer.latency_ms.toFixed(2)} ms` : 'no reply';
  el('d-jitter').textContent = layer.ok ? `${layer.jitter_ms.toFixed(2)} ms` : '—';
  el('d-loss').textContent = `${layer.loss_pct.toFixed(0)} %`;
  el('d-port').textContent = layer.probe_port ? String(layer.probe_port) : 'none answered';
  el('d-source').textContent = layer.user_defined ? 'you set it' : 'auto-detected';

  let meaning = hop.meaning;
  if (!layer.ok) {
    meaning +=
      ' This one answered on none of the ports tried. That is worth knowing, but it is not proof of a fault: plenty of routers ignore probes while forwarding traffic perfectly. Read it together with the hops after it.';
  }
  el('d-meaning').textContent = meaning;
}

/*
 * The reading. Three rows of numbers do not tell anyone which segment is at
 * fault; the conclusion has to be stated. The rules below are deliberately
 * cautious — a silent hop whose successors are healthy is reported as a
 * curiosity, not an alarm.
 */
function renderVerdict() {
  if (!report) return;

  const byKind = Object.fromEntries((report.layers || []).map((l) => [l.kind, l]));
  const gw = byKind.gateway;
  const isp = byKind.regional_egress;
  const world = byKind.international;

  let level = 'ok';
  let title = 'This connection looks healthy';
  let body =
    'Every segment answered, and none of them lost anything. If something still feels slow, the cause is more likely the service at the other end than the path to it.';

  if (gw && !gw.ok && isp?.ok && world?.ok) {
    level = 'warn';
    title = 'Your router did not answer, and that is probably fine';
    body =
      'Nothing came back from the first hop on any port tried, yet everything beyond it answered normally — so it is forwarding traffic and doing its job. Many routers ignore probes like these.';
  } else if (gw?.ok && (gw.loss_pct > 0 || gw.latency_ms > 30)) {
    level = 'bad';
    title = 'The problem starts inside your own network';
    body =
      'Your first hop is already slow or losing packets, and everything past it inherits that. Start with the router: distance, interference, or a restart. Your provider is not the cause.';
  } else if (isp && !isp.ok) {
    level = 'bad';
    title = 'Your provider is not answering';
    body =
      'Your own network is fine, but the resolver that stands in for your provider returned nothing. If the wider internet also fails, the link out of your building is the thing to report.';
  } else if (world && !world.ok) {
    level = 'bad';
    title = 'The wider internet is unreachable';
    body =
      'Your network and your provider both answer, but nothing beyond them does. Local services may still work while everything else appears down.';
  } else if (world?.ok && isp?.ok && world.latency_ms > isp.latency_ms * 4 + 100) {
    level = 'warn';
    title = 'Your home and your provider are fine';
    body =
      'The first two segments are healthy; only the long haul is slow. Local sites will feel normal while distant ones drag. There is nothing to fix on your side.';
  }

  ui.verdict.className = `verdict verdict--${level}`;
  ui.verdictTitle.textContent = title;
  ui.verdictBody.textContent = body;
}

function rate(node, value, unit, ok) {
  if (!ok) {
    node.classList.add('is-absent');
    node.textContent = 'not measured';
    return;
  }
  node.classList.remove('is-absent');
  node.innerHTML = '';
  node.append(document.createTextNode(value.toFixed(2)));
  const small = document.createElement('small');
  small.textContent = unit;
  node.append(small);
}

function renderReport(result) {
  report = result.report;

  renderPath();
  renderDetail();
  renderVerdict();

  rate(ui.download, report.download?.mbps, 'Mbps', report.download?.ok);
  rate(ui.upload, report.upload?.mbps, 'Mbps', report.upload?.ok);
  rate(ui.dns, report.dns?.resolve_ms, 'ms', report.dns?.ok);

  el('s-time').textContent = new Date(report.timestamp).toLocaleTimeString();
  el('s-schema').textContent = `schema ${report.schema_version}`;
  const exit = el('s-exit');
  exit.textContent = `exit ${result.exit_code}`;
  exit.dataset.ok = String(result.exit_code === 0);

  ui.raw.textContent = JSON.stringify(report, null, 2);
}

/* ---------- running ---------- */

async function run() {
  if (running) return;
  running = true;
  ui.run.disabled = true;
  ui.phase.textContent = 'measuring…';

  try {
    const result = await invoke('run_diagnostic', { icmp: ui.method.value === 'icmp' });
    renderReport(result);
    ui.phase.textContent = 'done';
  } catch (error) {
    // The engine failing is itself a finding, and hiding it would leave the
    // window showing a stale result as though it were current.
    ui.phase.textContent = 'failed';
    ui.verdict.className = 'verdict verdict--bad';
    ui.verdictTitle.textContent = 'The engine could not be run';
    ui.verdictBody.textContent = String(error);
    ui.raw.textContent = String(error);
  } finally {
    ui.run.disabled = false;
    running = false;
    if (ui.watch.checked) {
      clearTimeout(watchTimer);
      watchTimer = setTimeout(run, 30000);
    }
  }
}

ui.run.addEventListener('click', run);

ui.watch.addEventListener('change', () => {
  if (!ui.watch.checked) {
    clearTimeout(watchTimer);
  } else if (!running) {
    watchTimer = setTimeout(run, 1000);
  }
});

el('toggle-raw').addEventListener('click', () => {
  ui.raw.hidden = !ui.raw.hidden;
});

renderPath();
