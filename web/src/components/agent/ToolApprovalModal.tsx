import { useState } from 'react'
import {
  AlertTriangle,
  CheckCircle2,
  XCircle,
  Terminal,
  FileText,
  Search,
  Play,
  Wrench,
  Loader2,
} from 'lucide-react'
import type { ToolCall } from '@/types'
import { cn } from '@/lib/utils'

// Tool icons mapping
const TOOL_ICONS: Record<string, React.ReactNode> = {
  execute_shell: <Terminal className="w-5 h-5" />,
  read_file: <FileText className="w-5 h-5" />,
  write_file: <FileText className="w-5 h-5" />,
  web_search: <Search className="w-5 h-5" />,
  create_task: <Play className="w-5 h-5" />,
  default: <Wrench className="w-5 h-5" />,
}

// Get icon for tool
function getToolIcon(toolName: string): React.ReactNode {
  return TOOL_ICONS[toolName] || TOOL_ICONS.default
}

// Parse tool arguments
function parseArguments(argsString: string): Record<string, unknown> {
  try {
    return JSON.parse(argsString)
  } catch {
    return {}
  }
}

// Dangerous command patterns
const DANGEROUS_PATTERNS = [
  /rm\s+-rf/,
  /rm\s+-r/,
  /rm\s+\/$/,
  /:\(\)\{.*;\};/,
  />\s*\/dev\/sda/,
  /mkfs/,
  /dd\s+if=/,
  /chmod\s+777/,
  /chown\s+.*\//,
]

// Check if command is dangerous
function isDangerousCommand(command: string): boolean {
  return DANGEROUS_PATTERNS.some((pattern) => pattern.test(command))
}

// Approval Modal Props
interface ToolApprovalModalProps {
  toolCall: ToolCall
  onApprove: (approved: boolean, comment?: string) => void
  onClose: () => void
}

export function ToolApprovalModal({
  toolCall,
  onApprove,
  onClose,
}: ToolApprovalModalProps) {
  const [comment, setComment] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const args = parseArguments(toolCall.function.arguments)
  const toolName = toolCall.function.name

  // Check for dangerous operations
  let isDangerous = false
  let warningMessage = ''

  if (toolName === 'execute_shell' && typeof args.command === 'string') {
    isDangerous = isDangerousCommand(args.command)
    if (isDangerous) {
      warningMessage =
        'This command appears to be potentially dangerous. Please review carefully before approving.'
    }
  }

  if (toolName === 'write_file') {
    isDangerous = true
    warningMessage =
      'This operation will write or overwrite a file. Please verify the content before approving.'
  }

  const handleApprove = async (approved: boolean) => {
    setIsLoading(true)
    try {
      await onApprove(approved, comment || undefined)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative bg-card rounded-xl shadow-2xl max-w-lg w-full mx-4 overflow-hidden border">
        {/* Header */}
        <div
          className={cn(
            'px-6 py-4 border-b',
            isDangerous
              ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
              : 'bg-muted/50'
          )}
        >
          <div className="flex items-center gap-3">
            <div
              className={cn(
                'flex items-center justify-center w-10 h-10 rounded-lg',
                isDangerous
                  ? 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400'
                  : 'bg-primary/10 text-primary'
              )}
            >
              {getToolIcon(toolName)}
            </div>
            <div>
              <h2 className="text-lg font-semibold">Tool Approval Required</h2>
              <p className="text-sm text-muted-foreground">
                The agent wants to execute: <strong>{toolName}</strong>
              </p>
            </div>
          </div>
        </div>

        {/* Warning */}
        {isDangerous && (
          <div className="mx-6 mt-4 p-3 rounded-lg bg-red-50 border border-red-200 dark:bg-red-900/20 dark:border-red-800">
            <div className="flex items-start gap-2">
              <AlertTriangle className="w-5 h-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
              <div className="text-sm text-red-800 dark:text-red-300">
                {warningMessage}
              </div>
            </div>
          </div>
        )}

        {/* Content */}
        <div className="px-6 py-4 space-y-4">
          {/* Tool arguments */}
          <div>
            <div className="text-sm font-medium text-muted-foreground mb-2">
              Arguments
            </div>
            <div className="bg-muted/50 rounded-lg p-3 overflow-x-auto">
              <pre className="text-sm font-mono whitespace-pre-wrap break-all">
                {JSON.stringify(args, null, 2)}
              </pre>
            </div>
          </div>

          {/* Specific previews */}
          {toolName === 'execute_shell' && typeof args.command === 'string' && (
            <div>
              <div className="text-sm font-medium text-muted-foreground mb-2">
                Command to execute
              </div>
              <div className="bg-slate-900 text-slate-100 rounded-lg p-3 overflow-x-auto">
                <code className="text-sm font-mono">{args.command}</code>
              </div>
            </div>
          )}

          {toolName === 'write_file' && (
            <>
              {typeof args.path === 'string' && (
                <div>
                  <div className="text-sm font-medium text-muted-foreground mb-2">
                    File path
                  </div>
                  <div className="bg-muted/50 rounded-lg p-3 text-sm font-mono">
                    {args.path}
                  </div>
                </div>
              )}
              {typeof args.content === 'string' && (
                <div>
                  <div className="text-sm font-medium text-muted-foreground mb-2">
                    Content to write
                  </div>
                  <div className="bg-slate-900 text-slate-100 rounded-lg p-3 max-h-40 overflow-auto">
                    <pre className="text-sm font-mono whitespace-pre-wrap">
                      {args.content.length > 500
                        ? args.content.slice(0, 500) + '...(truncated)'
                        : args.content}
                    </pre>
                  </div>
                </div>
              )}
            </>
          )}

          {/* Comment input */}
          <div>
            <label className="text-sm font-medium text-muted-foreground mb-2 block">
              Comment (optional)
            </label>
            <input
              type="text"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder="Add a comment for your decision..."
              className="w-full px-3 py-2 text-sm border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary/50"
            />
          </div>
        </div>

        {/* Actions */}
        <div className="px-6 py-4 border-t bg-muted/30 flex items-center justify-end gap-3">
          <button
            onClick={() => handleApprove(false)}
            disabled={isLoading}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg border border-red-200 text-red-700 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/20 transition-colors disabled:opacity-50"
          >
            {isLoading ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <XCircle className="w-4 h-4" />
            )}
            Reject
          </button>
          <button
            onClick={() => handleApprove(true)}
            disabled={isLoading}
            className={cn(
              'flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg transition-colors disabled:opacity-50',
              isDangerous
                ? 'bg-red-600 text-white hover:bg-red-700'
                : 'bg-green-600 text-white hover:bg-green-700'
            )}
          >
            {isLoading ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <CheckCircle2 className="w-4 h-4" />
            )}
            Approve
          </button>
        </div>
      </div>
    </div>
  )
}

