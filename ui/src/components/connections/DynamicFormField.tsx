import { useId } from 'react';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import type { FormFieldMeta } from '@/types/provider-metadata';

type Props = {
  field: FormFieldMeta;
  value: string;
  onChange: (value: string) => void;
  className?: string;
};

export function DynamicFormField({ field, value, onChange, className }: Props) {
  const id = useId();
  const isSecret = field.type === 'password' || field.secret;
  const inputType = isSecret ? 'password' : field.type === 'number' ? 'number' : 'text';

  const label = (
    <label htmlFor={id} className="text-xs font-medium">
      {field.label}
      {field.required ? <span className="text-destructive"> *</span> : null}
    </label>
  );

  if (field.type === 'textarea') {
    return (
      <div className="space-y-1">
        {label}
        <Textarea
          id={id}
          name={field.name}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder}
          autoComplete="off"
          spellCheck={false}
          rows={3}
          className={className ?? 'text-xs font-mono'}
        />
      </div>
    );
  }

  if (field.type === 'select') {
    return (
      <div className="space-y-1">
        {label}
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger id={id} className={className ?? 'w-full text-xs'}>
            <SelectValue placeholder={field.placeholder} />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((option) => (
              <SelectItem key={option} value={option}>{option}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        {field.helpText ? <p className="text-[11px] text-muted-foreground">{field.helpText}</p> : null}
      </div>
    );
  }

  return (
    <div className="space-y-1">
      {label}
      <Input
        id={id}
        name={field.name}
        type={inputType}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={field.placeholder}
        autoComplete={isSecret ? 'off' : undefined}
        spellCheck={isSecret || field.name === 'apiKey' ? false : undefined}
        className={cn(className ?? (isSecret || field.name === 'apiKey' ? 'text-xs font-mono' : 'text-xs'))}
      />
      {field.helpText ? <p className="text-[11px] text-muted-foreground">{field.helpText}</p> : null}
    </div>
  );
}
