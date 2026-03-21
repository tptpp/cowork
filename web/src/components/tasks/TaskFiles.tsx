import { useEffect, useState } from 'react'
import { FileIcon, Download, Trash2, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useFileStore, formatFileSize } from '@/stores/fileStore'
import { cn } from '@/lib/utils'
import type { TaskFile } from '@/types'

interface TaskFilesProps {
  taskId: string
  className?: string
}

export function TaskFiles({ taskId, className }: TaskFilesProps) {
  const [files, setFiles] = useState<TaskFile[]>([])
  const [loading, setLoading] = useState(true)
  const { fetchTaskFiles, downloadFile, deleteFile, error } = useFileStore()

  useEffect(() => {
    loadFiles()
  }, [taskId])

  const loadFiles = async () => {
    setLoading(true)
    const taskFiles = await fetchTaskFiles(taskId)
    setFiles(taskFiles)
    setLoading(false)
  }

  const handleDownload = async (fileId: number) => {
    await downloadFile(fileId)
  }

  const handleDelete = async (fileId: number) => {
    const success = await deleteFile(fileId)
    if (success) {
      setFiles((prev) => prev.filter((f) => f.id !== fileId))
    }
  }

  if (loading) {
    return (
      <div className={cn('flex items-center justify-center py-4', className)}>
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (files.length === 0) {
    return (
      <div
        className={cn(
          'text-sm text-muted-foreground text-center py-4',
          className
        )}
      >
        No files attached
      </div>
    )
  }

  return (
    <div className={cn('space-y-2', className)}>
      {error && (
        <div className="text-sm text-red-500 bg-red-50 dark:bg-red-900/20 p-2 rounded">
          {error}
        </div>
      )}
      <div className="space-y-1">
        {files.map((file) => (
          <div
            key={file.id}
            className="flex items-center gap-2 p-2 bg-muted/50 hover:bg-muted rounded-md transition-colors group"
          >
            <FileIcon className="w-4 h-4 text-muted-foreground flex-shrink-0" />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium truncate">{file.name}</div>
              <div className="text-xs text-muted-foreground">
                {formatFileSize(file.size)} •{' '}
                {new Date(file.created_at).toLocaleDateString()}
              </div>
            </div>
            <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={() => handleDownload(file.id)}
                title="Download"
              >
                <Download className="w-4 h-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
                onClick={() => handleDelete(file.id)}
                title="Delete"
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}