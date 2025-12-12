import { useCallback, useEffect, useRef, useState } from 'react';
import type { FlowDefinition } from '@/api';

interface HistoryState {
  past: FlowDefinition[];
  present: FlowDefinition | null;
  future: FlowDefinition[];
}

const MAX_HISTORY = 50;
const BATCH_DELAY = 100; // ms to wait for batching related changes

export function useFlowHistory(initialState: FlowDefinition | null) {
  const [history, setHistory] = useState<HistoryState>({
    past: [],
    present: initialState,
    future: [],
  });

  // Track if this is the initial load
  const initialized = useRef(false);

  // Batching refs - track the state before rapid changes started
  const batchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const batchStartState = useRef<FlowDefinition | null>(null);

  // Update present when initial state loads
  useEffect(() => {
    if (initialState && !initialized.current) {
      initialized.current = true;
      setHistory({
        past: [],
        present: initialState,
        future: [],
      });
    }
  }, [initialState]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (batchTimeoutRef.current) {
        clearTimeout(batchTimeoutRef.current);
      }
    };
  }, []);

  const set = useCallback((newState: FlowDefinition) => {
    setHistory((h) => {
      // Don't add to history if it's the same as present
      if (h.present && JSON.stringify(h.present) === JSON.stringify(newState)) {
        return h;
      }

      // Batching: if we're in a batch window, don't push to history yet
      // Just update present and keep the original batch start state
      if (batchTimeoutRef.current) {
        // Clear and reset the batch timeout
        clearTimeout(batchTimeoutRef.current);
        batchTimeoutRef.current = setTimeout(() => {
          // Batch window ended - push the batch start state to history
          setHistory((currentH) => {
            if (batchStartState.current && currentH.present) {
              const newPast = [...currentH.past, batchStartState.current].slice(-MAX_HISTORY);
              batchStartState.current = null;
              return {
                ...currentH,
                past: newPast,
              };
            }
            return currentH;
          });
          batchTimeoutRef.current = null;
        }, BATCH_DELAY);

        // Just update present without adding to history
        return {
          ...h,
          present: newState,
          future: [], // Clear future on new action
        };
      }

      // Start a new batch - save current state as batch start
      batchStartState.current = h.present;
      batchTimeoutRef.current = setTimeout(() => {
        // Batch window ended - push the batch start state to history
        setHistory((currentH) => {
          if (batchStartState.current && currentH.present) {
            const newPast = [...currentH.past, batchStartState.current].slice(-MAX_HISTORY);
            batchStartState.current = null;
            return {
              ...currentH,
              past: newPast,
            };
          }
          return currentH;
        });
        batchTimeoutRef.current = null;
      }, BATCH_DELAY);

      // Update present without immediately adding to history
      return {
        ...h,
        present: newState,
        future: [], // Clear future on new action
      };
    });
  }, []);

  const undo = useCallback(() => {
    setHistory((h) => {
      if (h.past.length === 0) return h;

      const previous = h.past[h.past.length - 1];
      const newPast = h.past.slice(0, -1);

      return {
        past: newPast,
        present: previous,
        future: h.present ? [h.present, ...h.future] : h.future,
      };
    });
  }, []);

  const redo = useCallback(() => {
    setHistory((h) => {
      if (h.future.length === 0) return h;

      const next = h.future[0];
      const newFuture = h.future.slice(1);

      return {
        past: h.present ? [...h.past, h.present] : h.past,
        present: next,
        future: newFuture,
      };
    });
  }, []);

  const reset = useCallback((newState: FlowDefinition | null) => {
    initialized.current = !!newState;
    setHistory({
      past: [],
      present: newState,
      future: [],
    });
  }, []);

  return {
    state: history.present,
    set,
    undo,
    redo,
    reset,
    canUndo: history.past.length > 0,
    canRedo: history.future.length > 0,
  };
}
