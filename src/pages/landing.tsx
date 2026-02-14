import { storeProvider } from '../auth';

interface LandingProps {
  apiUrl: string;
}

export function Landing({ apiUrl }: LandingProps) {
  return (
    <div class="min-h-screen bg-white text-black font-mono flex flex-col items-center justify-center px-4">
      <div class="max-w-2xl w-full text-center space-y-12">
        <div>
          <h1 class="text-6xl font-bold tracking-widest mb-4">VIRE</h1>
          <p class="text-lg">
            Connect your stock portfolio to Claude via MCP.
          </p>
        </div>

        <div class="space-y-4">
          <a
            href={`${apiUrl}/api/auth/login/google`}
            class="block border-2 border-black bg-black text-white text-center py-3 px-6 text-sm tracking-wide no-underline hover:bg-white hover:text-black"
            onClick={() => storeProvider('google')}
          >
            SIGN IN WITH GOOGLE
          </a>
          <a
            href={`${apiUrl}/api/auth/login/github`}
            class="block border-2 border-black bg-white text-black text-center py-3 px-6 text-sm tracking-wide no-underline hover:bg-black hover:text-white"
            onClick={() => storeProvider('github')}
          >
            SIGN IN WITH GITHUB
          </a>
        </div>

        <div class="border-t-2 border-black pt-8 grid grid-cols-1 md:grid-cols-3 gap-6 text-left text-sm">
          <div class="border-2 border-black p-4">
            <div class="font-bold mb-2">[01] API KEYS</div>
            <p>Bring your own EODHD, Navexa, and Gemini API keys. Your data, your keys.</p>
          </div>
          <div class="border-2 border-black p-4">
            <div class="font-bold mb-2">[02] MCP PROXY</div>
            <p>One-click provisioning of your dedicated MCP endpoint on Cloud Run.</p>
          </div>
          <div class="border-2 border-black p-4">
            <div class="font-bold mb-2">[03] CONNECT</div>
            <p>Copy your config into Claude Code or Claude Desktop and start querying.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
