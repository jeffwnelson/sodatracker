import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { getMonthlyReview } from "../../api/client";
import { useMachine } from "../../api/machine";
import { Button } from "../../components/Button";
import { Card } from "../../components/Card";
import { Select } from "../../components/Select";
import { SkeletonCard } from "../../components/Skeleton";
import { timezone } from "./analytics";

function monthKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

/** This month and the twelve before it, newest first. */
function recentMonths(): { key: string; label: string }[] {
  const now = new Date();
  return Array.from({ length: 13 }, (_, i) => {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    const label = d.toLocaleDateString(undefined, { month: "long", year: "numeric" });
    return { key: monthKey(d), label: i === 0 ? `${label} (so far)` : label };
  });
}

/**
 * A month's stats written up as a Slack message: pick the month, copy, paste
 * into the channel. The server writes the text, in Slack's markup.
 */
export function ReviewPage() {
  const { machineId } = useMachine();
  const months = recentMonths();
  const [params, setParams] = useSearchParams();
  // Defaults to last month, the one a monthly post is about.
  const month = params.get("month") ?? months[1].key;
  const [copied, setCopied] = useState(false);

  const review = useQuery({
    queryKey: ["analytics", "review", machineId, month, timezone],
    queryFn: () => getMonthlyReview({ machineId, month, tz: timezone }),
    enabled: !!machineId,
  });

  const copy = async () => {
    if (!review.data) return;
    await navigator.clipboard.writeText(review.data.text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-3">
        <Select
          label="Month"
          value={month}
          onChange={(e) => {
            const next = new URLSearchParams(params);
            next.set("month", e.target.value);
            setParams(next, { replace: true });
          }}
          className="w-56"
        >
          {months.map((m) => (
            <option key={m.key} value={m.key}>
              {m.label}
            </option>
          ))}
        </Select>
        <Button onClick={copy} disabled={!review.data}>
          {copied ? "Copied" : "Copy for Slack"}
        </Button>
      </div>

      {review.isLoading ? (
        <SkeletonCard />
      ) : review.isError ? (
        <Card>
          <p className="text-sm text-status-empty">Couldn't load the review: {review.error.message}</p>
        </Card>
      ) : (
        <Card>
          {/* Shown as Slack will receive it: *bold* and _italic_ markup and all. */}
          <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-relaxed text-text-primary">
            {review.data?.text}
          </pre>
        </Card>
      )}
      <p className="text-xs text-text-tertiary">
        Slack turns the *asterisks* into bold when the message is sent. Weights, heights and
        planes are rough, for fun.
      </p>
    </div>
  );
}
