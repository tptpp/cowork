import { useCallback, useRef, useState } from 'react'
import { Upload, X, FileIcon, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { formatFileSize } from '@/stores/fileStore'
import type { UploadedFile } from '@/types'

interface FileInputProps {
  onFileSelect: (files: UploadedFile[]) => void
  accept?: string // e.g., ".txt,.pdf,image/*"
  multiple?: boolean
  maxSize?: number // in bytes
  disabled?: boolean
  className?: string
  uploading?: boolean
}

export function FileInput({
  onFileSelect,
  accept,
  multiple = false,
  maxSize,
  disabled = false,
  className,
  uploading = false,
}: FileInputProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [selectedFiles, setSelectedFiles] = useState<File[]>([])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!disabled) {
      setIsDragging(true)
    }
  }, [disabled])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
  }, [])

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      setIsDragging(false)

      if (disabled) return

      const files = Array.from(e.dataTransfer.files)
      handleFiles(files)
    },
    [disabled]
  )

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const files = Array.from(e.target.files || [])
      handleFiles(files)
      // Reset input so the same file can be selected again
      if (inputRef.current) {
        inputRef.current.value = ''
      }
    },
    []
  )

  const handleFiles = (files: File[]) => {
    // Filter by accept pattern
    let filteredFiles = files
    if (accept) {
      const acceptPatterns = accept.split(',').map((p) => p.trim())
      filteredFiles = files.filter((file) =>
        acceptPatterns.some((pattern) => {
          if (pattern.startsWith('.')) {
            return file.name.toLowerCase().endsWith(pattern.toLowerCase())
          }
          if (pattern.endsWith('/*')) {
            return file.type.startsWith(pattern.replace('/*', '/'))
          }
          return file.type === pattern
        })
      )
    }

    // Filter by max size
    if (maxSize) {
      filteredFiles = filteredFiles.filter((file) => file.size <= maxSize)
    }

    // Limit to single file if not multiple
    if (!multiple) {
      filteredFiles = filteredFiles.slice(0, 1)
    }

    setSelectedFiles((prev) => [...prev, ...filteredFiles])
  }

  const removeFile = (index: number) => {
    setSelectedFiles((prev) => prev.filter((_, i) => i !== index))
  }

  const handleClick = () => {
    if (!disabled && inputRef.current) {
      inputRef.current.click()
    }
  }

  return (
    <div className={cn('relative', className)}>
      {/* Hidden file input */}
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        multiple={multiple}
        onChange={handleFileChange}
        className="hidden"
        disabled={disabled}
      />

      {/* Drop zone */}
      <div
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={handleClick}
        className={cn(
          'border-2 border-dashed rounded-lg p-4 transition-all cursor-pointer',
          isDragging
            ? 'border-primary bg-primary/5'
            : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/30',
          disabled && 'opacity-50 cursor-not-allowed'
        )}
      >
        <div className="flex flex-col items-center gap-2 text-center">
          {uploading ? (
            <Loader2 className="w-8 h-8 text-muted-foreground animate-spin" />
          ) : (
            <Upload className="w-8 h-8 text-muted-foreground" />
          )}
          <div className="text-sm text-muted-foreground">
            {uploading ? (
              <span>Uploading...</span>
            ) : (
              <>
                <span className="font-medium text-foreground">
                  Click to upload
                </span>{' '}
                or drag and drop
              </>
            )}
          </div>
          {maxSize && (
            <div className="text-xs text-muted-foreground">
              Max size: {formatFileSize(maxSize)}
            </div>
          )}
        </div>
      </div>

      {/* Selected files list */}
      {selectedFiles.length > 0 && (
        <div className="mt-2 space-y-1">
          {selectedFiles.map((file, index) => (
            <div
              key={`${file.name}-${index}`}
              className="flex items-center gap-2 p-2 bg-muted/50 rounded-md text-sm"
            >
              <FileIcon className="w-4 h-4 text-muted-foreground flex-shrink-0" />
              <span className="flex-1 truncate">{file.name}</span>
              <span className="text-xs text-muted-foreground">
                {formatFileSize(file.size)}
              </span>
              <Button
                variant="ghost"
                size="icon"
                className="h-5 w-5"
                onClick={(e) => {
                  e.stopPropagation()
                  removeFile(index)
                }}
                disabled={uploading}
              >
                <X className="w-3 h-3" />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// Simple file attachment button for chat inputs
interface FileAttachButtonProps {
  onFileSelect: (files: File[]) => void
  accept?: string
  multiple?: boolean
  disabled?: boolean
}

export function FileAttachButton({
  onFileSelect,
  accept,
  multiple = false,
  disabled = false,
}: FileAttachButtonProps) {
  const inputRef = useRef<HTMLInputElement>(null)

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    if (files.length > 0) {
      onFileSelect(multiple ? files : [files[0]])
    }
    if (inputRef.current) {
      inputRef.current.value = ''
    }
  }

  return (
    <>
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        multiple={multiple}
        onChange={handleChange}
        className="hidden"
        disabled={disabled}
      />
      <Button
        variant="ghost"
        size="icon"
        onClick={() => inputRef.current?.click()}
        disabled={disabled}
        title="Attach file"
        className="h-8 w-8"
      >
        <Upload className="w-4 h-4" />
      </Button>
    </>
  )
}