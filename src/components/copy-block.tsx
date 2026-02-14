import { useState } from 'preact/hooks';

interface CopyBlockProps {
  title: string;
  content: string;
}

export function CopyBlock({ title, content }: CopyBlockProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div class="border-2 border-black mb-6">
      <div class="flex items-center justify-between border-b-2 border-black px-4 py-2 bg-black text-white">
        <span class="text-sm font-bold tracking-wide">{title}</span>
        <button
          class="bg-white text-black border-none px-3 py-1 text-xs font-mono cursor-pointer hover:bg-black hover:text-white hover:outline hover:outline-1 hover:outline-white"
          onClick={handleCopy}
        >
          {copied ? 'COPIED' : 'COPY'}
        </button>
      </div>
      <pre class="p-4 overflow-x-auto text-sm whitespace-pre-wrap break-all bg-white text-black m-0">
        {content}
      </pre>
      <div aria-live="polite" class="sr-only">
        {copied ? 'Copied to clipboard' : ''}
      </div>
    </div>
  );
}
