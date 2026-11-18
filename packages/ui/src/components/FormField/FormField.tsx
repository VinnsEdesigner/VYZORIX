import { Field, type FieldProps } from '../Field';

export interface FormFieldProps extends Omit<FieldProps, 'children'> {

  label: string;

  children: React.ReactNode;
}

export function FormField(props: FormFieldProps) {
  return <Field {...props} />;
}
