import { useState, useEffect } from 'react'
import { CheckCircle2, Circle, Plus, Trash2, ListTodo } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface Todo {
  id: string
  text: string
  completed: boolean
  createdAt: number
}

const STORAGE_KEY = 'cowork-todos'

export function TodoWidget() {
  const [todos, setTodos] = useState<Todo[]>([])
  const [newTodo, setNewTodo] = useState('')
  const [isAdding, setIsAdding] = useState(false)

  // Load todos from localStorage
  useEffect(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY)
      if (stored) {
        setTodos(JSON.parse(stored))
      }
    } catch (e) {
      console.error('Failed to load todos:', e)
    }
  }, [])

  // Save todos to localStorage
  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(todos))
    } catch (e) {
      console.error('Failed to save todos:', e)
    }
  }, [todos])

  const addTodo = () => {
    if (!newTodo.trim()) return
    
    const todo: Todo = {
      id: `todo-${Date.now()}`,
      text: newTodo.trim(),
      completed: false,
      createdAt: Date.now(),
    }
    
    setTodos(prev => [todo, ...prev])
    setNewTodo('')
    setIsAdding(false)
  }

  const toggleTodo = (id: string) => {
    setTodos(prev => prev.map(todo => 
      todo.id === id ? { ...todo, completed: !todo.completed } : todo
    ))
  }

  const deleteTodo = (id: string) => {
    setTodos(prev => prev.filter(todo => todo.id !== id))
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      addTodo()
    } else if (e.key === 'Escape') {
      setNewTodo('')
      setIsAdding(false)
    }
  }

  const completedCount = todos.filter(t => t.completed).length
  const pendingCount = todos.length - completedCount

  return (
    <div className="h-full flex flex-col p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <ListTodo className="w-4 h-4 text-primary" />
          <span className="text-sm font-medium">Todo List</span>
        </div>
        <div className="flex items-center gap-1.5">
          {pendingCount > 0 && (
            <span className="text-xs text-muted-foreground">
              {pendingCount} pending
            </span>
          )}
          {completedCount > 0 && (
            <span className="text-xs text-muted-foreground">
              {completedCount} done
            </span>
          )}
        </div>
      </div>

      {/* Add Todo Input */}
      {isAdding ? (
        <div className="flex items-center gap-2 mb-3">
          <input
            type="text"
            value={newTodo}
            onChange={(e) => setNewTodo(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Add a task..."
            autoFocus
            className="flex-1 px-3 py-1.5 text-sm bg-muted/50 border border-border rounded-lg focus:outline-none focus:ring-1 focus:ring-primary/50"
          />
          <Button size="sm" onClick={addTodo} disabled={!newTodo.trim()}>
            Add
          </Button>
          <Button 
            size="sm" 
            variant="ghost" 
            onClick={() => { setNewTodo(''); setIsAdding(false); }}
          >
            Cancel
          </Button>
        </div>
      ) : (
        <Button
          variant="outline"
          size="sm"
          onClick={() => setIsAdding(true)}
          className="mb-3 gap-1.5 w-full border-dashed"
        >
          <Plus className="w-4 h-4" />
          Add Task
        </Button>
      )}

      {/* Todo List */}
      <div className="flex-1 overflow-auto space-y-1.5">
        {todos.length === 0 ? (
          <div className="h-full flex flex-col items-center justify-center text-muted-foreground">
            <div className="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mb-3">
              <ListTodo className="w-6 h-6 opacity-50" />
            </div>
            <p className="text-sm font-medium">No tasks yet</p>
            <p className="text-xs text-muted-foreground">Add your first todo</p>
          </div>
        ) : (
          todos.map((todo) => (
            <div
              key={todo.id}
              className={cn(
                'group flex items-center gap-3 p-2.5 rounded-xl border border-border/50 bg-muted/30',
                'hover:bg-muted/50 transition-all',
                todo.completed && 'opacity-60'
              )}
            >
              <button
                onClick={() => toggleTodo(todo.id)}
                className="flex-shrink-0 focus:outline-none"
              >
                {todo.completed ? (
                  <CheckCircle2 className="w-5 h-5 text-primary" />
                ) : (
                  <Circle className="w-5 h-5 text-muted-foreground hover:text-primary transition-colors" />
                )}
              </button>
              
              <span
                className={cn(
                  'flex-1 text-sm truncate',
                  todo.completed && 'line-through text-muted-foreground'
                )}
              >
                {todo.text}
              </span>
              
              <button
                onClick={() => deleteTodo(todo.id)}
                className="flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity p-1 hover:bg-destructive/10 rounded"
              >
                <Trash2 className="w-4 h-4 text-muted-foreground hover:text-destructive" />
              </button>
            </div>
          ))
        )}
      </div>
    </div>
  )
}