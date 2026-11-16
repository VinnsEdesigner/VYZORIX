import { forwardRef } from 'react';
import { Button, type ButtonFill, type ButtonSize, type ButtonVariant } from '../Button';
import { type IconName } from '../Icon';

export interface IconButtonProps extends Omit<React.ComponentProps<typeof Button>, 'children' | 'icon'> {
  /** The icon to render. */
  name: IconName;
  /** Required: an icon-only control has no visible text to announce. */
  'aria-label': string;
  variant?: ButtonVariant;
  fill?: ButtonFill;
  size?: ButtonSize;
}

/**
 * A button whose only content is an icon.
 *
 * Separate from `Button` so the accessibility requirement is enforced by the
 * type: `aria-label` is not optional here.
 */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ name, size = 'md', ...rest }, ref) => (
    <Button ref={ref} icon={name} size={size} {...rest} />
  ),
);

IconButton.displayName = 'IconButton';
