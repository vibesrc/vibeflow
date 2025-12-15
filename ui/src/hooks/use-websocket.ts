import { useEffect, useRef, useState, useCallback } from 'react';

export type EventType =
  | 'flow.start'
  | 'flow.stop'
  | 'debug';

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
  subscribe: (pattern: string) => void;
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
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onEventRef = useRef(onEvent);

  // Keep callback ref updated
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  const subscribe = useCallback((pattern: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ subscribe: pattern }));
    }
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

  return { status, events, clearEvents, subscribe };
}

/**
 * Hook for subscribing to events for a specific flow.
 */
export function useFlowEvents(flowId: string | null) {
  const [flowEvents, setFlowEvents] = useState<WSEvent[]>([]);

  const handleEvent = useCallback(
    (event: WSEvent) => {
      if (flowId && event.flow_id === flowId) {
        setFlowEvents((prev) => {
          const next = [...prev, event];
          return next.slice(-500); // Keep last 500 events for this flow
        });
      }
    },
    [flowId]
  );

  const { status, clearEvents: clearAllEvents } = useWebSocket({
    onEvent: handleEvent,
  });

  const clearEvents = useCallback(() => {
    setFlowEvents([]);
  }, []);

  // Clear flow events when flow changes
  useEffect(() => {
    setFlowEvents([]);
  }, [flowId]);

  return { status, events: flowEvents, clearEvents, clearAllEvents };
}
