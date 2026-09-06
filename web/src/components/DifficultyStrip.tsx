import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

type DifficultyOption = { id: string; label: string; color: string };

export function DifficultyStrip({ options, selectedId, label, onSelect }: {
  options: DifficultyOption[];
  selectedId: string;
  label: string;
  onSelect: (id: string) => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const strip = useRef<HTMLDivElement>(null);
  const drag = useRef<{ pointerId: number; x: number; scroll: number; moved: boolean } | null>(null);
  const suppressClick = useRef(false);
  const [dragging, setDragging] = useState(false);
  const [edges, setEdges] = useState({ overflow: false, left: false, right: false });
  const ids = options.map(option => option.id).join(",");

  function measure() {
    const row = strip.current;
    if (!row || !root.current) return;
    setEdges({
      overflow: row.scrollWidth > root.current.clientWidth + 1,
      left: row.scrollLeft > 1,
      right: row.scrollLeft + row.clientWidth < row.scrollWidth - 1,
    });
  }

  function revealSelected() {
    const row = strip.current;
    const selected = row?.querySelector<HTMLButtonElement>('[aria-pressed="true"]');
    if (!row || !selected) return;
    const bounds = row.getBoundingClientRect();
    const item = selected.getBoundingClientRect();
    if (item.left < bounds.left + 4) row.scrollLeft -= bounds.left + 4 - item.left;
    else if (item.right > bounds.right - 4) row.scrollLeft += item.right - bounds.right + 4;
    measure();
  }

  useEffect(() => {
    const observer = new ResizeObserver(() => { revealSelected(); });
    if (root.current) observer.observe(root.current);
    if (strip.current) observer.observe(strip.current);
    return () => observer.disconnect();
  }, []);

  useEffect(() => { revealSelected(); }, [selectedId, ids]);

  function scroll(direction: number) {
    const row = strip.current;
    if (row) row.scrollLeft += direction * Math.max(28, row.clientWidth * .75);
    measure();
  }

  return <div className="pp-difficulty-strip" ref={root}>
    {edges.overflow && <button type="button" className="pp-diff-arrow" aria-label="Scroll to easier difficulties" disabled={!edges.left} onClick={() => scroll(-1)}><ChevronLeft size={14} /></button>}
    <div className="pp-difficulty-dots" ref={strip} role="group" aria-label={label} onScroll={measure}
      data-draggable={edges.overflow || undefined} data-dragging={dragging || undefined}
      onPointerDown={event => {
        suppressClick.current = false;
        if (event.pointerType !== "mouse" || event.button !== 0 || !edges.overflow) return;
        drag.current = { pointerId: event.pointerId, x: event.clientX, scroll: event.currentTarget.scrollLeft, moved: false };
      }}
      onPointerMove={event => {
        const gesture = drag.current;
        if (!gesture || gesture.pointerId !== event.pointerId) return;
        const distance = event.clientX - gesture.x;
        if (!gesture.moved && Math.abs(distance) < 5) return;
        if (!gesture.moved) {
          gesture.moved = true;
          suppressClick.current = true;
          event.currentTarget.setPointerCapture(event.pointerId);
          setDragging(true);
        }
        event.preventDefault();
        event.currentTarget.scrollLeft = gesture.scroll - distance;
      }}
      onPointerUp={() => { drag.current = null; setDragging(false); }}
      onPointerCancel={() => { drag.current = null; setDragging(false); }}
      onLostPointerCapture={() => { drag.current = null; setDragging(false); }}
      onPointerLeave={() => { if (!drag.current?.moved) drag.current = null; }}
      onClickCapture={event => {
        if (suppressClick.current && event.detail !== 0) {
          event.preventDefault(); event.stopPropagation(); suppressClick.current = false;
        }
      }}>
      {options.map((option, index) => <button type="button" key={option.id} aria-label={option.label} title={option.label} aria-pressed={option.id === selectedId} tabIndex={option.id === selectedId ? 0 : -1} onClick={() => onSelect(option.id)} onKeyDown={event => {
        const next = event.key === "ArrowRight" ? Math.min(index + 1, options.length - 1)
          : event.key === "ArrowLeft" ? Math.max(index - 1, 0)
          : event.key === "Home" ? 0 : event.key === "End" ? options.length - 1 : -1;
        if (next < 0) return;
        event.preventDefault();
        onSelect(options[next].id);
        strip.current?.querySelectorAll<HTMLButtonElement>("button")[next]?.focus({ preventScroll: true });
      }}><span style={{ backgroundColor: option.color }} /></button>)}
    </div>
    {edges.overflow && <button type="button" className="pp-diff-arrow" aria-label="Scroll to harder difficulties" disabled={!edges.right} onClick={() => scroll(1)}><ChevronRight size={14} /></button>}
  </div>;
}
