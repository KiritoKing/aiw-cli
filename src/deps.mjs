import { commandExists, commandPath } from "./run.mjs";
import { workstationLabel, workstationRequirements } from "./workstation.mjs";

const COMMON_TOOLS = [
  "git",
  "cmux",
  "tmux",
  "wt",
  "yazi",
  "nvim",
  "lazygit",
  "delta",
  "fd",
  "rg",
  "fzf",
  "bat",
  "eza",
  "codex",
  "claude",
  "opencode",
  "gemini",
  "aider",
  "cmux-git-diff"
];

export function collectDoctor(config) {
  const tools = COMMON_TOOLS.map((name) => ({
    name,
    ok: commandExists(name),
    path: commandPath(name)
  }));
  const agents = Object.entries(config.agents).map(([name, agent]) => ({
    name,
    cmd: agent.cmd,
    ok: typeof agent.cmd === "string" && commandExists(agent.cmd),
    path: typeof agent.cmd === "string" ? commandPath(agent.cmd) : ""
  }));
  return { tools, agents };
}

export function gate(profile, config, agent) {
  const requirements = requirementsFor(profile, config, agent);
  const missing = [];
  const satisfied = [];
  const missingRecommended = [];
  const satisfiedRecommended = [];

  for (const requirement of requirements.commands) {
    if (commandExists(requirement)) {
      satisfied.push(requirement);
    } else {
      missing.push(requirement);
    }
  }

  for (const alternative of requirements.anyOf) {
    const found = alternative.find((candidate) => commandExists(candidate));
    if (found) {
      satisfied.push(found);
    } else {
      missing.push(alternative.join(" or "));
    }
  }

  for (const requirement of requirements.recommended) {
    if (commandExists(requirement)) {
      satisfiedRecommended.push(requirement);
    } else {
      missingRecommended.push(requirement);
    }
  }

  return {
    ok: missing.length === 0,
    profile,
    satisfied,
    missing,
    satisfiedRecommended,
    missingRecommended
  };
}

export function assertGate(profile, config, agent) {
  const result = gate(profile, config, agent);
  if (result.ok) {
    printRecommendedWarnings(result);
    return result;
  }
  const error = new Error(
    [
      `dependency gate '${profile}' failed`,
      ...result.missing.map((item) => `  [missing] ${item}`),
      "Install missing tools or change ~/.config/aiw config before retrying."
    ].join("\n")
  );
  error.exitCode = 10;
  throw error;
}

export function printDoctor(config, options = {}) {
  const doctor = collectDoctor(config);
  const gateName = options.gate;
  const gateResult = gateName ? gate(gateName, config, options.agent) : gate("p0", config, options.agent);
  const payload = {
    ok: gateResult.ok,
    gate: gateResult,
    tools: doctor.tools,
    agents: doctor.agents,
    config: {
      configDir: config.configDir,
      aiwPath: config.aiwPath,
      agentsPath: config.agentsPath,
      workstation: workstationLabel(config)
    }
  };

  if (options.json) {
    console.log(JSON.stringify(payload, null, 2));
  } else {
    for (const tool of doctor.tools) {
      const status = tool.ok ? "[ok]" : "[missing]";
      const suffix = tool.path ? ` ${tool.path}` : "";
      console.log(`${status} ${tool.name}${suffix}`);
    }
    console.log("");
    console.log(`gate: ${gateResult.profile}`);
    if (gateResult.ok) {
      console.log("[ok] dependency gate passed");
    } else {
      for (const item of gateResult.missing) {
        console.log(`[missing] ${item}`);
      }
    }
    for (const item of gateResult.missingRecommended) {
      console.log(`[recommended missing] ${item}`);
    }
  }

  if (!gateResult.ok) {
    const error = new Error(`dependency gate '${gateResult.profile}' failed`);
    error.exitCode = 10;
    throw error;
  }
}

function requirementsFor(profile, config, agent) {
  const agentCmd = agent?.cmd;
  const gitDeps = [config.defaults.git || "lazygit", ...lazygitOverlayDeps(config)];
  const workstationDeps = workstationRequirements(profile, config, agent);
  if (workstationDeps.commands.length > 0) {
    return req(workstationDeps.commands, [], workstationDeps.recommended);
  }
  switch (profile) {
    case "base":
      return req(["git"]);
    case "worktrunk":
    case "workspace":
      return req(["git", "wt"]);
    case "files":
      return req([config.defaults.files || "yazi"]);
    case "git":
      return req(gitDeps);
    case "edit":
      return req([config.defaults.editor || "nvim"]);
    case "grep":
      return req(["rg", "fzf", "bat", config.defaults.editor || "nvim"]);
    case "pick":
      return req(["fd", "fzf", "bat", config.defaults.editor || "nvim"]);
    case "tree":
      return req([], [["eza", "find"]]);
    case "diff":
      return req(["git"], [["cmux-git-diff", "delta"]]);
    case "commit":
      return req(["git", agentCmd].filter(Boolean));
    case "p0":
    default:
      return req(["git", "wt", ...workstationRequirements("layout", config, agent).commands, "fd", "rg", "fzf", "bat", "eza"], [], workstationRequirements("layout", config, agent).recommended);
  }
}

function req(commands, anyOf = [], recommended = []) {
  return { commands, anyOf, recommended };
}

function lazygitOverlayDeps(config) {
  return config.git.lazygit_config ? ["delta"] : [];
}

function printRecommendedWarnings(result) {
  for (const item of result.missingRecommended) {
    console.warn(`[warn] recommended dependency missing: ${item}`);
  }
}
