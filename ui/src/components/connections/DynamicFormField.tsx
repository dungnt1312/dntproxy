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
  const inputType = isSecret ? 'password' : field.type === 'number' ? 'number' : field.type === 'url' ? 'url' : 'text';
  const helpId = field.helpText ? `${id}-help` : undefined;
  const help = field.helpText ? <p id={helpId} className="text-xs text-muted-foreground">{field.helpText}</p> : null;

  const label = (
    <label htmlFor={id} className="text-sm font-medium">
      {field.label}
      {field.required ? <span className="text-destructive" aria-hidden> *</span> : null}
    </label>
  );

  if (field.type === 'textarea') {
    return (
      <div className="space-y-1.5">
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
          required={field.required}
          aria-describedby={helpId}
          className={className ?? 'font-mono text-sm'}
        />
        {help}
      </div>
    );
  }

  if (field.type === 'select') {
    return (
      <div className="space-y-1.5">
        {label}
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger id={id} aria-describedby={helpId} className={className ?? 'w-full'}>
            <SelectValue placeholder={field.placeholder} />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((option) => (
              <SelectItem key={option} value={option}>{option}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        {help}
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      {label}
      <Input
        id={id}
        name={field.name}
        type={inputType}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={field.placeholder}
        autoComplete={isSecret ? 'new-password' : 'off'}
        spellCheck={isSecret || field.name === 'apiKey' ? false : undefined}
        required={field.required}
        aria-describedby={helpId}
        className={cn(className ?? (isSecret || field.name === 'apiKey' || inputType === 'url' ? 'font-mono text-sm' : 'text-sm'))}
      />
      {help}
    </div>
  );
}
