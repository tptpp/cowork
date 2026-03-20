import { useState } from 'react'
import {
  Wrench,
  ChevronDown,
  ChevronRight,
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
  AlertCircle,
  Code,
  FileText,
  Terminal,
  Search,
  Play,
} from 'lucide-react'
import type { ToolCall, ToolExecution } from '@/types'
import { cn } from '@/lib/utils'

// Tool icons mapping
const TOOL_ICONS: Record<string, React.ReactNode> = {
  execute_shell: <Terminal className="w-4 h-4" />,
  read_file: <FileText className="w-4 h-4" />,
  write_file: <FileText className="w-4 h-4" />,
  web_search: <Search className="w-4 h-4" />,
  create_task: <Play className="w-4 h-4" />,
  default: <Wrench className="w-4 h-4" />,
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

// Status badge component
function StatusBadge({ status }: { status: ToolExecution['status'] }) {
  const statusConfig = {
    pending: {
      icon: <Clock className="w-3 h-3" />,
      text: 'Pending',
      className: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    },
    running: {
      icon: <Loader2 className="w-3 h-3 animate-spin" />,
      text: 'Running',
      className: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
    },
    completed: {
      icon: <CheckCircle2 className="w-3 h-3" />,
      text: 'Completed',
      className: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    },
    failed: {
      icon: <XCircle className="w-3 h-3" />,
      text: 'Failed',
      className: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    },
  }

  const config = statusConfig[status] || statusConfig.pending

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium',
        config.className
      )}
    >
      {config.icon}
      {config.text}
    </span>
  )
}

// JSON viewer component
function JsonViewer({ data, title }: { data: unknown; title?: string }) {
  const [expanded, setExpanded] = useState(false)

  const jsonString = JSON.stringify(data, null, 2)
  const shouldTruncate = jsonString.length > 200

  return (
    <div className="bg-muted/50 rounded-md p-2 text-xs font-mono">
      {title && (
        <div className="text-muted-foreground mb-1 text-[10px] uppercase tracking-wide">
          {title}
        </div>
      )}
      <pre className="text-foreground overflow-x-auto whitespace-pre-wrap break-all">
        {shouldTruncate && !expanded
          ? jsonString.slice(0, 200) + '...'
          : jsonString}
      </pre>
      {shouldTruncate && (
        <button
          onClick={() => setExpanded(!expanded)}
          className="text-primary text-[10px] mt-1 hover:underline"
        >
          {expanded ? 'Show less' : 'Show more'}
        </button>
      )}
    </div>
  )
}

// Tool Call display component
interface ToolCallDisplayProps {
  toolCall: ToolCall
  execution?: ToolExecution
  onApprove?: (approved: boolean) => void
  showApproval?: boolean
}

export function ToolCallDisplay({
  toolCall,
  execution,
  onApprove,
  showApproval = false,
}: ToolCallDisplayProps) {
  const [expanded, setExpanded] = useState(false)
  const args = parseArguments(toolCall.function.arguments)

  const toolName = toolCall.function.name
  const status = execution?.status || 'pending'

  return (
    <div className="border rounded-lg overflow-hidden bg-card shadow-sm">
      {/* Header */}
      <div
        className={cn(
          'flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-muted/50 transition-colors',
          status === 'running' && 'animate-pulse bg-blue-50/50 dark:bg-blue-900/10'
        )}
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-muted-foreground" />
        ) : (
          <ChevronRight className="w-4 h-4 text-muted-foreground" />
        )}

        <div className="flex items-center justify-center w-7 h-7 rounded-md bg-primary/10 text-primary">
          {getToolIcon(toolName)}
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-sm truncate">{toolName}</span>
            {execution && <StatusBadge status={status} />}
          </div>
          <span className="text-xs text-muted-foreground">
            Tool Call ID: {toolCall.id}
          </span>
        </div>
      </div>

      {/* Expanded content */}
      {expanded && (
        <div className="border-t px-3 py-2 space-y-2">
          {/* Arguments */}
          <div>
            <div className="flex items-center gap-1 text-xs font-medium text-muted-foreground mb-1">
              <Code className="w-3 h-3" />
              Arguments
            </div>
            <JsonViewer data={args} />
          </div>

          {/* Execution result */}
          {execution && (
            <div>
              <div className="flex items-center gap-1 text-xs font-medium text-muted-foreground mb-1">
                <FileText className="w-3 h-3" />
                Result
              </div>
              {execution.result ? (
                <div
                  className={cn(
                    'rounded-md p-2 text-xs font-mono',
                    execution.is_error
                      ? 'bg-red-50 text-red-800 dark:bg-red-900/20 dark:text-red-400'
                      : 'bg-green-50 text-green-800 dark:bg-green-900/20 dark:text-green-400'
                  )}
                >
                  <pre className="whitespace-pre-wrap break-all overflow-x-auto">
                    {execution.result}
                  </pre>
                </div>
              ) : status === 'running' ? (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Loader2 className="w-3 h-3 animate-spin" />
                  Executing...
                </div>
              ) : (
                <div className="text-xs text-muted-foreground">No result yet</div>
              )}
            </div>
          )}

          {/* Approval buttons */}
          {showApproval && onApprove && (
            <div className="flex items-center gap-2 pt-2 border-t">
              <button
                onClick={() => onApprove(true)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-md bg-green-600 text-white hover:bg-green-700 transition-colors"
              >
                <CheckCircle2 className="w-3 h-3" />
                Approve
              </button>
              <button
                onClick={() => onApprove(false)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-md bg-red-600 text-white hover:bg-red-700 transition-colors"
              >
                <XCircle className="w-3 h-3" />
                Reject
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// Tool calls list component
interface ToolCallsListProps {
  toolCalls: ToolCall[]
  executions: ToolExecution[]
  onApprove?: (toolCallId: string, approved: boolean) => void
  showApproval?: boolean
}

export function ToolCallsList({
  toolCalls,
  executions,
  onApprove,
  showApproval = false,
}: ToolCallsListProps) {
  if (toolCalls.length === 0) return null

  return (
    <div className="space-y-2">
      {toolCalls.map((toolCall) => {
        const execution = executions.find((e) => e.tool_call_id === toolCall.id)
        return (
          <ToolCallDisplay
            key={toolCall.id}
            toolCall={toolCall}
            execution={execution}
            onApprove={onApprove ? (approved) => onApprove(toolCall.id, approved) : undefined}
            showApproval={showApproval}
          />
        )
      })}
    </div>
  )
}

// Tool execution error display
export function ToolExecutionError({
  execution,
}: {
  execution: ToolExecution
}) {
  if (!execution.is_error) return null

  return (
    <div className="flex items-start gap-2 p-3 rounded-md bg-red-50 border border-red-200 dark:bg-red-900/20 dark:border-red-800">
      <AlertCircle className="w-4 h-4 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        <div className="font-medium text-sm text-red-800 dark:text-red-300">
          Tool Execution Failed
        </div>
        <div className="text-xs text-red-600 dark:text-red-400 mt-1 font-mono">
          {execution.result}
        </div>
      </div>
    </div>
  )
}

// Pending tool calls badge
export function PendingToolsBadge({ count }: { count: number }) {
  if (count === 0) return null

  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400">
      <Clock className="w-3 h-3" />
      {count} pending tool{count !== 1 ? 's' : ''}
    </span>
  )
}