import { useEffect, useRef, useState, useCallback } from 'react';

export type EventType =
  | 'flow.start'
  | 'flow.stop'
  | 'flow.error'
  | 'node.error'
  | 'debug'
  | 'variable';

export interface WSEvent {
  type: EventType;
  timestamp: string;
  flow_id?: string;
  node_id?: string;
  data?: unknown;
}

export interface DebugData {
  node_name: string;
  topic: string;
  payload: unknown;
  level: string;
}

export interface ProcessData {
  message_id: string;
  port: string;
  payload: unknown;
}

export interface ErrorData {
  message: string;
}

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error';

// Subscription categories that can be requested
export type SubscriptionType = 'debug' | 'errors' | 'variables' | 'lifecycle';

export interface Subscription {
  flow_id?: string;
  types?: SubscriptionType[];
}

interface UseWebSocketOptions {
  url?: string;
  reconnectDelay?: number;
  maxReconnectAttempts?: number;
  onEvent?: (event: WSEvent) => void;
}

interface UseWebSocketReturn {
  status: ConnectionStatus;
  events: WSEvent[];
  clearEvents: () => void;
  subscribe: (sub: Subscription) => void;
  unsubscribe: () => void;
  isSubscribed: boolean;
}

export function useWebSocket(options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const {
    url = `ws://${window.location.host}/api/ws`,
    reconnectDelay = 3000,
    maxReconnectAttempts = 10,
    onEvent,
  } = options;

  const [status, setStatus] = useState<ConnectionStatus>('disconnected');
  const [events, setEvents] = useState<WSEvent[]>([]);
  const [isSubscribed, setIsSubscribed] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onEventRef = useRef(onEvent);
  const pendingSubscriptionRef = useRef<Subscription | null>(null);

  // Keep callback ref updated
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  const subscribe = useCallback((sub: Subscription) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ subscribe: sub }));
      setIsSubscribed(true);
    } else {
      // Store for when connection is ready
      pendingSubscriptionRef.current = sub;
    }
  }, []);

  const unsubscribe = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ unsubscribe: true }));
    }
    setIsSubscribed(false);
    pendingSubscriptionRef.current = null;
  }, []);

  useEffect(() => {
    let mounted = true;

    const connect = () => {
      if (!mounted) return;

      setStatus('connecting');
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        if (!mounted) return;
        setStatus('connected');
        reconnectAttemptsRef.current = 0;

        // Send pending subscription if any
        if (pendingSubscriptionRef.current) {
          ws.send(JSON.stringify({ subscribe: pendingSubscriptionRef.current }));
          setIsSubscribed(true);
        }
      };

      ws.onmessage = (event) => {
        if (!mounted) return;

        // WebSocket batches messages with newlines
        const lines = event.data.split('\n');
        for (const line of lines) {
          if (!line.trim()) continue;

          try {
            const parsed: WSEvent = JSON.parse(line);
            setEvents((prev) => {
              // Keep last 1000 events to prevent memory bloat
              const next = [...prev, parsed];
              return next.slice(-1000);
            });

            // Call optional event handler
            onEventRef.current?.(parsed);
          } catch (err) {
            console.warn('Failed to parse WebSocket message:', err);
          }
        }
      };

      ws.onclose = () => {
        if (!mounted) return;
        setStatus('disconnected');
        setIsSubscribed(false);
        wsRef.current = null;

        // Attempt reconnect
        if (reconnectAttemptsRef.current < maxReconnectAttempts) {
          reconnectAttemptsRef.current++;
          reconnectTimeoutRef.current = setTimeout(connect, reconnectDelay);
        }
      };

      ws.onerror = () => {
        if (!mounted) return;
        setStatus('error');
      };
    };

    connect();

    return () => {
      mounted = false;
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [url, reconnectDelay, maxReconnectAttempts]);

  return { status, events, clearEvents, subscribe, unsubscribe, isSubscribed };
}

/**
 * Hook for subscribing to events for a specific flow.
 * Automatically subscribes when flowId changes and handles pause/resume.
 */
export function useFlowEvents(
  flowId: string | null,
  options?: {
    types?: SubscriptionType[];
    onEvent?: (event: WSEvent) => void;
  }
) {
  const { types, onEvent } = options ?? {};
  const [flowEvents, setFlowEvents] = useState<WSEvent[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const onEventRef = useRef(onEvent);

  // Keep callback ref updated
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const handleEvent = useCallback((event: WSEvent) => {
    setFlowEvents((prev) => {
      const next = [...prev, event];
      return next.slice(-500); // Keep last 500 events for this flow
    });
    // Call optional external handler
    onEventRef.current?.(event);
  }, []);

  const { status, clearEvents: clearAllEvents, subscribe, unsubscribe, isSubscribed } = useWebSocket({
    onEvent: handleEvent,
  });

  const clearEvents = useCallback(() => {
    setFlowEvents([]);
  }, []);

  // Clear events when flow changes
  useEffect(() => {
    setFlowEvents([]);
  }, [flowId]);

  // Subscribe/unsubscribe based on flowId and pause state
  useEffect(() => {
    if (flowId && !isPaused) {
      subscribe({ flow_id: flowId, types });
    } else if (!flowId) {
      unsubscribe();
    }
    // Note: don't clear events here - only on flow change
  }, [flowId, isPaused, types, subscribe, unsubscribe]);

  // Pause/resume functions
  const pause = useCallback(() => {
    setIsPaused(true);
    unsubscribe();
  }, [unsubscribe]);

  const resume = useCallback(() => {
    setIsPaused(false);
    if (flowId) {
      subscribe({ flow_id: flowId, types });
    }
  }, [flowId, types, subscribe]);

  return {
    status,
    events: flowEvents,
    clearEvents,
    clearAllEvents,
    isSubscribed,
    isPaused,
    pause,
    resume,
  };
}
