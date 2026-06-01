import { spawnSync } from "node:child_process";

export function runAgentForText(agent, prompt, options = {}) {
  const commitArgs = agent.commitArgs && agent.commitArgs.length > 0 ? agent.commitArgs : agent.args;
  const expandedArgs = [];
  let promptInArgs = false;

  for (const arg of commitArgs) {
    if (arg.includes("{{prompt}}")) {
      expandedArgs.push(arg.replaceAll("{{prompt}}", prompt));
      promptInArgs = true;
    } else {
      expandedArgs.push(arg);
    }
  }

  const result = spawnSync(agent.cmd, expandedArgs, {
    cwd: options.cwd,
    env: process.env,
    encoding: "utf8",
    input: promptInArgs ? undefined : prompt,
    maxBuffer: 1024 * 1024 * 16,
    stdio: ["pipe", "pipe", "pipe"]
  });

  return {
    ok: result.status === 0,
    status: result.status || 0,
    stdout: result.stdout || "",
    stderr: result.stderr || "",
    command: [agent.cmd, ...expandedArgs]
  };
}

export function cleanAgentText(text) {
  let cleaned = stripAnsi(text).trim();
  const fenced = cleaned.match(/```(?:[a-zA-Z0-9_-]+)?\s*([\s\S]*?)```/);
  if (fenced) {
    cleaned = fenced[1].trim();
  }
  cleaned = cleaned.replace(/^commit message:\s*/i, "").trim();
  if (
    (cleaned.startsWith('"') && cleaned.endsWith('"')) ||
    (cleaned.startsWith("'") && cleaned.endsWith("'"))
  ) {
    cleaned = cleaned.slice(1, -1).trim();
  }
  return cleaned;
}

function stripAnsi(text) {
  return text.replace(/\u001B\[[0-?]*[ -/]*[@-~]/g, "");
}
