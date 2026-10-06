import { useEffect, useRef, useState } from "react";
import { TeamBilling } from "./TeamBilling";
import { errorText } from "../../api/product";
import { teamServiceApi, type TeamService } from "../../api/teamService";

export function TeamServiceStatus({ teamId }: { teamId: string }) {
  const [service, setService] = useState<TeamService>();
  const [error, setError] = useState("");
  const [reading, setReading] = useState(true);
  const [attempt, setAttempt] = useState(0);
  const generation = useRef(0);
  useEffect(() => {
    const current = ++generation.current;
    setService(undefined); setError(""); setReading(true);
    void teamServiceApi.read(teamId).then(value => {
      if (generation.current === current) setService(value);
    }).catch(cause => {
      if (generation.current === current) setError(errorText(cause));
    }).finally(() => {
      if (generation.current === current) setReading(false);
    });
    return () => { generation.current++; };
  }, [teamId, attempt]);
  return <section aria-label="Team service status">
    {reading && <p role="status">Checking team service access…</p>}
    {error && <p className="journey-error" role="alert">Service access could not be checked: {error}</p>}
    {service && <>
      <p role="status">{service.billingEnabled ? (service.status === "suspended" ? "Operator execution access is suspended" : "Operator execution access is active") : service.mode === "self-hosted" ? "Self-hosted team · no subscription required" : service.status === "suspended" ? "Managed execution is suspended" : "Managed execution is active"}</p>
      {service.status === "suspended" && <p>Keep reviewing and editing context, export your work, or stop owned workloads using your existing permissions. New agent work, checks and deployments are blocked. Ask the service operator to restore execution access.</p>}
      {service.reason && <p className="journey-muted">Operator note: {service.reason}</p>}
    </>}
    {service?.billingEnabled && <TeamBilling key={teamId} teamId={teamId} />}
    <button className="luci-button" disabled={reading} onClick={() => setAttempt(value => value + 1)}>Refresh service access</button>
  </section>;
}
