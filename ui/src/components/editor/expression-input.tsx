import { useRef, useEffect, useCallback } from 'react';
import Editor, { type OnMount } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';
import { registerTheme } from './theme';
import { registerExprLanguage } from './expr-language';

interface ExpressionInputProps {
  value: string;
  onChange: (value: string) => void;
  className?: string;
  readOnly?: boolean;
}

export function ExpressionInput({
  value,
  onChange,
  className,
  readOnly = false,
}: ExpressionInputProps) {
  const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);

  const stopPropagation = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  const handleEditorDidMount: OnMount = useCallback((editor, monaco) => {
    editorRef.current = editor;
    registerTheme(monaco);
    registerExprLanguage(monaco);
    monaco.editor.setTheme('vibeflow-dark');
  }, []);

  const handleChange = useCallback((value: string | undefined) => {
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
      className={`rounded border border-border ${className ?? ''}`}
      onKeyDown={stopPropagation}
      onKeyUp={stopPropagation}
      onKeyPress={stopPropagation}
    >
      <Editor
        height={32}
        language="expr"
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
          quickSuggestions: true,
          suggestOnTriggerCharacters: true,
          acceptSuggestionOnEnter: 'smart',
          wordBasedSuggestions: 'off',
          fixedOverflowWidgets: true,
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
