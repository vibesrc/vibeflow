import type { Monaco } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';

let registered = false;

export function registerGoTemplateLanguage(monaco: Monaco) {
  if (registered) return;
  registered = true;

  // Register a custom language for Go templates
  monaco.languages.register({ id: 'gotemplate' });

  // Tokenization for syntax highlighting
  monaco.languages.setMonarchTokensProvider('gotemplate', {
    defaultToken: 'string',
    tokenizer: {
      root: [
        // Template delimiters and content
        [/\{\{-?/, { token: 'delimiter.bracket', next: '@template' }],
        // Everything else is plain text
        [/[^{]+/, 'string'],
        [/\{/, 'string'],
      ],
      template: [
        // Close delimiter
        [/-?\}\}/, { token: 'delimiter.bracket', next: '@root' }],
        // Comments
        [/\/\*/, 'comment', '@comment'],
        // Strings
        [/"[^"]*"/, 'string.template'],
        [/`[^`]*`/, 'string.template'],
        // Numbers
        [/\d+\.?\d*/, 'number'],
        // Keywords/actions
        [/\b(if|else|end|range|with|define|template|block|nil|true|false)\b/, 'keyword'],
        // Pipeline operator
        [/\|/, 'operator'],
        // Comparison operators
        [/\b(eq|ne|lt|le|gt|ge|and|or|not)\b/, 'keyword'],
        // Built-in functions
        [/\b(print|printf|println|len|index|slice|html|js|urlquery|call)\b/, 'function'],
        // Variables starting with $ or .
        [/\$[a-zA-Z_]\w*/, 'variable'],
        [/\.[a-zA-Z_]\w*/, 'variable'],
        [/\./, 'variable'],
        // Identifiers
        [/[a-zA-Z_]\w*/, 'identifier'],
        // Whitespace
        [/\s+/, 'white'],
      ],
      comment: [
        [/\*\//, 'comment', '@pop'],
        [/./, 'comment'],
      ],
    },
  });

  // Completion provider
  monaco.languages.registerCompletionItemProvider('gotemplate', {
    triggerCharacters: ['.', '{', '|', ' '],
    provideCompletionItems: (model: editor.ITextModel, position: { lineNumber: number; column: number }) => {
      const word = model.getWordUntilPosition(position);
      const range = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: word.startColumn,
        endColumn: word.endColumn,
      };

      const suggestions: Array<{
        label: string;
        kind: number;
        insertText: string;
        insertTextRules?: number;
        detail?: string;
        documentation?: string;
        range: typeof range;
      }> = [];

      // Template actions
      suggestions.push(
        { label: '{{ }}', kind: 14, insertText: '{{ ${1} }}', insertTextRules: 4, detail: 'Template expression', range },
        { label: '{{if}}', kind: 14, insertText: '{{if ${1:condition}}}\n${2}\n{{end}}', insertTextRules: 4, detail: 'If block', range },
        { label: '{{if else}}', kind: 14, insertText: '{{if ${1:condition}}}\n${2}\n{{else}}\n${3}\n{{end}}', insertTextRules: 4, detail: 'If-else block', range },
        { label: '{{range}}', kind: 14, insertText: '{{range ${1:.Items}}}\n${2}\n{{end}}', insertTextRules: 4, detail: 'Range loop', range },
        { label: '{{with}}', kind: 14, insertText: '{{with ${1:.Field}}}\n${2}\n{{end}}', insertTextRules: 4, detail: 'With block', range },
        { label: '{{define}}', kind: 14, insertText: '{{define "${1:name}"}}\n${2}\n{{end}}', insertTextRules: 4, detail: 'Define template', range },
        { label: '{{template}}', kind: 14, insertText: '{{template "${1:name}" ${2:.}}}', insertTextRules: 4, detail: 'Include template', range },
      );

      // Message variables
      suggestions.push(
        { label: '.', kind: 5, insertText: '.', detail: 'Current context (message)', range },
        { label: '.Payload', kind: 5, insertText: '.Payload', detail: 'Message payload', range },
        { label: '.Metadata', kind: 5, insertText: '.Metadata', detail: 'Message metadata', range },
        { label: '.Context', kind: 5, insertText: '.Context', detail: 'Message context', range },
        { label: '.ID', kind: 5, insertText: '.ID', detail: 'Message ID', range },
      );

      // Comparison functions
      suggestions.push(
        { label: 'eq', kind: 1, insertText: 'eq ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Equal', documentation: 'Returns true if arg1 == arg2', range },
        { label: 'ne', kind: 1, insertText: 'ne ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Not equal', range },
        { label: 'lt', kind: 1, insertText: 'lt ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Less than', range },
        { label: 'le', kind: 1, insertText: 'le ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Less or equal', range },
        { label: 'gt', kind: 1, insertText: 'gt ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Greater than', range },
        { label: 'ge', kind: 1, insertText: 'ge ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Greater or equal', range },
        { label: 'and', kind: 1, insertText: 'and ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Logical AND', range },
        { label: 'or', kind: 1, insertText: 'or ${1:arg1} ${2:arg2}', insertTextRules: 4, detail: 'Logical OR', range },
        { label: 'not', kind: 1, insertText: 'not ${1:arg}', insertTextRules: 4, detail: 'Logical NOT', range },
      );

      // Built-in functions
      suggestions.push(
        { label: 'print', kind: 1, insertText: 'print ${1:args}', insertTextRules: 4, detail: 'Print values', range },
        { label: 'printf', kind: 1, insertText: 'printf "${1:%s}" ${2:args}', insertTextRules: 4, detail: 'Formatted print', range },
        { label: 'println', kind: 1, insertText: 'println ${1:args}', insertTextRules: 4, detail: 'Print with newline', range },
        { label: 'len', kind: 1, insertText: 'len ${1:value}', insertTextRules: 4, detail: 'Length of array/string/map', range },
        { label: 'index', kind: 1, insertText: 'index ${1:collection} ${2:key}', insertTextRules: 4, detail: 'Index into collection', range },
        { label: 'slice', kind: 1, insertText: 'slice ${1:collection} ${2:start} ${3:end}', insertTextRules: 4, detail: 'Slice collection', range },
        { label: 'html', kind: 1, insertText: 'html ${1:value}', insertTextRules: 4, detail: 'HTML escape', range },
        { label: 'js', kind: 1, insertText: 'js ${1:value}', insertTextRules: 4, detail: 'JS escape', range },
        { label: 'urlquery', kind: 1, insertText: 'urlquery ${1:value}', insertTextRules: 4, detail: 'URL query escape', range },
      );

      // Keywords
      suggestions.push(
        { label: 'if', kind: 13, insertText: 'if ', detail: 'Conditional', range },
        { label: 'else', kind: 13, insertText: 'else', detail: 'Else branch', range },
        { label: 'end', kind: 13, insertText: 'end', detail: 'End block', range },
        { label: 'range', kind: 13, insertText: 'range ', detail: 'Iterate', range },
        { label: 'with', kind: 13, insertText: 'with ', detail: 'Change context', range },
        { label: 'nil', kind: 13, insertText: 'nil', detail: 'Nil value', range },
        { label: 'true', kind: 13, insertText: 'true', detail: 'Boolean true', range },
        { label: 'false', kind: 13, insertText: 'false', detail: 'Boolean false', range },
      );

      return { suggestions };
    },
  });
}
