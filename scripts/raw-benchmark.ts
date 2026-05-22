import { readdir, readFile, writeFile, mkdir } from "fs/promises";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
import { execFile } from "child_process";
import { promisify } from "util";

const execFileAsync = promisify(execFile);
const __dirname = dirname(fileURLToPath(import.meta.url));

const ITERATIONS = 10;
const CLI_PATH = join(__dirname, "..", "gdrive-bench");
const DOCS_DIR = join(__dirname, "..", "docs");

interface TimingResult {
  operation: string;
  path: "cli" | "local";
  times_ms: number[];
  avg_ms: number;
  p50_ms: number;
  p95_ms: number;
  min_ms: number;
  max_ms: number;
}

async function timeExec(cmd: string[]): Promise<number> {
  const start = performance.now();
  try {
    await execFileAsync(cmd[0], cmd.slice(1));
  } catch {
    return -1; // CLI not available/authenticated
  }
  return performance.now() - start;
}

async function timeLocal(fn: () => Promise<void>): Promise<number> {
  const start = performance.now();
  await fn();
  return performance.now() - start;
}

function stats(times: number[]): Omit<TimingResult, "operation" | "path"> {
  const sorted = [...times].sort((a, b) => a - b);
  return {
    times_ms: sorted.map((t) => Math.round(t)),
    avg_ms: Math.round(times.reduce((a, b) => a + b, 0) / times.length),
    p50_ms: Math.round(sorted[Math.floor(sorted.length * 0.5)]),
    p95_ms: Math.round(sorted[Math.floor(sorted.length * 0.95)]),
    min_ms: Math.round(sorted[0]),
    max_ms: Math.round(sorted[sorted.length - 1]),
  };
}

async function benchmarkFind(): Promise<TimingResult[]> {
  console.log("Benchmarking: Find...");

  const cliTimes: number[] = [];
  const localTimes: number[] = [];

  for (let i = 0; i < ITERATIONS; i++) {
    const cliTime = await timeExec([CLI_PATH, "find", "--name", "revenue"]);
    if (cliTime >= 0) cliTimes.push(cliTime);

    localTimes.push(
      await timeLocal(async () => {
        const files = await readdir(DOCS_DIR);
        files.filter((f) => f.toLowerCase().includes("revenue"));
      })
    );
  }

  const results: TimingResult[] = [];
  if (cliTimes.length > 0) results.push({ operation: "find", path: "cli", ...stats(cliTimes) });
  results.push({ operation: "find", path: "local", ...stats(localTimes) });
  return results;
}

async function benchmarkRead(): Promise<TimingResult[]> {
  console.log("Benchmarking: Read...");

  const indexRaw = await readFile(join(DOCS_DIR, "index.json"), "utf-8");
  const index = JSON.parse(indexRaw);
  const firstDoc = index[0];

  const cliTimes: number[] = [];
  const localTimes: number[] = [];

  for (let i = 0; i < ITERATIONS; i++) {
    if (firstDoc.driveId) {
      const cliTime = await timeExec([CLI_PATH, "read", "--id", firstDoc.driveId]);
      if (cliTime >= 0) cliTimes.push(cliTime);
    }
    localTimes.push(
      await timeLocal(async () => {
        await readFile(join(DOCS_DIR, firstDoc.localPath), "utf-8");
      })
    );
  }

  const results: TimingResult[] = [];
  if (cliTimes.length > 0) results.push({ operation: "read", path: "cli", ...stats(cliTimes) });
  results.push({ operation: "read", path: "local", ...stats(localTimes) });
  return results;
}

async function benchmarkSearch(): Promise<TimingResult[]> {
  console.log("Benchmarking: Search...");

  const cliTimes: number[] = [];
  const localTimes: number[] = [];

  for (let i = 0; i < ITERATIONS; i++) {
    const cliTime = await timeExec([CLI_PATH, "search", "--query", "compliance deadline"]);
    if (cliTime >= 0) cliTimes.push(cliTime);

    localTimes.push(
      await timeLocal(async () => {
        const files = await readdir(DOCS_DIR);
        const mdFiles = files.filter((f) => f.endsWith(".md"));
        for (const f of mdFiles) {
          const content = await readFile(join(DOCS_DIR, f), "utf-8");
          content.includes("compliance deadline");
        }
      })
    );
  }

  const results: TimingResult[] = [];
  if (cliTimes.length > 0) results.push({ operation: "search", path: "cli", ...stats(cliTimes) });
  results.push({ operation: "search", path: "local", ...stats(localTimes) });
  return results;
}

async function main() {
  console.log(`Running raw benchmark (${ITERATIONS} iterations per operation)\n`);

  const results: TimingResult[] = [];

  results.push(...(await benchmarkFind()));
  results.push(...(await benchmarkRead()));
  results.push(...(await benchmarkSearch()));

  console.log("\n# Raw Benchmark Results\n");
  console.log("| Operation | Path | Avg (ms) | P50 (ms) | P95 (ms) | Min (ms) | Max (ms) |");
  console.log("|-----------|------|----------|----------|----------|----------|----------|");

  for (const r of results) {
    console.log(
      `| ${r.operation} | ${r.path} | ${r.avg_ms} | ${r.p50_ms} | ${r.p95_ms} | ${r.min_ms} | ${r.max_ms} |`
    );
  }

  // Speedup summary
  console.log("\n## Speedup (CLI / Local)\n");
  const operations = [...new Set(results.map((r) => r.operation))];
  for (const op of operations) {
    const cli = results.find((r) => r.operation === op && r.path === "cli");
    const local = results.find(
      (r) => r.operation === op && r.path === "local"
    );
    if (cli && local && local.avg_ms > 0) {
      console.log(
        `- **${op}**: ${(cli.avg_ms / local.avg_ms).toFixed(0)}x slower via CLI`
      );
    }
  }

  await mkdir(join(__dirname, "..", "benchmark", "results"), {
    recursive: true,
  });
  const outPath = join(
    __dirname,
    "..",
    "benchmark",
    "results",
    `raw-${new Date().toISOString().replace(/[:.]/g, "-")}.json`
  );
  await writeFile(outPath, JSON.stringify(results, null, 2));
  console.log(`\nResults saved to ${outPath}`);
}

main().catch(console.error);
