
import Editor from '@monaco-editor/react'

interface CodeEditorProps {
  code: string
  onChange: (code: string) => void
  readOnly?: boolean
}

export default function CodeEditor({
  code,
  onChange,
  readOnly = false,
}: CodeEditorProps) {
  return (
    <div className="code-editor">
      <Editor
        height="100%"
        language="python"
        theme="vs-dark"
        value={code}
        onChange={(value) =>
          onChange(value ?? '')
        }
        options={{
          readOnly,
          fontSize: 15,

          fontFamily:
            "'Cascadia Code', Consolas, monospace",

          fontLigatures: true,

          minimap: {
            enabled: false,
          },

          automaticLayout: true,
          scrollBeyondLastLine: false,

          lineNumbers: 'on',
          renderLineHighlight: 'all',

          tabSize: 4,
          insertSpaces: true,

          wordWrap: 'on',
          smoothScrolling: true,

          cursorSmoothCaretAnimation: 'on',

          padding: {
            top: 16,
            bottom: 16,
          },

          bracketPairColorization: {
            enabled: true,
          },

          guides: {
            indentation: true,
            bracketPairs: true,
          },
        }}
      />
    </div>
  )
}
