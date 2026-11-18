import { type ResponsiveProp } from './utils/responsiveness';

export type AlignItems =
  | 'stretch'
  | 'flex-start'
  | 'flex-end'
  | 'center'
  | 'baseline'
  | 'start'
  | 'end'
  | 'self-start'
  | 'self-end';

export type JustifyContent =
  | 'flex-start'
  | 'flex-end'
  | 'center'
  | 'space-between'
  | 'space-around'
  | 'space-evenly'
  | 'stretch'
  | 'start'
  | 'end'
  | 'left'
  | 'right';

export type Direction = 'row' | 'row-reverse' | 'column' | 'column-reverse';
export type Wrap = boolean | 'nowrap' | 'wrap' | 'wrap-reverse';

type FlexGrow = number;
type FlexShrink = number;
type FlexBasis = 'auto' | 'initial' | '0' | `${number}%` | `${number}px`;

type Flex = FlexGrow | `${FlexGrow}` | `${FlexGrow} ${FlexShrink}` | `${FlexGrow} ${FlexShrink} ${FlexBasis}`;

export type FlexProps = {
  grow?: ResponsiveProp<FlexGrow>;
  shrink?: ResponsiveProp<FlexShrink>;
  basis?: ResponsiveProp<FlexBasis>;
  flex?: ResponsiveProp<Flex>;
};
