import React, { useState, useEffect } from 'react';
import { X, Terminal, RotateCw, Copy, Check } from 'lucide-react';
import { DemoInstance } from '../types';

interface LogsModalProps {
  instance: DemoInstance | null;
  onClose: () => void;
}

export function LogsModal({ instance, onClose }: LogsModalProps) {
  const [logs, setLogs] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [copied, setCopied] = useState<boolean>(false);

  const fetchLogs = async () => {
    if (!instance) return;
    setIsLoading(true);
    try {
      const res = await fetch(`/api/instances/${instance.id}/logs?lines=150`);
      const data = await res.json();
      setLogs(data.logs || 'No output recorded.');
    } catch (err: any) {
      setLogs(`Error fetching logs: ${err.message}`);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (instance) {
      fetchLogs();
    }
  }, [instance]);

  if (!instance) return null;

  const handleCopy = () => {
    navigator.clipboard.writeText(logs);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-xs flex items-center justify-center p-4">
      <div className="bg-slate-900 rounded-2xl border border-slate-700 shadow-2xl max-w-3xl w-full overflow-hidden flex flex-col h-[520px]">
        {/* Terminal Header */}
        <div className="flex items-center justify-between px-4 py-3 bg-slate-800/80 border-b border-slate-700">
          <div className="flex items-center gap-2">
            <div className="flex items-center gap-1.5 mr-2">
              <div className="w-3 h-3 rounded-full bg-rose-500/80" />
              <div className="w-3 h-3 rounded-full bg-amber-500/80" />
              <div className="w-3 h-3 rounded-full bg-emerald-500/80" />
            </div>
            <Terminal className="w-4 h-4 text-slate-400" />
            <span className="text-xs font-mono text-slate-200">
              service/{instance.serviceName || instance.id} &bull; {instance.templateName}
            </span>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={fetchLogs}
              disabled={isLoading}
              className="p-1.5 rounded text-slate-400 hover:text-white hover:bg-slate-700 transition-colors"
              title="Refresh logs"
            >
              <RotateCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin' : ''}`} />
            </button>
            <button
              onClick={handleCopy}
              className="p-1.5 rounded text-slate-400 hover:text-white hover:bg-slate-700 transition-colors"
              title="Copy to clipboard"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
            <button
              onClick={onClose}
              className="p-1.5 rounded text-slate-400 hover:text-white hover:bg-slate-700 transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Terminal Content */}
        <div className="p-4 flex-1 overflow-y-auto font-mono text-[11px] text-slate-300 bg-slate-950/90 leading-relaxed whitespace-pre-wrap selection:bg-blue-600 selection:text-white">
          {isLoading && !logs ? (
            <div className="text-slate-500 animate-pulse">Streaming logs from Cloud Run service...</div>
          ) : (
            logs
          )}
        </div>
      </div>
    </div>
  );
}
