export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-[14px] bg-white/[0.05] ${className}`} />;
}
