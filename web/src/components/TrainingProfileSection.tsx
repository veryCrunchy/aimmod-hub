import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { PageSection } from "./ui/PageSection";
import { Button } from "./ui/Button";
import { Skeleton } from "./ui/Skeleton";
import { EmptyState } from "./ui/EmptyState";
import { fetchTraining, formatPracticeTime, setTrainingVisibility, timingChange, trainingActivityDays, trainingModes, type TrainingProfile } from "../lib/training";
import "./training-profile.css";

export function TrainingProfileSection({ handle, owner = false }: { handle: string; owner?: boolean }) {
  const [days, setDays] = useState(30), [mode, setMode] = useState("");
  const [data, setData] = useState<TrainingProfile | null>(null), [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0), [pending, setPending] = useState<string | null>(null), [saveError, setSaveError] = useState("");
  const [visibleCount, setVisibleCount] = useState(10);
  useEffect(() => {
    const controller = new AbortController(); setData(null); setError(""); setVisibleCount(10);
    void fetchTraining(handle, days, mode, owner, controller.signal).then(value => { if (!controller.signal.aborted) setData(value); }).catch(reason => { if (!controller.signal.aborted) setError(reason.message); });
    return () => controller.abort();
  }, [handle, days, mode, owner, attempt]);
  async function changeVisibility(id: string, visibility: "public" | "private") {
    setPending(id); setSaveError("");
    try { await setTrainingVisibility(id, visibility); setAttempt(value => value + 1); }
    catch (reason) { setSaveError(reason instanceof Error ? reason.message : "Could not update sharing."); }
    finally { setPending(null); }
  }
  return <PageSection className="training-profile" aria-label="osu! training">
    <div className="training-heading">
      <div><p className="training-eyebrow">osu! practice</p><h2>Training & progress</h2><p className="text-muted text-sm">{owner ? "Your completed sessions. Choose which ones appear on your public profile." : "Practice history, timing consistency and progress on matching drills."}</p></div>
      <div className="training-filters">
        <label>Period<select aria-label="Training period" value={days} onChange={event => setDays(Number(event.target.value))}><option value={30}>30 days</option><option value={90}>90 days</option><option value={365}>1 year</option></select></label>
        <label>Skill<select aria-label="Training skill" value={mode} onChange={event => setMode(event.target.value)}><option value="">All skills</option>{Object.entries(trainingModes).map(([key, label]) => <option value={key} key={key}>{label}</option>)}</select></label>
      </div>
    </div>
    {error ? <EmptyState title="Training unavailable" body={error}><Button onClick={() => setAttempt(value => value + 1)}>Try again</Button></EmptyState>
      : !data ? <div role="status" aria-label="Loading training"><Skeleton className="h-64" /></div>
      : data.sessions === 0 ? <EmptyState title={mode ? "No sessions for this skill" : "No shared practice yet"} body={owner ? "Link your account in the desktop app, then enable training sync in Settings → Account & sharing → Training. New completed sessions will appear here." : "Completed sessions shared from AimMod appear here. Try a longer period to see earlier practice."}>{mode && <Button onClick={() => setMode("")}>Show all skills</Button>}</EmptyState>
      : <>
        <div className="training-totals">{[["Sessions", data.sessions], ["Practice time", formatPracticeTime(data.seconds)], ["Active days", data.activeDays], ["Skills practised", data.skills.length]].map(([label, value]) => <div key={label}><span>{label}</span><strong>{value}</strong></div>)}</div>
        <div className="training-chart" role="img" aria-label={`Practice time by day: ${data.activity.map(day => `${day.date}: ${Math.round(day.seconds / 60)} minutes`).join(", ")}`}>
          <div className="training-subheading"><h3>Practice time</h3><span>Minutes per day · UTC</span></div>
          <ResponsiveContainer width="100%" height={170}><BarChart data={trainingActivityDays(data.activity, days).map(day => ({ ...day, minutes: Math.round(day.seconds / 6) / 10 }))} margin={{ top: 8, right: 10, left: -20, bottom: 0 }}>
            <CartesianGrid stroke="rgba(255,255,255,.07)" vertical={false} /><XAxis dataKey="date" tickFormatter={value => value.slice(5)} stroke="#9bafa8" tick={{ fontSize: 11 }} minTickGap={28} /><YAxis stroke="#9bafa8" tick={{ fontSize: 11 }} /><Tooltip contentStyle={{ background: "#10221b", border: "1px solid #345348", borderRadius: 10, color: "#edf8f3" }} /><Bar dataKey="minutes" name="Minutes" fill="#30d5a0" radius={[3, 3, 0, 0]} maxBarSize={28} />
          </BarChart></ResponsiveContainer>
        </div>
        <div className="training-subheading"><h3>Skills</h3><span>Clean runs: ≥95% accuracy, ≥95% targets hit, ≤25 ms timing spread</span></div>
        <div className="training-skills">{data.skills.map(skill => <article key={skill.mode} className="training-skill">
          <div className="training-subheading"><h4>{trainingModes[skill.mode]}</h4><span>{skill.sessions} runs</span></div>
          <div className="training-skill-metric"><strong>{skill.mode === "reaction" ? skill.medianResponseMs?.toFixed(0) ?? "—" : skill.cleanPeakNps?.toFixed(1) ?? "—"}</strong><span>{skill.mode === "reaction" ? <>ms<br />median response</> : <>notes/sec<br />clean peak pace</>}</span></div>
          <div className="training-skill-detail">{formatPracticeTime(skill.seconds)} practised{skill.mode !== "reaction" && <> · {skill.cleanSessions} clean runs{skill.medianSpreadMs != null && <> · {skill.medianSpreadMs.toFixed(1)} ms median spread</>}</>}</div>
          {skill.comparedSessions > 0 ? <div className="training-improvement"><strong>{skill.mode === "reaction" ? skill.responseChange == null ? "Response unavailable" : `${Math.abs(skill.responseChange).toFixed(0)} ms ${skill.responseChange <= 0 ? "faster" : "slower"}` : skill.accuracyChange == null ? "Accuracy unavailable" : `${skill.accuracyChange > 0 ? "+" : ""}${skill.accuracyChange.toFixed(1)} percentage points`}</strong>{skill.mode !== "reaction" && <span>{skill.spreadChange == null ? "Timing unavailable" : timingChange(skill.spreadChange)}</span>}<small>First 3 vs latest 3 · matching setup{skill.mode !== "reaction" && skill.comparisonBpm > 0 && ` · ${skill.comparisonBpm} BPM`}</small></div> : <div className="training-improvement"><span>Building a baseline</span><small>Complete 6 runs with the same fixed setup to compare progress.</small></div>}
        </article>)}</div>
        <p className="training-note">Clean peak pace needs at least 3 qualifying runs and does not measure sustained speed. Random layouts contribute to practice history, but aren’t used for before-and-after comparisons. Trainer progress does not establish improvement on ranked maps.</p>
        <div className="training-subheading"><h3>Recent sessions</h3><span>Showing {Math.min(visibleCount, data.recent.length)} of {data.recent.length} recent sessions</span></div>
        {saveError && <p role="alert">{saveError}</p>}
        <div className="training-table-wrap"><table className="training-table"><thead><tr><th>Drill</th><th>Played</th><th>Length</th><th>Accuracy</th><th>Timing spread</th><th>Targets hit</th>{owner && <th>Sharing</th>}</tr></thead><tbody>{data.recent.slice(0, visibleCount).map(session => <tr key={session.id}>
          <td><strong>{trainingModes[session.setup.mode]}</strong><small>{session.setup.musicSource === "installed" ? "Song tempo" : `${session.setup.bpm} BPM`} · {session.setup.randomized ? "Adaptive layout" : "Fixed setup"}</small></td>
          <td><time dateTime={session.completedAt}>{new Date(session.completedAt).toLocaleDateString(undefined, { month: "short", day: "numeric" })}</time></td><td>{Math.round(session.playedSeconds)}s</td>
          <td>{session.accuracy == null ? "—" : `${session.accuracy.toFixed(2)}%`}</td><td>{session.spreadMs == null ? "—" : `${session.spreadMs.toFixed(1)} ms`}</td><td>{session.hits}/{session.notes}</td>
          {owner && <td><button disabled={pending !== null} onClick={() => void changeVisibility(session.id, session.visibility === "public" ? "private" : "public")} aria-label={`${session.visibility === "public" ? "Hide" : "Share"} ${trainingModes[session.setup.mode]} session from ${new Date(session.completedAt).toLocaleString()}`}>{pending === session.id ? "Saving…" : session.visibility === "public" ? "Public · Hide" : "Private · Share"}</button></td>}
        </tr>)}</tbody></table></div>
        {data.recent.length > visibleCount && <Button className="mt-4" onClick={() => setVisibleCount(value => value + 10)}>Show more sessions</Button>}
        {data.truncated && <p role="status" className="training-note">Showing the latest 5,000 sessions in this period. Totals and progress cover this selection. Choose a shorter period for complete coverage.</p>}
      </>}
    {!owner && <Link className="training-manage" to="/osu/training">Manage your training & sharing →</Link>}
  </PageSection>;
}
