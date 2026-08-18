'use client';

import React from 'react';
import { DemoInstance } from '@/lib/types';
import { InstanceRow } from './InstanceRow';
import { PlayCircle, Layers, Plus } from 'lucide-react';

interface InstanceTableProps {
  instances: DemoInstance[];
  isLoading: boolean;
  onOpenLogs: (instance: DemoInstance) => void;
  onRestart: (instanceId: string) => void;
  onTeardown: (instanceId: string) => void;
  onLaunchNew: () => void;
  onBrowseCatalog?: () => void;
}

export function InstanceTable({
  instances,
  isLoading,
  onOpenLogs,
  onRestart,
  onTeardown,
  onLaunchNew,
  onBrowseCatalog,
}: InstanceTableProps) {
  if (isLoading) {
    return (
      <div className="w-full bg-white rounded-xl border border-slate-200 p-12 text-center">
        <div className="animate-spin w-8 h-8 border-3 border-blue-600 border-t-transparent rounded-full mx-auto mb-3" />
        <p className="text-xs text-slate-500 font-medium">Synchronizing with Cloud Run & Firestore...</p>
      </div>
    );
  }

  if (instances.length === 0) {
    return (
      <div className="w-full bg-white rounded-xl border border-slate-200 p-12 text-center">
        <div className="w-12 h-12 rounded-full bg-blue-50 text-blue-600 flex items-center justify-center mx-auto mb-3">
          <PlayCircle className="w-6 h-6" />
        </div>
        <h3 className="text-sm font-semibold text-slate-900 mb-1">No Active Demo Instances</h3>
        <p className="text-xs text-slate-500 max-w-sm mx-auto mb-4">
          There are currently no demo environments running. Launch a pre-configured demo from the catalog to begin your demonstration.
        </p>
        <div className="flex items-center justify-center gap-3">
          <button
            onClick={onLaunchNew}
            className="bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold px-4 py-2 rounded-lg shadow-sm transition-all flex items-center gap-1.5"
          >
            <Plus className="w-3.5 h-3.5" />
            <span>Launch First Demo</span>
          </button>
          {onBrowseCatalog && (
            <button
              onClick={onBrowseCatalog}
              className="bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-semibold px-4 py-2 rounded-lg transition-all flex items-center gap-1.5"
            >
              <Layers className="w-3.5 h-3.5" />
              <span>Browse Catalog</span>
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="w-full bg-white rounded-xl border border-slate-200 overflow-hidden shadow-2xs">
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="bg-slate-50/70 border-b border-slate-200 text-[11px] font-semibold text-slate-500 uppercase tracking-wider">
              <th className="py-3 pl-6 pr-4">Demo Environment</th>
              <th className="py-3 px-4">Status</th>
              <th className="py-3 px-4">Created</th>
              <th className="py-3 px-4">Assigned CE</th>
              <th className="py-3 px-4">Direct Link</th>
              <th className="py-3 pr-6 pl-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {instances.map((inst) => (
              <InstanceRow
                key={inst.id}
                instance={inst}
                onOpenLogs={onOpenLogs}
                onRestart={onRestart}
                onTeardown={onTeardown}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