// Batch approval modal for multiple tool calls
interface BatchApprovalModalProps {
  toolCalls: ToolCall[]
  onApproveAll: (approved: boolean) => void
  onApproveIndividual: (toolCallId: string, approved: boolean) => void
  onClose: () => void
}

export function BatchApprovalModal({
  toolCalls,
  onApproveAll,
  onApproveIndividual,
  onClose,
}: BatchApprovalModalProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative bg-card rounded-xl shadow-2xl max-w-2xl w-full mx-4 overflow-hidden border max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="px-6 py-4 border-b bg-muted/50">
          <h2 className="text-lg font-semibold">
            {toolCalls.length} Tool Approvals Required
          </h2>
          <p className="text-sm text-muted-foreground">
            Review and approve or reject each tool execution
          </p>
        </div>

        {/* Tool calls list */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-3">
          {toolCalls.map((toolCall) => {
            const args = parseArguments(toolCall.function.arguments)
            const isExpanded = expandedId === toolCall.id

            return (
              <div
                key={toolCall.id}
                className="border rounded-lg overflow-hidden"
              >
                <div
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-muted/50"
                  onClick={() => setExpandedId(isExpanded ? null : toolCall.id)}
                >
                  <div className="flex items-center justify-center w-8 h-8 rounded-md bg-primary/10 text-primary">
                    {getToolIcon(toolCall.function.name)}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium">{toolCall.function.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {toolCall.id}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        onApproveIndividual(toolCall.id, false)
                      }}
                      className="p-2 rounded-md hover:bg-red-100 text-red-600 dark:hover:bg-red-900/30"
                    >
                      <XCircle className="w-4 h-4" />
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        onApproveIndividual(toolCall.id, true)
                      }}
                      className="p-2 rounded-md hover:bg-green-100 text-green-600 dark:hover:bg-green-900/30"
                    >
                      <CheckCircle2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {isExpanded && (
                  <div className="px-4 py-3 border-t bg-muted/30">
                    <pre className="text-xs font-mono whitespace-pre-wrap">
                      {JSON.stringify(args, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Actions */}
        <div className="px-6 py-4 border-t bg-muted/30 flex items-center justify-between">
          <button
            onClick={() => onApproveAll(false)}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg border text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 transition-colors"
          >
            <XCircle className="w-4 h-4" />
            Reject All
          </button>
          <button
            onClick={() => onApproveAll(true)}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg bg-green-600 text-white hover:bg-green-700 transition-colors"
          >
            <CheckCircle2 className="w-4 h-4" />
            Approve All
          </button>
        </div>
      </div>
    </div>
  )
}