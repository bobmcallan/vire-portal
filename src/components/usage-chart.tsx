
interface UsageChartProps {
  totalRequests: number;
  quotaLimit: number;
  dailyCounts: Array<{ date: string; count: number }>;
}

export function UsageChart({ totalRequests, quotaLimit, dailyCounts }: UsageChartProps) {
  const percentage = Math.round((totalRequests / quotaLimit) * 100);
  const maxDaily = Math.max(...dailyCounts.map((d) => d.count), 1);

  return (
    <div class="space-y-6">
      <div>
        <div class="flex justify-between text-sm mb-1">
          <span>{totalRequests.toLocaleString()} requests</span>
          <span>{quotaLimit.toLocaleString()} limit</span>
        </div>
        <div
          class="border-2 border-black h-6 bg-white"
          data-testid="quota-bar"
          role="progressbar"
          aria-valuenow={percentage}
          aria-valuemin={0}
          aria-valuemax={100}
        >
          <div
            class="bg-black h-full"
            style={{ width: `${percentage}%` }}
          />
        </div>
      </div>

      {dailyCounts.length > 0 && (
        <div>
          <div class="text-sm font-bold mb-2">DAILY TREND</div>
          <div class="flex items-end gap-1 h-24">
            {dailyCounts.map((day) => (
              <div
                key={day.date}
                data-testid="daily-bar"
                class="flex-1 bg-black"
                style={{ height: `${(day.count / maxDaily) * 100}%` }}
                title={`${day.date}: ${day.count}`}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
