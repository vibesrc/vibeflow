import type { Monaco } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';

let registered = false;

export function registerExprLanguage(monaco: Monaco) {
  if (registered) return;
  registered = true;

  // Register a custom language for expr
  monaco.languages.register({ id: 'expr' });

  // Tokenization for syntax highlighting
  monaco.languages.setMonarchTokensProvider('expr', {
    defaultToken: 'identifier',
    tokenizer: {
      root: [
        // Strings
        [/"[^"]*"/, 'string'],
        [/'[^']*'/, 'string'],
        // Numbers
        [/\d+\.?\d*/, 'number'],
        // Booleans
        [/\b(true|false|nil)\b/, 'keyword'],
        // Operators (word-based)
        [/\b(in|not|and|or|matches|contains|startsWith|endsWith)\b/, 'keyword'],
        // Operators (symbol-based)
        [/&&|\|\||\?\?|\.\./, 'operator'],
        [/[<>=!]+/, 'operator'],
        [/[+\-*/%?:]/, 'operator'],
        // Built-in functions
        [/\b(len|all|any|one|none|map|filter|count|sum|min|max|first|last|take|keys|values|sort|reverse|uniq|flatten|toJSON|fromJSON|trim|upper|lower|split|join|replace|now|duration|date)\b/, 'function'],
        // Global objects (msg, flow, env)
        [/\b(msg|flow|env)\b/, { token: 'variable.predefined', next: '@property' }],
        // Payload shorthand
        [/\b(payload|value)\b/, { token: 'variable.predefined', next: '@property' }],
        // Lambda placeholder
        [/#/, 'variable.predefined'],
        // Regular identifiers
        [/[a-zA-Z_]\w*/, 'identifier'],
        // Delimiters
        [/[{}()\[\]]/, 'bracket'],
        [/[,]/, 'delimiter'],
        [/\./, 'delimiter'],
      ],
      property: [
        // Property access chain - continue in property state
        [/\./, 'delimiter'],
        // Known msg properties
        [/\b(payload|metadata|context|id)\b/, 'variable'],
        // Any other property
        [/[a-zA-Z_]\w*/, 'variable'],
        // Exit property state on anything else (whitespace, operators, etc)
        [/./, { token: '@rematch', next: '@pop' }],
      ],
    },
  });

  // Completion provider
  monaco.languages.registerCompletionItemProvider('expr', {
    triggerCharacters: ['.', '('],
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

      // Variables
      suggestions.push(
        // Message object
        { label: 'msg', kind: 5, insertText: 'msg', detail: 'Full message object', range },
        { label: 'msg.payload', kind: 5, insertText: 'msg.payload', detail: 'Message payload', range },
        { label: 'msg.metadata', kind: 5, insertText: 'msg.metadata', detail: 'Message metadata', range },
        { label: 'msg.context', kind: 5, insertText: 'msg.context', detail: 'Message context', range },
        { label: 'msg.id', kind: 5, insertText: 'msg.id', detail: 'Message ID', range },
        // Shorthand
        { label: 'payload', kind: 5, insertText: 'payload', detail: 'Message payload (shorthand)', range },
        { label: 'value', kind: 5, insertText: 'value', detail: 'Extracted property value (switch)', range },
        // Flow context
        { label: 'flow', kind: 5, insertText: 'flow', detail: 'Flow-level context', range },
        // Environment
        { label: 'env', kind: 5, insertText: 'env', detail: 'Environment variables', range },
        // Lambda placeholder
        { label: '#', kind: 5, insertText: '#', detail: 'Current element in lambda', documentation: 'filter(items, # > 10)', range },
      );

      // Operators
      suggestions.push(
        { label: 'in', kind: 13, insertText: 'in ', detail: 'Check if value in array', documentation: 'value in [1, 2, 3]', range },
        { label: 'not in', kind: 13, insertText: 'not in ', detail: 'Check if value not in array', range },
        { label: 'contains', kind: 13, insertText: 'contains ', detail: 'Check if string/array contains', documentation: '"hello" contains "ell"', range },
        { label: 'startsWith', kind: 13, insertText: 'startsWith ', detail: 'Check string prefix', range },
        { label: 'endsWith', kind: 13, insertText: 'endsWith ', detail: 'Check string suffix', range },
        { label: 'matches', kind: 13, insertText: 'matches ', detail: 'Regex match', documentation: '"test" matches "^t.*"', range },
      );

      // Functions
      suggestions.push(
        { label: 'len', kind: 1, insertText: 'len(${1:value})', insertTextRules: 4, detail: 'Length of string/array', range },
        { label: 'all', kind: 1, insertText: 'all(${1:array}, ${2:# > 0})', insertTextRules: 4, detail: 'All elements match predicate', range },
        { label: 'any', kind: 1, insertText: 'any(${1:array}, ${2:# > 0})', insertTextRules: 4, detail: 'Any element matches predicate', range },
        { label: 'filter', kind: 1, insertText: 'filter(${1:array}, ${2:# > 0})', insertTextRules: 4, detail: 'Filter array by predicate', range },
        { label: 'map', kind: 1, insertText: 'map(${1:array}, ${2:# * 2})', insertTextRules: 4, detail: 'Transform array elements', range },
        { label: 'count', kind: 1, insertText: 'count(${1:array}, ${2:# > 0})', insertTextRules: 4, detail: 'Count matching elements', range },
        { label: 'sum', kind: 1, insertText: 'sum(${1:array})', insertTextRules: 4, detail: 'Sum of array', range },
        { label: 'min', kind: 1, insertText: 'min(${1:array})', insertTextRules: 4, detail: 'Minimum value', range },
        { label: 'max', kind: 1, insertText: 'max(${1:array})', insertTextRules: 4, detail: 'Maximum value', range },
        { label: 'first', kind: 1, insertText: 'first(${1:array})', insertTextRules: 4, detail: 'First element', range },
        { label: 'last', kind: 1, insertText: 'last(${1:array})', insertTextRules: 4, detail: 'Last element', range },
        { label: 'keys', kind: 1, insertText: 'keys(${1:map})', insertTextRules: 4, detail: 'Get map keys', range },
        { label: 'values', kind: 1, insertText: 'values(${1:map})', insertTextRules: 4, detail: 'Get map values', range },
        { label: 'trim', kind: 1, insertText: 'trim(${1:string})', insertTextRules: 4, detail: 'Trim whitespace', range },
        { label: 'upper', kind: 1, insertText: 'upper(${1:string})', insertTextRules: 4, detail: 'Uppercase string', range },
        { label: 'lower', kind: 1, insertText: 'lower(${1:string})', insertTextRules: 4, detail: 'Lowercase string', range },
        { label: 'split', kind: 1, insertText: 'split(${1:string}, ${2:","})', insertTextRules: 4, detail: 'Split string', range },
        { label: 'join', kind: 1, insertText: 'join(${1:array}, ${2:","})', insertTextRules: 4, detail: 'Join array to string', range },
      );

      // Keywords
      suggestions.push(
        { label: 'true', kind: 13, insertText: 'true', detail: 'Boolean true', range },
        { label: 'false', kind: 13, insertText: 'false', detail: 'Boolean false', range },
        { label: 'nil', kind: 13, insertText: 'nil', detail: 'Nil value', range },
        { label: 'and', kind: 13, insertText: 'and ', detail: 'Logical AND (same as &&)', range },
        { label: 'or', kind: 13, insertText: 'or ', detail: 'Logical OR (same as ||)', range },
        { label: 'not', kind: 13, insertText: 'not ', detail: 'Logical NOT (same as !)', range },
        { label: '&&', kind: 11, insertText: '&& ', detail: 'Logical AND', range },
        { label: '||', kind: 11, insertText: '|| ', detail: 'Logical OR', range },
        { label: '??', kind: 11, insertText: '?? ', detail: 'Nil coalescing (default if nil)', documentation: 'value ?? "default"', range },
        { label: '?:', kind: 11, insertText: '${1:condition} ? ${2:true} : ${3:false}', insertTextRules: 4, detail: 'Ternary operator', documentation: 'x > 0 ? "positive" : "negative"', range },
        { label: '..', kind: 11, insertText: '${1:start}..${2:end}', insertTextRules: 4, detail: 'Range operator', documentation: '1..10 creates [1,2,3,...,10]', range },
      );

      return { suggestions };
    },
  });
}
