import { Field, type FieldProps } from '../Field';

export interface FormFieldProps extends Omit<FieldProps, 'children'> {
  /** The label shown above the control. */
  label: string;
  /** Render the control; receives the id to wire `htmlFor`/`aria-describedby`. */
  children: React.ReactNode;
}

/**
 * Alias for `Field` under the name forms tend to reach for. Use whichever reads
 * better at the call site; they render identically.
 */
export function FormField(props: FormFieldProps) {
  return <Field {...props} />;
}
