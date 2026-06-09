import path from "node:path";
import { aiwBinPath, resolveAgent } from "./config.mjs";
import { quoteShell } from "./run.mjs";

export function buildLayout(config, agentName) {
  return buildProjectLayout(config, agentName);
}

export function buildProjectLayout(config, agentName) {
  const agent = resolveAgent(config, agentName);
  const aiw = quoteShell(aiwBinPath());
  const agentCommand = [agent.cmd, ...agent.args].map(quoteShell).join(" ");
  return {
    direction: "vertical",
    split: 0.56,
    children: [
      {
        direction: "horizontal",
        split: 0.34,
        children: [
          terminalPane("Files", `${aiw} files`),
          terminalPane(agentTitle(agent.name), agentCommand)
        ]
      },
      terminalPane("Git", `${aiw} git`)
    ]
  };
}

export function buildScratchLayout(config, agentName) {
  const agent = resolveAgent(config, agentName);
  const aiw = quoteShell(aiwBinPath());
  const agentCommand = [agent.cmd, ...agent.args].map(quoteShell).join(" ");
  return {
    direction: "horizontal",
    split: 0.34,
    children: [
      terminalPane("Files", `${aiw} files`),
      terminalPane(agentTitle(agent.name), agentCommand)
    ]
  };
}

export function workspaceName(cwd, agentName) {
  const repo = path.basename(cwd);
  return `AI ${agentName}: ${repo}`;
}

export function scratchWorkspaceName(cwd, agentName) {
  const date = path.basename(path.dirname(cwd));
  const session = path.basename(cwd);
  return `AI ${agentName}: ${date}/${session}`;
}

function terminalPane(name, command) {
  return {
    pane: {
      surfaces: [
        {
          type: "terminal",
          name,
          command
        }
      ]
    }
  };
}

function agentTitle(agentName) {
  return agentName.slice(0, 1).toUpperCase() + agentName.slice(1);
}
