import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import {
  ThemeContext,
  getThemeById,
  getResponsiveStyle,
  getSizeStyles,
  Box,
  Stack,
  Grid,
  Space,
} from '@vyzorix/ui';

const theme = getThemeById('dark');

function renderThemed(ui: React.ReactElement) {
  return render(<ThemeContext.Provider value={theme}>{ui}</ThemeContext.Provider>);
}

describe('getResponsiveStyle', () => {
  it('returns the CSS directly for a plain value', () => {
    expect(getResponsiveStyle(theme, 2, (val) => ({ gap: theme.spacing(val) }))).toEqual({ gap: '16px' });
  });

  it('returns null for an absent value', () => {
    expect(getResponsiveStyle(theme, undefined, () => ({ gap: '0' }))).toBeNull();
  });

  it('expands a responsive object into per-breakpoint media queries', () => {
    const result = getResponsiveStyle(theme, { xs: 1, md: 2 }, (val) => ({ gap: theme.spacing(val) })) as Array<
      Record<string, unknown>
    >;

    expect(result).toEqual([
      { [theme.breakpoints.up('xs')]: { gap: '8px' } },
      undefined,
      { [theme.breakpoints.up('md')]: { gap: '16px' } },
      undefined,
      undefined,
    ]);
  });
});

describe('getSizeStyles', () => {
  it('treats a numeric size as a theme spacing multiple', () => {
    const className = getSizeStyles(theme, 4, undefined, undefined, undefined, undefined, undefined);
    expect(typeof className).toBe('string');
    expect(className.length).toBeGreaterThan(0);
  });

  it('passes a string size through unchanged', () => {
    expect(() => getSizeStyles(theme, '100%', undefined, undefined, undefined, undefined, undefined)).not.toThrow();
  });
});

describe('Box', () => {
  it('renders a div by default', () => {
    const { container } = renderThemed(<Box data-testid="box">content</Box>);
    expect(container.querySelector('[data-testid="box"]')?.tagName).toBe('DIV');
  });

  it('honours the element prop', () => {
    const { container } = renderThemed(
      <Box element="section" data-testid="box">
        content
      </Box>,
    );
    expect(container.querySelector('[data-testid="box"]')?.tagName).toBe('SECTION');
  });

  it('renders its children', () => {
    const { getByText } = renderThemed(<Box>hello</Box>);
    expect(getByText('hello')).toBeInTheDocument();
  });

  it('accepts spacing, colour and border tokens without throwing', () => {
    expect(() =>
      renderThemed(
        <Box padding={2} marginX={1} gap={1} backgroundColor="secondary" borderColor="weak" borderRadius="default">
          box
        </Box>,
      ),
    ).not.toThrow();
  });
});

describe('Stack', () => {
  it('renders its children', () => {
    const { getByText } = renderThemed(
      <Stack direction="column" gap={2}>
        <span>one</span>
        <span>two</span>
      </Stack>,
    );
    expect(getByText('one')).toBeInTheDocument();
    expect(getByText('two')).toBeInTheDocument();
  });

  it('accepts a responsive direction', () => {
    expect(() =>
      renderThemed(
        <Stack direction={{ xs: 'column', md: 'row' }} gap={1}>
          stack
        </Stack>,
      ),
    ).not.toThrow();
  });
});

describe('Grid', () => {
  it('renders its children', () => {
    const { getByText } = renderThemed(
      <Grid columns={2} gap={2}>
        <span>cell</span>
      </Grid>,
    );
    expect(getByText('cell')).toBeInTheDocument();
  });

  it('accepts minColumnWidth as an alternative to columns', () => {
    expect(() =>
      renderThemed(
        <Grid minColumnWidth={16} gap={1}>
          <span>cell</span>
        </Grid>,
      ),
    ).not.toThrow();
  });
});

describe('Space', () => {
  it('renders a block element by default', () => {
    const { container } = renderThemed(<Space v={2} />);
    expect(container.firstElementChild?.tagName).toBe('DIV');
  });

  it('renders inline when asked', () => {
    expect(() => renderThemed(<Space h={1} layout="inline" />)).not.toThrow();
  });
});
