import { readdir, readFile } from "fs/promises";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));

interface TaskResult {
  task_id: number;
  run: number;
  latency_ms: number;
  tool_calls: number;
  success: boolean;
  cli_elapsed_ms?: number;
  notes: string;
}

interface RunFile {
  timestamp: string;
  path: "cli" | "local";
  results: TaskResult[];
}

const TASK_NAMES: Record<number, string> = {
  1: "Find by Name",
  2: "Read Content",
  3: "Full-Text Search",
  4: "Create Document",
  5: "Update Document",
};

async function main() {
  const resultsDir = join(__dirname, "results");
  const files = await readdir(resultsDir);
  const jsonFiles = files.filter((f) => f.endsWith(".json"));

  if (jsonFiles.length === 0) {
    console.error("No result files found in benchmark/results/");
    process.exit(1);
  }

  const runs: RunFile[] = [];
  for (const f of jsonFiles) {
    const content = await readFile(join(resultsDir, f), "utf-8");
    runs.push(JSON.parse(content));
  }

  const cliResults = runs
    .filter((r) => r.path === "cli")
    .flatMap((r) => r.results);
  const localResults = runs
    .filter((r) => r.path === "local")
    .flatMap((r) => r.results);

  console.log("# Benchmark Results: CLI vs. Local Markdown\n");
  console.log(
    "| Task | CLI Avg (ms) | Local Avg (ms) | Speedup | CLI Tools | Local Tools | CLI Success | Local Success |"
  );
  console.log(
    "|------|-------------|----------------|---------|-----------|-------------|-------------|---------------|"
  );

  let totalCliLatency = 0;
  let totalLocalLatency = 0;
  let totalCliCalls = 0;
  let totalLocalCalls = 0;
  let cliSuccesses = 0;
  let localSuccesses = 0;
  let cliTotal = 0;
  let localTotal = 0;

  for (let taskId = 1; taskId <= 5; taskId++) {
    const cliTask = cliResults.filter((r) => r.task_id === taskId);
    const localTask = localResults.filter((r) => r.task_id === taskId);

    const cliAvgLatency = avg(cliTask.map((r) => r.latency_ms));
    const localAvgLatency = avg(localTask.map((r) => r.latency_ms));
    const speedup =
      localAvgLatency > 0 ? (cliAvgLatency / localAvgLatency).toFixed(1) : "-";

    const cliAvgCalls = avg(cliTask.map((r) => r.tool_calls));
    const localAvgCalls = avg(localTask.map((r) => r.tool_calls));

    const cliSR = cliTask.filter((r) => r.success).length;
    const localSR = localTask.filter((r) => r.success).length;

    totalCliLatency += cliAvgLatency;
    totalLocalLatency += localAvgLatency;
    totalCliCalls += cliAvgCalls;
    totalLocalCalls += localAvgCalls;
    cliSuccesses += cliSR;
    localSuccesses += localSR;
    cliTotal += cliTask.length;
    localTotal += localTask.length;

    console.log(
      `| ${TASK_NAMES[taskId]} | ${cliAvgLatency.toFixed(0)} | ${localAvgLatency.toFixed(0)} | ${speedup}x | ${cliAvgCalls.toFixed(1)} | ${localAvgCalls.toFixed(1)} | ${cliSR}/${cliTask.length} | ${localSR}/${localTask.length} |`
    );
  }

  console.log(
    `| **Total** | **${totalCliLatency.toFixed(0)}** | **${totalLocalLatency.toFixed(0)}** | **${(totalCliLatency / totalLocalLatency).toFixed(1)}x** | **${totalCliCalls.toFixed(1)}** | **${totalLocalCalls.toFixed(1)}** | **${cliSuccesses}/${cliTotal}** | **${localSuccesses}/${localTotal}** |`
  );

  // CLI self-timing breakdown
  const cliWithTiming = cliResults.filter(
    (r) => r.cli_elapsed_ms !== undefined
  );
  if (cliWithTiming.length > 0) {
    console.log("\n## CLI Self-Timing (API latency only)\n");
    for (let taskId = 1; taskId <= 5; taskId++) {
      const task = cliWithTiming.filter((r) => r.task_id === taskId);
      if (task.length > 0) {
        const apiAvg = avg(task.map((r) => r.cli_elapsed_ms!));
        const e2eAvg = avg(task.map((r) => r.latency_ms));
        const overhead = e2eAvg - apiAvg;
        console.log(
          `- **${TASK_NAMES[taskId]}**: API ${apiAvg.toFixed(0)}ms, Agent overhead ${overhead.toFixed(0)}ms (${((overhead / e2eAvg) * 100).toFixed(0)}%)`
        );
      }
    }
  }
}

function avg(nums: number[]): number {
  if (nums.length === 0) return 0;
  return nums.reduce((a, b) => a + b, 0) / nums.length;
}

main().catch(console.error);
