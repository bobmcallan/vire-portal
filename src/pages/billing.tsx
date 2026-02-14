
interface BillingProps {
  plan: string;
  onCheckout: () => void;
  onManage: () => void;
  sessionId: string | null;
}

export function Billing({ plan, onCheckout, onManage, sessionId }: BillingProps) {
  const isPro = plan === 'pro';

  return (
    <div class="space-y-8">
      <div class="border-b-2 border-black pb-4">
        <h1 class="text-2xl font-bold tracking-widest">BILLING</h1>
      </div>

      {sessionId && (
        <div class="border-2 border-black p-4 bg-black text-white">
          <p class="text-sm font-bold">
            [OK] Payment processing. Your plan will be upgraded shortly.
          </p>
        </div>
      )}

      <div class="border-2 border-black p-6">
        <h2 class="text-sm font-bold tracking-wide mb-4">CURRENT PLAN</h2>
        <div class="flex items-center gap-4">
          <span class="border-2 border-black px-4 py-2 text-lg font-bold">
            {plan.toUpperCase()}
          </span>
          {isPro && (
            <span class="text-sm">Active subscription</span>
          )}
        </div>
      </div>

      <div class="border-2 border-black">
        <div class="grid grid-cols-1 md:grid-cols-2">
          <div class="p-6 border-b-2 md:border-b-0 md:border-r-2 border-black">
            <h3 class="font-bold mb-2">FREE</h3>
            <ul class="text-sm space-y-1 mb-4">
              <li>- 1,000 requests/month</li>
              <li>- Basic MCP tools</li>
              <li>- Community support</li>
            </ul>
            {!isPro && (
              <span class="text-sm font-bold">[CURRENT]</span>
            )}
          </div>
          <div class="p-6">
            <h3 class="font-bold mb-2">PRO</h3>
            <ul class="text-sm space-y-1 mb-4">
              <li>- 10,000 requests/month</li>
              <li>- All MCP tools</li>
              <li>- Priority support</li>
            </ul>
            {isPro ? (
              <span class="text-sm font-bold">[CURRENT]</span>
            ) : (
              <button
                class="bg-black text-white border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-white hover:text-black"
                onClick={onCheckout}
              >
                UPGRADE TO PRO
              </button>
            )}
          </div>
        </div>
      </div>

      {isPro && (
        <button
          class="bg-white text-black border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-black hover:text-white"
          onClick={onManage}
        >
          MANAGE SUBSCRIPTION
        </button>
      )}
    </div>
  );
}
