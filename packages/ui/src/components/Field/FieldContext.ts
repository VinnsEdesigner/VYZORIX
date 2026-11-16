import { createContext, useContext } from 'react';

/**
 * The wiring a form control needs from its surrounding `Field`.
 *
 * This is passed down through context rather than by cloning the child, so a
 * control participates in labelling simply by calling `useFieldContext()`. That
 * keeps `<Field><Input/></Field>` working with no render prop or prop drilling,
 * and it is why `Input`/`Select`/etc. read this.
 */
export interface FieldContextValue {
  id: string;
  'aria-describedby'?: string;
  'aria-invalid'?: boolean;
}

export const FieldContext = createContext<FieldContextValue | null>(null);

/** Read the surrounding field's wiring, or null when not inside a `Field`. */
export function useFieldContext(): FieldContextValue | null {
  return useContext(FieldContext);
}
