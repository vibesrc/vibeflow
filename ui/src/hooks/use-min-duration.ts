import { useState, useEffect, useRef } from 'react';

/**
 * Extends a boolean state to last for a minimum duration.
 * Useful for preventing loading spinners from flashing too quickly.
 *
 * @example
 * const isLoading = useMinDuration(mutation.isPending, 400);
 * {isLoading ? <Spinner /> : <Icon />}
 */
export function useMinDuration(value: boolean, minDuration = 400): boolean {
  const [extended, setExtended] = useState(value);
  const startTimeRef = useRef<number | null>(null);

  useEffect(() => {
    if (value) {
      // Started - record time and set true
      startTimeRef.current = Date.now();
      setExtended(true);
    } else if (startTimeRef.current !== null) {
      // Ended - check if we need to extend
      const elapsed = Date.now() - startTimeRef.current;
      const remaining = minDuration - elapsed;

      if (remaining > 0) {
        const timer = setTimeout(() => {
          setExtended(false);
          startTimeRef.current = null;
        }, remaining);
        return () => clearTimeout(timer);
      } else {
        setExtended(false);
        startTimeRef.current = null;
      }
    }
  }, [value, minDuration]);

  return extended;
}
