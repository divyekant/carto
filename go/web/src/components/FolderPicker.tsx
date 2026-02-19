import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'

interface BrowseResult {
  current: string
  parent: string
  directories: { name: string; path: string }[]
}

interface FolderPickerProps {
  value: string
  onChange: (path: string) => void
}

export function FolderPicker({ value, onChange }: FolderPickerProps) {
  const [open, setOpen] = useState(false)
  const [browsePath, setBrowsePath] = useState('')
  const [data, setData] = useState<BrowseResult | null>(null)
  const [loading, setLoading] = useState(false)

  function browse(path: string) {
    setLoading(true)
    const params = path ? `?path=${encodeURIComponent(path)}` : ''
    fetch(`/api/browse${params}`)
      .then(r => r.json())
      .then((result: BrowseResult) => {
        setData(result)
        setBrowsePath(result.current)
      })
      .catch(() => setData(null))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    if (open && !data) {
      browse(value || '')
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  function select() {
    onChange(browsePath)
    setOpen(false)
  }

  if (!open) {
    return (
      <div className="flex gap-2">
        <div className="flex-1 px-3 py-2 text-sm border border-border rounded-md bg-background truncate">
          {value || <span className="text-muted-foreground">No folder selected</span>}
        </div>
        <Button variant="secondary" size="sm" onClick={() => setOpen(true)}>
          Browse
        </Button>
      </div>
    )
  }

  return (
    <div className="border border-border rounded-md bg-background">
      <div className="px-3 py-2 border-b border-border flex items-center gap-1 text-sm overflow-x-auto">
        <span className="text-muted-foreground truncate">{browsePath}</span>
      </div>
      <div className="max-h-48 overflow-y-auto">
        {loading ? (
          <div className="px-3 py-4 text-sm text-muted-foreground">Loading...</div>
        ) : data ? (
          <>
            {data.parent !== data.current && (
              <button
                className="w-full px-3 py-1.5 text-sm text-left hover:bg-muted flex items-center gap-2"
                onClick={() => browse(data.parent)}
              >
                <span className="text-muted-foreground">..</span>
                <span className="text-muted-foreground text-xs">Parent directory</span>
              </button>
            )}
            {data.directories.length === 0 && (
              <div className="px-3 py-4 text-sm text-muted-foreground">No subdirectories</div>
            )}
            {data.directories.map(dir => (
              <button
                key={dir.path}
                className="w-full px-3 py-1.5 text-sm text-left hover:bg-muted flex items-center gap-2"
                onClick={() => browse(dir.path)}
              >
                <span>📁</span>
                <span>{dir.name}</span>
              </button>
            ))}
          </>
        ) : (
          <div className="px-3 py-4 text-sm text-red-400">Failed to load</div>
        )}
      </div>
      <div className="px-3 py-2 border-t border-border flex gap-2 justify-end">
        <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>Cancel</Button>
        <Button size="sm" onClick={select}>Select This Folder</Button>
      </div>
    </div>
  )
}
