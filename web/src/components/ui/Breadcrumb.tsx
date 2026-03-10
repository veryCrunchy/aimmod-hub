import { Link } from "react-router-dom";

type Crumb = { label: string; to?: string };

type Props = {
  crumbs: Crumb[];
};

export function Breadcrumb({ crumbs }: Props) {
  return (
    <nav className="mb-3 flex flex-wrap items-center gap-1 text-[11px] text-muted-2" aria-label="Breadcrumb">
      {crumbs.map((crumb, i) => (
        <span key={i} className="flex items-center gap-1">
          {i > 0 && <span className="select-none opacity-40">/</span>}
          {crumb.to ? (
            <Link to={crumb.to} className="transition-colors hover:text-text">
              {crumb.label}
            </Link>
          ) : (
            <span className="text-muted">{crumb.label}</span>
          )}
        </span>
      ))}
    </nav>
  );
}
