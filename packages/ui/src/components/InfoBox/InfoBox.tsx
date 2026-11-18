import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';
import { Text } from '../Text';

export interface InfoBoxProps {
  title: string;
  children?: React.ReactNode;
  icon?: IconName;
  className?: string;
}

const getStyles = (theme: VyzorixTheme) => ({
  box: css({
    display: 'flex',
    gap: theme.spacing(1),
    padding: theme.spacing(1.5),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.secondary,
  }),
  body: css({ flex: 1 }),
});

export function InfoBox({ title, children, icon = 'info', className }: InfoBoxProps) {
  const styles = useStyles2(getStyles);
  return (
    <div className={cx(styles.box, className)}>
      <Icon name={icon} size="md" />
      <div className={styles.body}>
        <Text variant="base" weight="medium" element="div">
          {title}
        </Text>
        {children ? <Text variant="sm" color="secondary">{children}</Text> : null}
      </div>
    </div>
  );
}
