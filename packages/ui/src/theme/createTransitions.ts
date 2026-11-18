const easing = {

  easeInOut: 'cubic-bezier(0.4, 0, 0.2, 1)',

  easeOut: 'cubic-bezier(0.0, 0, 0.2, 1)',

  easeIn: 'cubic-bezier(0.4, 0, 1, 1)',

  sharp: 'cubic-bezier(0.4, 0, 0.6, 1)',
};

const duration = {
  shortest: 150,
  shorter: 200,
  short: 250,

  standard: 300,

  complex: 375,

  enteringScreen: 225,

  leavingScreen: 195,
};

interface CreateTransitionOptions {
  duration?: number | string;
  easing?: string;
  delay?: number | string;
}

function create(props: string | string[] = ['all'], options: CreateTransitionOptions = {}) {
  const { duration: durationOption = duration.standard, easing: easingOption = easing.easeInOut, delay = 0 } = options;

  return (Array.isArray(props) ? props : [props])
    .map(
      (animatedProp) =>
        `${animatedProp} ${
          typeof durationOption === 'string' ? durationOption : formatMs(durationOption)
        } ${easingOption} ${typeof delay === 'string' ? delay : formatMs(delay)}`
    )
    .join(',');
}

type ReducedMotionProps = 'no-preference' | 'reduce';

function handleMotion(...props: ReducedMotionProps[]) {
  return `@media ${props.map((prop) => `(prefers-reduced-motion: ${prop})`).join(',')}`;
}

function getAutoHeightDuration(height: number) {
  if (!height) {
    return 0;
  }

  const constant = height / 36;

  return Math.round((4 + 15 * constant ** 0.25 + constant / 5) * 10);
}

function formatMs(milliseconds: number) {
  return `${Math.round(milliseconds)}ms`;
}

export interface ThemeTransitions {
  create: typeof create;
  duration: typeof duration;
  easing: typeof easing;
  getAutoHeightDuration: typeof getAutoHeightDuration;
  handleMotion: typeof handleMotion;
}

export function createTransitions(): ThemeTransitions {
  return {
    create,
    duration,
    easing,
    getAutoHeightDuration,
    handleMotion,
  };
}
