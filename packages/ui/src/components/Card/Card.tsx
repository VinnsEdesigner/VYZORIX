import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Text } from '../Text';

export interface CardProps {
  /** Optional heading; also sets the accessible name. */
  heading?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  /** Remove the inner padding, for a card that manages its own layout. */
  noPadding?: boolean;
  as?: 'section' | 'article' | 'div';
}

function getStyles(theme: VyzorixTheme, noPadding?: boolean) {
  return {
    card: css({
      display: 'flex',
      flexDirection: 'column',
      background: theme.colors.background.primary,
      border: `1px solid ${theme.colors.border.medium}`,
      borderRadius: theme.shape.radius.default,
      overflow: 'hidden',
    }),
    header: css({
      padding: `${theme.spacing(1.5)} ${theme.spacing(2)}`,
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      background: theme.colors.background.secondary,
    }),
    body: css({ padding: noPadding ? 0 : theme.spacing(2), flex: 1 }),
  };
}

/** A bordered surface with an optional header. */
export function Card({ heading, children, className, noPadding, as = 'section' }: CardProps) {
  const styles = useStyles2(getStyles, noPadding);
  const Tag = as;

  return (
    <Tag className={cx(styles.card, className)} aria-label={typeof heading === 'string' ? heading : undefined}>
      {heading ? (
        <div className={styles.header}>
          <Text variant="md" weight="medium" element="div">
            {heading}
          </Text>
        </div>
      ) : null}
      <div className={styles.body}>{children}</div>
    </Tag>
  );
}
