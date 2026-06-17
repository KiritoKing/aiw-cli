import path from "node:path";
import { aiwBinPath, resolveAgent } from "./config.mjs";
import { quoteShell } from "./run.mjs";

export function buildLayout(config, agentName) {
  return buildCmuxLayout(buildProjectLayoutModel(config, agentName));
}

export function buildProjectLayout(config, agentName) {
  return buildLayout(config, agentName);
}

export function buildProjectLayoutModel(config, agentName) {
  const agent = resolveAgent(config, agentName);
  const aiw = quoteShell(aiwBinPath());
  const agentCommand = [agent.cmd, ...agent.args].map(quoteShell).join(" ");
  return {
    type: "project",
    panes: [
      terminalPane("Files", `${aiw} files`),
      terminalPane(agentTitle(agent.name), agentCommand),
      terminalPane("Git", `${aiw} git`)
    ]
  };
}

export function buildCmuxLayout(model) {
  if (model.type === "scratch") {
    return {
      direction: "horizontal",
      split: 0.34,
      children: model.panes.map(cmuxTerminalPane)
    };
  }
  return {
    direction: "vertical",
    split: 0.56,
    children: [
      {
        direction: "horizontal",
        split: 0.34,
        children: [
          cmuxTerminalPane(model.panes[0]),
          cmuxTerminalPane(model.panes[1])
        ]
      },
      cmuxTerminalPane(model.panes[2])
    ]
  };
}

export function buildScratchLayout(config, agentName) {
  return buildCmuxLayout(buildScratchLayoutModel(config, agentName));
}

export function buildScratchLayoutModel(config, agentName) {
  const agent = resolveAgent(config, agentName);
  const aiw = quoteShell(aiwBinPath());
  const agentCommand = [agent.cmd, ...agent.args].map(quoteShell).join(" ");
  return {
    type: "scratch",
    panes: [
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
  return { name, command };
}

function cmuxTerminalPane(pane) {
  return {
    pane: {
      surfaces: [
        {
          type: "terminal",
          name: pane.name,
          command: pane.command
        }
      ]
    }
  };
}

function agentTitle(agentName) {
  return agentName.slice(0, 1).toUpperCase() + agentName.slice(1);
}
