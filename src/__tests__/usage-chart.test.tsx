import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';

/**
 * Tests for src/components/usage-chart.tsx
 *
 * The usage-chart component displays:
 * - Quota bar: CSS width percentage, black fill on white background
 * - Daily trend bars: CSS flex with height percentage, solid black bars
 * - Monochrome styling throughout
 */

describe('UsageChart component', () => {
  it('renders a quota progress bar', async () => {
    const { UsageChart } = await import('../components/usage-chart');
    const { container } = render(
      <UsageChart
        totalRequests={3421}
        quotaLimit={10000}
        dailyCounts={[
          { date: '2026-02-01', count: 120 },
          { date: '2026-02-02', count: 185 },
        ]}
      />,
    );
    const bar = container.querySelector('[role="progressbar"], [data-testid="quota-bar"]');
    expect(bar).toBeTruthy();
  });

  it('sets correct progress percentage on quota bar', async () => {
    const { UsageChart } = await import('../components/usage-chart');
    const { container } = render(
      <UsageChart
        totalRequests={5000}
        quotaLimit={10000}
        dailyCounts={[]}
      />,
    );
    const bar = container.querySelector('[role="progressbar"]');
    expect(bar).toBeTruthy();
    const value = bar!.getAttribute('aria-valuenow');
    expect(value).toBe('50');
  });

  it('renders daily trend bars', async () => {
    const { UsageChart } = await import('../components/usage-chart');
    const { container } = render(
      <UsageChart
        totalRequests={500}
        quotaLimit={10000}
        dailyCounts={[
          { date: '2026-02-01', count: 120 },
          { date: '2026-02-02', count: 185 },
          { date: '2026-02-03', count: 195 },
        ]}
      />,
    );
    const bars = container.querySelectorAll('[data-testid="daily-bar"]');
    expect(bars.length).toBe(3);
  });

  it('displays total request count', async () => {
    const { UsageChart } = await import('../components/usage-chart');
    const { container } = render(
      <UsageChart
        totalRequests={3421}
        quotaLimit={10000}
        dailyCounts={[]}
      />,
    );
    expect(container.textContent).toMatch(/3,?421/);
  });

  it('displays quota limit', async () => {
    const { UsageChart } = await import('../components/usage-chart');
    const { container } = render(
      <UsageChart
        totalRequests={3421}
        quotaLimit={10000}
        dailyCounts={[]}
      />,
    );
    expect(container.textContent).toMatch(/10,?000/);
  });
});
