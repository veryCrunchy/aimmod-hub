import { useEffect, useRef, useState } from "react";

export function useAnimatedNumber(target: number | null | undefined, duration = 500): number | null {
  const [value, setValue] = useState<number | null>(target ?? null);
  const frameRef = useRef<number | null>(null);
  const valueRef = useRef<number | null>(target ?? null);

  useEffect(() => {
    const next = target ?? null;
    if (frameRef.current != null) {
      cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    }

    if (next == null || !Number.isFinite(next)) {
      valueRef.current = null;
      setValue(null);
      return;
    }

    const startValue = valueRef.current ?? next;
    if (Math.abs(startValue - next) < 0.001) {
      valueRef.current = next;
      setValue(next);
      return;
    }

    let startTime: number | null = null;
    const step = (timestamp: number) => {
      if (startTime == null) startTime = timestamp;
      const progress = Math.min((timestamp - startTime) / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = startValue + (next - startValue) * eased;
      valueRef.current = current;
      setValue(current);
      if (progress < 1) {
        frameRef.current = requestAnimationFrame(step);
      } else {
        valueRef.current = next;
        frameRef.current = null;
      }
    };

    frameRef.current = requestAnimationFrame(step);
    return () => {
      if (frameRef.current != null) {
        cancelAnimationFrame(frameRef.current);
        frameRef.current = null;
      }
    };
  }, [duration, target]);

  return value;
}
