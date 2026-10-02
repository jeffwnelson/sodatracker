import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getSlackSettings, testSlack, updateSlackSettings } from "../api/client";
import type { SlackStatus, SlackUpdate } from "../api/types";
import { Card } from "../components/Card";
import { Input } from "../components/Input";
import { Button } from "../components/Button";
import { Badge } from "../components/Badge";

const COMMANDS: Record<SlackStatus["commands"], { label: string; tone: "ok" | "low" | "empty" | "neutral" }> = {
  off: { label: "Off", tone: "neutral" },
  connecting: { label: "Connecting", tone: "low" },
  connected: { label: "Connected", tone: "ok" },
  error: { label: "Can't connect", tone: "empty" },
};

export function SlackSection() {
  const queryClient = useQueryClient();
  // Polled while connecting, so the badge settles without a reload.
  const { data: status } = useQuery({
    queryKey: ["slack"],
    queryFn: getSlackSettings,
    refetchInterval: (q) => (q.state.data?.commands === "connecting" ? 2000 : false),
  });
  const [webhook, setWebhook] = useState("");
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const onStatus = (s: SlackStatus) => {
    queryClient.setQueryData(["slack"], s);
    setError("");
  };
  const saveMut = useMutation({
    mutationFn: (u: SlackUpdate) => updateSlackSettings(u),
    onSuccess: (s) => {
      onStatus(s);
      setWebhook("");
      setToken("");
      setNotice("Saved.");
    },
    onError: (err: Error) => setError(err.message),
  });
  const testMut = useMutation({
    mutationFn: testSlack,
    onSuccess: (s) => {
      onStatus(s);
      setNotice("Test message sent. Check the channel.");
    },
    onError: (err: Error) => setError(err.message),
  });

  if (!status) return null;
  const cmd = COMMANDS[status.commands];

  const save = (u: SlackUpdate) => {
    setError("");
    setNotice("");
    saveMut.mutate(u);
  };

  return (
    <Card>
      <div className="flex items-center justify-between mb-4 gap-3">
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
          Slack
        </p>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setError("");
            setNotice("");
            testMut.mutate();
          }}
          loading={testMut.isPending}
          disabled={!status.webhookSet}
        >
          Send test message
        </Button>
      </div>

      <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
        <div>
          <dt className="text-xs text-text-tertiary">Alerts</dt>
          <dd className="mt-0.5 text-sm font-medium text-text-primary">
            {status.webhookSet ? "Posting to the webhook's channel" : "Off: no webhook set"}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-text-tertiary">/soda commands</dt>
          <dd className="mt-1 flex flex-wrap items-center gap-2">
            <Badge variant={cmd.tone}>{cmd.label}</Badge>
            {status.commandsError && (
              <span className="text-xs text-status-empty">{status.commandsError}</span>
            )}
          </dd>
        </div>
      </dl>

      <form
        className="border-t border-border-primary pt-4 space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          const u: SlackUpdate = {};
          if (webhook) u.webhookUrl = webhook;
          if (token) u.appToken = token;
          if (!webhook && !token) {
            setError("Enter a webhook URL or an app token to change.");
            return;
          }
          save(u);
        }}
      >
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <Input
              label="Incoming webhook URL"
              type="password"
              placeholder={status.webhookSet ? "Stored. Leave blank to keep it" : "https://hooks.slack.com/services/…"}
              value={webhook}
              onChange={(e) => setWebhook(e.target.value)}
              autoComplete="off"
            />
            {status.webhookSet && (
              <Button
                variant="link-danger"
                size="sm"
                className="mt-1"
                onClick={() => save({ webhookUrl: "" })}
              >
                Remove webhook
              </Button>
            )}
          </div>
          <div>
            <Input
              label="App-level token"
              type="password"
              placeholder={status.appTokenSet ? "Stored. Leave blank to keep it" : "xapp-…"}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              autoComplete="off"
            />
            {status.appTokenSet && (
              <Button
                variant="link-danger"
                size="sm"
                className="mt-1"
                onClick={() => save({ appToken: "" })}
              >
                Remove token
              </Button>
            )}
          </div>
        </div>
        <p className="text-xs text-text-tertiary">
          The webhook is where stock, backup and not-registering alerts post. The
          app-level token (scope <code>connections:write</code>) lets the Slack app
          answer <code>/soda</code> over Socket Mode, which connects out from the
          Pi, so nothing has to be reachable from the internet. Both are stored
          on the Pi, never shown again, and left out of backups.
        </p>
        {error && <p className="text-sm text-status-empty">{error}</p>}
        <div className="flex items-center justify-end gap-3">
          {notice && !error && <span className="text-sm text-status-ok">{notice}</span>}
          <Button type="submit" loading={saveMut.isPending}>
            Save
          </Button>
        </div>
      </form>
    </Card>
  );
}
