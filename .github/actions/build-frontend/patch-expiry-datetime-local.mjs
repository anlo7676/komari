import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const target = resolve(
  process.argv[2] ?? "komari-web/src/pages/admin/index.tsx",
);
let source = readFileSync(target, "utf8");
const lineEnding = source.includes("\r\n") ? "\r\n" : "\n";
source = source.replaceAll("\r\n", "\n");

function replaceExactlyOnce(label, before, after) {
  const first = source.indexOf(before);
  const second = first === -1 ? -1 : source.indexOf(before, first + before.length);
  if (first === -1 || second !== -1) {
    throw new Error(`Unable to apply ${label}: expected exactly one match`);
  }
  source = source.slice(0, first) + after + source.slice(first + before.length);
}

const billingButtonStart =
  "function BillingButton({ node }: { node: NodeDetail }) {";
replaceExactlyOnce(
  "local datetime formatter",
  billingButtonStart,
  `function toLocalDateTimeInputValue(value?: string | Date) {
  const date = value ? new Date(value) : new Date();
  if (Number.isNaN(date.getTime()) || date.getUTCFullYear() <= 1) {
    return toLocalDateTimeInputValue();
  }

  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

${billingButtonStart}`,
);

replaceExactlyOnce(
  "expiration serialization",
  '? new Date(`${expiredAtValue}T00:00:00Z`).toISOString()',
  "? new Date(expiredAtValue).toISOString()",
);

replaceExactlyOnce(
  "expiration input",
  `              defaultValue={
                node.expired_at
                  ? new Date(node.expired_at).toISOString().slice(0, 10)
                  : "0001-01-01"
              }
              type="date"`,
  `              defaultValue={toLocalDateTimeInputValue(node.expired_at)}
              type="datetime-local"
              step={60}`,
);

replaceExactlyOnce(
  "long-term expiration",
  "dateInput.value = futureDate.toISOString().slice(0, 10);",
  "dateInput.value = toLocalDateTimeInputValue(futureDate);",
);

writeFileSync(
  target,
  lineEnding === "\r\n" ? source.replaceAll("\n", "\r\n") : source,
  "utf8",
);
