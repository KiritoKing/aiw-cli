import { spawn, spawnSync } from "node:child_process";

export function commandExists(command) {
  const result = spawnSync("sh", ["-lc", `command -v ${quoteShell(command)} >/dev/null 2>&1`], {
    stdio: "ignore"
  });
  return result.status === 0;
}

export function commandPath(command) {
  const result = spawnSync("sh", ["-lc", `command -v ${quoteShell(command)}`], {
    encoding: "utf8"
  });
  return result.status === 0 ? result.stdout.trim() : "";
}

export function capture(command, args = [], options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd,
    env: options.env,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"]
  });
  if (result.status !== 0) {
    const error = new Error((result.stderr || result.stdout || `${command} failed`).trim());
    error.exitCode = result.status || 1;
    throw error;
  }
  return result.stdout.trim();
}

export function tryCapture(command, args = [], options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd,
    env: options.env,
    encoding: "utf8",
    input: options.input,
    stdio: [options.input === undefined ? "ignore" : "pipe", "pipe", "pipe"]
  });
  return {
    ok: result.status === 0,
    status: result.status,
    stdout: result.stdout.trim(),
    stderr: result.stderr.trim()
  };
}

export function runInherit(command, args = [], options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      env: options.env || process.env,
      stdio: "inherit"
    });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      const error = new Error(`${command} exited with code ${code}`);
      error.exitCode = code || 1;
      reject(error);
    });
  });
}

export function quoteShell(value) {
  const text = String(value);
  if (text.length === 0) {
    return "''";
  }
  return `'${text.replace(/'/g, `'\\''`)}'`;
}

export function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
