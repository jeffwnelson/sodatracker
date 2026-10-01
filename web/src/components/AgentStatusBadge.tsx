import type { AgentStatus } from "../api/types";
import { Badge } from "./Badge";

function timeAgo(iso: string): string {
  const s = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  if (s < 60) return `${s}s ago`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m} min ago`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h}h ago`;
  return `${Math.round(h / 24)}d ago`;
}

/**
 * Whether the machine's Pi agent is heartbeating. Offline means dispenses
 * aren't being counted, which otherwise looks the same as nobody taking a soda.
 */
export function AgentStatusBadge({ agent }: { agent?: AgentStatus }) {
  if (!agent) return null;
  const title = agent.lastSeen
    ? `Last heartbeat ${new Date(agent.lastSeen).toLocaleString()}`
    : "No heartbeat since the server started";
  return (
    <span title={title}>
      {agent.online ? (
        <Badge variant="ok">Pi agent online</Badge>
      ) : agent.lastSeen ? (
        <Badge variant="empty">
          Pi agent offline · {timeAgo(agent.lastSeen)}
        </Badge>
      ) : (
        <Badge variant="neutral">Waiting for Pi agent</Badge>
      )}
    </span>
  );
}
