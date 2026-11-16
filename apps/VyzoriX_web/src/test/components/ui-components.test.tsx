import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  Collapse,
  Divider,
  EmptyState,
  Field,
  Icon,
  IconButton,
  InfoBox,
  Input,
  Link,
  LoadingPlaceholder,
  RadioButtonGroup,
  Select,
  Spinner,
  Switch,
  Tabs,
  Tag,
  Text,
  TextArea,
} from '@vyzorix/ui';

/** Render a component inside the real theme context, as the app does. */
function renderThemed(ui: React.ReactElement, themeId = 'dark') {
  return render(<ThemeContext.Provider value={getThemeById(themeId)}>{ui}</ThemeContext.Provider>);
}

describe('Icon', () => {
  it('renders a known icon', () => {
    renderThemed(<Icon name="dashboard" />);
    expect(screen.getByTestId('icon-dashboard')).toBeInTheDocument();
  });

  it('is hidden from assistive tech when decorative', () => {
    renderThemed(<Icon name="cog" />);
    expect(screen.getByTestId('icon-cog')).toHaveAttribute('aria-hidden', 'true');
  });

  it('is exposed when given a title', () => {
    renderThemed(<Icon name="cog" title="Settings" />);
    expect(screen.getByTestId('icon-cog')).not.toHaveAttribute('aria-hidden');
  });

  it('warns and renders nothing for an unknown name', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const { container } = renderThemed(<Icon name={'nope' as never} />);
    expect(container).toBeEmptyDOMElement();
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });
});

describe('Text', () => {
  it('renders the variant as the matching heading element', () => {
    renderThemed(<Text variant="h2">Heading</Text>);
    expect(screen.getByText('Heading').tagName).toBe('H2');
  });

  it('defaults to a paragraph', () => {
    renderThemed(<Text>Body</Text>);
    expect(screen.getByText('Body').tagName).toBe('P');
  });
});

