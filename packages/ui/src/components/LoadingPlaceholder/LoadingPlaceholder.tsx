import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Spinner } from '../Spinner';
import { Text } from '../Text';

export interface LoadingPlaceholderProps {
  text: string | React.ReactNode;
  className?: string;
}

const getStyles = (theme: VyzorixTheme) => ({
  wrapper: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    padding: theme.spacing(2),
  }),
});

/** A spinner with a message, for a loading region. */
export function LoadingPlaceholder({ text, className }: LoadingPlaceholderProps) {
  const styles = useStyles2(getStyles);
  return (
    <div className={cx(styles.wrapper, className)} role="status">
      <Spinner size="sm" />
      <Text color="secondary">{text}</Text>
    </div>
  );
}
