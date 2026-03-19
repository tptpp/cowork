import { useState } from 'react'
import { Plus, Loader2, X } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { useTaskStore } from '@/stores/taskStore'
import type { Priority, Task } from '@/types'

interface TaskFormProps {
  onSuccess?: (task: Task) => void
  onCancel?: () => void
  showCard?: boolean
}

const taskTypeOptions = [
  { value: 'code_generation', label: 'Code Generation' },
  { value: 'code_review', label: 'Code Review' },
  { value: 'documentation', label: 'Documentation' },
  { value: 'testing', label: 'Testing' },
  { value: 'refactoring', label: 'Refactoring' },
  { value: 'analysis', label: 'Analysis' },
  { value: 'custom', label: 'Custom' },
]

const priorityOptions = [
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
]

const commonTags = [
  'golang', 'typescript', 'python', 'react', 'nodejs',
  'database', 'api', 'frontend', 'backend', 'testing',
]

const commonModels = [
  { value: '', label: 'Any (No preference)' },
  { value: 'claude-3-opus', label: 'Claude 3 Opus' },
  { value: 'claude-3-sonnet', label: 'Claude 3 Sonnet' },
  { value: 'gpt-4', label: 'GPT-4' },
  { value: 'gpt-3.5-turbo', label: 'GPT-3.5 Turbo' },
]

export function TaskForm({ onSuccess, onCancel, showCard = true }: TaskFormProps) {
  const { createTask } = useTaskStore()
  const [loading, setLoading] = useState(false)
  const [formData, setFormData] = useState({
    type: 'code_generation',
    description: '',
    priority: 'medium' as Priority,
    preferred_model: '',
  })
  const [tags, setTags] = useState<string[]>([])
  const [newTag, setNewTag] = useState('')
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    try {
      const task = await createTask({
        type: formData.type,
        description: formData.description,
        priority: formData.priority,
        required_tags: tags,
        preferred_model: formData.preferred_model,
        input: {},
      })

      if (task) {
        // Reset form
        setFormData({
          type: 'code_generation',
          description: '',
          priority: 'medium',
          preferred_model: '',
        })
        setTags([])
        onSuccess?.(task)
      } else {
        setError('Failed to create task. Please try again.')
      }
    } catch {
      setError('An unexpected error occurred.')
    } finally {
      setLoading(false)
    }
  }

  const addTag = (tag: string) => {
    const trimmedTag = tag.trim().toLowerCase()
    if (trimmedTag && !tags.includes(trimmedTag)) {
      setTags([...tags, trimmedTag])
    }
    setNewTag('')
  }

  const removeTag = (tag: string) => {
    setTags(tags.filter((t) => t !== tag))
  }

  const handleTagKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      addTag(newTag)
    }
  }

  const formContent = (
    <form onSubmit={handleSubmit} className="space-y-4">
      {/* Task Type */}
      <div className="space-y-2">
        <label className="text-sm font-medium">Task Type</label>
        <Select
          options={taskTypeOptions}
          value={formData.type}
          onChange={(e) => setFormData({ ...formData, type: e.target.value })}
        />
      </div>

      {/* Description */}
      <div className="space-y-2">
        <label className="text-sm font-medium">Description</label>
        <Textarea
          placeholder="Describe what you want the task to accomplish..."
          value={formData.description}
          onChange={(e) => setFormData({ ...formData, description: e.target.value })}
          rows={4}
        />
      </div>

      {/* Priority */}
      <div className="space-y-2">
        <label className="text-sm font-medium">Priority</label>
        <Select
          options={priorityOptions}
          value={formData.priority}
          onChange={(e) => setFormData({ ...formData, priority: e.target.value as Priority })}
        />
      </div>

      {/* Required Tags */}
      <div className="space-y-2">
        <label className="text-sm font-medium">Required Tags</label>
        <div className="flex gap-2">
          <Input
            placeholder="Add a tag..."
            value={newTag}
            onChange={(e) => setNewTag(e.target.value)}
            onKeyDown={handleTagKeyDown}
          />
          <Button
            type="button"
            variant="outline"
            onClick={() => addTag(newTag)}
            disabled={!newTag.trim()}
          >
            Add
          </Button>
        </div>
        <div className="flex flex-wrap gap-1 mt-2">
          {tags.map((tag) => (
            <Badge key={tag} variant="secondary" className="cursor-pointer">
              {tag}
              <X
                className="w-3 h-3 ml-1"
                onClick={() => removeTag(tag)}
              />
            </Badge>
          ))}
        </div>
        <div className="flex flex-wrap gap-1 mt-2">
          <span className="text-xs text-muted-foreground mr-1">Quick add:</span>
          {commonTags.filter((t) => !tags.includes(t)).slice(0, 6).map((tag) => (
            <button
              key={tag}
              type="button"
              className="text-xs text-muted-foreground hover:text-foreground"
              onClick={() => addTag(tag)}
            >
              +{tag}
            </button>
          ))}
        </div>
      </div>

      {/* Preferred Model */}
      <div className="space-y-2">
        <label className="text-sm font-medium">Preferred Model</label>
        <Select
          options={commonModels}
          value={formData.preferred_model}
          onChange={(e) => setFormData({ ...formData, preferred_model: e.target.value })}
        />
      </div>

      {/* Error */}
      {error && (
        <div className="text-sm text-destructive">{error}</div>
      )}

      {/* Actions */}
      <div className="flex gap-2 pt-4">
        <Button type="submit" disabled={loading}>
          {loading ? (
            <Loader2 className="w-4 h-4 animate-spin mr-1" />
          ) : (
            <Plus className="w-4 h-4 mr-1" />
          )}
          Create Task
        </Button>
        {onCancel && (
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        )}
      </div>
    </form>
  )

  if (!showCard) {
    return formContent
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Create New Task</CardTitle>
        <CardDescription>Submit a new task to the processing queue</CardDescription>
      </CardHeader>
      <CardContent>{formContent}</CardContent>
    </Card>
  )
}