describe('Button', () => {
  it('fires onClick', () => {
    const onClick = vi.fn();
    renderThemed(<Button onClick={onClick}>Save</Button>);
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('does not fire when disabled', () => {
    const onClick = vi.fn();
    renderThemed(
      <Button disabled onClick={onClick}>
        Save
      </Button>,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(onClick).not.toHaveBeenCalled();
  });

  it('renders an icon when given one', () => {
    renderThemed(<Button icon="plus">Add</Button>);
    expect(screen.getByTestId('icon-plus')).toBeInTheDocument();
  });
});

describe('IconButton', () => {
  it('requires and applies an accessible label', () => {
    renderThemed(<IconButton name="trash" aria-label="Delete device" />);
    expect(screen.getByRole('button', { name: 'Delete device' })).toBeInTheDocument();
  });
});

describe('form controls', () => {
  it('Input accepts typing', () => {
    renderThemed(<Input placeholder="name" />);
    const input = screen.getByPlaceholderText('name');
    fireEvent.change(input, { target: { value: 'hello' } });
    expect(input).toHaveValue('hello');
  });

  it('Input marks invalid state for assistive tech', () => {
    renderThemed(<Input invalid aria-label="email" />);
    expect(screen.getByLabelText('email')).toHaveAttribute('aria-invalid', 'true');
  });

  it('TextArea accepts typing', () => {
    renderThemed(<TextArea placeholder="notes" />);
    const area = screen.getByPlaceholderText('notes');
    fireEvent.change(area, { target: { value: 'line' } });
    expect(area).toHaveValue('line');
  });

  it('Select renders its options', () => {
    renderThemed(
      <Select aria-label="theme">
        <option value="dark">Dark</option>
        <option value="light">Light</option>
      </Select>,
    );
    expect(screen.getByRole('combobox', { name: 'theme' })).toBeInTheDocument();
    expect(screen.getAllByRole('option')).toHaveLength(2);
  });

  it('Switch toggles', () => {
    renderThemed(<Switch label="Enabled" />);
    const sw = screen.getByRole('checkbox', { name: 'Enabled' });
    fireEvent.click(sw);
    expect(sw).toBeChecked();
  });

  it('Checkbox toggles', () => {
    renderThemed(<Checkbox label="Agree" />);
    const cb = screen.getByRole('checkbox', { name: 'Agree' });
    fireEvent.click(cb);
    expect(cb).toBeChecked();
  });

  it('RadioButtonGroup selects on click', () => {
    const onChange = vi.fn();
    renderThemed(
      <RadioButtonGroup
        aria-label="range"
        value="1h"
        onChange={onChange}
        options={[
          { label: '1h', value: '1h' },
          { label: '24h', value: '24h' },
        ]}
      />,
    );
    fireEvent.click(screen.getByRole('radio', { name: '24h' }));
    expect(onChange).toHaveBeenCalledWith('24h');
  });

  it('RadioButtonGroup marks the active option checked', () => {
    renderThemed(
      <RadioButtonGroup
        aria-label="range"
        value="24h"
        onChange={() => {}}
        options={[
          { label: '1h', value: '1h' },
          { label: '24h', value: '24h' },
        ]}
      />,
    );
    expect(screen.getByRole('radio', { name: '24h' })).toHaveAttribute('aria-checked', 'true');
  });
});

describe('Field', () => {
  it('associates the label with the control', () => {
    renderThemed(
      <Field label="Device name">
        <Input />
      </Field>,
    );
    expect(screen.getByLabelText('Device name')).toBeInTheDocument();
  });

  it('announces the error message', () => {
    renderThemed(
      <Field label="Email" error="Invalid address">
        <Input />
      </Field>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Invalid address');
  });
});

describe('overlays and structure', () => {
  it('Tabs reports selection', () => {
    renderThemed(
      <Tabs
        aria-label="views"
        value="a"
        onChange={() => {}}
        tabs={[
          { label: 'One', value: 'a' },
          { label: 'Two', value: 'b' },
        ]}
      />,
    );
    expect(screen.getByRole('tab', { name: 'One' })).toHaveAttribute('aria-selected', 'true');
  });

  it('Collapse expands on click', () => {
    renderThemed(
      <Collapse label="Details">
        <span>hidden</span>
      </Collapse>,
    );
    expect(screen.queryByText('hidden')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Details/ }));
    expect(screen.getByText('hidden')).toBeInTheDocument();
  });

  it('Badge renders its text', () => {
    renderThemed(<Badge text="Online" color="success" />);
    expect(screen.getByText('Online')).toBeInTheDocument();
  });

  it('Tag renders its name', () => {
    renderThemed(<Tag name="production" />);
    expect(screen.getByText('production')).toBeInTheDocument();
  });

  it('Alert has the alert role', () => {
    renderThemed(<Alert title="Something failed" severity="error" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Something failed');
  });

  it('InfoBox renders title and body', () => {
    renderThemed(<InfoBox title="Note">Extra detail</InfoBox>);
    expect(screen.getByText('Note')).toBeInTheDocument();
    expect(screen.getByText('Extra detail')).toBeInTheDocument();
  });

  it('Spinner renders a status region', () => {
    renderThemed(<Spinner />);
    expect(screen.getByTestId('Spinner')).toBeInTheDocument();
  });

  it('LoadingPlaceholder shows its message', () => {
    renderThemed(<LoadingPlaceholder text="Loading devices" />);
    expect(screen.getByText('Loading devices')).toBeInTheDocument();
  });

  it('EmptyState shows its message', () => {
    renderThemed(<EmptyState message="No devices yet" />);
    expect(screen.getByText('No devices yet')).toBeInTheDocument();
  });

  it('Card renders a heading and content', () => {
    renderThemed(
      <Card heading="Summary">
        <span>body</span>
      </Card>,
    );
    expect(screen.getByText('Summary')).toBeInTheDocument();
    expect(screen.getByText('body')).toBeInTheDocument();
  });

  it('Divider renders a separator', () => {
    const { container } = renderThemed(<Divider />);
    expect(container.querySelector('hr')).toBeInTheDocument();
  });

  it('Link marks external links safe', () => {
    renderThemed(
      <Link href="https://example.com" external>
        Docs
      </Link>,
    );
    const link = screen.getByRole('link', { name: /Docs/ });
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link).toHaveAttribute('target', '_blank');
  });
});
