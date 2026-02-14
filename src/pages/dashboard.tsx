import type { UsageData } from '../types';
import { UsageChart } from '../components/usage-chart';

interface DashboardProps {
  usage: UsageData;
  user: { plan: string; proxy_status?: string };
}

export function Dashboard({ usage, user }: DashboardProps) {
  const statusLabel = (user.proxy_status ?? 'not_provisioned').toUpperCase();

  return (
    <div class="space-y-8">
      <div class="flex items-center justify-between border-b-2 border-black pb-4">
        <h1 class="text-2xl font-bold tracking-widest">DASHBOARD</h1>
        <div class="flex items-center gap-4">
          <span class="border-2 border-black px-3 py-1 text-xs font-bold">
            {user.plan.toUpperCase()}
          </span>
          <span class="text-sm">[{statusLabel}]</span>
        </div>
      </div>

      <div class="border-2 border-black p-6">
        <h2 class="text-sm font-bold tracking-wide mb-4">USAGE — {usage.period}</h2>
        <UsageChart
          totalRequests={usage.total_requests}
          quotaLimit={usage.quota_limit}
          dailyCounts={usage.daily_counts}
        />
      </div>

      {usage.top_endpoints.length > 0 && (
        <div class="border-2 border-black">
          <div class="bg-black text-white px-4 py-2 text-sm font-bold tracking-wide">
            TOP ENDPOINTS
          </div>
          <table class="w-full text-sm">
            <tbody>
              {usage.top_endpoints.map((ep) => (
                <tr key={ep.endpoint} class="border-b border-black last:border-b-0">
                  <td class="px-4 py-2 font-mono">{ep.endpoint}</td>
                  <td class="px-4 py-2 text-right">{ep.count.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
