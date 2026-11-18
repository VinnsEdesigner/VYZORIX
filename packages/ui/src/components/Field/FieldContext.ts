import { createContext, useContext } from 'react';

export interface FieldContextValue {
  id: string;
  'aria-describedby'?: string;
  'aria-invalid'?: boolean;
}

export const FieldContext = createContext<FieldContextValue | null>(null);

export function useFieldContext(): FieldContextValue | null {
  return useContext(FieldContext);
}
