// The desktop shell.
//
// It performs no measurement. The engine is the same wifitest binary the
// command line ships, bundled here as a sidecar and invoked with --json; this
// process spawns it, reads the object it prints, and hands that to the window.
//
// Keeping the boundary at a process rather than a library binding is what lets
// the engine stay a Go program with no knowledge that a desktop build exists.

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use serde::Serialize;
use tauri_plugin_shell::ShellExt;

/// What the window receives for one run.
///
/// The exit code travels with the report rather than being turned into an
/// error, because the interesting ones are not failures: 3 means every
/// throughput endpoint failed, and the layered diagnostics in that same report
/// are still perfectly good.
#[derive(Serialize)]
struct Diagnostic {
    report: serde_json::Value,
    exit_code: i32,
}

#[tauri::command]
async fn run_diagnostic(app: tauri::AppHandle, icmp: bool) -> Result<Diagnostic, String> {
    // --no-history: the window is not the owner of the user's history file, and
    // writing to it from here would interleave with a command line run.
    let mut args = vec!["--json", "--no-history"];
    if icmp {
        args.push("--icmp");
    }

    let output = app
        .shell()
        .sidecar("wifitest")
        .map_err(|e| format!("the engine could not be located: {e}"))?
        .args(args)
        .output()
        .await
        .map_err(|e| format!("the engine could not be started: {e}"))?;

    let stdout = String::from_utf8_lossy(&output.stdout);
    let exit_code = output.status.code().unwrap_or(-1);

    if stdout.trim().is_empty() {
        // Nothing on stdout means no report at all, so the stderr text is the
        // only thing that can explain what happened.
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!(
            "the engine exited with {exit_code} and printed no report: {}",
            stderr.trim()
        ));
    }

    let report: serde_json::Value = serde_json::from_str(&stdout)
        .map_err(|e| format!("the engine printed something that is not a report: {e}"))?;

    Ok(Diagnostic { report, exit_code })
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![run_diagnostic])
        .run(tauri::generate_context!())
        .expect("error while running the application");
}
