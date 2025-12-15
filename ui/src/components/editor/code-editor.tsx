import { useRef, useEffect, useCallback, useState } from 'react';
import Editor, { type OnMount } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';
import { Maximize2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { registerTheme } from './theme';

interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  language?: string;
  height?: string | number;
  minHeight?: string | number;
  readOnly?: boolean;
  placeholder?: string;
  className?: string;
  label?: string;
}

// Shared editor options
const getEditorOptions = (readOnly: boolean, compact = false): editor.IStandaloneEditorConstructionOptions => ({
  minimap: { enabled: !compact },
  fontSize: compact ? 12 : 14,
  fontFamily: "'JetBrains Mono', ui-monospace, monospace",
  lineNumbers: 'on',
  lineNumbersMinChars: 3,
  folding: true,
  wordWrap: 'on',
  scrollBeyondLastLine: false,
  automaticLayout: true,
  tabSize: 2,
  insertSpaces: true,
  readOnly,
  renderLineHighlight: 'line',
  cursorBlinking: 'smooth',
  cursorSmoothCaretAnimation: 'on',
  smoothScrolling: true,
  padding: { top: 8, bottom: 8 },
  scrollbar: {
    verticalScrollbarSize: 8,
    horizontalScrollbarSize: 8,
  },
  overviewRulerBorder: false,
  hideCursorInOverviewRuler: true,
  overviewRulerLanes: 0,
});

export function CodeEditor({
  value,
  onChange,
  language = 'javascript',
  height = 150,
  minHeight,
  readOnly = false,
  className,
  label,
}: CodeEditorProps) {
  const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);
  const [isExpanded, setIsExpanded] = useState(false);

  const handleEditorDidMount: OnMount = useCallback((editor, monaco) => {
    editorRef.current = editor;
    registerTheme(monaco);
    monaco.editor.setTheme('vibeflow-dark');

    // Configure JavaScript to allow top-level return (function body mode)
    if (language === 'javascript') {
      monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions({
        noSemanticValidation: false,
        noSyntaxValidation: false,
      });

      // Add msg type definition for autocomplete
      monaco.languages.typescript.javascriptDefaults.addExtraLib(
        `
        interface Message {
          id: string;
          payload: any;
          metadata: Record<string, string>;
          context: Record<string, any>;
        }
        declare const msg: Message;
        `,
        'vibeflow-types.d.ts'
      );
    }
  }, [language]);

  const handleChange = useCallback((value: string | undefined) => {
    onChange(value ?? '');
  }, [onChange]);

  // Update editor options when props change
  useEffect(() => {
    if (editorRef.current) {
      editorRef.current.updateOptions({ readOnly });
    }
  }, [readOnly]);

  // Stop keyboard events from propagating to parent components (e.g., React Flow)
  const stopPropagation = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  return (
    <>
      <div
        className={`relative rounded border border-border overflow-hidden w-full ${className ?? ''}`}
        style={{ minHeight }}
        onKeyDown={stopPropagation}
        onKeyUp={stopPropagation}
        onKeyPress={stopPropagation}
      >
        {/* Expand button */}
        <Button
          variant="ghost"
          size="icon"
          className="absolute top-1 right-1 z-10 h-6 w-6 bg-background/80 hover:bg-background"
          onClick={() => setIsExpanded(true)}
          title="Expand editor"
        >
          <Maximize2 className="h-3.5 w-3.5" />
        </Button>
        <Editor
          height={height}
          language={language}
          value={value}
          onChange={handleChange}
          onMount={handleEditorDidMount}
          theme="vibeflow-dark"
          options={getEditorOptions(readOnly, true)}
          loading={
            <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
              Loading editor...
            </div>
          }
        />
      </div>

      {/* Expanded modal editor */}
      <Dialog open={isExpanded} onOpenChange={setIsExpanded}>
        <DialogContent
          className="p-0 gap-0 overflow-hidden flex flex-col"
          style={{ width: '95vw', maxWidth: '1400px', height: '85vh' }}
          onEscapeKeyDown={(e) => e.preventDefault()}
          onInteractOutside={(e) => e.preventDefault()}
        >
          <DialogHeader className="px-4 py-3 border-b border-border flex-row items-center justify-between space-y-0 shrink-0">
            <DialogTitle className="text-sm font-medium">
              {label || 'Code Editor'}
              <span className="ml-2 text-xs text-muted-foreground font-normal">({language})</span>
            </DialogTitle>
          </DialogHeader>
          <div
            className="flex-1 min-h-0 w-full"
            onKeyDown={stopPropagation}
            onKeyUp={stopPropagation}
            onKeyPress={stopPropagation}
          >
            <Editor
              height="100%"
              language={language}
              value={value}
              onChange={handleChange}
              theme="vibeflow-dark"
              options={getEditorOptions(readOnly, false)}
              loading={
                <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
                  Loading editor...
                </div>
              }
            />
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

// Single-line code input with syntax highlighting
interface CodeInputProps {
  value: string;
  onChange: (value: string) => void;
  language?: string;
  placeholder?: string;
  className?: string;
  readOnly?: boolean;
}

export function CodeInput({
  value,
  onChange,
  language = 'javascript',
  className,
  readOnly = false,
}: CodeInputProps) {
  const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);

  // Stop keyboard events from propagating to parent components (e.g., React Flow)
  const stopPropagation = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  const handleEditorDidMount: OnMount = useCallback((editor, monaco) => {
    editorRef.current = editor;
    registerTheme(monaco);
    monaco.editor.setTheme('vibeflow-dark');
  }, []);

  const handleChange = useCallback((value: string | undefined) => {
    // Strip newlines for single-line mode
    const cleanValue = (value ?? '').replace(/[\r\n]/g, '');
    onChange(cleanValue);
  }, [onChange]);

  // Prevent Enter key from inserting newlines
  useEffect(() => {
    if (editorRef.current) {
      const disposable = editorRef.current.onKeyDown((e) => {
        if (e.keyCode === 3) { // Enter key
          e.preventDefault();
          e.stopPropagation();
        }
      });
      return () => disposable.dispose();
    }
  }, []);

  return (
    <div
      className={`rounded border border-border overflow-hidden ${className ?? ''}`}
      onKeyDown={stopPropagation}
      onKeyUp={stopPropagation}
      onKeyPress={stopPropagation}
    >
      <Editor
        height={32}
        language={language}
        value={value}
        onChange={handleChange}
        onMount={handleEditorDidMount}
        theme="vibeflow-dark"
        options={{
          minimap: { enabled: false },
          fontSize: 13,
          fontFamily: "'JetBrains Mono', ui-monospace, monospace",
          lineNumbers: 'off',
          folding: false,
          wordWrap: 'off',
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          insertSpaces: true,
          readOnly,
          renderLineHighlight: 'none',
          cursorBlinking: 'smooth',
          smoothScrolling: true,
          padding: { top: 6, bottom: 6 },
          scrollbar: {
            vertical: 'hidden',
            horizontal: 'hidden',
          },
          overviewRulerBorder: false,
          hideCursorInOverviewRuler: true,
          overviewRulerLanes: 0,
          glyphMargin: false,
          lineDecorationsWidth: 8,
          lineNumbersMinChars: 0,
        }}
        loading={
          <div className="flex items-center justify-center h-full text-muted-foreground text-xs">
            ...
          </div>
        }
      />
    </div>
  );
}
