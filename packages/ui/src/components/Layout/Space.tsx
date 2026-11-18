import { type ThemeSpacingTokens } from '../../theme/createSpacing';
import { Box } from './Box/Box';
import { type ResponsiveProp } from './utils/responsiveness';

export interface SpaceProps {
  v?: ResponsiveProp<ThemeSpacingTokens>;
  h?: ResponsiveProp<ThemeSpacingTokens>;
  layout?: 'block' | 'inline';
}

export const Space = ({ v = 0, h = 0, layout }: SpaceProps) => {
  return <Box paddingRight={h} paddingBottom={v} display={layout === 'inline' ? 'inline-block' : 'block'} />;
};
