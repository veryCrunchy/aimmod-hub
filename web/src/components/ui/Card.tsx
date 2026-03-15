import { useRef, type HTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export function Card({ className, children, onMouseMove, onMouseLeave, ...props }: HTMLAttributes<HTMLDivElement>) {
  const spotRef = useRef<HTMLDivElement>(null);

  function handleMouseMove(e: React.MouseEvent<HTMLDivElement>) {
    onMouseMove?.(e);
    if (!spotRef.current) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * 100;
    const y = ((e.clientY - rect.top) / rect.height) * 100;
    spotRef.current.style.background =
      `radial-gradient(circle at ${x}% ${y}%, rgba(121,201,151,0.05) 0%, transparent 72%)`;
    spotRef.current.style.opacity = "1";
  }

  function handleMouseLeave(e: React.MouseEvent<HTMLDivElement>) {
    onMouseLeave?.(e);
    if (spotRef.current) spotRef.current.style.opacity = "0";
  }

  return (
    <div
      {...props}
      onMouseMove={handleMouseMove}
      onMouseLeave={handleMouseLeave}
      className={cn(
        "relative min-h-0 min-w-0 rounded-[18px] border border-line bg-panel shadow-panel max-[720px]:rounded-[16px]",
        !className?.includes("overflow-") && "overflow-hidden",
        className
      )}
    >
      {children}
      <div
        ref={spotRef}
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-500"
      />
    </div>
  );
}
