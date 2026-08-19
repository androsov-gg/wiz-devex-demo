import React from 'react';
import { Play, Cpu, HardDrive, Code2, ShieldAlert, Layers, ShieldCheck } from 'lucide-react';
import { DemoTemplate } from '../types';

interface CatalogCardProps {
  template: DemoTemplate;
  onLaunch: (template: DemoTemplate) => void;
}

export function CatalogCard({ template, onLaunch }: CatalogCardProps) {
  const renderIcon = () => {
    switch (template.icon) {
      case 'Code2':
        return <Code2 className="w-5 h-5 text-blue-600" />;
      case 'ShieldAlert':
        return <ShieldAlert className="w-5 h-5 text-amber-600" />;
      case 'Layers':
        return <Layers className="w-5 h-5 text-emerald-600" />;
      default:
        return <ShieldCheck className="w-5 h-5 text-blue-600" />;
    }
  };

  const versionCount = template.versions?.length || 1;
  const latestVersion = template.versions?.[0]?.version || '1.0.0';

  return (
    <div className="bg-white rounded-xl border border-slate-200 p-5 hover:border-blue-300 hover:shadow-md transition-all flex flex-col justify-between group">
      <div>
        {/* Top Header */}
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="w-10 h-10 rounded-lg bg-blue-50 border border-blue-100 flex items-center justify-center shrink-0">
            {renderIcon()}
          </div>
          <div className="flex items-center gap-1.5 flex-wrap justify-end">
            <span className="px-2 py-0.5 rounded-full text-[10px] font-mono font-medium bg-blue-50 text-blue-700 border border-blue-200">
              {versionCount > 1 ? `${versionCount} versions (latest: v${latestVersion})` : `v${latestVersion}`}
            </span>
          </div>
        </div>

        {/* Title & Description */}
        <h3 className="text-sm font-bold text-slate-900 group-hover:text-blue-600 transition-colors mb-1.5">
          {template.name}
        </h3>
        <p className="text-xs text-slate-500 line-clamp-3 leading-relaxed mb-4">
          {template.description}
        </p>

        {/* Resource Chips */}
        <div className="flex items-center gap-3 text-[11px] text-slate-500 font-medium pb-4 border-b border-slate-100 mb-4">
          <span className="flex items-center gap-1">
            <Cpu className="w-3.5 h-3.5 text-slate-400" />
            {template.cpu} vCPU
          </span>
          <span className="flex items-center gap-1">
            <HardDrive className="w-3.5 h-3.5 text-slate-400" />
            {template.memory}
          </span>
        </div>
      </div>

      {/* Action CTA */}
      <button
        onClick={() => onLaunch(template)}
        className="w-full py-2 px-3 rounded-lg bg-blue-50 hover:bg-blue-600 text-blue-600 hover:text-white font-semibold text-xs transition-all flex items-center justify-center gap-2 group/btn"
      >
        <Play className="w-3.5 h-3.5 fill-current group-hover/btn:scale-110 transition-transform" />
        <span>Launch Environment</span>
      </button>
    </div>
  );
}
