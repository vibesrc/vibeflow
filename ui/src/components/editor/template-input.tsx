import { useRef, useCallback } from 'react';
import Editor, { type OnMount } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';
import { registerTheme } from './theme';
import { registerGoTemplateLanguage } from './gotemplate-language';

interface TemplateInputProps {
  value: string;
  onChange: (value: string) => void;
  className?: string;
  readOnly?: boolean;
  height?: number | string;
}

export function TemplateInput({
  value,
  onChange,
  className,
  readOnly = false,
  height = 120,
}: TemplateInputProps) {
  const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);

  const stopPropagation = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  const handleEditorDidMount: OnMount = useCallback((editor, monaco) => {
    editorRef.current = editor;
    registerTheme(monaco);
    registerGoTemplateLanguage(monaco);
    monaco.editor.setTheme('vibeflow-dark');
  }, []);

  const handleChange = useCallback((value: string | undefined) => {
    onChange(value ?? '');
  }, [onChange]);

  return (
    <div
      className={`rounded border border-border ${className ?? ''}`}
      onKeyDown={stopPropagation}
      onKeyUp={stopPropagation}
      onKeyPress={stopPropagation}
    >
      <Editor
        height={height}
        language="gotemplate"
        value={value}
        onChange={handleChange}
        onMount={handleEditorDidMount}
        theme="vibeflow-dark"
        options={{
          minimap: { enabled: false },
          fontSize: 13,
          fontFamily: "'JetBrains Mono', ui-monospace, monospace",
          lineNumbers: 'on',
          folding: false,
          wordWrap: 'on',
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          insertSpaces: true,
          readOnly,
          renderLineHighlight: 'line',
          cursorBlinking: 'smooth',
          smoothScrolling: true,
          padding: { top: 8, bottom: 8 },
          scrollbar: {
            vertical: 'auto',
            horizontal: 'hidden',
            verticalScrollbarSize: 8,
          },
          overviewRulerBorder: false,
          hideCursorInOverviewRuler: true,
          overviewRulerLanes: 0,
          glyphMargin: false,
          lineDecorationsWidth: 8,
          lineNumbersMinChars: 3,
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
