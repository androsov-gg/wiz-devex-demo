'use client';

import React, { useState } from 'react';
import {
  ExternalLink,
  Terminal,
  RotateCw,
  Trash2,
  Clock,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  Code2,
} from 'lucide-react';
import { DemoInstance } from '@/lib/types';

interface InstanceRowProps {
  instance: DemoInstance;
  onOpenLogs: (instance: DemoInstance) => void;
  onRestart: (instanceId: string) => void;
  onTeardown: (instanceId: string) => void;
}

export function InstanceRow({
  instance,
  onOpenLogs,
  onRestart,
  onTeardown,
}: InstanceRowProps) {
  // Compute uptime / creation age
  const createdDate = new Date(instance.createdAt);
  const now = new Date();
  const diffMs = Math.max(0, now.getTime() - createdDate.getTime());
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const hoursAgo = Math.floor(diffMins / 60);
  const minsAgo = diffMins % 60;
  const timeDisplay = hoursAgo > 0 ? `${hoursAgo}h ${minsAgo}m ago` : `${minsAgo}m ago`;

  // Action capability helpers based on status
  const canViewLogs = instance.status === 'RUNNING' || instance.status === 'PROVISIONING' || instance.status === 'RESTARTING' || instance.status === 'FAILED';
  const canRestart = instance.status === 'RUNNING';
  const canTeardown = instance.status !== 'STOPPING';
  const isStopping = instance.status === 'STOPPING';

  // Status badge styling
  const renderStatusBadge = () => {
    switch (instance.status) {
      case 'RUNNING':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
            <CheckCircle2 className="w-3 h-3 text-emerald-600" />
            Ready / Running
          </span>
        );
      case 'RESTARTING':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200 animate-pulse">
            <Loader2 className="w-3 h-3 text-indigo-600 animate-spin" />
            Restarting Service
          </span>
        );
      case 'PROVISIONING':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200 animate-pulse">
            <Loader2 className="w-3 h-3 text-amber-600 animate-spin" />
            {instance.cloudRunReadyAt ? 'Propagating IAP Access...' : 'Deploying Service'}
          </span>
        );
      case 'BUILDING_IMAGE':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-orange-50 text-orange-700 border border-orange-200 animate-pulse">
            <Loader2 className="w-3 h-3 text-orange-600 animate-spin" />
            Building Image (Cloud Build)
          </span>
        );
      case 'STOPPING':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-100 text-slate-600 border border-slate-200">
            <Loader2 className="w-3 h-3 animate-spin" />
            Stopping...
          </span>
        );
      case 'FAILED':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200">
            <AlertTriangle className="w-3 h-3 text-rose-600" />
            Failed
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200">
            {instance.status}
          </span>
        );
    }
  };

  return (
    <tr className="hover:bg-slate-50/80 transition-colors border-b border-slate-100 text-xs text-slate-700">
      {/* Demo Name & ID */}
      <td className="py-3.5 pl-6 pr-4">
        <div className="flex items-center gap-3">
          <div className="w-7 h-7 rounded-md bg-amber-50 border border-amber-200 text-amber-600 flex items-center justify-center shrink-0">
            <Code2 className="w-4 h-4" />
          </div>
          <div>
            <div className="flex items-center gap-1.5 font-semibold text-slate-900 leading-tight">
              <span>{instance.templateName}</span>
              {instance.version && (
                <span className="px-1.5 py-0.2 rounded text-[10px] font-mono bg-blue-50 text-blue-600 border border-blue-200">
                  {instance.version}
                </span>
              )}
            </div>
            <div className="text-[11px] text-slate-400 font-mono mt-0.5">
              ID: {instance.id}
            </div>
          </div>
        </div>
      </td>

      {/* Status Badge */}
      <td className="py-3.5 px-4">{renderStatusBadge()}</td>

      {/* Created / Uptime */}
      <td className="py-3.5 px-4">
        <div className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 text-slate-600">
          <Clock className="w-3 h-3 text-slate-400" />
          <span>{timeDisplay}</span>
        </div>
      </td>

      {/* Owner / Assigned CE */}
      <td className="py-3.5 px-4">
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded-full bg-blue-100 text-blue-700 font-semibold text-[10px] flex items-center justify-center">
            {instance.ownerName.substring(0, 2).toUpperCase()}
          </div>
          <span className="text-slate-800 font-medium">{instance.ownerEmail}</span>
        </div>
      </td>

      {/* Direct Open Link */}
      <td className="py-3.5 px-4">
        {instance.status === 'RUNNING' && instance.serviceUrl ? (
          <a
            href={instance.serviceUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white font-medium shadow-2xs hover:shadow-sm transition-all"
          >
            <ExternalLink className="w-3.5 h-3.5" />
            <span>Open IDE</span>
          </a>
        ) : instance.status === 'RESTARTING' ? (
          <span className="text-indigo-600 italic text-[11px] flex items-center gap-1">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span>Restarting...</span>
          </span>
        ) : instance.status === 'PROVISIONING' ? (
          <span className="text-amber-600 italic text-[11px] flex items-center gap-1">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span>{instance.cloudRunReadyAt ? 'Propagating IAP...' : 'Provisioning...'}</span>
          </span>
        ) : instance.status === 'BUILDING_IMAGE' ? (
          <span className="text-orange-600 italic text-[11px] flex items-center gap-1">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span>Building Image...</span>
          </span>
        ) : instance.status === 'STOPPING' ? (
          <span className="text-slate-400 italic text-[11px]">Stopping...</span>
        ) : instance.status === 'FAILED' ? (
          <span className="text-rose-500 font-medium text-[11px]">Unavailable</span>
        ) : (
          <span className="text-slate-400 italic text-[11px]">—</span>
        )}
      </td>

      {/* Actions */}
      <td className="py-3.5 pr-6 pl-4 text-right">
        {isStopping ? (
          <div className="inline-flex items-center gap-1 text-slate-400 text-[11px] italic">
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
            <span>Stopping...</span>
          </div>
        ) : (
          <div className="relative inline-flex items-center gap-1">
            {canViewLogs && (
              <button
                onClick={() => onOpenLogs(instance)}
                className="p-1.5 rounded text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 transition-colors"
                title="View Live Logs"
              >
                <Terminal className="w-4 h-4" />
              </button>
            )}
            {canRestart && (
              <button
                onClick={() => onRestart(instance.id)}
                className="p-1.5 rounded text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 transition-colors"
                title="Restart Instance"
              >
                <RotateCw className="w-4 h-4" />
              </button>
            )}
            {canTeardown && (
              <button
                onClick={() => onTeardown(instance.id)}
                className="p-1.5 rounded text-rose-400 hover:text-rose-600 hover:bg-rose-50 transition-colors"
                title="Teardown Demo"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        )}
      </td>
    </tr>
  );
}
