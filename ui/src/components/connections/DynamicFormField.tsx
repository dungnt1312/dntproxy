import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import type { FormFieldMeta } from '@/types/provider-metadata';

type Props = {
  field: FormFieldMeta;
  value: string;
  onChange: (value: string) => void;
  className?: string;
};

export function DynamicFormField({ field, value, onChange, className }: Props) {
  const placeholder = field.placeholder ?? field.label + (field.required ? ' *' : ' (optional)');

  if (field.type === 'textarea') {
    return (
      <Textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        rows={3}
        className={className ?? 'text-xs font-mono'}
      />
    );
  }

  const inputType =
    field.type === 'password' || field.secret ? 'password' : field.type === 'number' ? 'number' : 'text';

  return (
    <Input
      type={inputType}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      className={className ?? (field.secret || field.name === 'apiKey' ? 'text-xs font-mono' : 'text-xs')}
    />
  );
}