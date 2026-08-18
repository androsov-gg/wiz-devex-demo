'use client';

import React from 'react';
import { Search, Eye, ChevronDown, RotateCcw, ArrowUpDown } from 'lucide-react';

interface FilterBarProps {
  viewScope: 'all' | 'me' | 'team';
  onViewScopeChange: (scope: 'all' | 'me' | 'team') => void;
  searchFilter: string;
  onSearchFilterChange: (text: string) => void;
  statusFilter: string;
  onStatusFilterChange: (status: string) => void;
  categoryFilter: string;
  onCategoryFilterChange: (cat: string) => void;
  totalCount: number;
  onReset: () => void;
}

export function FilterBar({
  viewScope,
  onViewScopeChange,
  searchFilter,
  onSearchFilterChange,
  statusFilter,
  onStatusFilterChange,
  categoryFilter,
  onCategoryFilterChange,
  totalCount,
  onReset,
}: FilterBarProps) {
  return (
    <div className="w-full bg-white border-b border-slate-200 px-8 py-3.5 space-y-3">
      {/* Scope Switcher: All | Me | Team */}
      <div className="flex items-center gap-1">
        {(['all', 'me', 'team'] as const).map((scope) => {
          const isActive = viewScope === scope;
          const label = scope === 'all' ? 'All' : scope === 'me' ? 'Me' : 'Team';
          return (
            <button
              key={scope}
              onClick={() => onViewScopeChange(scope)}
              className={`px-3 py-1 text-xs font-semibold rounded-md transition-all ${
                isActive
                  ? 'bg-blue-50 text-blue-600 border border-blue-300 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/70'
              }`}
            >
              {label}
            </button>
          );
        })}
      </div>

      {/* Filter Row */}
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-2 flex-wrap">
          {/* Quick Search */}
          <div className="relative flex items-center">
            <Search className="w-3.5 h-3.5 text-slate-400 absolute left-3 pointer-events-none" />
            <input
              type="text"
              placeholder="Search active demos..."
              value={searchFilter}
              onChange={(e) => onSearchFilterChange(e.target.value)}
              className="w-56 pl-8 pr-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-full focus:outline-none focus:ring-1 focus:ring-blue-500 focus:bg-white transition-colors placeholder:text-slate-400"
            />
          </div>

          {/* Quick Filter: All */}
          <button
            onClick={() => onStatusFilterChange('ALL')}
            className={`px-3 py-1.5 text-xs rounded-full border transition-colors ${
              statusFilter === 'ALL'
                ? 'bg-blue-50 text-blue-600 border-blue-300 font-semibold'
                : 'bg-white text-slate-600 border-slate-200 hover:bg-slate-50'
            }`}
          >
            All
          </button>

          {/* Watching Button */}
          <button className="px-3 py-1.5 text-xs rounded-full border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 flex items-center gap-1.5 transition-colors">
            <Eye className="w-3.5 h-3.5 text-slate-400" />
            <span>Watching</span>
          </button>

          {/* Status Dropdown Pill */}
          <div className="relative">
            <select
              value={statusFilter}
              onChange={(e) => onStatusFilterChange(e.target.value)}
              className="appearance-none px-3.5 py-1.5 pr-7 text-xs rounded-full border border-blue-200 bg-blue-50/60 text-blue-700 font-medium hover:bg-blue-50 focus:outline-none cursor-pointer"
            >
              <option value="ALL">Status: All</option>
              <option value="RUNNING">Running / Ready</option>
              <option value="PROVISIONING">Provisioning</option>
              <option value="BUILDING_IMAGE">Building Image</option>
              <option value="STOPPED">Stopped</option>
            </select>
            <ChevronDown className="w-3.5 h-3.5 text-blue-600 absolute right-2.5 top-2.5 pointer-events-none" />
          </div>

          {/* Category Dropdown Pill */}
          <div className="relative">
            <select
              value={categoryFilter}
              onChange={(e) => onCategoryFilterChange(e.target.value)}
              className="appearance-none px-3.5 py-1.5 pr-7 text-xs rounded-full border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 focus:outline-none cursor-pointer"
            >
              <option value="ALL">Category: All</option>
              <option value="AI & DevEx">AI & DevEx</option>
              <option value="Cloud Security">Cloud Security</option>
              <option value="Container Security">Container Security</option>
            </select>
            <ChevronDown className="w-3.5 h-3.5 text-slate-400 absolute right-2.5 top-2.5 pointer-events-none" />
          </div>

          {/* Reset Filter */}
          <button
            onClick={onReset}
            className="px-2.5 py-1 text-xs text-slate-500 hover:text-slate-800 flex items-center gap-1 transition-colors"
            title="Reset filters"
          >
            <RotateCcw className="w-3 h-3" />
            <span>Reset</span>
          </button>
        </div>

        {/* Counter Badge */}
        <div className="flex items-center gap-2 text-xs text-slate-500 font-medium">
          <span>{totalCount} demos</span>
          <ArrowUpDown className="w-3.5 h-3.5 text-slate-400" />
        </div>
      </div>
    </div>
  );
}
