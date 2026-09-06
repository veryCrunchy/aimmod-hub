import type { ReactNode } from "react";
import "./browse.css";

export function BrowseHeader({ title, description, actions }: { title: string; description?: string; actions?: ReactNode }) {
  return <header className="browse-heading"><div><h1>{title}</h1>{description && <p>{description}</p>}</div>{actions && <div className="browse-heading-actions">{actions}</div>}</header>;
}

export function PlayerIdentity({ name, avatar, detail, heading = false }: { name: string; avatar?: string; detail?: string; heading?: boolean }) {
  return <div className={`player-identity${heading ? " player-identity-large" : ""}`}>
    {avatar ? <img src={avatar} alt="" loading="lazy" /> : <span className="player-avatar-fallback" aria-hidden="true">{name.slice(0, 1).toUpperCase()}</span>}
    <div>{heading ? <h1>{name}</h1> : <strong>{name}</strong>}{detail && <p>{detail}</p>}</div>
  </div>;
}

export function MetricStrip({ metrics }: { metrics: { label: string; value: string }[] }) {
  return <dl className="browse-metrics">{metrics.map(metric => <div key={metric.label}><dt>{metric.label}</dt><dd>{metric.value}</dd></div>)}</dl>;
}
