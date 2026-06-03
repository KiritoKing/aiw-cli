import fs from "node:fs";
import path from "node:path";
import { cleanAgentText, runAgentForText } from "./agent.mjs";
import { expandHome, resolveAgent } from "./config.mjs";
import { capture, tryCapture } from "./run.mjs";

const DEFAULT_COMMIT_PROMPT = `Generate a Git commit message for the staged diff.

Rules:
- Output only the commit message. Do not add Markdown fences or explanations.
- Prefer Conventional Commits when the change clearly fits one type.
- Keep the first line concise and specific.
- Use a body only when it clarifies non-obvious context or hook failures.
- Do not mention generated tooling unless it is part of the change.`;

export async function runCommit(config, flags) {
  const repo = capture("git", ["rev-parse", "--show-toplevel"], { cwd: process.cwd() });
  const retries = Number(flags.retries || config.commit.retries || 3);
  const agent = resolveAgent(config, flags.agent || config.commit.agent || config.defaults.agent);
  const customPrompt = loadCustomPrompt(config, flags);
  let lastFailure = null;

  for (let attempt = 1; attempt <= retries; attempt += 1) {
    const staged = readStagedDiff(config, repo);
    if (!staged.diff) {
      throw withExit("no staged changes; stage files before running aiw commit", 6);
    }

    const prompt = buildCommitPrompt({
      basePrompt: customPrompt,
      staged,
      attempt,
      retries,
      lastFailure
    });

    if (flags.printPrompt) {
      console.log(prompt);
      return;
    }

    const agentResult = runAgentForText(agent, prompt, { cwd: repo });
    if (!agentResult.ok) {
      throw withExit(
        [
          `agent '${agent.name}' failed while generating commit message`,
          agentResult.stderr.trim(),
          agentResult.stdout.trim()
        ].filter(Boolean).join("\n"),
        agentResult.status || 1
      );
    }

    const message = cleanAgentText(agentResult.stdout);
    if (!message) {
      throw withExit(`agent '${agent.name}' returned an empty commit message`, 7);
    }

    if (flags.dryRun) {
      console.log(message);
      return;
    }

    console.log(`[aiw commit] attempt ${attempt}/${retries}`);
    console.log(firstLine(message));
    const commitResult = tryCommit(repo, message);
    if (commitResult.ok) {
      process.stdout.write(commitResult.stdout);
      process.stderr.write(commitResult.stderr);
      return;
    }

    lastFailure = {
      message,
      output: [commitResult.stdout, commitResult.stderr].filter(Boolean).join("\n").trim()
    };
    process.stderr.write(lastFailure.output ? `${lastFailure.output}\n` : "");
    if (attempt < retries) {
      console.error(`[aiw commit] commit failed; regenerating message and retrying (${attempt + 1}/${retries})`);
    }
  }

  throw withExit(`git commit failed after ${retries} attempts`, 8);
}

export function runCommitMessage(config, flags) {
  const prompt = flags.prompt || readStdin();
  if (!prompt.trim()) {
    throw withExit("commit-message prompt is required on stdin or via --prompt", 2);
  }
  const repo = capture("git", ["rev-parse", "--show-toplevel"], { cwd: process.cwd() });
  const message = generateCommitMessage(config, flags, prompt, repo);
  console.log(message);
}

function generateCommitMessage(config, flags, prompt, repo) {
  const agent = resolveAgent(config, flags.agent || config.commit.agent || config.defaults.agent);
  const agentResult = runAgentForText(agent, prompt, { cwd: repo });
  if (!agentResult.ok) {
    throw withExit(
      [
        `agent '${agent.name}' failed while generating commit message`,
        agentResult.stderr.trim(),
        agentResult.stdout.trim()
      ].filter(Boolean).join("\n"),
      agentResult.status || 1
    );
  }

  const message = cleanAgentText(agentResult.stdout);
  if (!message) {
    throw withExit(`agent '${agent.name}' returned an empty commit message`, 7);
  }
  return message;
}

function readStagedDiff(config, repo) {
  const diff = capture("git", ["diff", "--cached", "--no-ext-diff"], { cwd: repo });
  const statResult = tryCapture("git", ["diff", "--cached", "--stat"], { cwd: repo });
  const statusResult = tryCapture("git", ["status", "--short"], { cwd: repo });
  const maxChars = Number(config.commit.max_diff_chars || 120000);
  const truncated = diff.length > maxChars;
  return {
    diff: truncated ? diff.slice(0, maxChars) : diff,
    truncated,
    stat: statResult.ok ? statResult.stdout : "",
    status: statusResult.ok ? statusResult.stdout : ""
  };
}

function readStdin() {
  try {
    return fs.readFileSync(0, "utf8");
  } catch {
    return "";
  }
}

function buildCommitPrompt({ basePrompt, staged, attempt, retries, lastFailure }) {
  const sections = [
    basePrompt,
    `Attempt: ${attempt}/${retries}`,
    "Git status:",
    fenced(staged.status || "(empty)"),
    "Staged diff stat:",
    fenced(staged.stat || "(empty)")
  ];

  if (lastFailure) {
    sections.push(
      "The previous commit message failed during git commit hooks. Generate a corrected commit message.",
      "Previous commit message:",
      fenced(lastFailure.message),
      "Hook / git commit output:",
      fenced(lastFailure.output || "(empty)")
    );
  }

  sections.push(
    staged.truncated
      ? "Staged diff (truncated due to max_diff_chars):"
      : "Staged diff:",
    fenced(staged.diff)
  );

  return sections.join("\n\n");
}

function loadCustomPrompt(config, flags) {
  const parts = [];
  const defaultPromptFile = config.commit.prompt_file
    ? resolveConfigPath(config, config.commit.prompt_file)
    : "";
  if (defaultPromptFile && fs.existsSync(defaultPromptFile)) {
    parts.push(fs.readFileSync(defaultPromptFile, "utf8").trim());
  }
  if (flags.promptFile) {
    const promptFile = expandHome(flags.promptFile);
    parts.push(fs.readFileSync(promptFile, "utf8").trim());
  }
  if (flags.prompt) {
    parts.push(flags.prompt);
  }
  if (parts.length === 0) {
    parts.push(DEFAULT_COMMIT_PROMPT);
  }
  return parts.filter(Boolean).join("\n\nAdditional user prompt:\n");
}

function resolveConfigPath(config, value) {
  const expanded = expandHome(value);
  return path.isAbsolute(expanded) ? expanded : path.join(config.configDir, expanded);
}

function tryCommit(repo, message) {
  return tryCapture("git", ["commit", "-F", "-"], {
    cwd: repo,
    input: message.endsWith("\n") ? message : `${message}\n`
  });
}

function fenced(value) {
  return `\`\`\`\n${value}\n\`\`\``;
}

function firstLine(value) {
  return value.split(/\r?\n/).find(Boolean) || "(empty commit message)";
}

function withExit(message, exitCode) {
  const error = new Error(message);
  error.exitCode = exitCode;
  return error;
}
