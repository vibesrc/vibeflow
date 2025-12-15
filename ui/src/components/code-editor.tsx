import { useRef, useEffect, useCallback, useState } from 'react';
import Editor, { loader, type Monaco, type OnMount } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';
import { Maximize2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

// Industrial dark theme matching the app theme
const VIBEFLOW_THEME: editor.IStandaloneThemeData = {
  base: 'vs-dark',
  inherit: true,
  rules: [
    // Comments
    { token: 'comment', foreground: '6b7280', fontStyle: 'italic' },
    // Strings
    { token: 'string', foreground: 'f59e0b' },
    // Numbers
    { token: 'number', foreground: '06b6d4' },
    // Keywords
    { token: 'keyword', foreground: 'f59e0b', fontStyle: 'bold' },
    // Identifiers
    { token: 'identifier', foreground: 'e5e7eb' },
    // Types
    { token: 'type', foreground: '06b6d4' },
    // Functions
    { token: 'function', foreground: '22d3ee' },
    // Variables
    { token: 'variable', foreground: 'e5e7eb' },
    // Operators
    { token: 'operator', foreground: 'f59e0b' },
    // Delimiters
    { token: 'delimiter', foreground: '9ca3af' },
    // Brackets
    { token: 'bracket', foreground: '9ca3af' },
  ],
  colors: {
    // Editor background - matches card bg
    'editor.background': '#1f2328',
    // Text foreground
    'editor.foreground': '#e5e7eb',
    // Line numbers
    'editorLineNumber.foreground': '#4b5563',
    'editorLineNumber.activeForeground': '#9ca3af',
    // Cursor
    'editorCursor.foreground': '#f59e0b',
    // Selection
    'editor.selectionBackground': '#f59e0b33',
    'editor.inactiveSelectionBackground': '#f59e0b1a',
    // Current line
    'editor.lineHighlightBackground': '#ffffff08',
    'editor.lineHighlightBorder': '#00000000',
    // Matching brackets
    'editorBracketMatch.background': '#f59e0b33',
    'editorBracketMatch.border': '#f59e0b',
    // Indent guides
    'editorIndentGuide.background': '#374151',
    'editorIndentGuide.activeBackground': '#4b5563',
    // Scrollbar
    'scrollbarSlider.background': '#4b556366',
    'scrollbarSlider.hoverBackground': '#6b728099',
    'scrollbarSlider.activeBackground': '#9ca3afcc',
    // Widget (autocomplete, etc)
    'editorWidget.background': '#1f2328',
    'editorWidget.border': '#374151',
    'editorSuggestWidget.background': '#1f2328',
    'editorSuggestWidget.border': '#374151',
    'editorSuggestWidget.selectedBackground': '#f59e0b33',
    'editorSuggestWidget.highlightForeground': '#f59e0b',
    // Minimap
    'minimap.background': '#1a1d21',
    // Gutter
    'editorGutter.background': '#1f2328',
    // Focus border
    'focusBorder': '#f59e0b',
  },
};

let themeRegistered = false;

function registerTheme(monaco: Monaco) {
  if (!themeRegistered) {
    monaco.editor.defineTheme('vibeflow-dark', VIBEFLOW_THEME);
    themeRegistered = true;
  }
}

// Configure Monaco before loading
loader.init().then((monaco) => {
  registerTheme(monaco);
});

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
