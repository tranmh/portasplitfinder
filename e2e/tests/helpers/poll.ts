import { execFile } from "node:child_process";
import { promisify } from "node:util";
import path from "node:path";

const execFileAsync = promisify(execFile);

// Repo root is one level up from e2e/.
const REPO_ROOT = path.resolve(__dirname, "../../..");

/**
 * Run the poller once via docker compose.
 *
 * The mock München/Midea store listing is out-of-stock by default and flips
 * to in-stock when MOCK_TRANSITION=1. The poller only fires an availability
 * email on an out->in transition, so a deterministic test must:
 *   1) poll WITHOUT the flag  -> writes the listing as out-of-stock,
 *   2) poll WITH the flag     -> out->in transition -> sends the mail.
 *
 * Returns the number of mails the poll reported sending (`mails_sent`).
 */
async function runPollOnce(transition: boolean): Promise<number> {
  const args = ["compose", "exec", "-T"];
  if (transition) {
    args.push("-e", "MOCK_TRANSITION=1");
  }
  args.push("poller", "/app/poller", "--once");

  const { stdout, stderr } = await execFileAsync("docker", args, {
    cwd: REPO_ROOT,
    timeout: 60_000,
  });
  const out = `${stdout}\n${stderr}`;
  const m = out.match(/"mails_sent":\s*(\d+)/);
  return m ? Number(m[1]) : 0;
}

/** Reset the mock store listing to out-of-stock (baseline for a transition). */
export async function resetStockBaseline(): Promise<void> {
  await runPollOnce(false);
}

/**
 * Fire the out->in-stock transition poll. Returns reported `mails_sent`.
 * Call resetStockBaseline() first to guarantee the transition.
 */
export async function pollTransition(): Promise<number> {
  return runPollOnce(true);
}
