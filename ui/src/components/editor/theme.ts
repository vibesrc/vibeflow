import { loader, type Monaco } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';

// Industrial dark theme matching the app theme
export const VIBEFLOW_THEME: editor.IStandaloneThemeData = {
  base: 'vs-dark',
  inherit: true,
  rules: [
    // Comments
    { token: 'comment', foreground: '6b7280', fontStyle: 'italic' },
    // Strings
    { token: 'string', foreground: 'f59e0b' },
    { token: 'string.template', foreground: 'fbbf24' },
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
    { token: 'variable', foreground: 'a5b4fc' },
    // Predefined variables (msg, payload, flow, env)
    { token: 'variable.predefined', foreground: 'c084fc', fontStyle: 'bold' },
    // Operators
    { token: 'operator', foreground: 'f59e0b' },
    // Delimiters
    { token: 'delimiter', foreground: '9ca3af' },
    { token: 'delimiter.bracket', foreground: 'fbbf24' },
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

export function registerTheme(monaco: Monaco) {
  if (!themeRegistered) {
    monaco.editor.defineTheme('vibeflow-dark', VIBEFLOW_THEME);
    themeRegistered = true;
  }
}

// Configure Monaco before loading
loader.init().then((monaco) => {
  registerTheme(monaco);
});
