import { forwardRef } from 'react';
import { Button, type ButtonFill, type ButtonSize, type ButtonVariant } from '../Button';
import { type IconName } from '../Icon';

export interface IconButtonProps extends Omit<React.ComponentProps<typeof Button>, 'children' | 'icon'> {

  name: IconName;

  'aria-label': string;
  variant?: ButtonVariant;
  fill?: ButtonFill;
  size?: ButtonSize;
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ name, size = 'md', ...rest }, ref) => (
    <Button ref={ref} icon={name} size={size} {...rest} />
  ),
);

IconButton.displayName = 'IconButton';
