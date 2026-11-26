
import {
  useEffect,
  useState,
} from 'react'

import Editor from '@monaco-editor/react'

import type {
  OnMount,
} from '@monaco-editor/react'

import type {
  editor,
} from 'monaco-editor'

import {
  useExecutionStore,
} from '../../store/executionStore'

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
  const [editorInstance, setEditorInstance] =
    useState<editor.IStandaloneCodeEditor | null>(
      null,
    )

  const trace = useExecutionStore(
    (state) => state.trace,
  )

  const currentStep = useExecutionStore(
    (state) => state.currentStep,
  )

  const executedCode = useExecutionStore(
    (state) => state.executedCode,
  )

  const activeEvent =
    trace?.events[currentStep]

  const activeLine =
    activeEvent?.line ?? null

  const handleMount: OnMount = (
    instance,
  ) => {
    setEditorInstance(instance)
  }

  useEffect(() => {
    if (!editorInstance) return

    if (
      activeLine === null ||
      activeLine < 1 ||
      executedCode !== code
    ) {
      return
    }

    const model =
      editorInstance.getModel()

    if (
      !model ||
      activeLine > model.getLineCount()
    ) {
      return
    }

    const decorations =
      editorInstance.createDecorationsCollection([
        {
          range: {
            startLineNumber: activeLine,
            startColumn: 1,
            endLineNumber: activeLine,
            endColumn: 1,
          },

          options: {
            isWholeLine: true,

            className:
              'execution-line-highlight',

            glyphMarginClassName:
              'execution-line-glyph',

            linesDecorationsClassName:
              'execution-line-decoration',
          },
        },
      ])

    editorInstance
      .revealLineInCenterIfOutsideViewport(
        activeLine,
      )

    return () => {
      decorations.clear()
    }
  }, [
    editorInstance,
    activeLine,
    executedCode,
    code,
  ])

  return (
    <div className="code-editor">
      <Editor
        height="100%"
        language="python"
        theme="vs-dark"
        value={code}
        onMount={handleMount}
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
          glyphMargin: true,

          renderLineHighlight: 'none',

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
