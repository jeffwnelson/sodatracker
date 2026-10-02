import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { configureBackup, downloadBackup, runBackup } from "../api/client";
import type { BackupStatus } from "../api/types";
import { useBackupStatus } from "../api/backup";
import { Card } from "../components/Card";
import { Input } from "../components/Input";
import { Select } from "../components/Select";
import { Button } from "../components/Button";

const INTERVALS = [
  { hours: 0, label: "Off" },
  { hours: 6, label: "Every 6 hours" },
  { hours: 12, label: "Every 12 hours" },
  { hours: 24, label: "Daily" },
  { hours: 168, label: "Weekly" },
];

function when(iso: string): string {
  return new Date(iso).toLocaleString();
}

function kb(bytes: number): string {
  return `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

export function BackupSection() {
  const queryClient = useQueryClient();
  const { data: status } = useBackupStatus();
  const [error, setError] = useState("");

  const onStatus = (s: BackupStatus) => {
    queryClient.setQueryData(["backup"], s);
    setError("");
  };
  const backupMut = useMutation({
    mutationFn: runBackup,
    onSuccess: onStatus,
    onError: (err: Error) => {
      setError(err.message);
      queryClient.invalidateQueries({ queryKey: ["backup"] });
    },
  });
  const downloadMut = useMutation({
    mutationFn: downloadBackup,
    onError: (err: Error) => setError(err.message),
  });

  const newest = status?.snapshots[0];

  return (
    <Card>
      <div className="flex items-center justify-between mb-4 gap-3">
        <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">
          Backups
        </p>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            onClick={() => downloadMut.mutate()}
            loading={downloadMut.isPending}
            className="text-xs px-3 py-1.5"
          >
            Download
          </Button>
          <Button
            onClick={() => backupMut.mutate()}
            loading={backupMut.isPending}
            className="text-xs px-3 py-1.5"
          >
            Back up now
          </Button>
        </div>
      </div>

      {status && (
        <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
          <div>
            <dt className="text-xs text-text-tertiary">Last pushed to GitHub</dt>
            <dd className="mt-0.5 text-sm font-medium text-text-primary">
              {!status.pushEnabled
                ? "Not set up: copies stay on the Pi's SD card"
                : status.lastPushAt
                  ? when(status.lastPushAt)
                  : "Not yet"}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-text-tertiary">On the Pi</dt>
            <dd className="mt-0.5 text-sm font-medium text-text-primary">
              {newest
                ? `${status.snapshots.length} snapshot${status.snapshots.length === 1 ? "" : "s"}, newest ${when(newest.createdAt)} (${kb(newest.size)})`
                : "None yet"}
            </dd>
          </div>
        </dl>
      )}

      {status?.lastError && (
        <p className="text-sm text-status-empty mb-4">
          Last backup failed
          {status.lastErrorAt ? ` (${when(status.lastErrorAt)})` : ""}:{" "}
          {status.lastError}
        </p>
      )}
      {error && error !== status?.lastError && (
        <p className="text-sm text-status-empty mb-4">{error}</p>
      )}

      {status && (
        <BackupSettingsForm
          // Remount after a save so the fields show what was stored.
          key={`${status.repo}|${status.tokenSet}|${status.intervalHours}|${status.keep}`}
          status={status}
          onSaved={onStatus}
        />
      )}
    </Card>
  );
}

function BackupSettingsForm({
  status,
  onSaved,
}: {
  status: BackupStatus;
  onSaved: (s: BackupStatus) => void;
}) {
  const [repo, setRepo] = useState(status.repo);
  // Blank means "keep the stored token": it is never sent back to the browser.
  const [token, setToken] = useState("");
  const [intervalHours, setIntervalHours] = useState(status.intervalHours);
  const [keep, setKeep] = useState(String(status.keep));
  const [error, setError] = useState("");

  const saveMut = useMutation({
    mutationFn: (clearToken: boolean) =>
      configureBackup({
        repo,
        intervalHours,
        keep: Number(keep),
        ...(clearToken ? { token: "" } : token ? { token } : {}),
      }),
    onSuccess: onSaved,
    onError: (err: Error) => setError(err.message),
  });

  const intervals = INTERVALS.some((i) => i.hours === intervalHours)
    ? INTERVALS
    : [...INTERVALS, { hours: intervalHours, label: `Every ${intervalHours} hours` }];

  return (
    <form
      className="border-t border-border-primary pt-4 space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        setError("");
        saveMut.mutate(false);
      }}
    >
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Input
          label="GitHub repo"
          placeholder="owner/sodatracker-backups"
          value={repo}
          onChange={(e) => setRepo(e.target.value)}
          autoComplete="off"
        />
        <div>
          <Input
            label="GitHub token"
            type="password"
            placeholder={status.tokenSet ? "Stored. Leave blank to keep it" : "github_pat_…"}
            value={token}
            onChange={(e) => setToken(e.target.value)}
            autoComplete="new-password"
          />
          {status.tokenSet && (
            <button
              type="button"
              className="mt-1 text-xs text-text-tertiary hover:text-status-empty"
              onClick={() => {
                setError("");
                saveMut.mutate(true);
              }}
            >
              Remove token
            </button>
          )}
        </div>
        <Select
          label="Snapshot schedule"
          value={intervalHours}
          onChange={(e) => setIntervalHours(Number(e.target.value))}
        >
          {intervals.map((i) => (
            <option key={i.hours} value={i.hours}>
              {i.label}
            </option>
          ))}
        </Select>
        <Input
          label="Snapshots to keep on the Pi"
          type="number"
          min={1}
          max={365}
          value={keep}
          onChange={(e) => setKeep(e.target.value)}
        />
      </div>
      <p className="text-xs text-text-tertiary">
        Use a private repo and a fine-grained token that can only reach it, with{" "}
        <span className="font-medium">Contents: read and write</span>. Snapshots go
        to <code>snapshots/&lt;year&gt;/</code> in the repo. They leave out the token,
        so after restoring one, enter the token here again.
      </p>
      {error && <p className="text-sm text-status-empty">{error}</p>}
      <div className="flex justify-end">
        <Button type="submit" loading={saveMut.isPending}>
          Save
        </Button>
      </div>
    </form>
  );
}
