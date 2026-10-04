import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

/** LabeledNumber 设置表单中带标签的数字输入框 */
export function LabeledNumber({
  id,
  label,
  value,
  min,
  max,
  onChange,
}: {
  id: string
  label: string
  value: number
  min: number
  max: number
  onChange: (v: number) => void
}) {
  return (
    <span className="flex items-center gap-2">
      <Label htmlFor={id} className="font-normal">
        {label}
      </Label>
      <Input id={id} type="number" min={min} max={max} className="w-24" value={value} onChange={(e) => onChange(Number(e.target.value))} />
    </span>
  )
}
