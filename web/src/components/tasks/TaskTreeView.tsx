// web/src/components/tasks/TaskTreeView.tsx
import React from 'react';
import { useTaskTreeStore } from '@/stores/taskTreeStore';
import { useTaskStore } from '@/stores/taskStore';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { Task } from '@/types';

interface TaskTreeViewProps {
  onTaskClick?: (taskId: string) => void;
  onSelectTask?: (task: Task) => void;
}

export function TaskTreeView({ onTaskClick, onSelectTask }: TaskTreeViewProps) {
  const {
    tasks,
    rootTaskId,
    expandedNodes,
    toggleExpand,
    getTaskById,
    getChildTasks,
    expandAll,
    collapseAll,
  } = useTaskTreeStore();

  if (!rootTaskId) {
    return (
      <div className="p-4 text-muted-foreground">
        暂无任务树
      </div>
    );
  }

  const renderTaskNode = (taskId: string, depth: number = 0) => {
    const task = getTaskById(taskId);
    if (!task) return null;

    const children = getChildTasks(taskId);
    const isExpanded = expandedNodes.has(taskId);
    const hasChildren = children.length > 0;

    const statusColor = {
      pending: 'bg-gray-200',
      running: 'bg-blue-500',
      done: 'bg-green-500',
      failed: 'bg-red-500',
    };

    const progressColor = task.progress < 30 ? 'text-red-500' :
                          task.progress < 70 ? 'text-blue-500' :
                          'text-green-500';

    return (
      <div key={taskId} className="task-node">
        <div
          className={cn(
            "flex items-center gap-2 p-2 rounded hover:bg-muted cursor-pointer",
            depth > 0 && "ml-6"
          )}
          onClick={() => onTaskClick?.(taskId)}
        >
          {/* Expand/Collapse Toggle */}
          {hasChildren && (
            <button
              onClick={(e) => { e.stopPropagation(); toggleExpand(taskId); }}
              className="w-4 h-4 flex items-center justify-center"
            >
              {isExpanded ? '▼' : '▶'}
            </button>
          )}

          {/* Status Indicator */}
          <div className={cn("w-2 h-2 rounded-full", statusColor[task.status])} />

          {/* Task Title */}
          <span className="font-medium">{task.title}</span>

          {/* Progress Badge */}
          <Badge variant="outline" className={progressColor}>
            {task.progress}%
          </Badge>

          {/* Template Badge */}
          <Badge variant="secondary">
            {task.templateId}
          </Badge>
        </div>

        {/* Milestone Progress */}
        {task.milestone && Object.keys(task.milestone).length > 0 && (
          <div className="ml-6 mb-2 flex gap-1">
            {Object.entries(task.milestone).map(([name, status]) => (
              <Badge
                key={name}
                variant={status === 'done' ? 'default' : status === 'running' ? 'outline' : 'secondary'}
                className="text-xs"
              >
                {name}: {status === 'done' ? '✓' : status === 'running' ? '●' : '○'}
              </Badge>
            ))}
          </div>
        )}

        {/* Children */}
        {isExpanded && hasChildren && (
          <div className="children">
            {children.map(child => renderTaskNode(child.id, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="task-tree-view p-4 border rounded-lg">
      <div className="flex justify-between mb-4">
        <h3 className="font-semibold">任务树</h3>
        <div className="flex gap-2">
          <button
            onClick={() => expandAll()}
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            展开全部
          </button>
          <button
            onClick={() => collapseAll()}
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            收起全部
          </button>
        </div>
      </div>

      {renderTaskNode(rootTaskId)}
    </div>
  );
}