#!/usr/bin/env node
import { appendFile, readFile, stat, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const args = parseArgs(process.argv.slice(2));
const channel = args.channel;

if (!['stable', 'beta', 'alpha'].includes(channel)) {
  fail('Usage: node scripts/prepare-release.mjs --channel stable|beta|alpha [--base-version x.y.z] [--dry-run]');
}

const root = process.cwd();
const packagePath = resolve(root, 'package.json');
const packageJson = JSON.parse(await readFile(packagePath, 'utf8'));
const packageName = packageJson.name;
const currentVersion = parseVersion(packageJson.version);
const registryUrl = normalizeRegistryUrl(packageJson.publishConfig?.registry ?? 'https://registry.npmjs.org/');
const publishedVersions = await fetchPublishedVersions(registryUrl, packageName);
const explicitBase = args.baseVersion ? parseStableVersion(args.baseVersion, '--base-version') : null;
const nextVersion = computeNextVersion({
  channel,
  currentVersion,
  explicitBase,
  publishedVersions,
});
const npmTag = channel === 'stable' ? 'latest' : channel;
const gitTag = `v${nextVersion}`;

if (!args.dryRun) {
  packageJson.version = nextVersion;
  await writeJson(packagePath, packageJson);
  await updatePackageLock(root, packageName, nextVersion);
}

await writeOutput({
  channel,
  package_name: packageName,
  version: nextVersion,
  tag: gitTag,
  npm_tag: npmTag,
  registry: registryUrl,
});

console.log(`Prepared ${packageName}@${nextVersion} for ${channel} release (${npmTag} dist-tag).`);

function parseArgs(rawArgs) {
  const parsed = {
    channel: '',
    baseVersion: '',
    dryRun: false,
  };

  for (let index = 0; index < rawArgs.length; index += 1) {
    const arg = rawArgs[index];
    if (arg === '--channel') {
      parsed.channel = requireValue(rawArgs, index, arg);
      index += 1;
    } else if (arg.startsWith('--channel=')) {
      parsed.channel = arg.slice('--channel='.length);
    } else if (arg === '--base-version') {
      parsed.baseVersion = requireValue(rawArgs, index, arg);
      index += 1;
    } else if (arg.startsWith('--base-version=')) {
      parsed.baseVersion = arg.slice('--base-version='.length);
    } else if (arg === '--dry-run') {
      parsed.dryRun = true;
    } else {
      fail(`Unknown argument: ${arg}`);
    }
  }

  return parsed;
}

function requireValue(rawArgs, index, flag) {
  const value = rawArgs[index + 1];
  if (!value || value.startsWith('--')) {
    fail(`Missing value for ${flag}`);
  }
  return value;
}

function computeNextVersion({ channel, currentVersion, explicitBase, publishedVersions }) {
  const stableVersions = publishedVersions.filter((version) => version.prerelease.length === 0);
  const latestStable = maxBy(stableVersions, compareCore);
  const currentBase = toBaseVersion(currentVersion);
  const base = explicitBase ?? selectReleaseBase(channel, currentBase, latestStable);

  if (channel === 'stable') {
    return nextStableVersion(base, publishedVersions);
  }

  return nextPrereleaseVersion(base, channel, publishedVersions);
}

function selectReleaseBase(channel, currentBase, latestStable) {
  if (!latestStable) {
    return currentBase;
  }

  if (compareCore(currentBase, latestStable) > 0) {
    return currentBase;
  }

  if (channel === 'stable') {
    return bumpPatch(latestStable);
  }

  return bumpPatch(latestStable);
}

function nextStableVersion(base, publishedVersions) {
  let candidate = toBaseVersion(base);
  while (hasExactVersion(publishedVersions, formatVersion(candidate))) {
    candidate = bumpPatch(candidate);
  }
  return formatVersion(candidate);
}

function nextPrereleaseVersion(base, preid, publishedVersions) {
  const matchingNumbers = publishedVersions
    .filter((version) => compareCore(version, base) === 0)
    .map((version) => prereleaseNumber(version, preid))
    .filter((number) => number !== null);
  let nextNumber = matchingNumbers.length > 0 ? Math.max(...matchingNumbers) + 1 : 0;
  let candidate = `${formatVersion(base)}-${preid}.${nextNumber}`;

  while (hasExactVersion(publishedVersions, candidate)) {
    nextNumber += 1;
    candidate = `${formatVersion(base)}-${preid}.${nextNumber}`;
  }

  return candidate;
}

function prereleaseNumber(version, preid) {
  if (version.prerelease.length !== 2) {
    return null;
  }

  const [id, number] = version.prerelease;
  if (id !== preid || !Number.isInteger(number)) {
    return null;
  }

  return number;
}

function hasExactVersion(versions, versionText) {
  return versions.some((version) => version.raw === versionText);
}

function bumpPatch(version) {
  return {
    major: version.major,
    minor: version.minor,
    patch: version.patch + 1,
    prerelease: [],
    raw: `${version.major}.${version.minor}.${version.patch + 1}`,
  };
}

function toBaseVersion(version) {
  return {
    major: version.major,
    minor: version.minor,
    patch: version.patch,
    prerelease: [],
    raw: `${version.major}.${version.minor}.${version.patch}`,
  };
}

function maxBy(values, compare) {
  if (values.length === 0) {
    return null;
  }

  return values.reduce((max, value) => (compare(value, max) > 0 ? value : max), values[0]);
}

function compareCore(left, right) {
  for (const key of ['major', 'minor', 'patch']) {
    if (left[key] > right[key]) {
      return 1;
    }
    if (left[key] < right[key]) {
      return -1;
    }
  }
  return 0;
}

function parseStableVersion(value, label) {
  const version = parseVersion(value);
  if (version.prerelease.length > 0) {
    fail(`${label} must be a stable x.y.z version, got ${value}`);
  }
  return version;
}

function parseVersion(value) {
  if (typeof value !== 'string') {
    fail(`Invalid semver value: ${value}`);
  }

  const match = /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/.exec(value);
  if (!match) {
    fail(`Invalid semver value: ${value}`);
  }

  return {
    raw: value,
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    prerelease: match[4] ? match[4].split('.').map(parsePrereleasePart) : [],
  };
}

function parsePrereleasePart(value) {
  if (/^(0|[1-9]\d*)$/.test(value)) {
    return Number(value);
  }
  return value;
}

function formatVersion(version) {
  const core = `${version.major}.${version.minor}.${version.patch}`;
  if (!version.prerelease || version.prerelease.length === 0) {
    return core;
  }
  return `${core}-${version.prerelease.join('.')}`;
}

async function fetchPublishedVersions(registryUrl, packageName) {
  const metadataUrl = new URL(encodeURIComponent(packageName), registryUrl);
  const response = await fetch(metadataUrl);

  if (response.status === 404) {
    return [];
  }

  if (!response.ok) {
    fail(`Failed to fetch ${packageName} metadata from ${registryUrl}: HTTP ${response.status}`);
  }

  const metadata = await response.json();
  return Object.keys(metadata.versions ?? {}).map(parseVersion);
}

function normalizeRegistryUrl(value) {
  const url = new URL(value);
  if (!url.pathname.endsWith('/')) {
    url.pathname = `${url.pathname}/`;
  }
  return url.toString();
}

async function updatePackageLock(rootDir, packageName, version) {
  const lockPath = resolve(rootDir, 'package-lock.json');
  if (!(await fileExists(lockPath))) {
    return;
  }

  const lock = JSON.parse(await readFile(lockPath, 'utf8'));
  if (lock.name === packageName) {
    lock.version = version;
  }
  if (lock.packages?.['']) {
    lock.packages[''].version = version;
  }
  await writeJson(lockPath, lock);
}

async function fileExists(path) {
  try {
    await stat(path);
    return true;
  } catch (error) {
    if (error?.code === 'ENOENT') {
      return false;
    }
    throw error;
  }
}

async function writeJson(path, value) {
  await writeFile(path, `${JSON.stringify(value, null, 2)}\n`);
}

async function writeOutput(outputs) {
  const lines = Object.entries(outputs).map(([key, value]) => `${key}=${value}`);
  if (process.env.GITHUB_OUTPUT) {
    await appendFile(process.env.GITHUB_OUTPUT, `${lines.join('\n')}\n`);
  }
  for (const line of lines) {
    console.log(line);
  }
}

function fail(message) {
  console.error(message);
  process.exit(1);
}
