import { useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import { Check, X } from 'lucide-react';
import { Spinner } from '@/components/ui/spinner';

export type AsyncButtonState = 'idle' | 'loading' | 'success' | 'error';

interface UseAsyncButtonOptions {
  /** How long to show success/error state before returning to idle (ms) */
  feedbackDuration?: number;
  /** Minimum time to show loading state (ms) - prevents flash on fast operations */
  minLoadingDuration?: number;
  /** Labels for each state */
  labels?: {
    idle?: string;
    loading?: string;
    success?: string;
    error?: string;
  };
  /** Icons for each state (defaults provided) */
  icons?: {
    idle?: ReactNode;
    loading?: ReactNode;
    success?: ReactNode;
    error?: ReactNode;
  };
}

interface UseAsyncButtonReturn {
  /** Current state */
  state: AsyncButtonState;
  /** Whether the button should be disabled */
  isDisabled: boolean;
  /** Icon to render */
  icon: ReactNode;
  /** Label to render */
  label: string;
  /** Additional className for styling based on state */
  stateClassName: string;
  /** Call this to start the async operation */
  execute: <T>(asyncFn: () => Promise<T>) => Promise<T | undefined>;
  /** Manually set state (useful for React Query integration) */
  setState: (state: AsyncButtonState) => void;
  /** Reset to idle */
  reset: () => void;
}

/**
 * Hook for managing async button states with loading, success, and error feedback.
 *
 * @example
 * // With manual async function
 * const saveButton = useAsyncButton({
 *   labels: { idle: 'Save', loading: 'Saving...', success: 'Saved!' }
 * });
 *
 * <Button
 *   onClick={() => saveButton.execute(saveData)}
 *   disabled={saveButton.isDisabled}
 *   className={saveButton.stateClassName}
 * >
 *   {saveButton.icon}
 *   {saveButton.label}
 * </Button>
 *
 * @example
 * // With React Query mutation
 * const mutation = useMutation({ mutationFn: saveData });
 * const saveButton = useAsyncButton({ labels: { idle: 'Save' } });
 *
 * useEffect(() => {
 *   if (mutation.isPending) saveButton.setState('loading');
 *   else if (mutation.isSuccess) saveButton.setState('success');
 *   else if (mutation.isError) saveButton.setState('error');
 * }, [mutation.status]);
 */
export function useAsyncButton(options: UseAsyncButtonOptions = {}): UseAsyncButtonReturn {
  const {
    feedbackDuration = 2000,
    minLoadingDuration = 400,
    labels = {},
    icons = {},
  } = options;

  const [state, setState] = useState<AsyncButtonState>('idle');

  // Auto-reset after success/error
  useEffect(() => {
    if (state === 'success' || state === 'error') {
      const timer = setTimeout(() => setState('idle'), feedbackDuration);
      return () => clearTimeout(timer);
    }
  }, [state, feedbackDuration]);

  const execute = useCallback(async <T,>(asyncFn: () => Promise<T>): Promise<T | undefined> => {
    setState('loading');
    const startTime = Date.now();

    try {
      const result = await asyncFn();
      const elapsed = Date.now() - startTime;
      const remaining = minLoadingDuration - elapsed;

      if (remaining > 0) {
        await new Promise((resolve) => setTimeout(resolve, remaining));
      }
      setState('success');
      return result;
    } catch (err) {
      const elapsed = Date.now() - startTime;
      const remaining = minLoadingDuration - elapsed;

      if (remaining > 0) {
        await new Promise((resolve) => setTimeout(resolve, remaining));
      }
      setState('error');
      console.error('Async button error:', err);
      return undefined;
    }
  }, [minLoadingDuration]);

  const reset = useCallback(() => setState('idle'), []);

  // Default icons (shadcn Button handles spacing automatically)
  const defaultIcons = {
    idle: icons.idle ?? null,
    loading: icons.loading ?? <Spinner className="size-4" />,
    success: icons.success ?? <Check className="size-4" />,
    error: icons.error ?? <X className="size-4" />,
  };

  // Default labels
  const defaultLabels = {
    idle: labels.idle ?? 'Submit',
    loading: labels.loading ?? labels.idle ?? 'Loading...',
    success: labels.success ?? 'Done',
    error: labels.error ?? 'Failed',
  };

  // State-based styling
  const stateClassNames: Record<AsyncButtonState, string> = {
    idle: '',
    loading: '',
    success: 'bg-success/20 text-success border-success/30 hover:bg-success/20',
    error: 'bg-destructive/20 text-destructive border-destructive/30 hover:bg-destructive/20',
  };

  return {
    state,
    isDisabled: state === 'loading',
    icon: defaultIcons[state],
    label: defaultLabels[state],
    stateClassName: stateClassNames[state],
    execute,
    setState,
    reset,
  };
}

/**
 * Hook that syncs with a React Query mutation status.
 * Automatically updates button state based on mutation status.
 * Supports minimum loading duration to prevent flash on fast operations.
 */
export function useMutationButton(
  mutation: { isPending: boolean; isSuccess: boolean; isError: boolean; reset?: () => void },
  options: UseAsyncButtonOptions = {}
): Omit<UseAsyncButtonReturn, 'execute'> {
  const { minLoadingDuration = 0, ...restOptions } = options;
  const button = useAsyncButton(restOptions);
  const loadingStartRef = useRef<number | null>(null);
  const pendingStateRef = useRef<'success' | 'error' | null>(null);

  useEffect(() => {
    if (mutation.isPending) {
      // Started loading - record the time
      loadingStartRef.current = Date.now();
      pendingStateRef.current = null;
      button.setState('loading');
    } else if (mutation.isSuccess || mutation.isError) {
      const nextState = mutation.isSuccess ? 'success' : 'error';
      const loadingStart = loadingStartRef.current;

      if (loadingStart && minLoadingDuration > 0) {
        const elapsed = Date.now() - loadingStart;
        const remaining = minLoadingDuration - elapsed;

        if (remaining > 0) {
          // Need to wait before showing result
          pendingStateRef.current = nextState;
          const timer = setTimeout(() => {
            if (pendingStateRef.current === nextState) {
              button.setState(nextState);
              pendingStateRef.current = null;
            }
          }, remaining);
          return () => clearTimeout(timer);
        }
      }

      // No delay needed
      button.setState(nextState);
      loadingStartRef.current = null;
    }
  }, [mutation.isPending, mutation.isSuccess, mutation.isError, button, minLoadingDuration]);

  return button;
}
